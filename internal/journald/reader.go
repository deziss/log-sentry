package journald

import (
	"bufio"
	"encoding/json"
	"log"
	"os/exec"

	"log-sentry/internal/parser"
	"log-sentry/internal/worker"
)

type JournalEntry struct {
	Timestamp string `json:"__REALTIME_TIMESTAMP"`
	Host      string `json:"_HOSTNAME"`
	Message   string `json:"MESSAGE"`
	Command   string `json:"_COMM"`
	PID       string `json:"_PID"`
	UID       string `json:"_UID"`
}

// StartReader executes 'journalctl -f -o json' and feeds it into the Worker Pool
// Note: This requires the container to have access to the host's journal or socket.
func StartReader(wp *worker.Pool) {
	cmd := exec.Command("journalctl", "-f", "-o", "json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Journald: Failed to get stdout: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Journald: Failed to start journalctl (is it installed/accessible?): %v", err)
		return
	}

	log.Println("Journald: Started monitoring via journalctl...")
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		
		var jEntry JournalEntry
		if err := json.Unmarshal([]byte(line), &jEntry); err != nil {
			continue
		}

		// Convert to Worker Job
		// We need a specialized "System/Journal" parser or reuse Generic?
		// We'll construct a line compatible with our SystemParser or just parse it here manually
		// and submit a "pre-parsed" job?
		// actually logsentry expects raw lines usually, but here we have JSON.
		// Let's format it back to a standard string for the SystemParser to handle easily?
		// OR: We teach Worker/Job to accept pre-parsed entries?
		// For consistency, let's reconstruct a syslog-like line:
		// "Feb 2 15:04:05 host command: message"
		// This allows us to reuse the SystemParser logic (which we just built).
		
        // Simple approximation of timestamp
		syslogLine := jEntry.Host + " " + jEntry.Command + ": " + jEntry.Message
		
		wp.Submit(worker.Job{
			ServiceName: "journald",
			LogPath:     "journald",
			Line:        syslogLine,
			Parser:      &parser.JournalShimParser{},
		})
	}
    
    if err := cmd.Wait(); err != nil {
        log.Printf("Journald: command exited: %v", err)
    }
}
