package recorder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"log-sentry/internal/storage"
)

// LokiPusher sends log entries to Loki's HTTP push API
type LokiPusher struct {
	url    string
	client *http.Client
}

func NewLokiPusher(baseURL string) *LokiPusher {
	return &LokiPusher{
		url:    baseURL + "/loki/api/v1/push",
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// lokiPushRequest is the Loki push API payload
type lokiPushRequest struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

// Push sends a critical snapshot to Loki (structured JSON)
func (l *LokiPusher) Push(trigger string, snap Snapshot) {
	if l == nil {
		return
	}

	// Build structured JSON log line
	topProcs := make([]map[string]interface{}, 0, 5)
	for i, p := range snap.TopProcesses {
		if i >= 5 {
			break
		}
		topProcs = append(topProcs, map[string]interface{}{
			"name": p.Name, "pid": p.PID, "user": p.User,
			"mem_pct": p.MemPct, "rss_mb": p.MemRSSMB, "oom": p.OOMScore,
		})
	}

	logEntry := map[string]interface{}{
		"type":         "snapshot",
		"trigger":      trigger,
		"cpu_pct":      snap.TotalCPUPct,
		"mem_pct":      snap.TotalMemPct,
		"disk_pct":     snap.DiskPct,
		"total_mem_gb": snap.TotalMemGB,
		"top_procs":    topProcs,
	}

	// Add GPU info
	if len(snap.GPUs) > 0 {
		gpus := make([]map[string]interface{}, len(snap.GPUs))
		for i, g := range snap.GPUs {
			gpus[i] = map[string]interface{}{
				"id": g.ID, "util_pct": g.UtilPct,
				"mem_used_mb": g.MemUsedMB, "mem_total_mb": g.MemTotalMB, "temp_c": g.TempC,
			}
		}
		logEntry["gpus"] = gpus
	}

	l.push("critical", trigger, "snapshot", logEntry)
}

// PushCrashStart sends a crash lifecycle start event to Loki
func (l *LokiPusher) PushCrashStart(event *CrashEvent) {
	if l == nil {
		return
	}
	l.push("warning", event.Trigger, "crash_started", map[string]interface{}{
		"type":       "crash_started",
		"event_id":   event.ID,
		"trigger":    event.Trigger,
		"started_at": event.StartedAt.Format(time.RFC3339),
	})
}

// PushCrashResolved sends a crash lifecycle resolved event to Loki
func (l *LokiPusher) PushCrashResolved(event *CrashEvent) {
	if l == nil {
		return
	}
	duration := event.EndedAt.Sub(event.StartedAt).Seconds()
	l.push("info", event.Trigger, "crash_resolved", map[string]interface{}{
		"type":           "crash_resolved",
		"event_id":       event.ID,
		"trigger":        event.Trigger,
		"severity":       event.Severity,
		"verdict":        event.Verdict,
		"started_at":     event.StartedAt.Format(time.RFC3339),
		"ended_at":       event.EndedAt.Format(time.RFC3339),
		"duration_sec":   duration,
		"snapshot_count": len(event.Snapshots),
	})
}

// PushAttack sends a detected attack event to Loki
func (l *LokiPusher) PushAttack(entry *storage.AttackEntry) {
	if l == nil {
		return
	}
	l.push("warning", "attack", "attack", map[string]interface{}{
		"type":     "attack",
		"service":  entry.Service,
		"attack":   entry.Type,
		"severity": entry.Severity,
		"source":   entry.SourceIP,
		"endpoint": entry.Endpoint,
		"country":  entry.Country,
		"asn":      entry.ASN,
		"details":  entry.Details,
	})
}

// push is the internal method that sends a structured JSON line to Loki
func (l *LokiPusher) push(level, trigger, eventType string, data map[string]interface{}) {
	logLine, err := json.Marshal(data)
	if err != nil {
		log.Printf("Loki: marshal error: %v", err)
		return
	}

	ts := fmt.Sprintf("%d", time.Now().UnixNano())

	payload := lokiPushRequest{
		Streams: []lokiStream{
			{
				Stream: map[string]string{
					"job":        "log-sentry",
					"level":      level,
					"trigger":    trigger,
					"event_type": eventType,
				},
				Values: [][]string{
					{ts, string(logLine)},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Loki: marshal error: %v", err)
		return
	}

	resp, err := l.client.Post(l.url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Loki: push error: %v (url=%s)", err, l.url)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("Loki: push returned %d", resp.StatusCode)
	}
}
