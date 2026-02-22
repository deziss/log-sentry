package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"log-sentry/internal/alerts"
	"log-sentry/internal/api"
	"log-sentry/internal/recorder"
	"log-sentry/internal/analyzer"
	"log-sentry/internal/anomaly"
	"log-sentry/internal/collector"
	"log-sentry/internal/config"
	"log-sentry/internal/enricher"
	"log-sentry/internal/journald"
	"log-sentry/internal/monitor"
	"log-sentry/internal/parser"
	"log-sentry/internal/storage"
	"log-sentry/internal/syslog"
	"log-sentry/internal/tailer"
	"log-sentry/internal/worker"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 1. Load Configuration (Env vars still override)
	cfg := config.Load()
	log.Printf("Starting Log Sentry V2 on port %d...", cfg.Port)

	// 1a. Initialize BoltDB Storage
	store, err := storage.NewBoltStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()

	// 2. Initialize Core Components
	// Enricher needs to be initialized before Collector now
	enrich := enricher.NewEnricher(cfg.GeoIPCityPath, cfg.GeoIPASNPath)
	defer enrich.Close()

	coll := collector.NewLogCollector(enrich)
	coll.Register(prometheus.DefaultRegisterer)

	secAnalyzer := analyzer.NewAnalyzer()
	anomalyDetector := anomaly.NewAnomalyDetector()

	// 2a. Initialize Worker Pool
	wp := worker.NewPool(5, coll, secAnalyzer, anomalyDetector, enrich)
	wp.Start()

	// 3. Start Services from Config (Config-Driven Architecture)
	log.Printf("Registered parsers: %v", parser.AvailableParsers())
	log.Printf("Starting %d configured services...", len(cfg.Services))

	for _, svc := range cfg.Services {
		if !svc.Enabled {
			log.Printf("  [SKIP] %s (disabled)", svc.Name)
			continue
		}
		if svc.LogPath == "" {
			log.Printf("  [SKIP] %s (no log_path)", svc.Name)
			continue
		}

		// SSH is a special case — it has its own processing pipeline
		if svc.Type == "ssh" {
			log.Printf("  [SSH]  %s → %s", svc.Name, svc.LogPath)
			startSSHMonitoring(svc.LogPath, coll)
			continue
		}

		// All other types: resolve parser via registry (polymorphism)
		p, err := parser.Get(svc.Type)
		if err != nil {
			log.Printf("  [ERR]  %s: %v", svc.Name, err)
			continue
		}
		log.Printf("  [OK]   %s (%s) → %s", svc.Name, svc.Type, svc.LogPath)
		startWebMonitoring(svc.Name, svc.LogPath, p, wp)
	}

	// 4a. Start Syslog Server (Network Ingestion)
	syslogServer := syslog.NewSyslogServer(5140, coll, secAnalyzer, enrich)
	syslogServer.Start()

	// 4b. Alerting System
	alerter := alerts.NewDispatcher(cfg.WebhookURL)

	// 4c. Security Monitors
	sslMon := monitor.NewSSLMonitor()
	sslMon.Register(prometheus.DefaultRegisterer)
	sslMon.AddTarget("localhost:443")
	sslMon.Start(1 * time.Hour)

	fim := monitor.NewFIM(alerter)
	fim.Register(prometheus.DefaultRegisterer)
	fim.AddPath("/etc/passwd")
	fim.Start(30 * time.Second)

	procSent := monitor.NewProcessSentinel(alerter)
	procSent.Register(prometheus.DefaultRegisterer)
	if r, err := cfg.LoadRules(); err == nil {
		procSent.UpdateBlacklist(r.ProcessBlacklist)
	}
	cfg.WatchConfig(func() {
		if r, err := cfg.LoadRules(); err == nil {
			procSent.UpdateBlacklist(r.ProcessBlacklist)
		} else {
			log.Printf("Failed to reload rules: %v", err)
		}
	})
	procSent.Start(30 * time.Second)

	// 4d. System Integration
	go journald.StartReader(wp)

	// 4e. Resource Recorder (threshold-based crash detection + BoltDB persistence)
	rec := recorder.NewResourceRecorder(recorder.RecorderConfig{
		IntervalSec:   cfg.SnapshotInterval,
		Threshold:     cfg.Threshold,
		RetentionDays: cfg.RetentionDays,
		LokiURL:       cfg.LokiURL,
		WebhookURL:    cfg.WebhookURL,
		Store:         store,
	})
	rec.Register(prometheus.DefaultRegisterer)
	rec.Start()
	log.Printf("ResourceRecorder: threshold=%.0f%%, db=%s, retention=%dd, loki=%s",
		cfg.Threshold, cfg.DBPath, cfg.RetentionDays, cfg.LokiURL)

	// 5. REST API for UI
	apiHandler := api.NewAPI(cfg, rec)
	mux := http.DefaultServeMux
	apiHandler.RegisterRoutes(mux)

	// 6. Prometheus Metrics
	mux.Handle("/metrics", promhttp.Handler())

	// 7. Serve Frontend (static files from ui/dist)
	fs := http.FileServer(http.Dir("ui/dist"))
	mux.Handle("/", fs)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("HTTP server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func startWebMonitoring(service, path string, p parser.LogParser, wp *worker.Pool) {
	lines := make(chan string)
	tailer.TailFile(path, lines)

	go func() {
		for line := range lines {
			wp.Submit(worker.Job{
				ServiceName: service,
				LogPath:     path,
				Line:        line,
				Parser:      p,
			})
		}
	}()
}

func startSSHMonitoring(path string, coll *collector.LogCollector) {
	log.Printf("Monitoring SSH logs at: %s", path)
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
