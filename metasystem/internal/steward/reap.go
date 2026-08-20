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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
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
		// The notification queues BEFORE the reap mark: a crash
		// between them repeats the queue write (same nonce, an
		// overwrite) instead of closing a continuation silently.
		if err := QueueNotification(repoRoot, PendingNotification{
			Nonce:   it.Nonce + "-reap",
			Message: fmt.Sprintf("steward: continuation %s closed — %s", it.JobId, outcome),
		}); err != nil {
			return reports, err
		}
		if err := closeContinuationChain(repoRoot, it.JobId); err != nil {
			return reports, err
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
		reports = append(reports, ReapReport{Nonce: it.Nonce, JobId: it.JobId, Outcome: outcome})
	}
	return reports, nil
}

// closeContinuationChain marks the consumer-less job's chain closed —
// the steward is its reaper, and no permanently open chain survives
// it. Idempotent: an already-closed or missing record needs nothing.
func closeContinuationChain(repoRoot, jobId string) error {
	path := filepath.Join(repoRoot, "artifacts", "agents", "jobs", jobId+".json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("job record %s malformed at close: %w", jobId, err)
	}
	if closed, _ := record["chainClosed"].(bool); closed {
		return nil
	}
	record["chainClosed"] = true
	out, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// continuationOutcome decides whether one continuation's story ended
// and how. Still-running is the only not-done answer.
func continuationOutcome(repoRoot string, it Intent) (string, bool) {
	if !it.LaunchStamped {
		// Consumed but never stamped: either the crash boundary
		// between dispatch and stamp, or a dispatcher child still
		// mid-flight after its Go parent died. The job record
		// decides: a non-terminal record is a LIVE launch — leave it
		// and the guard alone; a terminal one, or a long absence,
		// reconciles visibly rather than guessing.
		recordPath := filepath.Join(repoRoot, "artifacts", "agents", "jobs", it.JobId+".json")
		if data, err := os.ReadFile(recordPath); err == nil {
			var rec struct {
				EndedAt string `json:"endedAt"`
			}
			if json.Unmarshal(data, &rec) == nil && rec.EndedAt == "" {
				return "", false
			}
			return "consumed but launch never stamped; its job ended — outcome unknown, inspect the record and receipts", true
		}
		info, err := os.Stat(filepath.Join(consumedDir(repoRoot), it.Nonce+".json"))
		if err == nil && time.Since(info.ModTime()) < 10*time.Minute {
			// The pending/setup window: the record may not exist yet.
			return "", false
		}
		return "consumed but launch never stamped and no job record appeared; outcome unknown — inspect receipts", true
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
	// The return lives in the job's payload rounds, and the standing
	// checker owns its validity — record, chain, schema, identity.
	returnPath := filepath.Join(repoRoot, "artifacts", "agents", it.JobId, "rounds", "1", "return.json")
	if _, err := os.Stat(returnPath); err != nil {
		return fmt.Sprintf("ended %s without a return record", record.Status), true
	}
	if violations := validate.ReturnCompleteJob(repoRoot, it.JobId); len(violations) > 0 {
		return fmt.Sprintf("ended %s with a PROTOCOL-ERROR return: %s", record.Status, violations[0]), true
	}
	return fmt.Sprintf("ended %s with a valid return", record.Status), true
}
