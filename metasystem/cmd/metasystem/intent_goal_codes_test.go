package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
)

// A goal owner's refusal code is data: it never leads the words a person
// reads, and it reaches --json (and --verbose) as the "refusal code: X"
// detail on every path a goal refusal takes ("Messages a Person Reads").
func TestGoalRefusalCodesReachJSON(t *testing.T) {
	t.Parallel()
	bed := newIntentBed(t, false, nil)
	// Claim proves the checkout holder before reading the ledger fresh.
	announceProofFixtureHolder(t, bed.root())
	other := *bed.goalFile(bedGoal)
	other.Id, other.State, other.Claimed = "other-work", goal.StateApproved, nil
	for index := range other.History {
		other.History[index].Targets = []string{other.Id}
	}
	bed.addGoal(&other)

	code, result := bed.runJSON(bed.owners(), "goal", "claim", "other-work", "--lineage", "m2")
	if code == 0 || result.Outcome != intentRefused || !slices.Contains(result.Details, "refusal code: "+goal.ClaimQuotaCode) ||
		strings.Contains(result.Summary, goal.ClaimQuotaCode) {
		t.Fatalf("claim over the quota: code=%d %+v", code, result)
	}
}

// A refused human act whose owner error carries a code keeps it for --json,
// and its words stay one line with the caller's remedy as line 2.
func TestHumanVerbRefusalKeepsTheOwnersCode(t *testing.T) {
	t.Parallel()
	report := &ownerReport{}
	values := &humanVerbValues{verb: "resume", report: report}
	busy := &goalrevision.Busy{Key: "g1/r2", Holder: "pid=7,tag=goal-resume"}
	refuseHumanVerb(values, 1, values.cause(errors.Join(busy)), runRemedy("metasystem", "goal", "resume", "g1"))
	if report.refusal == nil || report.refusal.refusalCode != goalrevision.BusyCode || strings.Contains(report.refusal.sentence, "LOCK_BUSY") {
		t.Fatalf("busy refusal: %+v", report.refusal)
	}
	result := ownerResult(report, 1, intentResult{})
	if !slices.Contains(result.Details, "refusal code: "+goalrevision.BusyCode) {
		t.Fatalf("the code did not reach the result's details: %+v", result)
	}

	// An owner error that brings its own line 2 keeps line 1 as the
	// sentence; the caller's remedy is the one line 2.
	report = &ownerReport{}
	values = &humanVerbValues{verb: "done", report: report}
	open := &goal.DoneReadItemsOpenError{Goal: "g1", ItemIDs: []string{"r1-1"}}
	refuseHumanVerb(values, 1, values.cause(open), runRemedy("metasystem", "goal", "notes", "g1"))
	if report.refusal == nil || report.refusal.sentence != "goal g1 has open review notes r1-1, so it can't be concluded yet." ||
		report.refusal.refusalCode != goal.DoneReadItemsOpenCode {
		t.Fatalf("two-line owner error: %+v", report.refusal)
	}
}

// An owner refusal that brings its own line 2 prints as two lines: its
// words behind ✗ and its remedy as the one hint, in place of the generic
// retry; a remedy the verb names itself still wins.
func TestOwnerLineTwoBecomesTheHint(t *testing.T) {
	t.Parallel()
	failure := intentResult{Outcome: intentRefused, code: 1, retry: "once the cause above is fixed",
		Summary: "an earlier goal change (op-1) was pushed, but whether it took effect is unknown\nrun: metasystem goal sync --recover"}
	_, _, stderr := renderOnce(t, "goal pause", []string{"g1", "--reason", "later"}, nil, failure)
	want := "✗ an earlier goal change (op-1) was pushed, but whether it took effect is unknown\n  → metasystem goal sync --recover\n"
	if stderr != want {
		t.Errorf("owner line 2 = %q, want %q", stderr, want)
	}
	failure.Summary = "goal g1 waits on g2, which isn't done\nnothing to do; wait for g2"
	if _, _, stderr := renderOnce(t, "goal pause", []string{"g1", "--reason", "later"}, nil, failure); stderr != "✗ goal g1 waits on g2, which isn't done\n  → nothing to do; wait for g2\n" {
		t.Errorf("owner nothing-to-do = %q", stderr)
	}
	named := intentResult{Outcome: intentRefused, code: 1, Summary: "words\nrun: metasystem goal sync", next: []string{"metasystem", "goal", "show", "g1"}}
	if _, _, stderr := renderOnce(t, "goal pause", []string{"g1", "--reason", "later"}, nil, named); !strings.Contains(stderr, "→ metasystem goal show g1") {
		t.Errorf("the verb's own remedy = %q", stderr)
	}
}
