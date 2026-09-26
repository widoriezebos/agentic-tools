package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// TestIntentBuildClaimsLawfully: build asks the real claim owner for an
// approved goal nobody holds, and returns that owner's refusal unchanged when
// its session proof is missing; it never takes a goal another machine holds.
func TestIntentBuildClaimsLawfully(t *testing.T) {
	t.Parallel()
	unclaim := func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.State, file.Claimed, file.StopCapability, file.StopFence = goal.StateApproved, nil, nil, nil
	}
	free := newIntentBed(t, false, unclaim)
	free.lineage = "m1"
	if file := free.goalFile(bedGoal); file.State != goal.StateApproved || file.Claimed != nil {
		t.Fatalf("the bed goal must be approved and unclaimed: %+v", file)
	}
	brief := filepath.Join(free.root(), "brief.md")
	os.WriteFile(brief, []byte("Build it.\n"), 0o644)
	_, result := buildJSON(t, free, "build", bedGoal, "--work", "main", "--brief", brief, "--lines", "5", "--json", "--check", "true")
	// The claim owner itself decides: in this bed no lease holder exists,
	// so it refuses with its own rule and the goal is left approved.
	file := free.goalFile(bedGoal)
	if result.Outcome != intentRefused || !strings.Contains(result.Summary, "build claims goal "+bedGoal+" first") ||
		!strings.Contains(result.Summary, "lease holder's positive claim epoch") || file.State != goal.StateApproved || file.Claimed != nil {
		t.Fatalf("build did not ask the claim owner, or changed the goal on its refusal: %+v, result %+v", file.Claimed, result)
	}

	foreign := newIntentBed(t, false, func(file *goal.GoalFile) {
		workApprovedBox(file)
		if file.Claimed != nil {
			file.Claimed.Machine, file.Claimed.Lineage = "mac-other", "o1"
		}
	})
	foreign.lineage = "m1"
	before := foreign.goalFile(bedGoal)
	if before.Claimed == nil || before.Claimed.Machine != "mac-other" {
		t.Fatalf("the fixture goal is not held by another machine: %+v", before.Claimed)
	}
	brief = filepath.Join(foreign.root(), "brief.md")
	os.WriteFile(brief, []byte("Build it.\n"), 0o644)
	_, result = buildJSON(t, foreign, "build", bedGoal, "--work", "main", "--brief", brief, "--lines", "5", "--json", "--check", "true")
	after := foreign.goalFile(bedGoal)
	if result.Outcome == intentConfirmed || after.Claimed == nil || after.Claimed.Machine != "mac-other" || after.Claimed.Lineage != "o1" {
		t.Fatalf("build took another machine's goal: %+v, result %+v", after.Claimed, result)
	}
}

// buildJSON runs build with --json before its --check, which ends the options.
func buildJSON(t *testing.T, bed *intentBed, args ...string) (int, intentResult) {
	t.Helper()
	code, stdout, stderr := bed.run(bed.owners(), args...)
	var result intentResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("%v printed no JSON: %v %q %q", args, err, stdout, stderr)
	}
	return code, result
}

// TestIntentBuildRetainedRequestSelection: with several work items, an
// unnamed build that repeats one item's exact retained request reaches that
// item's run and launches nothing new; any other request is refused with
// every name.
func TestIntentBuildRetainedRequestSelection(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	briefA := bed.brief("a.md", "Build part a.\n\nMaximum reader tool calls: 5\n")
	briefB := bed.brief("b.md", "Build part b.\n\nMaximum reader tool calls: 5\n")
	check := append([]string{"--check"}, workArgv...)
	_, first, _ := bed.work(append([]string{"build", bed.id, "--work", "a", "--brief", briefA, "--lines", "5"}, check...)...)
	_, second, _ := bed.work(append([]string{"build", bed.id, "--work", "b", "--brief", briefB, "--lines", "5"}, check...)...)
	if first.Outcome != intentConfirmed || second.Outcome != intentConfirmed {
		t.Fatalf("builds: %+v %+v", first, second)
	}
	launched := len(bed.starter.launched())
	code, again, _ := bed.work(append([]string{"build", bed.id, "--brief", briefB, "--lines", "5"}, check...)...)
	if code != 0 || resultData(t, again)["run"] != resultData(t, second)["run"] || len(bed.starter.launched()) != launched {
		t.Fatalf("an unnamed repeat of b: code=%d %+v launches=%d->%d", code, again, launched, len(bed.starter.launched()))
	}
	briefC := bed.brief("c.md", "Build part c.\n\nMaximum reader tool calls: 5\n")
	code, other, _ := bed.work(append([]string{"build", bed.id, "--brief", briefC, "--lines", "5"}, check...)...)
	if code != 2 || other.Outcome != intentRefused || !strings.Contains(other.Summary, "a, b") || len(bed.starter.launched()) != launched {
		t.Fatalf("an unnamed new request: code=%d %+v", code, other)
	}
}
