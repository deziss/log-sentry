// Stress Test: eats CPU and RAM to trigger Log Sentry's threshold-based recording.
// Run: go run cmd/stress/main.go
// Verify: curl http://localhost:9102/api/crashes

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

func main() {
	fmt.Println("=== Log Sentry Stress Test ===")
	fmt.Printf("CPUs: %d, triggering ≥90%% CPU + memory spike\n\n", runtime.NumCPU())

	// Phase 1: CPU saturation
	fmt.Println("[1/3] Saturating CPU...")
	numCPU := runtime.NumCPU()
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x := 1.0
			for {
				select {
				case <-stop:
					return
				default:
					x = math.Sin(x) + math.Cos(x)
				}
			}
		}()
	}

	// Phase 2: Memory allocation (allocate 70% of total RAM)
	fmt.Println("[2/3] Allocating memory...")
	var memBlocks [][]byte
	totalMem := getTotalMem()
	targetBytes := int(float64(totalMem) * 0.30) // 30% — combined with existing usage should push past 90%
	blockSize := 100 * 1024 * 1024              // 100MB per block
	allocated := 0

	for allocated < targetBytes {
		block := make([]byte, blockSize)
		// Touch every page to ensure physical allocation
		for j := 0; j < len(block); j += 4096 {
			block[j] = 0xFF
		}
		memBlocks = append(memBlocks, block)
		allocated += blockSize
		fmt.Printf("  Allocated: %d MB / %d MB target\n", allocated/1024/1024, targetBytes/1024/1024)
	}

	fmt.Println("[3/3] Holding for 30 seconds... (threshold should trigger)")
	fmt.Println("  Check: curl http://localhost:9102/api/crashes")

	// Wait 30 seconds, then release
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-time.After(30 * time.Second):
	case <-sigCh:
		fmt.Println("\n  Interrupted!")
	}

	// Release
	fmt.Println("\nReleasing resources...")
	close(stop)
	wg.Wait()
	memBlocks = nil
	runtime.GC()

	// Wait for hysteresis to resolve the event
	fmt.Println("Waiting 15s for hysteresis to close event...")
	time.Sleep(15 * time.Second)

	// Verify
	fmt.Println("\n=== Verifying Crash Events ===")
	resp, err := http.Get("http://localhost:9102/api/crashes")
	if err != nil {
		fmt.Printf("❌ Failed to query API: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var crashes []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&crashes)

	if len(crashes) == 0 {
		fmt.Println("❌ No crash events detected! Threshold may not have been reached.")
		os.Exit(1)
	}

	fmt.Printf("✅ %d crash event(s) detected:\n", len(crashes))
	for _, c := range crashes {
		fmt.Printf("  ID: %s\n", c["id"])
		fmt.Printf("  Trigger: %s\n", c["trigger"])
		fmt.Printf("  Severity: %s\n", c["severity"])
		fmt.Printf("  Verdict: %s\n", c["verdict"])
		fmt.Printf("  Snapshots: %.0f\n\n", c["snapshot_count"])
	}
}

func getTotalMem() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 16 * 1024 * 1024 * 1024 // fallback 16GB
	}
	for _, line := range splitLines(string(data)) {
		if len(line) > 9 && line[:9] == "MemTotal:" {
			var kb uint64
			fmt.Sscanf(line, "MemTotal: %d kB", &kb)
			return kb * 1024
		}
	}
	return 16 * 1024 * 1024 * 1024
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
