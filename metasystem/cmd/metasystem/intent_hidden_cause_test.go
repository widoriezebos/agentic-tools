package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	b.owners.branchState = func(string, string) (intentBranchState, error) {
		return intentBranchState{}, errors.New("exit status 128")
	}
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

// A build whose goal branch can't be reached says why when the owner's
// error carries its own line 2, and names that line's command.
func TestIntentBuildNamesTheEndpointCause(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n\n| Unit | Lines |\n| --- | --- |\n| u1 | 40 |\n")
	owners := bed.workOwners()
	owners.connection.endpoint = func(string) (goal.Endpoint, error) {
		return goal.Endpoint{}, errors.New("this checkout names no code origin\nrun: metasystem settings check")
	}
	command, rest, _ := resolveIntentArgv(append([]string{"work", "build", bed.id, "u1", "--brief", brief}, workCheck...))
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &stdout, &stderr, bed.root(), owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("no JSON result: %v; stderr=%q", err, stderr.String())
	}
	expectOutcome(t, "unreachable endpoint", code, result, intentRefused)
	if result.Summary != "this checkout names no code origin, so nothing was built" ||
		result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "settings", "check"}) {
		t.Fatalf("the endpoint's cause must be line 1 and its command line 2: %+v", result)
	}
}

// withCause lifts only a cause a person can act on: a goal branch commit's
// refusal or an owner's two-line message; a bare error leaves the result.
func TestWithCauseLiftsOnlyAnActionableCause(t *testing.T) {
	t.Parallel()
	base := intentResult{Summary: "the review couldn't be requested, so nothing was done", next: []string{"metasystem", "again"}, nextReason: "try again", Details: []string{"raw"}}
	if got := base.withCause(errors.New("open x: permission denied")); got.Summary != base.Summary || got.nextReason != "try again" {
		t.Fatalf("a bare error changed the result: %+v", got)
	}
	shape := &branch.RangeError{Code: branch.RangeCode, Commit: strings.Repeat("a", 40), Reason: "it has 2 parents, and a goal branch commit has one", Remedy: "run: metasystem work status g1"}
	got := base.withCause(fmt.Errorf("unit u1: %w", shape))
	if got.Summary != "commit aaaaaaaaa has 2 parents, and a goal branch commit has one, so nothing was done" || !slices.Equal(got.next, []string{"metasystem", "work", "status", "g1"}) {
		t.Fatalf("range refusal: %+v", got)
	}
	got = base.withCause(errors.New("the goal is already landed\nnothing to do; it is on main"))
	if got.Summary != "the goal is already landed, so nothing was done" || got.next != nil || got.nextReason != "nothing to do; it is on main" {
		t.Fatalf("nothing-to-do message: %+v", got)
	}
	if !slices.Equal(got.Details, []string{"raw"}) {
		t.Fatalf("details are kept: %q", got.Details)
	}
}

// A units table whose row for the unit holds no number says so in line 1;
// the bare "can't be read" left it to --verbose.
func TestIntentBuildNamesTheUnitsTableCause(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	brief := bed.brief("brief.md", "Build the unit.\n\n| Unit | Lines |\n| --- | --- |\n| u1 | many |\n")
	_, result, _ := bed.work(append([]string{"work", "build", bed.id, "u1", "--brief", brief}, workCheck...)...)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "unit u1 has no number in its size column") || !strings.Contains(result.Summary, "nothing was built") {
		t.Fatalf("the table's fault must be line 1: %+v", result)
	}
}
