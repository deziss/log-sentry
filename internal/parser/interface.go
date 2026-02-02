package parser

import (
	"time"
)

// GenericLogEntry represents a common structure for all web server logs
type GenericLogEntry struct {
	RemoteIP      string
	RemoteUser    string
	TimeLocal     time.Time
	Method        string
	Path          string // Includes query params
	Protocol      string
	Status        int
	BodyBytesSent int
	Referer       string
	UserAgent     string
	Service       string // e.g. "nginx", "apache", "caddy"
}

// LogParser interface that all specific parsers must implement
type LogParser interface {
	Parse(line string) (*GenericLogEntry, error)
}
