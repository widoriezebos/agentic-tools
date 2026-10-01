package batch

// landing prove's attempts (simple lane): every proof of the lane
// checkout's tree is recorded on each queued hand-off's batch, with the
// members it covers, before and after its child runs.

import (
	"fmt"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// SubjectBatch is the subject of every attempt landing prove records: the
// tree the lane checkout holds. Older records also name base and member
// subjects.
const SubjectBatch = "batch"

// The outcomes of an attempt: running until its child ends, then typed
// green, red or unavailable. Unavailable is never red (design §8).
const (
	AttemptRunning     = "running"
	AttemptGreen       = "green"
	AttemptRed         = "red"
	AttemptUnavailable = "unavailable"
)

// ProofAttempt is one landing prove of a batch.
type ProofAttempt struct {
	ID string `json:"id"`
	// OpID is the landing begin an older engine's attempt proved.
	OpID    string `json:"opId,omitempty"`
	Subject string `json:"subject"`
	// Member is an older member subject's member; Tree is the exact tree
	// the child ran, and Commit the lane checkout's commit of it.
	Member  string   `json:"member,omitempty"`
	Commit  string   `json:"commit,omitempty"`
	Tree    string   `json:"tree"`
	Purpose string   `json:"purpose"`
	Groups  []string `json:"groups,omitempty"`
	Actor   string   `json:"actor"`
	Status  string   `json:"status"`
	// ResultPath is the child's result projection; RunAttempt the test
	// run's own attempt id once it is read.
	ResultPath string   `json:"resultPath"`
	RunAttempt string   `json:"runAttempt,omitempty"`
	RedGroups  []string `json:"redGroups,omitempty"`
	// Covers are the batch's queued members when the attempt started.
	Covers []string `json:"covers,omitempty"`
	Reason string   `json:"reason,omitempty"`
	// Child is the test run child's process identity an older engine
	// recorded.
	Child     *identity.Ref `json:"child,omitempty"`
	StartedAt string        `json:"startedAt"`
	EndedAt   string        `json:"endedAt,omitempty"`
}

// Terminal reports whether the attempt's child has ended.
func (attempt ProofAttempt) Terminal() bool { return attempt.Status != AttemptRunning }

// StartAttempt durably records a new running attempt on batch id.
func StartAttempt(store Store, id string, attempt ProofAttempt) error {
	return store.Update(id, func(record *Record) error {
		if slices.ContainsFunc(record.Attempts, func(existing ProofAttempt) bool { return existing.ID == attempt.ID }) {
			return fmt.Errorf("attempt %s is already recorded", attempt.ID)
		}
		attempt.Status = AttemptRunning
		record.Attempts = append(record.Attempts, attempt)
		return nil
	})
}

// FinishAttempt records a running attempt's typed outcome.
func FinishAttempt(store Store, id, attemptID, status, runAttempt, reason string, redGroups []string, at time.Time) error {
	if status != AttemptGreen && status != AttemptRed && status != AttemptUnavailable {
		return fmt.Errorf("attempt outcome %q is not green, red or unavailable", status)
	}
	return updateAttempt(store, id, attemptID, func(attempt *ProofAttempt) {
		attempt.Status, attempt.RunAttempt, attempt.Reason = status, runAttempt, reason
		attempt.RedGroups = slices.Clone(redGroups)
		attempt.EndedAt = at.UTC().Format(time.RFC3339Nano)
	})
}

func updateAttempt(store Store, id, attemptID string, change func(*ProofAttempt)) error {
	return store.Update(id, func(record *Record) error {
		for index := range record.Attempts {
			if record.Attempts[index].ID != attemptID {
				continue
			}
			if record.Attempts[index].Terminal() {
				return fmt.Errorf("attempt %s already ended %s", attemptID, record.Attempts[index].Status)
			}
			change(&record.Attempts[index])
			return nil
		}
		return fmt.Errorf("attempt %s is not recorded on batch %s", attemptID, id)
	})
}

// Attempt is the recorded attempt attemptID of the batch.
func (record Record) Attempt(attemptID string) (ProofAttempt, bool) {
	for _, attempt := range record.Attempts {
		if attempt.ID == attemptID {
			return attempt, true
		}
	}
	return ProofAttempt{}, false
}

// Head is the commit a queued member hands the lane: what the landing agent
// merges, and what a pushed main must contain for the member to land.
func (unit Unit) Head() string {
	switch {
	case unit.IsChange():
		return unit.Change.Commit
	case unit.BranchTip != "":
		return unit.BranchTip
	}
	return unit.Chain
}
