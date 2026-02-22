package storage

import "time"

// ── Data Types ───────────────────────────────────────────────────

// CrashSummary is a lightweight view of a CrashEvent (no snapshots).
type CrashSummary struct {
	ID            string `json:"id"`
	StartedAt     string `json:"started_at"`
	EndedAt       string `json:"ended_at"`
	Trigger       string `json:"trigger"`
	Verdict       string `json:"verdict"`
	Severity      string `json:"severity"`
	Resolved      bool   `json:"resolved"`
	SnapshotCount int    `json:"snapshot_count"`
}

// AttackEntry records a single detected attack event.
type AttackEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Type      string `json:"type"`
	Severity  string `json:"severity"`
	SourceIP  string `json:"source_ip"`
	Endpoint  string `json:"endpoint"`
	Country   string `json:"country,omitempty"`
	ASN       string `json:"asn,omitempty"`
	Network   string `json:"network_type,omitempty"`
	Details   string `json:"details,omitempty"`
}

// AggregatedStats holds computed statistics.
type AggregatedStats struct {
	TotalCrashes     int     `json:"total_crashes"`
	ActiveCrashes    int     `json:"active_crashes"`
	AvgDurationSec   float64 `json:"avg_duration_sec"`
	TotalAttacks     int     `json:"total_attacks"`
	TopAttackType    string  `json:"top_attack_type"`
	TopAttackedSvc   string  `json:"top_attacked_service"`
	CriticalCount    int     `json:"critical_count"`
	HighCount        int     `json:"high_count"`
	MediumCount      int     `json:"medium_count"`
}

// ListOpts defines pagination and filtering for list queries.
type ListOpts struct {
	Page     int    // 1-indexed
	PageSize int    // default 20
	Severity string // filter by severity (empty = all)
	Trigger  string // filter by trigger (empty = all)
	Service  string // filter by service (empty = all)
	Since    time.Time
	Until    time.Time
}

// ListResult wraps a paginated result set.
type ListResult[T any] struct {
	Items      []T `json:"items"`
	Total      int `json:"total"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalPages int `json:"total_pages"`
}

// ── Store Interface ──────────────────────────────────────────────

// Store is the persistence interface for Log Sentry data.
// Implementations must be goroutine-safe.
type Store interface {
	// Crash events
	SaveCrashEvent(data []byte, id, severity, trigger string, startedAt time.Time, resolved bool, snapshotCount int) error
	GetCrashEvent(id string) ([]byte, error)
	ListCrashEvents(opts ListOpts) (*ListResult[CrashSummary], error)
	DeleteOldCrashEvents(olderThan time.Duration) (int, error)

	// Attack log
	SaveAttack(entry *AttackEntry) error
	ListAttacks(opts ListOpts) (*ListResult[AttackEntry], error)
	DeleteOldAttacks(olderThan time.Duration) (int, error)

	// Stats
	GetStats() (*AggregatedStats, error)

	// Lifecycle
	Close() error
}
