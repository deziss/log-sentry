package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type HAProxyParser struct{}

// HAProxy HTTP log format (default)
// Feb  6 12:14:14 localhost haproxy[14389]: 10.0.1.2:33313 [06/Feb/2009:12:14:14.655] frontend backend/srv1 10/0/30/69/109 200 2750 - - ---- 1/1/1/1/0 0/0 "GET /index.html HTTP/1.1"
// Regex groups:
// 1: ClientIP
// 2: Timestamp [06/Feb/2009:12:14:14.655]
// 3: StatusCode
// 4: BytesRead (Response size)
// 5: Method
// 6: Path
// 7: Protocol
var haproxyRegex = regexp.MustCompile(`]: (\S+):\d+ \[([^\]]+)\] \S+ \S+ \S+ (\d+) (\d+) \S+ \S+ \S+ \S+ \S+ "(\S+) (\S+) (\S+)"`)

func (p *HAProxyParser) Parse(line string) (*GenericLogEntry, error) {
	matches := haproxyRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("failed to parse haproxy line: %s", line)
	}

	// Time parsing: 06/Feb/2009:12:14:14.655
	layout := "02/Jan/2006:15:04:05.000"
	t, err := time.Parse(layout, matches[2])
	if err != nil {
		t = time.Now()
	}

	status, _ := strconv.Atoi(matches[3])
	bytesSent, _ := strconv.Atoi(matches[4])

	return &GenericLogEntry{
		Service:       "haproxy",
		RemoteIP:      matches[1],
		RemoteUser:    "-", 
		TimeLocal:     t,
		Method:        matches[5],
		Path:          matches[6],
		Protocol:      matches[7],
		Status:        status,
		BodyBytesSent: bytesSent,
		Referer:       "",
		UserAgent:     "",
	}, nil
}
