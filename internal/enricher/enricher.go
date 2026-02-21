package enricher

import (
	"net"
	"os/user"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

type Enricher struct {
	userCache map[string]string // UID -> Username
	mu        sync.RWMutex
	cityDB    *geoip2.Reader
	asnDB     *geoip2.Reader
}

func NewEnricher(cityDBPath, asnDBPath string) *Enricher {
	e := &Enricher{
		userCache: make(map[string]string),
	}
	
	if cityDBPath != "" {
		db, err := geoip2.Open(cityDBPath)
		if err == nil {
			e.cityDB = db
		}
	}
	if asnDBPath != "" {
		db, err := geoip2.Open(asnDBPath)
		if err == nil {
			e.asnDB = db
		}
	}
	
	return e
}

// Close should be called on shutdown
func (e *Enricher) Close() {
	if e.cityDB != nil {
		e.cityDB.Close()
	}
	if e.asnDB != nil {
		e.asnDB.Close()
	}
}

// GeoEnrich returns Country ISO code and ASN name if available
func (e *Enricher) GeoEnrich(ipStr string) (country string, asn string) {
	if ipStr == "-" || ipStr == "" {
		return "Unknown", "Unknown"
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "Invalid", "Invalid"
	}

	country = "Unknown"
	asn = "Unknown"

	if e.cityDB != nil {
		record, err := e.cityDB.Country(ip)
		if err == nil && record.Country.IsoCode != "" {
			country = record.Country.IsoCode
		}
	}

	if e.asnDB != nil {
		record, err := e.asnDB.ASN(ip)
		if err == nil && record.AutonomousSystemOrganization != "" {
			asn = record.AutonomousSystemOrganization
		}
	}

	return country, asn
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
