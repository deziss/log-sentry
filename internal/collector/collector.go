package collector

import (
	"log-sentry/internal/analyzer"
	"log-sentry/internal/anomaly"
	"log-sentry/internal/enricher"
	"log-sentry/internal/parser"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

type LogCollector struct {
	Enricher         *enricher.Enricher
	
	// Web Metrics (Generic for Nginx, Apache, etc.)
	WebRequests      *prometheus.CounterVec
	WebRequestBytes  *prometheus.CounterVec // often missing in standard logs
	WebResponseBytes *prometheus.CounterVec
	WebAttacks       *prometheus.CounterVec
	WebAnomalies     *prometheus.CounterVec

	// SSH Metrics
	SSHLoginAttempts  *prometheus.CounterVec
	SSHDisconnects    *prometheus.CounterVec
	SSHActiveSessions prometheus.Gauge
}

func NewLogCollector(enr *enricher.Enricher) *LogCollector {
	return &LogCollector{
		Enricher: enr,
		WebRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests.",
			},
			[]string{"service", "method", "status", "path", "remote_ip", "network_type", "country", "asn"},
		),
		WebRequestBytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_request_bytes_total",
				Help: "Total number of bytes received (estimated).",
			},
			[]string{"service", "method"},
		),
		WebResponseBytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_response_bytes_total",
				Help: "Total number of bytes sent.",
			},
			[]string{"service", "method", "remote_ip"},
		),
		WebAttacks: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "web_attack_detected_total",
				Help: "Total number of detected web attacks.",
			},
			[]string{"service", "type", "severity", "endpoint", "source_ip", "network_type"},
		),
		WebAnomalies: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "web_anomaly_detected_total",
				Help: "Total number of detected traffic anomalies (e.g., 404 floods).",
			},
			[]string{"service", "type", "source_ip"},
		),
		SSHLoginAttempts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ssh_login_attempts_total",
				Help: "Total number of SSH login attempts.",
			},
			[]string{"user", "ip", "status", "auth_method"},
		),
		SSHDisconnects: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ssh_disconnects_total",
				Help: "Total number of SSH disconnect events.",
			},
			[]string{},
		),
		SSHActiveSessions: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ssh_active_sessions",
				Help: "Estimated number of active SSH sessions.",
			},
		),
	}
}

func (c *LogCollector) Register(reg prometheus.Registerer) {
	reg.MustRegister(
		c.WebRequests,
		c.WebRequestBytes,
		c.WebResponseBytes,
		c.WebAttacks,
		c.WebAnomalies,
		c.SSHLoginAttempts,
		c.SSHDisconnects,
		c.SSHActiveSessions,
	)
}

func (c *LogCollector) ProcessWeb(entry *parser.GenericLogEntry, attack analyzer.AttackResult, anomalyType anomaly.AnomalyType, networkType string) {
	statusStr := strconv.Itoa(entry.Status)

	country, asn := "Unknown", "Unknown"
	if c.Enricher != nil {
		country, asn = c.Enricher.GeoEnrich(entry.RemoteIP)
	}

	c.WebRequests.WithLabelValues(
		entry.Service,
		entry.Method,
		statusStr,
		entry.Path,
		entry.RemoteIP,
		networkType,
		country,
		asn,
	).Inc()

	c.WebResponseBytes.WithLabelValues(
		entry.Service,
		entry.Method,
		entry.RemoteIP,
	).Add(float64(entry.BodyBytesSent))

	if attack.Detected {
		c.WebAttacks.WithLabelValues(
			entry.Service,
			attack.Type,
			attack.Severity,
			entry.Path,
			entry.RemoteIP,
			networkType,
		).Inc()
	}
	
	if anomalyType != "" {
		c.WebAnomalies.WithLabelValues(
			entry.Service,
			string(anomalyType),
			entry.RemoteIP,
		).Inc()
	}
}

func (c *LogCollector) ProcessSSH(entry *parser.SSHLogEntry) {
	if entry.Type == parser.SSHLoginSuccess {
		c.SSHLoginAttempts.WithLabelValues(entry.User, entry.IP, "success", entry.AuthMethod).Inc()
		c.SSHActiveSessions.Inc()
	} else if entry.Type == parser.SSHLoginFailed {
		c.SSHLoginAttempts.WithLabelValues(entry.User, entry.IP, "failed", entry.AuthMethod).Inc()
	} else if entry.Type == parser.SSHDisconnect {
		c.SSHDisconnects.WithLabelValues().Inc()
		c.SSHActiveSessions.Dec()
	}
}
