package collector

import (
	"log-sentry/internal/parser"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

type LogCollector struct {
	// Nginx Metrics
	NginxRequests      *prometheus.CounterVec
	NginxRequestBytes  *prometheus.CounterVec
	NginxResponseBytes *prometheus.CounterVec

	// SSH Metrics
	SSHLoginAttempts *prometheus.CounterVec
	SSHDisconnects   *prometheus.CounterVec
}

func NewLogCollector() *LogCollector {
	return &LogCollector{
		NginxRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nginx_http_requests_total",
				Help: "Total number of Nginx HTTP requests.",
			},
			[]string{"method", "status", "path", "remote_ip"},
		),
		NginxRequestBytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nginx_http_request_bytes_total",
				Help: "Total number of bytes received by Nginx.",
			},
			[]string{"method", "remote_ip"},
		),
		NginxResponseBytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "nginx_http_response_bytes_total",
				Help: "Total number of bytes sent by Nginx.",
			},
			[]string{"method", "remote_ip"},
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
	}
}

func (c *LogCollector) Register(reg prometheus.Registerer) {
	reg.MustRegister(
		c.NginxRequests,
		c.NginxRequestBytes,
		c.NginxResponseBytes,
		c.SSHLoginAttempts,
		c.SSHDisconnects,
	)
}

func (c *LogCollector) ProcessNginx(entry *parser.NginxLogEntry) {
	statusStr := strconv.Itoa(entry.Status)

	c.NginxRequests.WithLabelValues(
		entry.Method,
		statusStr,
		entry.Path,
		entry.RemoteIP,
	).Inc()

	// Bytes metrics - using RemoteIP to answer "by which IP"
	// Note: We don't have request specific bytes in common log format usually (BodyBytesSent is response size)
	// We'll track response bytes
	c.NginxResponseBytes.WithLabelValues(
		entry.Method,
		entry.RemoteIP,
	).Add(float64(entry.BodyBytesSent))
}

func (c *LogCollector) ProcessSSH(entry *parser.SSHLogEntry) {
	if entry.Type == parser.SSHLoginSuccess {
		c.SSHLoginAttempts.WithLabelValues(entry.User, entry.IP, "success", entry.AuthMethod).Inc()
	} else if entry.Type == parser.SSHLoginFailed {
		c.SSHLoginAttempts.WithLabelValues(entry.User, entry.IP, "failed", entry.AuthMethod).Inc()
	} else if entry.Type == parser.SSHDisconnect {
		c.SSHDisconnects.WithLabelValues().Inc()
	}
}
