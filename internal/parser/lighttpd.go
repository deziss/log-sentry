package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type LighttpdParser struct{}

// Lighttpd default is Common Log Format
// 127.0.0.1 - - [01/Feb/2026:12:00:00 +0000] "GET /index.html HTTP/1.0" 200 1234
var lighttpdRegex = regexp.MustCompile(`^(\S+) \S+ (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+|-)`)

func (p *LighttpdParser) Parse(line string) (*GenericLogEntry, error) {
	matches := lighttpdRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("failed to parse lighttpd line: %s", line)
	}

	layout := "02/Jan/2006:15:04:05 -0700"
	t, err := time.Parse(layout, matches[3])
	if err != nil {
		// Try without TZ or fallback
		t = time.Now()
	}

	status, _ := strconv.Atoi(matches[7])
	bytesSent := 0
	if matches[8] != "-" {
		bytesSent, _ = strconv.Atoi(matches[8])
	}

	return &GenericLogEntry{
		Service:       "lighttpd",
		RemoteIP:      matches[1],
		RemoteUser:    matches[2],
		TimeLocal:     t,
		Method:        matches[4],
		Path:          matches[5],
		Protocol:      matches[6],
		Status:        status,
		BodyBytesSent: bytesSent,
		// Referer/UA not in Common format usually
	}, nil
}
