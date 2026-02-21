package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type EnvoyParser struct{}

func init() { Register("envoy", func() LogParser { return &EnvoyParser{} }) }

// Envoy Default Access Log Format
// [START_TIME] "METHOD PATH PROTOCOL" RESPONSE_CODE RESPONSE_FLAGS BYTES_RECEIVED BYTES_SENT DURATION X-ENVOY-UPSTREAM-SERVICE-TIME "X-FORWARDED-FOR" "USER-AGENT" "REQUEST_ID" "AUTHORITY" "UPSTREAM_HOST"
// [2016-04-15T20:17:00.310Z] "POST /api/v1/locations HTTP/1.1" 204 - 154 0 226 100 "10.0.35.16" "Mozilla/5.0" "v23-234-234" "authority" "10.0.35.16:8080"
var envoyRegex = regexp.MustCompile(`^\[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) \S+ (\d+) (\d+) \S+ \S+ "([^"]*)" "([^"]*)"`)

func (p *EnvoyParser) Parse(line string) (*GenericLogEntry, error) {
	matches := envoyRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("failed to parse envoy line: %s", line)
	}

	// Time parsing: 2016-04-15T20:17:00.310Z
	t, err := time.Parse(time.RFC3339, matches[1])
	if err != nil {
		t = time.Now()
	}

	status, _ := strconv.Atoi(matches[5])
	bytesSent, _ := strconv.Atoi(matches[7])

	return &GenericLogEntry{
		Service:       "envoy",
		RemoteIP:      matches[8], // X-Forwarded-For usually
		RemoteUser:    "-", 
		TimeLocal:     t,
		Method:        matches[2],
		Path:          matches[3],
		Protocol:      matches[4],
		Status:        status,
		BodyBytesSent: bytesSent,
		Referer:       "",
		UserAgent:     matches[9],
	}, nil
}
