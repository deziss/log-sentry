package worker

import (
	"log"

	"log-sentry/internal/analyzer"
	"log-sentry/internal/anomaly"
	"log-sentry/internal/collector"
	"log-sentry/internal/enricher"
	"log-sentry/internal/parser"
)

type Job struct {
	ServiceName string
	LogPath     string
	Line        string
	Parser      parser.LogParser
}

type Pool struct {
	JobQueue   chan Job
	WorkerCount int
	Collector  *collector.LogCollector
	Analyzer   *analyzer.Analyzer
	AnomalyDetector *anomaly.AnomalyDetector
	Enricher   *enricher.Enricher
}

func NewPool(workers int, coll *collector.LogCollector, analyzer *analyzer.Analyzer, ad *anomaly.AnomalyDetector, enrich *enricher.Enricher) *Pool {
	return &Pool{
		JobQueue:    make(chan Job, 1000), // Buffered channel
		WorkerCount: workers,
		Collector:   coll,
		Analyzer:    analyzer,
		AnomalyDetector: ad,
		Enricher:    enrich,
	}
}

func (p *Pool) Start() {
	for i := 0; i < p.WorkerCount; i++ {
		go p.worker(i)
	}
	log.Printf("Worker pool started with %d workers", p.WorkerCount)
}

func (p *Pool) worker(id int) {
	for job := range p.JobQueue {
		// 1. Parse
		entry, err := job.Parser.Parse(job.Line)
		if err != nil {
			// Optional: log debug or count parse errors
			continue
		}
		
		// Enforce service name from job context
		entry.Service = job.ServiceName

		// 2. Security Analysis
		attack := p.Analyzer.DetectAttack(entry.Path, entry.UserAgent)
		if !attack.Detected {
			// Check for data exfiltration if no other attack detected (or in addition?)
			// Let's do in addition, but AttackResult is singular. Priority to Exfil?
			// Or just overwrite if Exfil detected?
			exfil := p.Analyzer.CheckDataExfiltration(entry.BodyBytesSent)
			if exfil.Detected {
				attack = exfil
			}
		}
		
		// 2b. Anomaly Detection
		anomalyType := p.AnomalyDetector.Check(entry.RemoteIP, entry.Status)

		// 2c. Enrichment
		netType := p.Enricher.ClassifyIP(entry.RemoteIP)
		// If entry.User exists, we could also resolve/enrich it here
		// e.g. realUser := p.Enricher.ResolveUser(entry.User)

		// 3. Record Metrics
		p.Collector.ProcessWeb(entry, attack, anomalyType, netType)
	}
}

func (p *Pool) Submit(job Job) {
	// Non-blocking try or blocking? blocking is safer for backpressure
	p.JobQueue <- job
}
