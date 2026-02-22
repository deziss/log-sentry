package storage

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketCrashEvents = []byte("crash_events")
	bucketCrashMeta   = []byte("crash_meta") // lightweight summaries
	bucketAttacks     = []byte("attacks")
	bucketAppState    = []byte("app_state")
)

// BoltStore implements the Store interface using bbolt.
type BoltStore struct {
	db *bolt.DB
}

// NewBoltStore opens (or creates) a bbolt database at the given path.
func NewBoltStore(path string) (*BoltStore, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt db: %w", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketCrashEvents, bucketCrashMeta, bucketAttacks, bucketAppState} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create buckets: %w", err)
	}

	log.Printf("BoltStore: opened %s", path)
	store := &BoltStore{db: db}

	// Unclean shutdown detection
	err = store.checkAndMarkRunning()
	if err != nil {
		log.Printf("BoltStore: warning, failed to process app state: %v", err)
	}

	return store, nil
}

// ── App State (Heartbeat) ────────────────────────────────────────

func (s *BoltStore) checkAndMarkRunning() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAppState)
		state := b.Get([]byte("status"))

		if state != nil && string(state) == "running" {
			// Unclean shutdown detected! The app died without calling MarkStopped
			log.Println("🚨 CRITICAL: Unclean shutdown detected. The application was terminated forcefully or lost power.")

			id := generateStoreID()
			now := time.Now()
			
			// Create a synthetic crash event
			summary := CrashSummary{
				ID:            id,
				StartedAt:     now.Format(time.RFC3339),
				EndedAt:       now.Format(time.RFC3339),
				Trigger:       "Forceful Shutdown / Power Loss",
				Verdict:       "The application was forcefully terminated (e.g., SIGKILL, OOM Killer, or sudden power loss) without a clean exit.",
				Severity:      "critical",
				Resolved:      true, // It's a past event
				SnapshotCount: 0,
			}
			
			meta, _ := json.Marshal(summary)
			evData := []byte(`{"error":"No snapshots available. System terminated abruptly."}`)
			
			tx.Bucket(bucketCrashEvents).Put([]byte(id), evData)
			tx.Bucket(bucketCrashMeta).Put([]byte(id), meta)
			log.Printf("BoltStore: Recorded synthetic crash event %s for forceful shutdown.", id)
		}

		// Mark as running for the current session
		return b.Put([]byte("status"), []byte("running"))
	})
}

// MarkStopped should be called during a graceful shutdown
func (s *BoltStore) MarkStopped() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		log.Println("BoltStore: marking state as stopped (clean entry)")
		return tx.Bucket(bucketAppState).Put([]byte("status"), []byte("stopped"))
	})
}

// ── Crash Events ─────────────────────────────────────────────────

func (s *BoltStore) SaveCrashEvent(data []byte, id, severity, trigger string, startedAt time.Time, resolved bool, snapshotCount int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		// Store the full event (raw JSON from recorder)
		if err := tx.Bucket(bucketCrashEvents).Put([]byte(id), data); err != nil {
			return err
		}

		// Store a lightweight summary for listing
		summary := CrashSummary{
			ID:            id,
			StartedAt:     startedAt.Format(time.RFC3339),
			Severity:      severity,
			Trigger:       trigger,
			Resolved:      resolved,
			SnapshotCount: snapshotCount,
		}
		meta, err := json.Marshal(summary)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketCrashMeta).Put([]byte(id), meta)
	})
}

func (s *BoltStore) GetCrashEvent(id string) ([]byte, error) {
	var data []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(bucketCrashEvents).Get([]byte(id))
		if v == nil {
			return fmt.Errorf("crash event %s not found", id)
		}
		data = make([]byte, len(v))
		copy(data, v)
		return nil
	})
	return data, err
}

func (s *BoltStore) ListCrashEvents(opts ListOpts) (*ListResult[CrashSummary], error) {
	opts = normalizeOpts(opts)

	var all []CrashSummary
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketCrashMeta).ForEach(func(_, v []byte) error {
			var cs CrashSummary
			if err := json.Unmarshal(v, &cs); err != nil {
				return nil // skip corrupt entries
			}
			if opts.Severity != "" && !strings.EqualFold(cs.Severity, opts.Severity) {
				return nil
			}
			if opts.Trigger != "" && !strings.EqualFold(cs.Trigger, opts.Trigger) {
				return nil
			}
			if !opts.Since.IsZero() {
				t, _ := time.Parse(time.RFC3339, cs.StartedAt)
				if t.Before(opts.Since) {
					return nil
				}
			}
			if !opts.Until.IsZero() {
				t, _ := time.Parse(time.RFC3339, cs.StartedAt)
				if t.After(opts.Until) {
					return nil
				}
			}
			all = append(all, cs)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	// Sort newest first
	sort.Slice(all, func(i, j int) bool {
		return all[i].StartedAt > all[j].StartedAt
	})

	return paginate(all, opts), nil
}

func (s *BoltStore) DeleteOldCrashEvents(olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	deleted := 0

	err := s.db.Update(func(tx *bolt.Tx) error {
		metaBucket := tx.Bucket(bucketCrashMeta)
		eventBucket := tx.Bucket(bucketCrashEvents)

		var toDelete [][]byte
		metaBucket.ForEach(func(k, v []byte) error {
			var cs CrashSummary
			if err := json.Unmarshal(v, &cs); err != nil {
				return nil
			}
			t, _ := time.Parse(time.RFC3339, cs.StartedAt)
			if !t.IsZero() && t.Before(cutoff) {
				key := make([]byte, len(k))
				copy(key, k)
				toDelete = append(toDelete, key)
			}
			return nil
		})

		for _, k := range toDelete {
			metaBucket.Delete(k)
			eventBucket.Delete(k)
			deleted++
		}
		return nil
	})

	if deleted > 0 {
		log.Printf("BoltStore: pruned %d crash events older than %s", deleted, olderThan)
	}
	return deleted, err
}

// ── Attacks ──────────────────────────────────────────────────────

func (s *BoltStore) SaveAttack(entry *AttackEntry) error {
	if entry.ID == "" {
		entry.ID = generateStoreID()
	}
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().Format(time.RFC3339)
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		// Use timestamp+id as key for chronological ordering
		key := []byte(entry.Timestamp + "_" + entry.ID)
		return tx.Bucket(bucketAttacks).Put(key, data)
	})
}

func (s *BoltStore) ListAttacks(opts ListOpts) (*ListResult[AttackEntry], error) {
	opts = normalizeOpts(opts)

	var all []AttackEntry
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketAttacks).ForEach(func(_, v []byte) error {
			var ae AttackEntry
			if err := json.Unmarshal(v, &ae); err != nil {
				return nil
			}
			if opts.Severity != "" && !strings.EqualFold(ae.Severity, opts.Severity) {
				return nil
			}
			if opts.Service != "" && !strings.EqualFold(ae.Service, opts.Service) {
				return nil
			}
			if !opts.Since.IsZero() {
				t, _ := time.Parse(time.RFC3339, ae.Timestamp)
				if t.Before(opts.Since) {
					return nil
				}
			}
			if !opts.Until.IsZero() {
				t, _ := time.Parse(time.RFC3339, ae.Timestamp)
				if t.After(opts.Until) {
					return nil
				}
			}
			all = append(all, ae)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	// Sort newest first
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp > all[j].Timestamp
	})

	return paginate(all, opts), nil
}

func (s *BoltStore) DeleteOldAttacks(olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)
	deleted := 0

	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketAttacks)
		var toDelete [][]byte
		bucket.ForEach(func(k, v []byte) error {
			var ae AttackEntry
			if err := json.Unmarshal(v, &ae); err != nil {
				return nil
			}
			t, _ := time.Parse(time.RFC3339, ae.Timestamp)
			if !t.IsZero() && t.Before(cutoff) {
				key := make([]byte, len(k))
				copy(key, k)
				toDelete = append(toDelete, key)
			}
			return nil
		})
		for _, k := range toDelete {
			bucket.Delete(k)
			deleted++
		}
		return nil
	})

	if deleted > 0 {
		log.Printf("BoltStore: pruned %d attack entries older than %s", deleted, olderThan)
	}
	return deleted, err
}

// ── Stats ────────────────────────────────────────────────────────

func (s *BoltStore) GetStats() (*AggregatedStats, error) {
	stats := &AggregatedStats{}
	typeCounts := map[string]int{}
	svcCounts := map[string]int{}
	var totalDuration float64

	err := s.db.View(func(tx *bolt.Tx) error {
		// Crash stats
		tx.Bucket(bucketCrashMeta).ForEach(func(_, v []byte) error {
			var cs CrashSummary
			if err := json.Unmarshal(v, &cs); err != nil {
				return nil
			}
			stats.TotalCrashes++
			if !cs.Resolved {
				stats.ActiveCrashes++
			}
			switch strings.ToLower(cs.Severity) {
			case "critical":
				stats.CriticalCount++
			case "high":
				stats.HighCount++
			case "medium":
				stats.MediumCount++
			}
			// Compute duration
			start, _ := time.Parse(time.RFC3339, cs.StartedAt)
			end, _ := time.Parse(time.RFC3339, cs.EndedAt)
			if !start.IsZero() && !end.IsZero() {
				totalDuration += end.Sub(start).Seconds()
			}
			return nil
		})

		// Attack stats
		tx.Bucket(bucketAttacks).ForEach(func(_, v []byte) error {
			var ae AttackEntry
			if err := json.Unmarshal(v, &ae); err != nil {
				return nil
			}
			stats.TotalAttacks++
			typeCounts[ae.Type]++
			if ae.Service != "" {
				svcCounts[ae.Service]++
			}
			return nil
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	if stats.TotalCrashes > 0 {
		stats.AvgDurationSec = totalDuration / float64(stats.TotalCrashes)
	}

	// Find top attack type
	maxCount := 0
	for t, c := range typeCounts {
		if c > maxCount {
			maxCount = c
			stats.TopAttackType = t
		}
	}
	// Find top attacked service
	maxCount = 0
	for svc, c := range svcCounts {
		if c > maxCount {
			maxCount = c
			stats.TopAttackedSvc = svc
		}
	}

	return stats, nil
}

// ── Lifecycle ────────────────────────────────────────────────────

func (s *BoltStore) Close() error {
	log.Println("BoltStore: closing database")
	s.MarkStopped()
	return s.db.Close()
}

// ── Helpers ──────────────────────────────────────────────────────

func normalizeOpts(opts ListOpts) ListOpts {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize < 1 || opts.PageSize > 100 {
		opts.PageSize = 20
	}
	return opts
}

func paginate[T any](all []T, opts ListOpts) *ListResult[T] {
	total := len(all)
	totalPages := (total + opts.PageSize - 1) / opts.PageSize
	if totalPages < 1 {
		totalPages = 1
	}

	start := (opts.Page - 1) * opts.PageSize
	if start >= total {
		return &ListResult[T]{Items: []T{}, Total: total, Page: opts.Page, PageSize: opts.PageSize, TotalPages: totalPages}
	}
	end := start + opts.PageSize
	if end > total {
		end = total
	}

	return &ListResult[T]{
		Items:      all[start:end],
		Total:      total,
		Page:       opts.Page,
		PageSize:   opts.PageSize,
		TotalPages: totalPages,
	}
}

func generateStoreID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
