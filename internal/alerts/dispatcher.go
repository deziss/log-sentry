package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type AlertPayload struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Source      string `json:"source"`
	Timestamp   string `json:"timestamp"`
}

type Dispatcher struct {
	WebhookURL string
	client     *http.Client
}

func NewDispatcher(webhookURL string) *Dispatcher {
	return &Dispatcher{
		WebhookURL: webhookURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (d *Dispatcher) Send(title, description, severity, source string) {
	if d.WebhookURL == "" {
		return // Webhooks disabled
	}

	// Wrapper for Slack/Discord standard generic formats if needed,
	// but sending raw JSON is usually enough if it's a custom webhook receiver.
	// For Discord, "content" is usually needed. We'll make it somewhat generic.
	type DiscordPayload struct {
		Content string `json:"content"`
	}

	msg := fmt.Sprintf("[%s] **%s**\n%s\nSource: %s", severity, title, description, source)
	dp := DiscordPayload{Content: msg}

	body, err := json.Marshal(dp)
	if err != nil {
		log.Printf("Failed to marshal alert: %v", err)
		return
	}

	go func() {
		resp, err := d.client.Post(d.WebhookURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Failed to send webhook: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("Webhook returned status %d", resp.StatusCode)
		}
	}()
}
