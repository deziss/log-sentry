package parser

import (
	"encoding/json"
	"fmt"
	"time"
)

type TraefikParser struct{}

func init() { Register("traefik", func() LogParser { return &TraefikParser{} }) }

// TraefikJSONEntry represents a Traefik access log line in JSON format
// Fields based on common Traefik access log structure
type TraefikJSONEntry struct {
	ClientHost            string            `json:"ClientHost"`
	ClientUsername        string            `json:"ClientUsername"`
	StartUTC              string            `json:"StartUTC"` // 2023-12-01T12:00:00Z
	RequestMethod         string            `json:"RequestMethod"`
	RequestPath           string            `json:"RequestPath"`
	RequestProtocol       string            `json:"RequestProtocol"`
	DownstreamStatus      int               `json:"DownstreamStatus"`
	DownstreamContentSize int               `json:"DownstreamContentSize"`
	// Headers might be flattened or in a map depending on config
	// Usually Traefik log doesn't include headers by default unless configured
	// We check for some common flattened keys if they exist in a dynamic map
	RequestHeaders map[string]interface{} `json:"-"` 
}

func (p *TraefikParser) Parse(line string) (*GenericLogEntry, error) {
	var entry TraefikJSONEntry
	
	// We unmarshal into the struct for known fields
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return nil, fmt.Errorf("failed to parse traefik json: %v", err)
	}
	
	// Timestamp
	t, err := time.Parse(time.RFC3339Nano, entry.StartUTC)
	if err != nil {
		// Try without Nano
		t, err = time.Parse(time.RFC3339, entry.StartUTC)
		if err != nil {
			t = time.Now()
		}
	}

	// Try to extract Referer/UA from the raw map if needed
	// For now, simpler implementation assuming default JSON access log without headers
	referer := ""
	userAgent := ""
	
	// Note: Traefik often puts headers in 'request_Referer' field key if "buffering" middleware or specific config used
	// We leave them empty for default config to avoid complex map parsing overhead unless requested

	return &GenericLogEntry{
		Service:       "traefik",
		RemoteIP:      entry.ClientHost,
		RemoteUser:    entry.ClientUsername,
		TimeLocal:     t,
		Method:        entry.RequestMethod,
		Path:          entry.RequestPath,
		Protocol:      entry.RequestProtocol,
		Status:        entry.DownstreamStatus,
		BodyBytesSent: entry.DownstreamContentSize,
		Referer:       referer,
		UserAgent:     userAgent,
	}, nil
}
