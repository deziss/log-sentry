package monitor

import (
	"testing"
)

func TestIsBlacklisted(t *testing.T) {
	ps := NewProcessSentinel()

	tests := []struct {
		name     string
		procName string
		want     bool
	}{
		{"Exact match nc", "nc", true},
		{"Exact match nmap", "nmap", true},
		{"Partial match runc", "containerd-shim-runc-v2", false}, 
		{"Partial match nc-openbsd", "nc-openbsd", false}, 
		{"Case insensitive NC", "NC", true},
		{"No match", "systemd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, got := ps.isBlacklisted(tt.procName)
			if got != tt.want {
				t.Errorf("isBlacklisted(%q) = %v; want %v", tt.procName, got, tt.want)
			}
		})
	}
}
