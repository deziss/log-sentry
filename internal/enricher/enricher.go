package enricher

import (
	"net"
	"os/user"
	"sync"
)

type Enricher struct {
	userCache map[string]string // UID -> Username
	mu        sync.RWMutex
}

func NewEnricher() *Enricher {
	return &Enricher{
		userCache: make(map[string]string),
	}
}

// ResolveUser translates a UID string to a Username.
// Returns the input UID if resolution fails.
func (e *Enricher) ResolveUser(uid string) string {
	if uid == "-" || uid == "" {
		return "-"
	}

	e.mu.RLock()
	name, exists := e.userCache[uid]
	e.mu.RUnlock()
	if exists {
		return name
	}

	// Lookup
	u, err := user.LookupId(uid)
	if err != nil {
		// Cache failure to avoid repeated lookups?
		// Or just return uid
		return uid
	}

	e.mu.Lock()
	e.userCache[uid] = u.Username
	e.mu.Unlock()
	return u.Username
}

// ClassifyIP returns "internal", "external", or "loopback"
func (e *Enricher) ClassifyIP(ipStr string) string {
	if ipStr == "-" || ipStr == "" {
		return "unknown"
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "invalid"
	}

	if ip.IsLoopback() {
		return "loopback"
	}
	
	if ip.IsPrivate() {
		return "internal"
	}

	return "external"
}
