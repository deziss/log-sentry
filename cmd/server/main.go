package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"log-sentry/internal/analyzer"
	"log-sentry/internal/anomaly"
	"log-sentry/internal/collector"
	"log-sentry/internal/config"
	"log-sentry/internal/discovery"
	"log-sentry/internal/enricher"
	"log-sentry/internal/intelligence"
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
	// 1. Load Configuration
	cfg := config.Load()
	log.Printf("Starting Log Sentry V2 on port %d...", cfg.Port)

	// 0. Initialize CrowdSec Bouncer (Optional)
	var bouncer *intelligence.CrowdSecBouncer
	if cfg.EnableCrowdSec {
		var err error
		bouncer, err = intelligence.NewCrowdSecBouncer(cfg.CrowdSecAPIKey, cfg.CrowdSecLAPIURL)
		if err != nil {
			log.Fatalf("Failed to initialize CrowdSec Bouncer: %v", err)
		}
		if err := bouncer.Start(); err != nil {
			log.Fatalf("Failed to start CrowdSec Bouncer: %v", err)
		}
		go bouncer.Run()
	}

	// 2. Initialize Core Components
	enr := enricher.NewEnricher()
	coll := collector.NewLogCollector(enr)
	if bouncer != nil {
		coll.Bouncer = bouncer
		bouncer.Register(prometheus.DefaultRegisterer) // Explicit registration
		log.Println("CrowdSec Integration Enabled")
	}
	coll.Register(prometheus.DefaultRegisterer)

	secAnalyzer := analyzer.NewAnalyzer()
	anomalyDetector := anomaly.NewAnomalyDetector()
	// enricher initialized earlier as 'enr'
	autoDisco := discovery.NewAutoDiscover()

	// 2a. Initialize Worker Pool
	wp := worker.NewPool(5, coll, secAnalyzer, anomalyDetector, enr)
	wp.Start()

	// 3. Auto-Discovery
	log.Println("Running Auto-Discovery...")
	services, err := autoDisco.Scan()
	if err != nil {
		log.Printf("Auto-discovery warning: %v", err)
	}

	// 4a. Start Syslog Server (Network Ingestion)
	syslogServer := syslog.NewSyslogServer(5140, coll, secAnalyzer, enr)
	syslogServer.Start()

	// 4b. Start Monitoring for Discovered/Configured Services
	monitoredCount := 0

	// Helper to start monitoring a web service
	monitorService := func(name, path string, p parser.LogParser) {
		if path == "" {
			return
		}
		log.Printf("Monitoring %s logs at: %s", name, path)

		// Metric Initialization (Ensure they appear as 0 instead of missing)
		// We initialize common vectors to ensure they show up in Prometheus output even if 0
		coll.WebRequests.WithLabelValues(name, "GET", "200", "/", "unknown", "unknown").Add(0)
		coll.WebRequestBytes.WithLabelValues(name, "GET").Add(0)
		coll.WebResponseBytes.WithLabelValues(name, "GET", "unknown").Add(0)
		
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
		
		// Use discovered path, or magic path if enabled
		path := svc.LogPath
		if cfg.EnableMagicLogAccess && svc.MagicLogPath != "" {
			path = svc.MagicLogPath
			log.Printf("Auto-ingesting %s via magic path: %s", svc.Name, path)
		}
		
		monitorService(svc.Name, path, p)
	}

	// 4d. Explicit Config Fallbacks (if not discovered or forced)
	if monitoredCount == 0 || cfg.NginxAccessLogPath != "" {
		log.Println("Loading configured log paths...")
		monitorService("nginx_manual", cfg.NginxAccessLogPath, &parser.NginxParser{})
	}
	
	// 4e. V2.2 Security Monitors
	// SSL Monitor (Default check localhost:443)
	sslMon := monitor.NewSSLMonitor()
	sslMon.Register(prometheus.DefaultRegisterer)
	sslMon.AddTarget("localhost:443")
	sslMon.Start(1 * time.Hour) // Check hourly

	// File Integrity Monitor (FIM)
	fim := monitor.NewFIM()
	fim.Register(prometheus.DefaultRegisterer)
	fim.AddPath("/etc/passwd") 
	fim.AddPath(cfg.NginxAccessLogPath)
	fim.Start(30 * time.Second)

	// Process Sentinel
	procSent := monitor.NewProcessSentinel()
	procSent.Register(prometheus.DefaultRegisterer)
	procSent.Start(30 * time.Second)

	// SSH Monitoring is distinct
	startSSHMonitoring(cfg.SSHAuthLogPath, coll)

    // 4f. System Integration
    go journald.StartReader(wp) // Uncomment to enable if journalctl is present
    
	// 5. Start HTTP Server
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Listening on %s", addr)
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
