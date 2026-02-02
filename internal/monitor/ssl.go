package monitor

import (
	"crypto/tls"
	"log"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type SSLMonitor struct {
	Targets []string // e.g., "localhost:443"
	ExpiryMetric *prometheus.GaugeVec
}

func NewSSLMonitor() *SSLMonitor {
	m := &SSLMonitor{
		Targets: []string{},
		ExpiryMetric: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ssl_cert_expiry_days",
			Help: "Days until SSL certificate expiration",
		}, []string{"target", "common_name"}),
	}
	return m
}

func (s *SSLMonitor) Register(reg prometheus.Registerer) {
	reg.MustRegister(s.ExpiryMetric)
}

func (s *SSLMonitor) AddTarget(target string) {
	s.Targets = append(s.Targets, target)
}

func (s *SSLMonitor) Start(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			s.checkAll()
			<-ticker.C
		}
	}()
}

func (s *SSLMonitor) checkAll() {
	for _, target := range s.Targets {
		s.checkOne(target)
	}
}

func (s *SSLMonitor) checkOne(target string) {
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", target, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Printf("SSL check failed for %s: %v", target, err)
		return
	}
	defer conn.Close()

	if len(conn.ConnectionState().PeerCertificates) > 0 {
		cert := conn.ConnectionState().PeerCertificates[0]
		days := time.Until(cert.NotAfter).Hours() / 24
		s.ExpiryMetric.WithLabelValues(target, cert.Subject.CommonName).Set(days)
	}
}
