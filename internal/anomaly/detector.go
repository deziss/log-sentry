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

type IPBucket struct {
	Tokens404 float64
	Tokens500 float64
	LastSeen  time.Time
}

type AnomalyDetector struct {
	mu           sync.Mutex
	Stats        map[string]*IPBucket
	Rate404      float64 // Tokens added per second
	Capacity404  float64 // Max tokens
	Rate500      float64
	Capacity500  float64
	Window       time.Duration // Time before dropping IP from memory
}

func NewAnomalyDetector() *AnomalyDetector {
	ad := &AnomalyDetector{
		Stats:       make(map[string]*IPBucket),
		Rate404:     10.0 / 60.0, // 10 per minute
		Capacity404: 10.0,        // allow burst of 10
		Rate500:     20.0 / 60.0, // 20 per minute
		Capacity500: 20.0,
		Window:      5 * time.Minute, // Cleanup after 5 mins of inactivity
	}
	go ad.cleanupLoop()
	return ad
}

func (ad *AnomalyDetector) cleanupLoop() {
	ticker := time.NewTicker(ad.Window)
	defer ticker.Stop()
	for range ticker.C {
		ad.mu.Lock()
		now := time.Now()
		for ip, bucket := range ad.Stats {
			if now.Sub(bucket.LastSeen) > ad.Window {
				delete(ad.Stats, ip)
			}
		}
		ad.mu.Unlock()
	}
}

// Check returns an anomaly type if detected, or empty string
func (ad *AnomalyDetector) Check(ip string, status int) AnomalyType {
	if status != 404 && (status < 500 || status > 599) {
		return ""
	}

	ad.mu.Lock()
	defer ad.mu.Unlock()

	now := time.Now()
	bucket, exists := ad.Stats[ip]
	
	if !exists {
		bucket = &IPBucket{
			Tokens404: ad.Capacity404, // Start full
			Tokens500: ad.Capacity500,
			LastSeen:  now,
		}
		ad.Stats[ip] = bucket
	} else {
		// Refill tokens based on time passed
		elapsed := now.Sub(bucket.LastSeen).Seconds()
		if elapsed > 0 {
			bucket.Tokens404 += elapsed * ad.Rate404
			if bucket.Tokens404 > ad.Capacity404 {
				bucket.Tokens404 = ad.Capacity404
			}
			
			bucket.Tokens500 += elapsed * ad.Rate500
			if bucket.Tokens500 > ad.Capacity500 {
				bucket.Tokens500 = ad.Capacity500
			}
		}
		bucket.LastSeen = now
	}

	// Consume tokens
	if status == 404 {
		if bucket.Tokens404 >= 1 {
			bucket.Tokens404 -= 1
		} else {
			return Flood404 // Out of tokens = flooded
		}
	} else if status >= 500 && status < 600 {
		if bucket.Tokens500 >= 1 {
			bucket.Tokens500 -= 1
		} else {
			return Burst500 // Out of tokens = burst
		}
	}

	return ""
}
