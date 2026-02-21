package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"log-sentry/internal/alerts"
	"log-sentry/internal/analyzer"
	"log-sentry/internal/anomaly"
	"log-sentry/internal/collector"
	"log-sentry/internal/config"
	"log-sentry/internal/discovery"
	"log-sentry/internal/enricher"
	"log-sentry/internal/journald"
	"log-sentry/internal/monitor"
	"log-sentry/internal/parser"
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

	// 2. Initialize Core Components
	// Enricher needs to be initialized before Collector now
	enrich := enricher.NewEnricher(cfg.GeoIPCityPath, cfg.GeoIPASNPath)
	defer enrich.Close()

	coll := collector.NewLogCollector(enrich)
	coll.Register(prometheus.DefaultRegisterer)

	secAnalyzer := analyzer.NewAnalyzer()
	anomalyDetector := anomaly.NewAnomalyDetector()
	autoDisco := discovery.NewAutoDiscover()

	// 2a. Initialize Worker Pool
	wp := worker.NewPool(5, coll, secAnalyzer, anomalyDetector, enrich)
	wp.Start()

	// 3. Auto-Discovery
	log.Println("Running Auto-Discovery...")
	services, err := autoDisco.Scan()
	if err != nil {
		log.Printf("Auto-discovery warning: %v", err)
	}

	// 4a. Start Syslog Server (Network Ingestion)
	syslogServer := syslog.NewSyslogServer(5140, coll, secAnalyzer, enrich)
	syslogServer.Start()

	// 4b. Start Monitoring for Discovered/Configured Services
	monitoredCount := 0

	// Helper to start monitoring a web service
	monitorService := func(name, path string, p parser.LogParser) {
		if path == "" {
			return
		}
		log.Printf("Monitoring %s logs at: %s", name, path)
		startWebMonitoring(name, path, p, wp)
		monitoredCount++
	}

	// 4c. Auto-Discovered Services
	for _, svc := range services {
		log.Printf("Discovered service: %s (PID: %d)", svc.Name, svc.PID)
		var p parser.LogParser
		switch svc.Name {
		case "nginx":
			p = &parser.NginxParser{}
		case "apache", "apache2", "httpd":
			p = &parser.ApacheParser{}
		case "caddy":
			p = &parser.CaddyParser{}
		case "tomcat":
			p = &parser.TomcatParser{}
		case "traefik":
			p = &parser.TraefikParser{}
		case "haproxy":
			p = &parser.HAProxyParser{}
		case "envoy":
			p = &parser.EnvoyParser{}
		case "lighttpd":
			p = &parser.LighttpdParser{}
		default:
			log.Printf("No parser for discovered service: %s", svc.Name)
			continue
		}
		
		// Use discovered path, or fallback to config if env var set for this specific service?
		// For now, auto-discovery takes precedence if it found a path, 
		// but our current auto-discover just guesses defaults.
		// We'll trust the guess for now.
		monitorService(svc.Name, svc.LogPath, p)
	}

	// 4b. Explicit Config Fallbacks (if not discovered)
	// If auto-discovery didn't find Nginx, but ENV is set:
	if monitoredCount == 0 || cfg.NginxAccessLogPath != "" {
		// Simple check: if we haven't monitored nginx yet, add it
		// Real logic would be more complex deduplication.
		// For verification, we Just Load Configured Paths as "manual" services
		log.Println("Loading configured log paths...")
		monitorService("nginx_manual", cfg.NginxAccessLogPath, &parser.NginxParser{})
	}
	
	// 4d. Alerting System
	alerter := alerts.NewDispatcher(cfg.WebhookURL)

	// 4e. V2.2 Security Monitors
	// SSL Monitor (Default check localhost:443)
	sslMon := monitor.NewSSLMonitor()
	sslMon.Register(prometheus.DefaultRegisterer)
	sslMon.AddTarget("localhost:443")
	sslMon.Start(1 * time.Hour) // Check hourly

	// File Integrity Monitor (FIM)
	// Passing alerter so FIM can send webhooks on critical file changes
	fim := monitor.NewFIM(alerter) 
	fim.Register(prometheus.DefaultRegisterer)
	fim.AddPath("/etc/passwd") 
	fim.AddPath(cfg.NginxAccessLogPath) // Just as an example watcher
	fim.Start(30 * time.Second)

	// Process Sentinel
	procSent := monitor.NewProcessSentinel(alerter)
	procSent.Register(prometheus.DefaultRegisterer)
	
	// Dynamic Rules loading for Process Sentinel
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

	// SSH Monitoring is distinct
	startSSHMonitoring(cfg.SSHAuthLogPath, coll)

    // 4f. System Integration
    go journald.StartReader(wp) // Uncomment to enable if journalctl is present
    
	// 5. Start HTTP Server
	http.Handle("/metrics", promhttp.Handler())
	addr := fmt.Sprintf(":%d", cfg.Port)
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
