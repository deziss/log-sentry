package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type ApacheParser struct{}

// Apache Combined Log Format is very similar to Nginx default
// %h %l %u %t \"%r\" %>s %b \"%{Referer}i\" \"%{User-Agent}i\"
var apacheRegex = regexp.MustCompile(`^(\S+) \S+ (\S+) \[([^\]]+)\] "(\S+) (\S+) (\S+)" (\d+) (\d+|-) "([^"]*)" "([^"]*)"`)

func (p *ApacheParser) Parse(line string) (*GenericLogEntry, error) {
	matches := apacheRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("failed to parse apache line: %s", line)
	}

	layout := "02/Jan/2006:15:04:05 -0700"
	t, _ := time.Parse(layout, matches[3])

	status, _ := strconv.Atoi(matches[7])
	
	bytesSent := 0
	if matches[8] != "-" {
		bytesSent, _ = strconv.Atoi(matches[8])
	}

	return &GenericLogEntry{
		Service:       "apache",
		RemoteIP:      matches[1],
		RemoteUser:    matches[2], // often "-"
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
