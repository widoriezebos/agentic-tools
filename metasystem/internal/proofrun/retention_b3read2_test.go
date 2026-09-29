package proofrun

// Witnesses of the second B3 read (the reader's probes, kept as they
// failed on 118b71a4f).

import (
	"os"
	"testing"
)

// R2-P1: a young attempt whose record cannot be decoded (here: a field of
// the wrong type, as a schema change would produce) reuses an old attempt.
// Retention should keep the old attempt (an unreadable record's references
// are unknown); does it?
func TestB3Read2UnreadableYoungAttemptReferencesAreDropped(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-old-0000000000000001", 60*retentionDay, true, nil)
	b.attempt("proof-new-0000000000000002", 1*retentionDay, true, func(a *Attempt) {
		a.TestResult = &TestResult{Groups: []GroupResult{{ID: "g", ReuseAttempt: "proof-old-0000000000000001"}}}
	})
	// Corrupt the young record's type of one unrelated field.
	data, _ := os.ReadFile(b.path("attempts", "proof-new-0000000000000002.json"))
	data = append([]byte(`{"reservedMinutes":"x",`), data[1:]...)
	os.WriteFile(b.path("attempts", "proof-new-0000000000000002.json"), data, 0o600)
	report := b.pass(b.retention(1))
	t.Logf("pending=%+v actions=%+v", report.Pending, report.Actions)
	if !b.exists("proof-old-0000000000000001") {
		t.Fatalf("old attempt reused by an unreadable young attempt was removed")
	}
}

// R2-P2: an attempt id minted by an older generator / not matching the
// retention regex, reused by a young attempt: its reference is dropped by
// IsAttemptID but the record itself is still a removal candidate.
func TestB3Read2NonMatchingIDReferenceDropped(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-legacy-1", 60*retentionDay, true, nil)
	b.attempt("proof-new-0000000000000002", 1*retentionDay, true, func(a *Attempt) {
		a.TestResult = &TestResult{Groups: []GroupResult{{ID: "g", ReuseAttempt: "proof-legacy-1"}}}
	})
	b.pass(b.retention(1))
	if _, err := os.Stat(b.path("attempts", "proof-legacy-1.json")); err != nil {
		t.Fatalf("legacy-shaped attempt reused by a young attempt was removed: %v", err)
	}
}

// R2-P4: an open goal's own terminal attempt older than the window, in the
// goal's current budget epoch: dispatch/budget.go charges it (Attempts,
// ReservedJobMinutes). Nothing in retention reads GoalID, so it goes and
// the open goal's spend is refunded.
func TestB3Read2OpenGoalsAccountedAttemptRemoved(t *testing.T) {
	t.Parallel()
	b := newRetentionBed(t)
	b.attempt("proof-old-0000000000000001", 20*retentionDay, true, func(a *Attempt) {
		a.GoalID = "long-open-goal"
		a.ReservedMinutes, a.ObservedMinutes = 60, 55
	})
	b.pass(b.retention(1))
	if !b.exists("proof-old-0000000000000001") {
		t.Fatalf("a terminal attempt charged to an open goal's budget was removed (budget.go:556-630 would stop counting it)")
	}
}
