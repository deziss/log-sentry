package main

import (
	"fmt"
	"log"
	"net/http"

	"log-sentry/internal/collector"
	"log-sentry/internal/config"
	"log-sentry/internal/parser"
	"log-sentry/internal/tailer"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 1. Load Configuration
	cfg := config.Load()
	log.Printf("Starting Log Sentry on port %d...", cfg.Port)
	log.Printf("Monitoring Nginx Access Log: %s", cfg.NginxAccessLogPath)
	log.Printf("Monitoring SSH Auth Log: %s", cfg.SSHAuthLogPath)

	// 2. Initialize Collector
	coll := collector.NewLogCollector()
	coll.Register(prometheus.DefaultRegisterer)

	// 3. Start Tailing & Parsing
	startNginxMonitoring(cfg.NginxAccessLogPath, coll)
	startSSHMonitoring(cfg.SSHAuthLogPath, coll)

	// 4. Start HTTP Server
	http.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func startNginxMonitoring(path string, coll *collector.LogCollector) {
	lines := make(chan string)
	tailer.TailFile(path, lines)

	go func() {
		for line := range lines {
			entry, err := parser.ParseNginxLine(line)
			if err != nil {
				// Verbose logging might be too noisy for prod, but good for debug
				// log.Printf("Failed to parse Nginx line: %v", err)
				continue
			}
			coll.ProcessNginx(entry)
		}
	}()
}

func startSSHMonitoring(path string, coll *collector.LogCollector) {
	lines := make(chan string)
	tailer.TailFile(path, lines)

	go func() {
		for line := range lines {
			entry, err := parser.ParseSSHLine(line)
			if err != nil {
				continue
			}
			if entry != nil {
				coll.ProcessSSH(entry)
			}
		}
	}()
}
