package recorder

import (
	"fmt"
	"sort"
)

// ForensicReport is the post-crash root-cause analysis result
type ForensicReport struct {
	SnapshotCount int               `json:"snapshot_count"`
	TimeRange     string            `json:"time_range"`
	Verdict       string            `json:"verdict"`
	Severity      string            `json:"severity"`
	CPUTrend      []TrendPoint      `json:"cpu_trend"`
	MemTrend      []TrendPoint      `json:"mem_trend"`
	TopCPU        []ProcessSnapshot `json:"top_cpu"`
	TopMem        []ProcessSnapshot `json:"top_mem"`
	OOMLeaders    []ProcessSnapshot `json:"oom_leaders"`
	GPUs          []GPUSnapshot     `json:"gpus,omitempty"`
	SpikeDetected bool              `json:"spike_detected"`
}

// TrendPoint is a single data point for timeline charts
type TrendPoint struct {
	Timestamp string  `json:"ts"`
	Value     float64 `json:"value"`
}

// Analyze generates a forensic report from the recorded snapshots
func Analyze(snapshots []Snapshot) ForensicReport {
	report := ForensicReport{
		SnapshotCount: len(snapshots),
	}

	if len(snapshots) == 0 {
		report.Verdict = "No snapshots available. The recorder may not have been running before the crash."
		report.Severity = "unknown"
		return report
	}

	first := snapshots[0]
	last := snapshots[len(snapshots)-1]
	report.TimeRange = fmt.Sprintf("%s → %s", first.Timestamp.Format("15:04:05"), last.Timestamp.Format("15:04:05"))

	// Build CPU/MEM timelines
	for _, s := range snapshots {
		report.CPUTrend = append(report.CPUTrend, TrendPoint{
			Timestamp: s.Timestamp.Format("15:04:05"),
			Value:     s.TotalCPUPct,
		})
		report.MemTrend = append(report.MemTrend, TrendPoint{
			Timestamp: s.Timestamp.Format("15:04:05"),
			Value:     s.TotalMemPct,
		})
	}

	// Use the last snapshot for process analysis
	report.TopCPU = sortByCPU(last.TopProcesses, 10)
	report.TopMem = sortByMem(last.TopProcesses, 10)
	report.OOMLeaders = last.OOMLeaders
	report.GPUs = last.GPUs

	// Detect spike: was CPU or MEM climbing in the last N snapshots?
	if len(snapshots) >= 3 {
		recentMem := snapshots[len(snapshots)-3:]
		if recentMem[2].TotalMemPct > recentMem[0].TotalMemPct+5 {
			report.SpikeDetected = true
		}
		recentCPU := snapshots[len(snapshots)-3:]
		if recentCPU[2].TotalCPUPct > recentCPU[0].TotalCPUPct+10 {
			report.SpikeDetected = true
		}
	}

	// Generate verdict
	report.Verdict, report.Severity = generateVerdict(last, report.SpikeDetected)

	return report
}

func generateVerdict(last Snapshot, spikeDetected bool) (string, string) {
	if len(last.TopProcesses) == 0 {
		return "No process data in last snapshot.", "unknown"
	}

	// Find the biggest memory consumer
	topMem := sortByMem(last.TopProcesses, 1)
	topCPU := sortByCPU(last.TopProcesses, 1)

	var verdict string
	severity := "high"

	// Check if memory was the likely cause (>80%)
	if last.TotalMemPct > 80 && len(topMem) > 0 {
		p := topMem[0]
		verdict = fmt.Sprintf("MEMORY EXHAUSTION: Process \"%s\" (PID %d, user: %s) was consuming %.1f%% RAM (%.0f MB RSS)",
			p.Name, p.PID, p.User, p.MemPct, p.MemRSSMB)

		if p.GPUMemMB > 0 {
			verdict += fmt.Sprintf(" + %d MB GPU memory", p.GPUMemMB)
		}

		if p.OOMScore > 800 {
			verdict += fmt.Sprintf(". OOM score: %d/1000 — this process would be killed by the OOM killer.", p.OOMScore)
			severity = "critical"
		}

		if spikeDetected {
			verdict += " Memory was climbing rapidly in the seconds before crash."
		}

		return verdict, severity
	}

	// Check if CPU was the likely cause (>90%)
	if last.TotalCPUPct > 90 && len(topCPU) > 0 {
		p := topCPU[0]
		verdict = fmt.Sprintf("CPU SATURATION: Process \"%s\" (PID %d, user: %s) was consuming %.1f%% CPU.",
			p.Name, p.PID, p.User, p.CPUPct)
		severity = "high"
		return verdict, severity
	}

	// Check GPU
	if len(last.GPUs) > 0 {
		for _, gpu := range last.GPUs {
			if gpu.MemUsedMB > 0 && float64(gpu.MemUsedMB)/float64(gpu.MemTotalMB) > 0.95 {
				verdict = fmt.Sprintf("GPU MEMORY EXHAUSTION: GPU %d at %d/%d MB (%.0f%% utilized). ",
					gpu.ID, gpu.MemUsedMB, gpu.MemTotalMB, float64(gpu.MemUsedMB)/float64(gpu.MemTotalMB)*100)
				if len(topMem) > 0 && topMem[0].GPUMemMB > 0 {
					verdict += fmt.Sprintf("Top GPU consumer: \"%s\" (PID %d, user: %s) using %d MB.",
						topMem[0].Name, topMem[0].PID, topMem[0].User, topMem[0].GPUMemMB)
				}
				severity = "critical"
				return verdict, severity
			}
		}
	}

	// No clear single cause
	if len(topMem) > 0 {
		p := topMem[0]
		verdict = fmt.Sprintf("Top resource consumer at last snapshot: \"%s\" (PID %d, user: %s) — %.1f%% MEM, %.1f%% CPU.",
			p.Name, p.PID, p.User, p.MemPct, p.CPUPct)
		severity = "medium"
	} else {
		verdict = "Unable to determine root cause from available data."
		severity = "unknown"
	}

	return verdict, severity
}

func sortByCPU(procs []ProcessSnapshot, n int) []ProcessSnapshot {
	sorted := make([]ProcessSnapshot, len(procs))
	copy(sorted, procs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CPUPct > sorted[j].CPUPct })
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

func sortByMem(procs []ProcessSnapshot, n int) []ProcessSnapshot {
	sorted := make([]ProcessSnapshot, len(procs))
	copy(sorted, procs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].MemRSSMB > sorted[j].MemRSSMB })
	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}
