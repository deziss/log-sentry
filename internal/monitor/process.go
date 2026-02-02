package monitor

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v3/process"
)

type ProcessSentinel struct {
	Blacklist []string
	AlertMetric *prometheus.GaugeVec
}

func NewProcessSentinel() *ProcessSentinel {
	return &ProcessSentinel{
		Blacklist: []string{"nc", "nmap", "hydra", "john", "xmrig"},
		AlertMetric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "security_unexpected_process_active",
			Help: "Indicates if a blacklisted process is currently running (1=active)",
		}, []string{"name", "pid", "cmdline"}),
	}
}

func (p *ProcessSentinel) Register(reg prometheus.Registerer) {
	reg.MustRegister(p.AlertMetric)
}

func (p *ProcessSentinel) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			p.scan()
			<-ticker.C
		}
	}()
}

func (p *ProcessSentinel) scan() {
	procs, err := process.Processes()
	if err != nil {
		log.Printf("ProcessSentinel scan failed: %v", err)
		return
	}

	p.AlertMetric.Reset() // Clear old alerts

	for _, proc := range procs {
		name, err := proc.Name()
		if err != nil {
			continue
		}

		for _, bad := range p.Blacklist {
			// Simple substring match for "nmap" or "xmrig"
			if strings.Contains(strings.ToLower(name), bad) {
				pid := proc.Pid
				cmd, _ := proc.Cmdline()
				if len(cmd) > 50 {
					cmd = cmd[:50] + "..."
				}
				
				p.AlertMetric.WithLabelValues(bad, fmt.Sprintf("%d", pid), cmd).Set(1)
				// Log it too
				log.Printf("SECURITY ALERT: Suspicious process detected: %s (PID: %d) Cmd: %s", name, pid, cmd)
			}
		}
	}
}
