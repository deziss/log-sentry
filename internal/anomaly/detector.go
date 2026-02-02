package anomaly

import (
	"sync"
	"time"
)

type AnomalyType string

const (
	Flood404 AnomalyType = "404_flood"
	Burst500 AnomalyType = "500_burst"
)

type IPStats struct {
	Count404   int
	Count500   int
	LastSeen   time.Time
}

type AnomalyDetector struct {
	mu           sync.Mutex
	Stats        map[string]*IPStats
	Threshold404 int
	Threshold500 int
	Window       time.Duration
}

func NewAnomalyDetector() *AnomalyDetector {
	ad := &AnomalyDetector{
		Stats:        make(map[string]*IPStats),
		Threshold404: 10, // 10 404s per minute is suspicious
		Threshold500: 20, // 20 500s per minute is definitely suspicious
		Window:       1 * time.Minute,
	}
	go ad.cleanupLoop()
	return ad
}

func (ad *AnomalyDetector) cleanupLoop() {
	ticker := time.NewTicker(ad.Window) // Cleanup every window
	defer ticker.Stop()
	for range ticker.C {
		ad.mu.Lock()
		now := time.Now()
		for ip, stat := range ad.Stats {
			if now.Sub(stat.LastSeen) > ad.Window {
				delete(ad.Stats, ip)
			} else {
				// Reset counts for the new window? 
				// Simple sliding window approximation: just clear counts periodically
				// Ideally we'd use a real sliding window, but this is "Lite"
				stat.Count404 = 0
				stat.Count500 = 0
			}
		}
		ad.mu.Unlock()
	}
}

// Check returns an anomaly type if detected, or empty string
func (ad *AnomalyDetector) Check(ip string, status int) AnomalyType {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	stat, exists := ad.Stats[ip]
	if !exists {
		stat = &IPStats{}
		ad.Stats[ip] = stat
	}
	stat.LastSeen = time.Now()

	if status == 404 {
		stat.Count404++
		if stat.Count404 > ad.Threshold404 {
			// Trigger only once per window per threshold crossing?
			// Or every time? Let's dampen it by resetting or modulo?
			// For simplicity, we return it every time it's above threshold.
			// The counters generally reset every minute.
			// To avoid metric explosion, maybe we just return it. The Collector controls increment.
			return Flood404
		}
	}

	if status >= 500 && status < 600 {
		stat.Count500++
		if stat.Count500 > ad.Threshold500 {
			return Burst500
		}
	}

	return ""
}
