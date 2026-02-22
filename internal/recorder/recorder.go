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

	"log-sentry/internal/storage"

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
	MemRSSMB   float64 `json:"mem_rss_mb"`
	GPUMemMB   int     `json:"gpu_mem_mb,omitempty"`
	OOMScore   int     `json:"oom_score"`
	ReadBytes  uint64  `json:"read_bytes"`
	WriteBytes uint64  `json:"write_bytes"`
	NetPorts   string  `json:"net_ports,omitempty"`
	NetRxBytes  uint64 `json:"net_rx_bytes"`
	NetTxBytes  uint64 `json:"net_tx_bytes"`
	IsExternal  bool   `json:"is_external"`
	FdCount     int    `json:"fd_count"`
	ThreadCount int    `json:"thread_count"`
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
	interval      time.Duration
	threshold     float64 // percentage to trigger recording (default: 90)
	hysteresis    float64 // percentage to stop recording (default: 85)
	retentionDays int
	lokiPusher    *LokiPusher
	webhookURL    string
	store         storage.Store
	mu            sync.Mutex
	activeEvent   *CrashEvent // nil when not in critical state

	// Prometheus metrics — basic
	snapshotCount  prometheus.Counter
	eventCount     prometheus.Counter
	lastCPU        prometheus.Gauge
	lastMem        prometheus.Gauge
	lastDisk       prometheus.Gauge
	criticalActive prometheus.Gauge

	// Prometheus metrics — enhanced exporter
	eventDuration     prometheus.Histogram
	eventsBySeverity  *prometheus.CounterVec
	eventsByTrigger   *prometheus.CounterVec
	maxOOMScore       prometheus.Gauge
	memTotalGB        prometheus.Gauge
	gpuUtil           *prometheus.GaugeVec
	gpuMemUsed        *prometheus.GaugeVec
	gpuTemp           *prometheus.GaugeVec
}

type RecorderConfig struct {
	IntervalSec   int
	Threshold     float64
	RetentionDays int
	LokiURL       string
	WebhookURL    string
	Store         storage.Store
}

func NewResourceRecorder(cfg RecorderConfig) *ResourceRecorder {
	var loki *LokiPusher
	if cfg.LokiURL != "" {
		loki = NewLokiPusher(cfg.LokiURL)
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = 90
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 30
	}
	return &ResourceRecorder{
		interval:      time.Duration(cfg.IntervalSec) * time.Second,
		threshold:     cfg.Threshold,
		hysteresis:    cfg.Threshold - 5,
		retentionDays: cfg.RetentionDays,
		lokiPusher:    loki,
		webhookURL:    cfg.WebhookURL,
		store:         cfg.Store,
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
		eventDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "crash_event_duration_seconds",
			Help:    "Duration of crash events in seconds",
			Buckets: []float64{5, 10, 30, 60, 120, 300, 600, 1800},
		}),
		eventsBySeverity: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "crash_events_by_severity_total",
			Help: "Crash events by severity",
		}, []string{"severity"}),
		eventsByTrigger: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "crash_events_by_trigger_total",
			Help: "Crash events by trigger type",
		}, []string{"trigger"}),
		maxOOMScore: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "resource_max_oom_score",
			Help: "Highest OOM score seen at last poll",
		}),
		memTotalGB: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "resource_mem_total_gb",
			Help: "Total system memory in GB",
		}),
		gpuUtil: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "resource_gpu_util_pct",
			Help: "GPU utilization percentage",
		}, []string{"gpu_id"}),
		gpuMemUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "resource_gpu_mem_used_mb",
			Help: "GPU memory used in MB",
		}, []string{"gpu_id"}),
		gpuTemp: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "resource_gpu_temp_c",
			Help: "GPU temperature in Celsius",
		}, []string{"gpu_id"}),
	}
}

func (r *ResourceRecorder) Register(reg prometheus.Registerer) {
	reg.MustRegister(
		r.snapshotCount, r.eventCount, r.lastCPU, r.lastMem, r.lastDisk, r.criticalActive,
		r.eventDuration, r.eventsBySeverity, r.eventsByTrigger,
		r.maxOOMScore, r.memTotalGB,
		r.gpuUtil, r.gpuMemUsed, r.gpuTemp,
	)
}

// Start begins the monitoring loop
func (r *ResourceRecorder) Start() {
	log.Printf("ResourceRecorder: threshold=%.0f%%, hysteresis=%.0f%%, interval=%s, retention=%dd",
		r.threshold, r.hysteresis, r.interval, r.retentionDays)

	// Monitoring loop
	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for range ticker.C {
			r.poll()
		}
	}()

	// Retention goroutine: prune old data daily
	if r.store != nil {
		go func() {
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				ttl := time.Duration(r.retentionDays) * 24 * time.Hour
				r.store.DeleteOldCrashEvents(ttl)
				r.store.DeleteOldAttacks(ttl)
			}
		}()
	}
}

// GetStore returns the underlying store for API handlers
func (r *ResourceRecorder) GetStore() storage.Store {
	return r.store
}

// GetEvent returns a single crash event by ID (from store)
func (r *ResourceRecorder) GetEvent(id string) *CrashEvent {
	// Check active event first
	r.mu.Lock()
	if r.activeEvent != nil && r.activeEvent.ID == id {
		ev := *r.activeEvent
		r.mu.Unlock()
		return &ev
	}
	r.mu.Unlock()

	if r.store == nil {
		return nil
	}
	data, err := r.store.GetCrashEvent(id)
	if err != nil {
		return nil
	}
	var ev CrashEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil
	}
	return &ev
}

// GetSnapshots returns combined snapshots from recent events (for forensic page)
func (r *ResourceRecorder) GetSnapshots(n int) []Snapshot {
	r.mu.Lock()
	var all []Snapshot
	if r.activeEvent != nil {
		all = append(all, r.activeEvent.Snapshots...)
	}
	r.mu.Unlock()

	// Also pull from last few stored events
	if r.store != nil {
		result, err := r.store.ListCrashEvents(storage.ListOpts{Page: 1, PageSize: 10})
		if err == nil {
			for _, cs := range result.Items {
				data, err := r.store.GetCrashEvent(cs.ID)
				if err != nil {
					continue
				}
				var ev CrashEvent
				if json.Unmarshal(data, &ev) == nil {
					all = append(all, ev.Snapshots...)
				}
			}
		}
	}

	if n <= 0 || n > len(all) {
		n = len(all)
	}
	if n == 0 {
		return nil
	}
	return all[len(all)-n:]
}

// ForcePoll forces an immediate system state poll and heartbeat snapshot write.
// This is used during graceful shutdown to capture the final moments before exit.
func (r *ResourceRecorder) ForcePoll() {
	r.poll()
}

func (r *ResourceRecorder) poll() {
	// Lightweight poll: only read aggregate metrics
	cpuPct := readCPUUsage()
	memPct, memGB := readMemInfo()
	diskPct := readDiskUsage("/")

	// Update Prometheus gauges (always)
	r.lastCPU.Set(cpuPct)
	r.lastMem.Set(memPct)
	r.lastDisk.Set(diskPct)
	r.memTotalGB.Set(memGB)

	// Check GPU (lightweight — single nvidia-smi call)
	gpus := ReadGPUs()
	maxGPU := 0.0
	for _, g := range gpus {
		id := strconv.Itoa(g.ID)
		r.gpuUtil.WithLabelValues(id).Set(float64(g.UtilPct))
		r.gpuMemUsed.WithLabelValues(id).Set(float64(g.MemUsedMB))
		r.gpuTemp.WithLabelValues(id).Set(float64(g.TempC))
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

	// Always take a full snapshot to use as a heartbeat
	snap := r.takeFullSnapshot(cpuPct, memPct, memGB, diskPct, gpus)
	if r.store != nil {
		data, _ := json.Marshal(snap)
		r.store.SaveHeartbeatSnapshot(data)
	}

	r.mu.Lock()
	isActive := r.activeEvent != nil
	r.mu.Unlock()

	if trigger != "" {
		// critical event triggered, snap is already taken

		// Track max OOM score
		for _, p := range snap.OOMLeaders {
			if float64(p.OOMScore) > 0 {
				r.maxOOMScore.Set(float64(p.OOMScore))
				break // already sorted desc
			}
		}

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

			// Push crash start to Loki
			if r.lokiPusher != nil {
				go r.lokiPusher.PushCrashStart(r.activeEvent)
			}
		}
		r.activeEvent.Snapshots = append(r.activeEvent.Snapshots, snap)
		r.activeEvent.EndedAt = snap.Timestamp
		r.mu.Unlock()

		r.snapshotCount.Inc()

		// Push snapshot to Loki
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

			// Enhanced Prometheus metrics
			duration := event.EndedAt.Sub(event.StartedAt).Seconds()
			r.eventDuration.Observe(duration)
			r.eventsBySeverity.WithLabelValues(event.Severity).Inc()
			triggerType := strings.SplitN(event.Trigger, ":", 2)[0]
			r.eventsByTrigger.WithLabelValues(triggerType).Inc()

			// Persist to BoltDB
			if r.store != nil {
				data, err := json.Marshal(event)
				if err == nil {
					if err := r.store.SaveCrashEvent(data, event.ID, event.Severity, event.Trigger, event.StartedAt, event.Resolved, len(event.Snapshots)); err != nil {
						log.Printf("BoltStore: save error: %v", err)
					}
				}
			}

			r.activeEvent = nil
			r.criticalActive.Set(0)
			log.Printf("✅ RESOLVED: Crash event %s ended (%d snapshots, verdict: %s, duration: %.0fs)",
				event.ID, len(event.Snapshots), event.Severity, duration)
			r.mu.Unlock()

			// Push crash resolved to Loki
			if r.lokiPusher != nil {
				go r.lokiPusher.PushCrashResolved(event)
			}
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

// ── SaveAttack ────────────────────────────────────────────────────

// SaveAttack persists a detected attack event to the store.
func (r *ResourceRecorder) SaveAttack(entry *storage.AttackEntry) {
	if r.store == nil {
		return
	}
	if err := r.store.SaveAttack(entry); err != nil {
		log.Printf("BoltStore: save attack error: %v", err)
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

		// Read process I/O
		if ioData, err := os.ReadFile(filepath.Join(procDir, "io")); err == nil {
			for _, line := range strings.Split(string(ioData), "\n") {
				fields := strings.Fields(line)
				if len(fields) == 2 {
					val, _ := strconv.ParseUint(fields[1], 10, 64)
					if fields[0] == "read_bytes:" {
						p.ReadBytes = val
					} else if fields[0] == "write_bytes:" {
						p.WriteBytes = val
					}
				}
			}
		}

		// Read Network I/O
		if netData, err := os.ReadFile(filepath.Join(procDir, "net/dev")); err == nil {
			for _, line := range strings.Split(string(netData), "\n") {
				if strings.Contains(line, "lo:") || !strings.Contains(line, ":") {
					continue
				}
				fields := strings.Fields(line)
				if len(fields) >= 10 {
					rx, _ := strconv.ParseUint(fields[1], 10, 64)
					tx, _ := strconv.ParseUint(fields[9], 10, 64)
					p.NetRxBytes += rx
					p.NetTxBytes += tx
				}
			}
		}

		// Read File Descriptors
		if fdEntries, err := os.ReadDir(filepath.Join(procDir, "fd")); err == nil {
			p.FdCount = len(fdEntries)
		}

		// Read Threads
		if taskEntries, err := os.ReadDir(filepath.Join(procDir, "task")); err == nil {
			p.ThreadCount = len(taskEntries)
		}

		// Read Network Ports (Heuristic: scanning active TCP connections and matching inodes)
		p.NetPorts, p.IsExternal = readNetPorts(pid, procDir)

		if p.MemRSSMB > 0 || p.ReadBytes > 0 || p.WriteBytes > 0 || p.NetRxBytes > 0 || p.NetTxBytes > 0 {
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
	data, err := os.ReadFile("/host/etc/passwd")
	if err != nil {
		data, err = os.ReadFile("/etc/passwd")
		if err != nil {
			return uid
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[2] == uid {
			return fmt.Sprintf("%s (%s)", uid, parts[0])
		}
	}
	return uid
}

func readNetPorts(pid int, procDir string) (string, bool) {
	fdDir := filepath.Join(procDir, "fd")
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return "", false
	}

	inodes := make(map[string]bool)
	for _, entry := range entries {
		link, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err == nil && strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
			inode := link[8 : len(link)-1]
			inodes[inode] = true
		}
	}

	if len(inodes) == 0 {
		return "", false
	}

	ports := make(map[string]bool)
	isExternal := false
	checkTcpMapping := func(file string) {
		data, err := os.ReadFile(filepath.Join(procDir, file))
		if err != nil {
			return
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] { // Skip header line
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}
			localAddr := fields[1]
			remoteAddr := fields[2]
			state := fields[3]
			inode := fields[9]
			if inodes[inode] { // Check if this connection belongs to our process
				parts := strings.Split(localAddr, ":")
				if len(parts) == 2 {
					portHex := parts[1]
					portDec, err := strconv.ParseInt(portHex, 16, 32)
					if err == nil {
						ports[fmt.Sprintf("%d", portDec)] = true
						
						// Flag as external if it's listening on all interfaces (0.0.0.0 or ::)
						// or if it has an ESTABLISHED (01) connection to a non-local IP.
						if state == "0A" && (strings.HasPrefix(localAddr, "00000000:") || strings.HasPrefix(localAddr, "00000000000000000000000000000000:")) {
							isExternal = true
						} else if state == "01" && remoteAddr != "00000000:0000" && !strings.HasPrefix(remoteAddr, "0100007F:") && !strings.HasPrefix(remoteAddr, "00000000000000000000000001000000:") {
							isExternal = true
						}
					}
				}
			}
		}
	}

	checkTcpMapping("net/tcp")
	checkTcpMapping("net/tcp6")

	var portList []string
	for port := range ports {
		portList = append(portList, port)
	}
	sort.Strings(portList)
	return strings.Join(portList, ","), isExternal
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
