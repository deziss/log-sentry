package parser

import (
	"encoding/json"
	"fmt"
	"time"
)

type CaddyParser struct{}

func init() { Register("caddy", func() LogParser { return &CaddyParser{} }) }

// CaddyJSONEntry reflects the standard structure of Caddy's JSON logs
type CaddyJSONEntry struct {
	Level   string  `json:"level"`
	Ts      float64 `json:"ts"`
	Logger  string  `json:"logger"`
	Msg     string  `json:"msg"`
	Request struct {
		RemoteIP  string              `json:"remote_ip"`
		Method    string              `json:"method"`
		URI       string              `json:"uri"`
		Proto     string              `json:"proto"`
		Headers   map[string][]string `json:"headers"`
	} `json:"request"`
	Status int `json:"status"`
	Size   int `json:"size"` // Response size
}

func (p *CaddyParser) Parse(line string) (*GenericLogEntry, error) {
	var entry CaddyJSONEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nil, fmt.Errorf("failed to parse caddy json: %v", err)
	}

	// Caddy timestamp is float64 unix seconds
	t := time.UnixMilli(int64(entry.Ts * 1000))

	// Referer and UA are in headers
	referer := ""
	if v, ok := entry.Request.Headers["Referer"]; ok && len(v) > 0 {
		referer = v[0]
	}
	userAgent := ""
	if v, ok := entry.Request.Headers["User-Agent"]; ok && len(v) > 0 {
		userAgent = v[0]
	}

	return &GenericLogEntry{
		Service:       "caddy",
		RemoteIP:      entry.Request.RemoteIP,
		RemoteUser:    "-", // Auth user not always in standard json structure easily
		TimeLocal:     t,
		Method:        entry.Request.Method,
		Path:          entry.Request.URI,
		Protocol:      entry.Request.Proto,
		Status:        entry.Status,
		BodyBytesSent: entry.Size,
		Referer:       referer,
		UserAgent:     userAgent,
	}, nil
}
