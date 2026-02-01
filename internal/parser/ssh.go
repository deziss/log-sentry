package parser

import (
	"regexp"
	"strings"
)

type SSHEventType int

const (
	SSHLoginSuccess SSHEventType = iota
	SSHLoginFailed
	SSHDisconnect
	SSHUnknown
)

type SSHLogEntry struct {
	Type       SSHEventType
	User       string
	IP         string
	AuthMethod string
}

// Regex patterns for common SSH logs (OpenSSH)
var (
	// Accepted password for root from 192.168.1.1 port 22 ssh2
	// Accepted publickey for user from 10.0.0.1 port 55555 ssh2: RSA SHA256:...
	sshAcceptedRegex = regexp.MustCompile(`Accepted (\S+) for (\S+) from (\S+)`)

	// Failed password for invalid user admin from 192.168.1.5 port 22 ssh2
	// Failed password for root from 192.168.1.5 port 22 ssh2
	sshFailedRegex = regexp.MustCompile(`Failed (\S+) for (?:invalid user )?(\S+) from (\S+)`)

	// Disconnected from user root 192.168.1.1 port 22
	// Disconnected from 1.2.3.4 port 22
	sshDisconnectRegex = regexp.MustCompile(`Disconnected from (?:user (\S+) )?(\S+)`)
)

func ParseSSHLine(line string) (*SSHLogEntry, error) {
	// Check for Accepted (Success)
	if matches := sshAcceptedRegex.FindStringSubmatch(line); matches != nil {
		return &SSHLogEntry{
			Type:       SSHLoginSuccess,
			AuthMethod: matches[1],
			User:       matches[2],
			IP:         matches[3],
		}, nil
	}

	// Check for Failed
	if matches := sshFailedRegex.FindStringSubmatch(line); matches != nil {
		return &SSHLogEntry{
			Type:       SSHLoginFailed,
			AuthMethod: matches[1],
			User:       matches[2],
			IP:         matches[3],
		}, nil
	}

	// Check for Disconnect (often indicates session end)
	// Note: Disconnect logs vary significantly, this is a best effort
	if strings.Contains(line, "sshd") && strings.Contains(line, "Disconnected from") {
		// Try regex
		// if matches := sshDisconnectRegex.FindStringSubmatch(line); matches != nil {
		// 	user := ""
		// 	// if capture group 1 is empty, it might just be "Disconnected from IP"
		// 	// This part is tricky because "Disconnected from 1.2.3.4" doesn't give us the user unless we track state or look at earlier logs.
		// 	// For now we might just count disconnects globally or by IP.
		//  return &SSHLogEntry{...}
		// }
		// Simplified return for disconnect
		return &SSHLogEntry{Type: SSHDisconnect}, nil
	}

	return nil, nil // Not a relevant line
}
