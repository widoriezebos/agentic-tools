package main

import (
	"errors"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

// An owner's refusal handed to complain keeps its code for --verbose and
// --json (Messages a Person Reads: codes are details): goal sync --publish's
// RECONCILE_OUTSIDE_SCOPE reached the public result as bare words.
func TestComplainKeepsTheRefusalCode(t *testing.T) {
	t.Parallel()
	report := &ownerReport{}
	dependencies := syncRequestDependencies{report: report}
	dependencies.complain(refusal.New("RECONCILE_OUTSIDE_SCOPE", "", errors.New("the goal files also hold edits of b")))
	if report.failure == nil || goal.RefusalCode(report.failure) != "RECONCILE_OUTSIDE_SCOPE" || report.failure.Error() != "the goal files also hold edits of b" {
		t.Fatalf("failure = %v (code %q)", report.failure, goal.RefusalCode(report.failure))
	}
	dependencies.complain("goal", "x", "has no route")
	if report.failure == nil || report.failure.Error() != "goal x has no route" {
		t.Fatalf("words failure = %v", report.failure)
	}
}
