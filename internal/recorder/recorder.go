package recorder

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ── Data Types ───────────────────────────────────────────────────

// ProcessSnapshot captures a single process's resource state
type ProcessSnapshot struct {
	PID      int     `json:"pid"`
	User     string  `json:"user"`
	Name     string  `json:"name"`
	Cmd      string  `json:"cmd"`
	CPUPct   float64 `json:"cpu_pct"`
	MemPct   float64 `json:"mem_pct"`
	MemRSSMB float64 `json:"mem_rss_mb"`
	GPUMemMB int     `json:"gpu_mem_mb,omitempty"`
	OOMScore int     `json:"oom_score"`
}

// GPUSnapshot captures a single GPU's state
type GPUSnapshot struct {
	ID         int `json:"id"`
	UtilPct    int `json:"util_pct"`
	MemUsedMB  int `json:"mem_used_mb"`
	MemTotalMB int `json:"mem_total_mb"`
	TempC      int `json:"temp_c"`
}

// Snapshot is a single point-in-time system resource capture
type Snapshot struct {
	Timestamp    time.Time         `json:"ts"`
	TotalCPUPct  float64           `json:"total_cpu_pct"`
	TotalMemPct  float64           `json:"total_mem_pct"`
	TotalMemGB   float64           `json:"total_mem_gb"`
	DiskPct      float64           `json:"disk_pct"`
	GPUs         []GPUSnapshot     `json:"gpus,omitempty"`
	TopProcesses []ProcessSnapshot `json:"top_processes"`
	OOMLeaders   []ProcessSnapshot `json:"oom_leaders"`
}

// CrashEvent groups a sequence of critical snapshots into one incident
type CrashEvent struct {
	ID        string     `json:"id"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   time.Time  `json:"ended_at"`
	Trigger   string     `json:"trigger"`   // "cpu:94.2%" / "mem:91.3%"
	Verdict   string     `json:"verdict"`
	Severity  string     `json:"severity"`
	Resolved  bool                  `json:"resolved"`
	Snapshots []Snapshot            `json:"snapshots"`
	ProcessDetails map[int]ProcessDetail `json:"process_details,omitempty"`
}

type ProcessDetail struct {
	ExePath string `json:"exe_path"`
	Logs    string `json:"logs"`
}

// ── ResourceRecorder ─────────────────────────────────────────────

// ResourceRecorder monitors system resources; only records full snapshots
// when CPU/MEM/DISK/GPU exceeds the threshold (storage-efficient).
type ResourceRecorder struct {
	interval     time.Duration
	filePath     string
	maxEvents    int
	threshold    float64 // percentage to trigger recording (default: 90)
	hysteresis   float64 // percentage to stop recording (default: 85)
	lokiPusher   *LokiPusher
	webhookURL   string
	mu           sync.Mutex
	events       []CrashEvent
	activeEvent  *CrashEvent // nil when not in critical state

	// Prometheus metrics
	snapshotCount  prometheus.Counter
	eventCount     prometheus.Counter
	lastCPU        prometheus.Gauge
	lastMem        prometheus.Gauge
	lastDisk       prometheus.Gauge
	criticalActive prometheus.Gauge
}

type RecorderConfig struct {
	IntervalSec int
	FilePath    string
	MaxEvents   int
	Threshold   float64
	LokiURL     string
	WebhookURL  string
}

func NewResourceRecorder(cfg RecorderConfig) *ResourceRecorder {
	var loki *LokiPusher
	if cfg.LokiURL != "" {
		loki = NewLokiPusher(cfg.LokiURL)
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 90
	}
	if cfg.MaxEvents <= 0 {
		cfg.MaxEvents = 100
	}
	return &ResourceRecorder{
		interval:   time.Duration(cfg.IntervalSec) * time.Second,
		filePath:   cfg.FilePath,
		maxEvents:  cfg.MaxEvents,
		threshold:  cfg.Threshold,
		hysteresis: cfg.Threshold - 5, // 5% hysteresis band
		lokiPusher: loki,
		webhookURL: cfg.WebhookURL,
		events:     make([]CrashEvent, 0),
		snapshotCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "resource_snapshots_total",
			Help: "Total critical snapshots taken",
		}),
		eventCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "crash_events_total",
			Help: "Total crash events detected",
		}),
		lastCPU: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "resource_last_cpu_pct",
			Help: "CPU percentage at last poll",
		}),
		lastMem: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "resource_last_mem_pct",
			Help: "Memory percentage at last poll",
		}),
		lastDisk: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "resource_last_disk_pct",
			Help: "Disk usage percentage at last poll",
		}),
		criticalActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "resource_critical_active",
			Help: "1 if a critical event is actively being recorded",
		}),
	}
}

func (r *ResourceRecorder) Register(reg prometheus.Registerer) {
	reg.MustRegister(r.snapshotCount, r.eventCount, r.lastCPU, r.lastMem, r.lastDisk, r.criticalActive)
}

// Start begins the monitoring loop
func (r *ResourceRecorder) Start() {
	r.loadFromDisk()
	log.Printf("ResourceRecorder: threshold=%.0f%%, hysteresis=%.0f%%, interval=%s",
		r.threshold, r.hysteresis, r.interval)

	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for range ticker.C {
			r.poll()
		}
	}()
}

// GetEvents returns all crash events (thread-safe)
func (r *ResourceRecorder) GetEvents() []CrashEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]CrashEvent, len(r.events))
	copy(result, r.events)
	return result
}

// GetEvent returns a single crash event by ID
func (r *ResourceRecorder) GetEvent(id string) *CrashEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		if r.events[i].ID == id {
			ev := r.events[i]
			return &ev
		}
	}
	return nil
}

// GetSnapshots returns combined snapshots from recent events (for forensic page)
func (r *ResourceRecorder) GetSnapshots(n int) []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []Snapshot
	for _, ev := range r.events {
		all = append(all, ev.Snapshots...)
	}
	if r.activeEvent != nil {
		all = append(all, r.activeEvent.Snapshots...)
	}
	if n <= 0 || n > len(all) {
		n = len(all)
	}
	if n == 0 {
		return nil
	}
	return all[len(all)-n:]
}

// ── Core Logic ───────────────────────────────────────────────────

func (r *ResourceRecorder) poll() {
	// Lightweight poll: only read aggregate metrics
	cpuPct := readCPUUsage()
	memPct, memGB := readMemInfo()
	diskPct := readDiskUsage("/")

	// Update Prometheus gauges (always)
	r.lastCPU.Set(cpuPct)
	r.lastMem.Set(memPct)
	r.lastDisk.Set(diskPct)

	// Check GPU (lightweight — single nvidia-smi call)
	gpus := ReadGPUs()
	maxGPU := 0.0
	for _, g := range gpus {
		if g.MemTotalMB > 0 {
			pct := float64(g.MemUsedMB) / float64(g.MemTotalMB) * 100
			if pct > maxGPU {
				maxGPU = pct
			}
		}
	}

	// Determine trigger
	trigger := ""
	if cpuPct >= r.threshold {
		trigger = fmt.Sprintf("cpu:%.1f%%", cpuPct)
	} else if memPct >= r.threshold {
		trigger = fmt.Sprintf("mem:%.1f%%", memPct)
	} else if diskPct >= r.threshold {
		trigger = fmt.Sprintf("disk:%.1f%%", diskPct)
	} else if maxGPU >= r.threshold {
		trigger = fmt.Sprintf("gpu:%.1f%%", maxGPU)
	}

	r.mu.Lock()
	isActive := r.activeEvent != nil
	r.mu.Unlock()

	if trigger != "" {
		// CRITICAL: take full snapshot
		snap := r.takeFullSnapshot(cpuPct, memPct, memGB, diskPct, gpus)

		r.mu.Lock()
		if r.activeEvent == nil {
			// Start new crash event
			r.activeEvent = &CrashEvent{
				ID:        generateID(),
				StartedAt: snap.Timestamp,
				Trigger:   trigger,
			}
			r.eventCount.Inc()
			r.criticalActive.Set(1)
			log.Printf("🚨 CRITICAL: Threshold breached → %s (starting crash event %s)", trigger, r.activeEvent.ID)
		}
		r.activeEvent.Snapshots = append(r.activeEvent.Snapshots, snap)
		r.activeEvent.EndedAt = snap.Timestamp
		r.mu.Unlock()

		r.snapshotCount.Inc()

		// Push to Loki
		if r.lokiPusher != nil {
			go r.lokiPusher.Push(trigger, snap)
		}

		// Send webhook on first detection
		if !isActive && r.webhookURL != "" {
			go sendWebhookAlert(r.webhookURL, trigger, snap)
		}

	} else if isActive {
		// Check hysteresis: only close event if ALL metrics below hysteresis
		if cpuPct < r.hysteresis && memPct < r.hysteresis && diskPct < r.hysteresis && maxGPU < r.hysteresis {
			r.mu.Lock()
			event := r.activeEvent
			// Run forensic analysis on the event
			report := Analyze(event.Snapshots)
			event.Verdict = report.Verdict
			event.Severity = report.Severity
			event.Resolved = true
            
			r.fetchProcessDetails(event)

			r.events = append(r.events, *event)
			if len(r.events) > r.maxEvents {
				r.events = r.events[len(r.events)-r.maxEvents:]
			}
			r.activeEvent = nil
			r.criticalActive.Set(0)
			log.Printf("✅ RESOLVED: Crash event %s ended (%d snapshots, verdict: %s)",
				event.ID, len(event.Snapshots), event.Severity)
			r.mu.Unlock()

			r.persistToDisk()
		}
	}
}

func (r *ResourceRecorder) takeFullSnapshot(cpuPct, memPct, memGB, diskPct float64, gpus []GPUSnapshot) Snapshot {
	snap := Snapshot{
		Timestamp:   time.Now().UTC(),
		TotalCPUPct: cpuPct,
		TotalMemPct: memPct,
		TotalMemGB:  memGB,
		DiskPct:     diskPct,
		GPUs:        gpus,
	}

	// Read all processes (expensive — only done during critical state)
	procs := readAllProcesses()

	// Attach GPU memory per process
	gpuMem := ReadGPUProcesses()
	for i := range procs {
		if mem, ok := gpuMem[procs[i].PID]; ok {
			procs[i].GPUMemMB = mem
		}
	}

	// Top 20 by CPU
	sort.Slice(procs, func(i, j int) bool { return procs[i].CPUPct > procs[j].CPUPct })
	topCPU := take(procs, 20)

	// Top 20 by Memory
	sort.Slice(procs, func(i, j int) bool { return procs[i].MemRSSMB > procs[j].MemRSSMB })
	topMem := take(procs, 20)

	snap.TopProcesses = dedup(append(topCPU, topMem...))

	// OOM leaders
	sort.Slice(procs, func(i, j int) bool { return procs[i].OOMScore > procs[j].OOMScore })
	snap.OOMLeaders = take(procs, 10)

	return snap
}

func (r *ResourceRecorder) fetchProcessDetails(event *CrashEvent) {
	if event.ProcessDetails == nil {
		event.ProcessDetails = make(map[int]ProcessDetail)
	}

	if len(event.Snapshots) == 0 {
		return
	}
	lastSnap := event.Snapshots[len(event.Snapshots)-1]

	pids := make(map[int]bool)
	for _, p := range lastSnap.TopProcesses {
		pids[p.PID] = true
	}
	for _, p := range lastSnap.OOMLeaders {
		pids[p.PID] = true
	}

	for pid := range pids {
		var detail ProcessDetail

		exeLink := fmt.Sprintf("/host/proc/%d/exe", pid)
		if path, err := os.Readlink(exeLink); err == nil {
			detail.ExePath = path
		}

		cmd := exec.Command("journalctl", fmt.Sprintf("_PID=%d", pid), "-n", "50", "--no-pager")
		if out, err := cmd.CombinedOutput(); err == nil && len(out) > 0 {
			detail.Logs = string(out)
		} else if detail.ExePath != "" {
			cmd2 := exec.Command("journalctl", fmt.Sprintf("_EXE=%s", detail.ExePath), "-n", "50", "--no-pager")
			if out2, err := cmd2.CombinedOutput(); err == nil && len(out2) > 0 {
				detail.Logs = string(out2)
			}
		}

		event.ProcessDetails[pid] = detail
	}
}

// ── Persistence ──────────────────────────────────────────────────

func (r *ResourceRecorder) persistToDisk() {
	r.mu.Lock()
	events := make([]CrashEvent, len(r.events))
	copy(events, r.events)
	r.mu.Unlock()

	dir := filepath.Dir(r.filePath)
	if dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}

	f, err := os.Create(r.filePath)
	if err != nil {
		log.Printf("ResourceRecorder: write error: %v", err)
		return
	}

	enc := json.NewEncoder(f)
	for _, ev := range events {
		enc.Encode(ev)
	}

	f.Sync()
	f.Close()
}

func (r *ResourceRecorder) loadFromDisk() {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var ev CrashEvent
		if err := json.Unmarshal([]byte(line), &ev); err == nil {
			r.events = append(r.events, ev)
		}
	}

	if len(r.events) > r.maxEvents {
		r.events = r.events[len(r.events)-r.maxEvents:]
	}

	if len(r.events) > 0 {
		log.Printf("ResourceRecorder: recovered %d crash events from %s", len(r.events), r.filePath)
	}
}

// ── /proc Readers ────────────────────────────────────────────────

func readMemInfo() (pct float64, totalGB float64) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var total, avail uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			avail = val
		}
	}
	if total == 0 {
		return 0, 0
	}
	totalGB = float64(total) / 1024 / 1024
	used := total - avail
	pct = float64(used) / float64(total) * 100
	return
}

func readCPUUsage() float64 {
	read := func() (idle, total uint64) {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return 0, 0
		}
		line := strings.Split(string(data), "\n")[0]
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0
		}
		var sum uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			sum += v
			if i == 4 {
				idle = v
			}
		}
		return idle, sum
	}

	idle1, total1 := read()
	time.Sleep(100 * time.Millisecond)
	idle2, total2 := read()

	idleDelta := float64(idle2 - idle1)
	totalDelta := float64(total2 - total1)
	if totalDelta == 0 {
		return 0
	}
	return (1 - idleDelta/totalDelta) * 100
}

func readDiskUsage(path string) float64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total == 0 {
		return 0
	}
	used := total - free
	return float64(used) / float64(total) * 100
}

func readAllProcesses() []ProcessSnapshot {
	hostProc := os.Getenv("HOST_PROC")
	if hostProc == "" {
		hostProc = "/proc"
	}

	entries, err := os.ReadDir(hostProc)
	if err != nil {
		return nil
	}

	var procs []ProcessSnapshot
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		p := ProcessSnapshot{PID: pid}
		procDir := filepath.Join(hostProc, entry.Name())

		if comm, err := os.ReadFile(filepath.Join(procDir, "comm")); err == nil {
			p.Name = strings.TrimSpace(string(comm))
		}

		if cmdline, err := os.ReadFile(filepath.Join(procDir, "cmdline")); err == nil {
			cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
			if len(cmd) > 120 {
				cmd = cmd[:120] + "..."
			}
			p.Cmd = strings.TrimSpace(cmd)
		}

		if status, err := os.ReadFile(filepath.Join(procDir, "status")); err == nil {
			for _, line := range strings.Split(string(status), "\n") {
				fields := strings.Fields(line)
				if len(fields) < 2 {
					continue
				}
				switch fields[0] {
				case "Uid:":
					p.User = resolveUser(fields[1])
				case "VmRSS:":
					rss, _ := strconv.ParseFloat(fields[1], 64)
					p.MemRSSMB = rss / 1024
				}
			}
		}

		if score, err := os.ReadFile(filepath.Join(procDir, "oom_score")); err == nil {
			p.OOMScore, _ = strconv.Atoi(strings.TrimSpace(string(score)))
		}

		totalMem, _ := readTotalMem()
		if totalMem > 0 {
			p.MemPct = p.MemRSSMB / totalMem * 100
		}

		if p.MemRSSMB > 0 {
			procs = append(procs, p)
		}
	}

	return procs
}

func readTotalMem() (float64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseFloat(fields[1], 64)
				return kb / 1024, nil
			}
		}
	}
	return 0, fmt.Errorf("MemTotal not found")
}

func resolveUser(uid string) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return uid
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[2] == uid {
			return parts[0]
		}
	}
	return uid
}

// ── Webhook ──────────────────────────────────────────────────────

func sendWebhookAlert(url, trigger string, snap Snapshot) {
	if url == "" {
		return
	}
	msg := fmt.Sprintf(`{"content":"🚨 **CRITICAL ALERT** — %s\nCPU: %.1f%% | MEM: %.1f%% | DISK: %.1f%%"}`,
		trigger, snap.TotalCPUPct, snap.TotalMemPct, snap.DiskPct)
	resp, err := http.Post(url, "application/json", strings.NewReader(msg))
	if err != nil {
		log.Printf("Webhook error: %v", err)
		return
	}
	resp.Body.Close()
}

// ── Helpers ──────────────────────────────────────────────────────

func take(procs []ProcessSnapshot, n int) []ProcessSnapshot {
	if len(procs) < n {
		n = len(procs)
	}
	result := make([]ProcessSnapshot, n)
	copy(result, procs[:n])
	return result
}

func dedup(procs []ProcessSnapshot) []ProcessSnapshot {
	seen := make(map[int]bool)
	var result []ProcessSnapshot
	for _, p := range procs {
		if !seen[p.PID] {
			seen[p.PID] = true
			result = append(result, p)
		}
	}
	return result
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
