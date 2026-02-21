package recorder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
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

// Push sends a critical snapshot to Loki
func (l *LokiPusher) Push(trigger string, snap Snapshot) {
	if l == nil {
		return
	}

	// Build log line with structured process data
	topProcs := ""
	for i, p := range snap.TopProcesses {
		if i >= 5 {
			break
		}
		topProcs += fmt.Sprintf(" [%s pid=%d user=%s mem=%.1f%% rss=%.0fMB oom=%d]",
			p.Name, p.PID, p.User, p.MemPct, p.MemRSSMB, p.OOMScore)
	}

	logLine := fmt.Sprintf("CRITICAL trigger=%s cpu=%.1f%% mem=%.1f%% disk=%.1f%% totalMemGB=%.1f%s",
		trigger, snap.TotalCPUPct, snap.TotalMemPct, snap.DiskPct, snap.TotalMemGB, topProcs)

	// Add GPU info
	for _, g := range snap.GPUs {
		logLine += fmt.Sprintf(" gpu%d_util=%d%% gpu%d_mem=%d/%dMB gpu%d_temp=%dC",
			g.ID, g.UtilPct, g.ID, g.MemUsedMB, g.MemTotalMB, g.ID, g.TempC)
	}

	ts := fmt.Sprintf("%d", time.Now().UnixNano())

	payload := lokiPushRequest{
		Streams: []lokiStream{
			{
				Stream: map[string]string{
					"job":     "log-sentry",
					"level":   "critical",
					"trigger": trigger,
				},
				Values: [][]string{
					{ts, logLine},
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
