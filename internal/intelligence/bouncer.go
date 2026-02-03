package intelligence

import (
	"context"
	"log"
	"sync"
	"github.com/crowdsecurity/crowdsec/pkg/models"
	csbouncer "github.com/crowdsecurity/go-cs-bouncer"
	"github.com/prometheus/client_golang/prometheus"
)

type CrowdSecBouncer struct {
	StreamBouncer *csbouncer.StreamBouncer
	BanMetric     prometheus.Counter
	
	// Local cache of banned IPs
	BannedIPs     map[string]bool
	mu            sync.RWMutex
}

func NewCrowdSecBouncer(apiKey, apiUrl string) (*CrowdSecBouncer, error) {
	cb := &CrowdSecBouncer{
		BannedIPs: make(map[string]bool),
		BanMetric: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "security_crowdsec_ban_detected",
			Help: "Total number of requests detected from IPs banned by CrowdSec",
		}),
	}
	
	cb.StreamBouncer = &csbouncer.StreamBouncer{
		APIKey:         apiKey,
		APIUrl:         apiUrl,
		TickerInterval: "15s",
		UserAgent:      "log-sentry/v2.3.2",
	}
	
	return cb, nil
}

func (cb *CrowdSecBouncer) Register(reg prometheus.Registerer) {
	reg.MustRegister(cb.BanMetric)
}

func (cb *CrowdSecBouncer) Start() error {
	log.Println("Starting CrowdSec StreamBouncer...")
	return cb.StreamBouncer.Init()
}

func (cb *CrowdSecBouncer) Run() {
	ctx := context.Background()
	
	// Start the library's runner in a goroutine (Produces events)
	go func() {
		if err := cb.StreamBouncer.Run(ctx); err != nil {
			log.Printf("[CrowdSec] Bouncer Run failed: %v", err)
		}
	}()

	// Consume events
	for decisions := range cb.StreamBouncer.Stream {
		cb.handleDecisions(decisions)
	}
}

func (cb *CrowdSecBouncer) handleDecisions(decisions *models.DecisionsStreamResponse) {
	if decisions == nil {
		return
	}
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	// Process New Decisions
	for _, decision := range decisions.New {
		if *decision.Type == "ban" {
			cb.BannedIPs[*decision.Value] = true
		}
	}
	
	// Process Deleted Decisions
	for _, decision := range decisions.Deleted {
		if *decision.Type == "ban" {
			delete(cb.BannedIPs, *decision.Value)
		}
	}
}

func (cb *CrowdSecBouncer) Check(ipStr string) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.BannedIPs[ipStr]
}
