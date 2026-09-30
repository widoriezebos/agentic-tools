package batch

// landing prove's attempts (lane runtime design r10 K6): every execution of
// a batch's subject is recorded on the batch, with its subject, before and
// after its child runs. The custody barrier (K9) reads the child identity,
// the budget (K10) counts the attempts, and returns (K8) cite them.

import (
	"fmt"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The subjects of landing prove.
const (
	SubjectBatch  = "batch"
	SubjectBase   = "base"
	SubjectMember = "member"
)

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
	// OpID is the landing begin whose series the attempt proves.
	OpID    string `json:"opId"`
	Subject string `json:"subject"`
	// Member is the member of a member subject, and Tree the exact tree the
	// child ran: the candidate's, B's, or B plus the member's contribution.
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
	Reason     string   `json:"reason,omitempty"`
	// Child is the test run child's process identity, recorded when it
	// starts (K9 reads it to settle custody).
	Child     *identity.Ref `json:"child,omitempty"`
	StartedAt string        `json:"startedAt"`
	EndedAt   string        `json:"endedAt,omitempty"`
}

// Terminal reports whether the attempt's child has ended.
func (attempt ProofAttempt) Terminal() bool { return attempt.Status != AttemptRunning }

// StartAttempt durably records a new running attempt on batch id; the
// attempt proves the batch's current opening.
func StartAttempt(store Store, id string, attempt ProofAttempt) error {
	return store.Update(id, func(record *Record) error {
		opening, ok := record.CurrentOpening()
		if !ok || opening.OpID != attempt.OpID {
			return fmt.Errorf("batch %s's series moved before the attempt started", id)
		}
		if slices.ContainsFunc(record.Attempts, func(existing ProofAttempt) bool { return existing.ID == attempt.ID }) {
			return fmt.Errorf("attempt %s is already recorded", attempt.ID)
		}
		attempt.Status = AttemptRunning
		record.Attempts = append(record.Attempts, attempt)
		return nil
	})
}

// SetAttemptChild records the started child of a running attempt.
func SetAttemptChild(store Store, id, attemptID string, child identity.Ref) error {
	return updateAttempt(store, id, attemptID, func(attempt *ProofAttempt) {
		attempt.Child = &child
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
