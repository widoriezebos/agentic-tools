package supervise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// The watcher's per-interval census pass. Each interval it computes the
// supervision fingerprint, scans the live process table into a verdict, and
// publishes that verdict to last-census.json — the file the owner's arming
// reads to confirm a fresh successful census. A fingerprint that cannot be
// computed still publishes a verdict, labelled CENSUS-FAILED, so a stuck
// fingerprint is visible to the owner rather than silently stale.

// censusVerdictFile is the published verdict's name inside the supervision dir.
const censusVerdictFile = "last-census.json"

// WatcherConfig drives one census pass. The fingerprint and scan are supplied
// as functions so the production path binds the real, process-touching census
// while tests bind deterministic stubs.
type WatcherConfig struct {
	// SupervisionDir is where the verdict is published.
	SupervisionDir string
	// Interval is the observation interval in seconds, stamped into the verdict
	// and the basis of the duration budget.
	Interval int
	// IntervalMS is the interval in milliseconds the scan duration is measured
	// against (Interval*1000 in production, an override for accelerated tests).
	IntervalMS int
	// BudgetPercent is census.max-interval-share-percent (1..100): a scan
	// slower than this share of the interval is warned as CENSUS-SLOW.
	BudgetPercent int
	// Fingerprint computes the supervision fingerprint, or an error when its
	// inputs cannot be read.
	Fingerprint func() (string, error)
	// Census scans into a verdict stamped with the fingerprint.
	Census func(fingerprint string, now time.Time) (census.Verdict, error)
	// Now is the wall clock (injectable for tests).
	Now func() time.Time
	// Warn receives CENSUS-SLOW diagnostics; stderr in production, nil to drop.
	Warn func(string)
}

// WatcherPass runs one census pass and publishes its verdict. It returns an
// error only when the verdict cannot be written; a failed fingerprint or scan
// is itself recorded as a CENSUS-FAILED verdict, not a returned error, so the
// caller keeps looping.
func (cfg WatcherConfig) WatcherPass() error {
	started := time.Now()
	now := cfg.Now()

	fingerprint, err := cfg.Fingerprint()
	if err != nil {
		verdict := censusFailedVerdict("FINGERPRINT-FAILED", cfg.Interval, "fingerprint:"+err.Error(), now)
		verdict.DurationMs = time.Since(started).Milliseconds()
		return cfg.publish(verdict)
	}

	verdict, err := cfg.Census(fingerprint, now)
	if err != nil {
		// The scan itself failed outright: publish a fresh CENSUS-FAILED verdict
		// under the real fingerprint rather than leaving a stale success behind.
		verdict = censusFailedVerdict(fingerprint, cfg.Interval, "census:"+err.Error(), now)
	}
	verdict.DurationMs = time.Since(started).Milliseconds()
	if err := cfg.publish(verdict); err != nil {
		return err
	}
	cfg.warnIfSlow(verdict.DurationMs)
	return nil
}

func (cfg WatcherConfig) publish(verdict census.Verdict) error {
	path := filepath.Join(cfg.SupervisionDir, censusVerdictFile)
	// The scanSeq counter lives in the published file, not in this process, so
	// it stays monotonic across watcher relaunches and the repeated one-shot
	// watcher-pass invocations the fixtures drive: seed from the prior verdict
	// and stamp prior+1. It advances only when the atomicWrite below succeeds —
	// a failed publish leaves the old file in place, so the next pass reads the
	// same prior value and retries the same next value (no gap a stale in-flight
	// pass could exploit). This is the census actor's "attempt" marker for
	// attempt-based fixture patience (records/patience/patience-attempts.md). Single-writer
	// is guaranteed by the census-writer lock the watcher-pass verb holds.
	verdict.ScanSeq = lastPublishedScanSeq(path) + 1
	encoded, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		return fmt.Errorf("encode census verdict: %w", err)
	}
	return atomicWrite(path, append(encoded, '\n'))
}

// lastPublishedScanSeq returns the scanSeq of the verdict currently at path, or
// 0 when the file is absent or unreadable (the first publish then stamps 1).
func lastPublishedScanSeq(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var prev struct {
		ScanSeq int64 `json:"scanSeq"`
	}
	if err := json.Unmarshal(data, &prev); err != nil || prev.ScanSeq < 0 {
		// A negative (corrupt) marker is garbage, not a count: restart from 0 so
		// the sequence climbs back into the positive integers readers expect.
		return 0
	}
	return prev.ScanSeq
}

// warnIfSlow emits the CENSUS-SLOW warning the census budget defines: a scan
// slower than the whole interval is the serious defect (it cannot keep up);
// slower than the configured budget share is the softer warning.
func (cfg WatcherConfig) warnIfSlow(durationMs int64) {
	if cfg.Warn == nil {
		return
	}
	intervalMs := int64(cfg.IntervalMS)
	budgetMs := intervalMs * int64(cfg.BudgetPercent) / 100
	switch {
	case durationMs > intervalMs:
		cfg.Warn(fmt.Sprintf("WARNING CENSUS-SLOW durationMs=%d intervalMs=%d budgetPercent=%d budgetMs=%d defect=scan-exceeds-interval",
			durationMs, intervalMs, cfg.BudgetPercent, budgetMs))
	case durationMs > budgetMs:
		cfg.Warn(fmt.Sprintf("WARNING CENSUS-SLOW durationMs=%d intervalMs=%d budgetPercent=%d budgetMs=%d defect=none",
			durationMs, intervalMs, cfg.BudgetPercent, budgetMs))
	}
}

// censusFailedVerdict builds the well-formed CENSUS-FAILED verdict published
// when the fingerprint or scan cannot produce a real one, so a reader still
// gets a schema-valid, timestamped result carrying the failure reason.
func censusFailedVerdict(fingerprint string, interval int, reason string, now time.Time) census.Verdict {
	completed := now.UTC()
	return census.Verdict{
		SchemaVersion:    2,
		Writer:           "watch-background-jobs.sh",
		Verdict:          "CENSUS-FAILED",
		CompletedAt:      completed.Format(isoSecond),
		CompletedAtEpoch: completed.Unix(),
		IntervalSec:      interval,
		Fingerprint:      fingerprint,
		Generation:       nil,
		StateDigest:      nil,
		Counts:           map[string]int{"CUSTODY": 0, "ANNOUNCED": 0, "UNTRACKED": 0},
		Inventory:        []census.InventoryItem{},
		Diagnostics:      []string{},
		Errors:           []string{reason},
	}
}
