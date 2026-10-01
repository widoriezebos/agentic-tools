package main

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// A goal branch whose commit lacks its Goal-Unit trailer refuses the landing
// in the two lines of "Messages a Person Reads": line 1 names the commit and
// the missing trailer, line 2 the command that adds it with the goal filled
// in. The generic "can't be read" sentence hid both behind --verbose.
func TestIntentLandNamesTheMissingGoalUnitTrailer(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	commit := "289c41c7f0123456789abcdef0123456789abcde"
	refusal := &branch.RangeError{Code: branch.RangeCode, Commit: commit,
		Reason: "it doesn't say which goal and unit it builds; add the trailer Goal-Unit: standing-validation/UNIT",
		Remedy: `run: git commit --amend --no-edit --trailer "Goal-Unit: standing-validation/UNIT"`,
		Fix:    []string{"git", "commit", "--amend", "--no-edit", "--trailer", "Goal-Unit: standing-validation/UNIT"}}
	b.owners.branchState = func(string, string) (intentBranchState, error) {
		return intentBranchState{}, fmt.Errorf("goal branch: %w", refusal)
	}
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "missing trailer", code, result, intentRefused)
	if !strings.Contains(result.Summary, "commit 289c41c7f ") || !strings.Contains(result.Summary, "Goal-Unit: standing-validation/UNIT") ||
		strings.Contains(result.Summary, "can't be read") || !strings.Contains(result.Summary, "nothing was landed") {
		t.Fatalf("line 1 must name the commit and the trailer: %q", result.Summary)
	}
	if result.Next == nil || !slices.Equal(result.Next.Argv, refusal.Fix) {
		t.Fatalf("line 2 must add the trailer: %+v", result.Next)
	}
	if !slices.ContainsFunc(result.Details, func(detail string) bool { return strings.Contains(detail, commit) }) {
		t.Fatalf("the raw refusal stays a detail: %q", result.Details)
	}
	_, _, stderr := b.run(b.landOwners(), "work", "land", "standing-validation")
	text := strings.Join(strings.Fields(stderr), " ")
	if !strings.HasPrefix(text, "✗ commit 289c41c7f doesn't say which goal and unit it builds, so nothing was landed; add the trailer Goal-Unit: standing-validation/UNIT") ||
		!strings.HasSuffix(text, "→ git commit --amend --no-edit --trailer 'Goal-Unit: standing-validation/UNIT'") {
		t.Fatalf("text refusal must be the two lines: %q", stderr)
	}
}

// An error that carries no remedy of its own keeps the verb's sentence:
// there is nothing more a person could act on.
func TestIntentLandKeepsItsSentenceForAnInternalError(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	b.owners.branchState = func(string, string) (intentBranchState, error) { return intentBranchState{}, errors.New("exit status 128") }
	code, result := b.do("work", "land", "standing-validation")
	expectOutcome(t, "internal error", code, result, intentRefused)
	if result.Summary != "the goal branch can't be read, so nothing was landed" || result.Next == nil || result.Next.Argv[0] != "metasystem" {
		t.Fatalf("internal error: %+v", result)
	}
}

func (b *deliveryBed) landOwners() intentOwners {
	owners := b.intentBed.owners()
	owners.delivery = b.owners
	owners.connection = b.connection
	return owners
}
