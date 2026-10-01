package branch

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// A goal-branch refusal reads as its words; its code is data for --verbose,
// --json and records ("Messages a Person Reads").
func TestBranchRefusalsKeepTheirCodeOutOfTheirWords(t *testing.T) {
	t.Parallel()
	op := fmt.Errorf("goal branch commit: %w", operationRefusal(NotHolderCode, "goal %s is held by another session", "g1"))
	if op.Error() != "goal branch commit: goal g1 is held by another session" || goal.RefusalCode(op) != NotHolderCode ||
		goal.RecordText(op) != NotHolderCode+": goal branch commit: goal g1 is held by another session" {
		t.Fatalf("operation refusal: words %q code %q record %q", op.Error(), goal.RefusalCode(op), goal.RecordText(op))
	}
	rangeErr := rangeRefusal("g1", "abc123", "the commit touches two units")
	if rangeErr.Error() != "commit abc123: the commit touches two units\nrun: metasystem work status g1" || goal.RefusalCode(rangeErr) != RangeCode {
		t.Fatalf("range refusal: words %q code %q", rangeErr.Error(), goal.RefusalCode(rangeErr))
	}
}

// A commit that says nothing of its kind names the trailer it lacks, with
// the goal filled in; the range owner can't tell whether the commit is the
// goal branch's tip, so its line 2 stays the goal's work and it carries the
// trailer for the verb that can (no amend of whatever is checked out).
func TestMissingKindTrailerNamesTheTrailerToAdd(t *testing.T) {
	t.Parallel()
	commit := "289c41c7f0123456789abcdef0123456789abcde"
	_, err := kindOfWithGit("repo", commit, "goal-a", func(string, ...string) ([]byte, error) { return []byte("\n"), nil })
	var refusal *RangeError
	if !errors.As(err, &refusal) || refusal.Code != RangeCode {
		t.Fatalf("missing trailer: %v", err)
	}
	if !strings.Contains(refusal.Reason, "doesn't say which goal and unit it builds") || !strings.Contains(refusal.Reason, "Goal-Unit: goal-a/UNIT") {
		t.Fatalf("missing trailer reason: %q", refusal.Reason)
	}
	if refusal.Trailer != "Goal-Unit: goal-a/UNIT" || refusal.Remedy != "run: metasystem work status goal-a" || strings.Contains(err.Error(), "amend") {
		t.Fatalf("missing trailer remedy: trailer %q remedy %q", refusal.Trailer, refusal.Remedy)
	}
	_, err = kindOfWithGit("repo", commit, "goal-a", func(string, ...string) ([]byte, error) {
		return []byte("Goal-Unit: goal-a/u1\nGoal-Plan: goal-a\n"), nil
	})
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Reason, "2 times") || refusal.Trailer != "" {
		t.Fatalf("two trailers keep their own words: %v", err)
	}
}
