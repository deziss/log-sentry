package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NginxLogEntry represents a parsed Nginx access log line
type NginxLogEntry struct {
	RemoteIP      string
	RemoteUser    string
	TimeLocal     time.Time
	Request       string
	Method        string
	Path          string
	Protocol      string
// Common/Combined Log Format Regex
// IP - User [Time] "Method Path Protocol" Status Bytes "Referer" "UserAgent"
// Note: Path will include query string if present in the log (e.g., /index.html?foo=bar)
var nginxRegex = regexp.MustCompile(`^(\S+) - (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+) "([^"]*)" "([^"]*)"`)

// ParseNginxLine parses a single line of Nginx access log
func ParseNginxLine(line string) (*NginxLogEntry, error) {
	matches := nginxRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("failed to parse line: %s", line)
	}

	// Time parsing (12/Dec/2023:14:00:00 +0000)
	layout := "02/Jan/2006:15:04:05 -0700"
	t, _ := time.Parse(layout, matches[3])

	status, _ := strconv.Atoi(matches[7])
	bytesSent, _ := strconv.Atoi(matches[8])

	return &NginxLogEntry{
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

// ExtractQueryParams returns a map of query parameters from the path
func (e *NginxLogEntry) ExtractQueryParams() map[string]string {
	params := make(map[string]string)
	if idx := strings.Index(e.Path, "?"); idx != -1 {
		query := e.Path[idx+1:]
		pairs := strings.Split(query, "&")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				params[kv[0]] = kv[1]
			} else if len(kv) == 1 {
				params[kv[0]] = ""
			}
		}
	}
	return params
}
