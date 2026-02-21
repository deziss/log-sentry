package recorder

import (
	"os/exec"
	"strconv"
	"strings"
)

// ReadGPUs queries nvidia-smi for GPU utilization, memory, and temperature.
// Returns empty slice if nvidia-smi is not available (non-GPU servers).
func ReadGPUs() []GPUSnapshot {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=index,utilization.gpu,memory.used,memory.total,temperature.gpu",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil // nvidia-smi not available — graceful fallback
	}

	var gpus []GPUSnapshot
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, ", ")
		if len(fields) < 5 {
			continue
		}
		id, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		util, _ := strconv.Atoi(strings.TrimSpace(fields[1]))
		memUsed, _ := strconv.Atoi(strings.TrimSpace(fields[2]))
		memTotal, _ := strconv.Atoi(strings.TrimSpace(fields[3]))
		temp, _ := strconv.Atoi(strings.TrimSpace(fields[4]))

		gpus = append(gpus, GPUSnapshot{
			ID:         id,
			UtilPct:    util,
			MemUsedMB:  memUsed,
			MemTotalMB: memTotal,
			TempC:      temp,
		})
	}
	return gpus
}

// ReadGPUProcesses returns a map of PID → GPU memory used (MB).
func ReadGPUProcesses() map[int]int {
	out, err := exec.Command("nvidia-smi",
		"--query-compute-apps=pid,used_memory",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil
	}

	result := make(map[int]int)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, ", ")
		if len(fields) < 2 {
			continue
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(fields[0]))
		mem, _ := strconv.Atoi(strings.TrimSpace(fields[1]))
		if pid > 0 {
			result[pid] = mem
		}
	}
	return result
}
