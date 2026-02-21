package recorder

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

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
	GPUs         []GPUSnapshot     `json:"gpus,omitempty"`
	TopProcesses []ProcessSnapshot `json:"top_processes"`
	OOMLeaders   []ProcessSnapshot `json:"oom_leaders"`
}

// ResourceRecorder continuously snapshots system state to a crash-resilient file
type ResourceRecorder struct {
	interval  time.Duration
	filePath  string
	maxCount  int
	mu        sync.Mutex
	snapshots []Snapshot

	// Prometheus metrics
	snapshotCount prometheus.Counter
	lastCPU       prometheus.Gauge
	lastMem       prometheus.Gauge
}

func NewResourceRecorder(intervalSec int, filePath string, maxCount int) *ResourceRecorder {
	return &ResourceRecorder{
		interval:  time.Duration(intervalSec) * time.Second,
		filePath:  filePath,
		maxCount:  maxCount,
		snapshots: make([]Snapshot, 0, maxCount),
		snapshotCount: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "resource_snapshots_total",
			Help: "Total number of resource snapshots taken",
		}),
		lastCPU: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "resource_last_cpu_pct",
			Help: "CPU percentage at last snapshot",
		}),
		lastMem: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "resource_last_mem_pct",
			Help: "Memory percentage at last snapshot",
		}),
	}
}

func (r *ResourceRecorder) Register(reg prometheus.Registerer) {
	reg.MustRegister(r.snapshotCount, r.lastCPU, r.lastMem)
}

// Start begins the continuous recording loop
func (r *ResourceRecorder) Start() {
	// Load existing snapshots from disk (crash recovery)
	r.loadFromDisk()
	log.Printf("ResourceRecorder: starting (interval=%s, file=%s, max=%d)", r.interval, r.filePath, r.maxCount)

	go func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for range ticker.C {
			r.takeSnapshot()
		}
	}()
}

// GetSnapshots returns the last N snapshots (thread-safe)
func (r *ResourceRecorder) GetSnapshots(n int) []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n <= 0 || n > len(r.snapshots) {
		n = len(r.snapshots)
	}
	start := len(r.snapshots) - n
	result := make([]Snapshot, n)
	copy(result, r.snapshots[start:])
	return result
}

func (r *ResourceRecorder) takeSnapshot() {
	snap := Snapshot{
		Timestamp: time.Now().UTC(),
	}

	// 1. Read total CPU/MEM from /proc/meminfo and /proc/stat
	snap.TotalMemPct, snap.TotalMemGB = readMemInfo()
	snap.TotalCPUPct = readCPUUsage()

	// 2. Read per-process info from /proc
	procs := readAllProcesses()

	// 3. GPU info (best-effort)
	snap.GPUs = ReadGPUs()
	gpuMem := ReadGPUProcesses()
	for i := range procs {
		if mem, ok := gpuMem[procs[i].PID]; ok {
			procs[i].GPUMemMB = mem
		}
	}

	// 4. Top 20 by CPU
	sort.Slice(procs, func(i, j int) bool { return procs[i].CPUPct > procs[j].CPUPct })
	topCPU := take(procs, 20)

	// 5. Top 20 by Memory
	sort.Slice(procs, func(i, j int) bool { return procs[i].MemRSSMB > procs[j].MemRSSMB })
	topMem := take(procs, 20)

	// 6. Merge & dedup
	snap.TopProcesses = dedup(append(topCPU, topMem...))

	// 7. OOM leaders (top 10 by oom_score)
	sort.Slice(procs, func(i, j int) bool { return procs[i].OOMScore > procs[j].OOMScore })
	snap.OOMLeaders = take(procs, 10)

	// 8. Update prometheus
	r.snapshotCount.Inc()
	r.lastCPU.Set(snap.TotalCPUPct)
	r.lastMem.Set(snap.TotalMemPct)

	// 9. Append to ring buffer and fsync to disk
	r.mu.Lock()
	r.snapshots = append(r.snapshots, snap)
	if len(r.snapshots) > r.maxCount {
		r.snapshots = r.snapshots[len(r.snapshots)-r.maxCount:]
	}
	r.mu.Unlock()

	r.persistToDisk()
}

// persistToDisk writes the full ring buffer as JSONL and fsyncs
func (r *ResourceRecorder) persistToDisk() {
	r.mu.Lock()
	snaps := make([]Snapshot, len(r.snapshots))
	copy(snaps, r.snapshots)
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
	for _, s := range snaps {
		enc.Encode(s)
	}

	// CRITICAL: fsync ensures data survives kernel panic / power loss
	f.Sync()
	f.Close()
}

// loadFromDisk recovers snapshots after a crash
func (r *ResourceRecorder) loadFromDisk() {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return // file doesn't exist yet, that's fine
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var s Snapshot
		if err := json.Unmarshal([]byte(line), &s); err == nil {
			r.snapshots = append(r.snapshots, s)
		}
	}

	if len(r.snapshots) > r.maxCount {
		r.snapshots = r.snapshots[len(r.snapshots)-r.maxCount:]
	}

	if len(r.snapshots) > 0 {
		log.Printf("ResourceRecorder: recovered %d snapshots from %s (last: %s)",
			len(r.snapshots), r.filePath, r.snapshots[len(r.snapshots)-1].Timestamp.Format(time.RFC3339))
	}
}

// ── /proc readers ────────────────────────────────────────────────

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

// readCPUUsage reads instantaneous CPU usage from /proc/stat
// Uses a 100ms sample to measure delta
func readCPUUsage() float64 {
	read := func() (idle, total uint64) {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return 0, 0
		}
		line := strings.Split(string(data), "\n")[0] // cpu line
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0
		}
		var sum uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			sum += v
			if i == 4 { // idle is field 4
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
			continue // not a PID directory
		}

		p := ProcessSnapshot{PID: pid}
		procDir := filepath.Join(hostProc, entry.Name())

		// Read comm (process name)
		if comm, err := os.ReadFile(filepath.Join(procDir, "comm")); err == nil {
			p.Name = strings.TrimSpace(string(comm))
		}

		// Read cmdline
		if cmdline, err := os.ReadFile(filepath.Join(procDir, "cmdline")); err == nil {
			cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
			if len(cmd) > 120 {
				cmd = cmd[:120] + "..."
			}
			p.Cmd = strings.TrimSpace(cmd)
		}

		// Read status for user and memory
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
					p.MemRSSMB = rss / 1024 // kB → MB
				}
			}
		}

		// Read OOM score
		if score, err := os.ReadFile(filepath.Join(procDir, "oom_score")); err == nil {
			p.OOMScore, _ = strconv.Atoi(strings.TrimSpace(string(score)))
		}

		// Calculate memory percentage
		totalMem, _ := readTotalMem()
		if totalMem > 0 {
			p.MemPct = p.MemRSSMB / totalMem * 100
		}

		// Skip kernel threads (no RSS)
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
				return kb / 1024, nil // MB
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
