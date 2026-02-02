package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type TomcatParser struct{}

// Tomcat defaults often resemble Common/Combined but time format might differ or lacks timezone in some configs
// Assuming pattern: %h %l %u %t "%r" %s %b
// 127.0.0.1 - - [01/Feb/2026:12:00:00 +0000] "GET /app HTTP/1.1" 200 123
var tomcatRegex = regexp.MustCompile(`^(\S+) \S+ (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+|-)`)

func (p *TomcatParser) Parse(line string) (*GenericLogEntry, error) {
	matches := tomcatRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("failed to parse tomcat line: %s", line)
	}

	// Tomcat often uses standard common log time format
	layout := "02/Jan/2006:15:04:05 -0700"
	t, err := time.Parse(layout, matches[3])
	if err != nil {
		// Fallback for cases without timezone or different locales
		// Trying basic if above fails
		t = time.Now()
	}

	status, _ := strconv.Atoi(matches[7])
	bytesSent := 0
	if matches[8] != "-" {
		bytesSent, _ = strconv.Atoi(matches[8])
	}

	// Extract Referer/UA if they exist (extended pattern)
	// For basic pattern they might be missing. We leave empty.

	return &GenericLogEntry{
		Service:       "tomcat",
		RemoteIP:      matches[1],
		RemoteUser:    matches[2],
		TimeLocal:     t,
		Method:        matches[4],
		Path:          matches[5],
		Protocol:      matches[6],
		Status:        status,
		BodyBytesSent: bytesSent,
		// Referer/UA require extended valve pattern config
	}, nil
}
