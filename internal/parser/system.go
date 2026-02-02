package parser

import (
	"regexp"
	"strings"
	"time"
)

// Example: Feb  2 15:04:05 myhost sudo: pam_unix(sudo:session): session opened for user root
type SystemParser struct{}

// Sep  9 22:56:22
var syslogTimeRegex = regexp.MustCompile(`^([A-Z][a-z]{2}\s+\d+\s\d{2}:\d{2}:\d{2})\s+([^\s]+)\s+([^:]+):\s+(.*)$`)

func (p *SystemParser) Parse(line string) (*GenericLogEntry, error) {
	// Simple syslog parsing
	// Detect if it matches common pattern
	matches := syslogTimeRegex.FindStringSubmatch(line)
	if len(matches) < 5 {
		// Fallback or error?
		// For now, treat entire line as message if it doesn't match
		return &GenericLogEntry{
			Path:    line, // Use Path to store the "message" for system logs? Or add a Message field?
			// GenericLogEntry is Web-centric. We might need to overload fields or add Message.
			// Let's overload "Path" as "Message/Command" and "Method" as "Process"
			Method:  "SYSTEM", 
			Status:  0,
			Service: "system",
			TimeLocal: time.Now(), // Default to now if parse fails
		}, nil
	}

	// matches[1] = Timestamp (no year!)
	// matches[2] = Host
	// matches[3] = Process (e.g. "sudo", "sshd[123]")
	// matches[4] = Message

	// Parse Time (Warning: Syslog usually lacks year, assumes current year)
	// We'll skip complex date adjust logic for this Lite version and use Now() if needed, 
	// or best effort parse.
	timestamp, err := time.Parse("Jan 2 15:04:05", matches[1])
	if err == nil {
		// Fix year
		currentYear := time.Now().Year()
		timestamp = timestamp.AddDate(currentYear, 0, 0)
	} else {
		timestamp = time.Now()
	}

	process := matches[3]
	message := matches[4]

	entry := &GenericLogEntry{
		TimeLocal: timestamp,
		RemoteIP:  matches[2], // Use Host as RemoteIP/Source
		Method:    process,    // Process Name
		Path:      message,    // The Content
		Service:   "system",
		Status:    200, // Info
	}

	// Simple heuristic for "Status"
	if strings.Contains(strings.ToLower(message), "failed") || strings.Contains(strings.ToLower(message), "error") {
		entry.Status = 500
	} else if strings.Contains(strings.ToLower(message), "sudo") {
		entry.Status = 202 // Accepted/Privilege
	}

	return entry, nil
}
