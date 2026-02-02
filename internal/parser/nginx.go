package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// NginxParser implements LogParser interface
type NginxParser struct{}

// Common/Combined Log Format Regex
// IP - User [Time] "Method Path Protocol" Status Bytes "Referer" "UserAgent"
var nginxRegex = regexp.MustCompile(`^(\S+) - (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+) "([^"]*)" "([^"]*)"`)

// Parse implements LogParser
func (p *NginxParser) Parse(line string) (*GenericLogEntry, error) {
	matches := nginxRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("failed to parse line: %s", line)
	}

	// Time parsing (12/Dec/2023:14:00:00 +0000)
	layout := "02/Jan/2006:15:04:05 -0700"
	t, _ := time.Parse(layout, matches[3])

	status, _ := strconv.Atoi(matches[7])
	bytesSent, _ := strconv.Atoi(matches[8])

	return &GenericLogEntry{
		Service:       "nginx",
		RemoteIP:      matches[1],
		RemoteUser:    matches[2],
		TimeLocal:     t,
		Method:        matches[4],
		Path:          matches[5],
		Protocol:      matches[6],
		Status:        status,
		BodyBytesSent: bytesSent,
		Referer:       matches[9],
		UserAgent:     matches[10],
	}, nil
}
