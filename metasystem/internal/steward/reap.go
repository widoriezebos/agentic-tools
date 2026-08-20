package steward

// The reaper: each tick closes what the last revival left open. A
// stamped continuation whose job record ended is reaped with its
// outcome; a consumed-but-unstamped intent is the crash boundary,
// reconciled as a notified unknown; a stamped one whose job record
// never appeared is an incident, not a guess. Every close frees the
// one-active-continuation guard and tells the operator.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ReapReport is one closed continuation, for the tick's output.
type ReapReport struct {
	Nonce   string `json:"nonce"`
	JobId   string `json:"jobId"`
	Outcome string `json:"outcome"`
}

// ReapContinuations closes every active consumed intent whose story
// has ended, and reports the ones still running untouched.
func ReapContinuations(repoRoot string) ([]ReapReport, error) {
	active, err := ConsumedActive(repoRoot)
	if err != nil {
		return nil, err
	}
	var reports []ReapReport
	for _, it := range active {
		outcome, done := continuationOutcome(repoRoot, it)
		if !done {
			continue
		}
		it.ReapedAt = time.Now().UTC().Format(time.RFC3339)
		it.Outcome = outcome
		data, err := json.MarshalIndent(it, "", "  ")
		if err != nil {
			return reports, err
		}
		path := filepath.Join(consumedDir(repoRoot), it.Nonce+".json")
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return reports, err
		}
		if err := os.Rename(tmp, path); err != nil {
			return reports, err
		}
		if err := QueueNotification(repoRoot, PendingNotification{
			Nonce:   it.Nonce + "-reap",
			Message: fmt.Sprintf("steward: continuation %s closed — %s", it.JobId, outcome),
		}); err != nil {
			return reports, err
		}
		reports = append(reports, ReapReport{Nonce: it.Nonce, JobId: it.JobId, Outcome: outcome})
	}
	return reports, nil
}

// continuationOutcome decides whether one continuation's story ended
// and how. Still-running is the only not-done answer.
func continuationOutcome(repoRoot string, it Intent) (string, bool) {
	if !it.LaunchStamped {
		// Consumed but never stamped: the crash boundary between
		// dispatch and stamp. The launch outcome is unknowable from
		// here — reconcile visibly rather than guess.
		return "consumed but launch never stamped; outcome unknown — inspect the job record and receipts", true
	}
	recordPath := filepath.Join(repoRoot, "artifacts", "agents", "jobs", it.JobId+".json")
	data, err := os.ReadFile(recordPath)
	if os.IsNotExist(err) {
		return "launch stamped but no job record exists; the dispatch left no trace — inspect receipts", true
	}
	if err != nil {
		return "job record unreadable: " + err.Error(), true
	}
	var record struct {
		Status  string `json:"status"`
		EndedAt string `json:"endedAt"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return "job record malformed: " + err.Error(), true
	}
	if record.EndedAt == "" {
		return "", false // still running; leave it be
	}
	returnPath := filepath.Join(repoRoot, "artifacts", "agents", "jobs", it.JobId, "return.json")
	if _, err := os.Stat(returnPath); err == nil {
		return fmt.Sprintf("ended %s with a return record", record.Status), true
	}
	return fmt.Sprintf("ended %s without a return record", record.Status), true
}
