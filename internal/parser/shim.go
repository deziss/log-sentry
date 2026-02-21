package parser

import (
	"strings"
	"time"
)

// JournalShimParser expects a line formatted like "HOSTNAME PROCESS: MESSAGE"
// It assumes the reader already handled the heavy lifting of JSON decoding.
type JournalShimParser struct{}

func init() { Register("journald", func() LogParser { return &JournalShimParser{} }) }

func (p *JournalShimParser) Parse(line string) (*GenericLogEntry, error) {
	parts := strings.SplitN(line, ": ", 2)
	header := parts[0]
	message := ""
	if len(parts) > 1 {
		message = parts[1]
	}

	headerParts := strings.Fields(header)
	host := "localhost"
	process := "system"
	
	if len(headerParts) >= 2 {
		host = headerParts[0]
		process = headerParts[1]
	}

	return &GenericLogEntry{
		TimeLocal: time.Now(),
		RemoteIP:  host,
		Method:    process,
		Path:      message,
		Status:    200,
		Service:   "journald",
	}, nil
}
