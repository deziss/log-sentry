package enricher

import (
	"net"
	"os/user"
	"strings"
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

// ClassifyUserAgent returns "bot", "tool", "browser", "mobile", or "other"
func (e *Enricher) ClassifyUserAgent(ua string) string {
	if ua == "" || ua == "-" {
		return "unknown"
	}
	
	lowerUA := strings.ToLower(ua)
	
	// Bots
	if strings.Contains(lowerUA, "bot") || 
	   strings.Contains(lowerUA, "crawl") || 
	   strings.Contains(lowerUA, "slurp") || 
	   strings.Contains(lowerUA, "spider") ||
	   strings.Contains(lowerUA, "mediapartners") {
		return "bot"
	}
	
	// Tools
	if strings.Contains(lowerUA, "curl") || 
	   strings.Contains(lowerUA, "wget") || 
	   strings.Contains(lowerUA, "python") || 
	   strings.Contains(lowerUA, "go-http") ||
	   strings.Contains(lowerUA, "postman") ||
	   strings.Contains(lowerUA, "crowdsec") {
		return "tool"
	}
	
	// Mobile (check before generic browser)
	if strings.Contains(lowerUA, "mobile") || 
	   strings.Contains(lowerUA, "android") || 
	   strings.Contains(lowerUA, "iphone") {
		return "mobile"
	}
	
	// Browsers
	if strings.Contains(lowerUA, "mozilla") || 
	   strings.Contains(lowerUA, "chrome") || 
	   strings.Contains(lowerUA, "safari") {
		return "browser"
	}
	
	return "other"
}
