package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func tablelessBoundaryBed(t *testing.T, route string) (*deliveryBed, *landingOwners, string) {
	t.Helper()
	b, owners, install := admissionBed(t, route)
	b.writeFile(filepath.Join(b.root(), "plans", "designs", "landing-work.md"),
		"# Landing work\n\n- Kind: design\n- Id: landing-work\n- Status: accepted\n- Goals: standing-validation\n")
	return b, owners, install
}

func TestWorkLandBranchEndIncludesLaterUnits(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"lane", "hand"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b, owners, _ := tablelessBoundaryBed(t, route)
			owners.status.Status.Units[1].Whole = true
			owners.status.Status.Prefix = 0
			code, result := b.do("work", "land", "standing-validation")
			expectOutcome(t, "unit before end unread", code, result, intentRefused)
			if !strings.Contains(result.Summary, "u1") {
				t.Fatalf("earlier unit's missing read: %+v", result)
			}
			owners.status.Status.Prefix = 1
			code, result = b.do("work", "land", "standing-validation")
			expectOutcome(t, "marked unit unread", code, result, intentRefused)
			if !strings.Contains(result.Summary, "u2 has no clean read") {
				t.Fatalf("marked unit's missing read: %+v", result)
			}
			owners.status.Status.Prefix = 2
			owners.status.Status.Units = append(owners.status.Status.Units, branch.UnitStatus{Unit: "after", Commit: strings.Repeat("4", 40)})
			code, result = b.do("work", "land", "standing-validation")
			expectOutcome(t, "unit after end unread", code, result, intentRefused)
			if !strings.Contains(result.Summary, "after has no clean read") {
				t.Fatalf("later unit's missing read: %+v", result)
			}
			owners.status.Status.Units[2].ReadState = "read clean"
			code, result = b.do("work", "land", "standing-validation")
			expectOutcome(t, "all branch units read", code, result, intentConfirmed)
		})
	}
}

func TestWorkLandWholeNeedsPersonAndAllReads(t *testing.T) {
	t.Parallel()
	b, owners, install := tablelessBoundaryBed(t, "lane")
	for _, args := range [][]string{
		{"--whole"}, {"--whole", "--by", "SomeoneElse"},
	} {
		code, result := b.do(append([]string{"work", "land", "standing-validation"}, args...)...)
		expectOutcome(t, "unproved declaration", code, result, intentRefused)
	}
	b.lineage = "agent-session"
	code, result := b.do("work", "land", "standing-validation", "--whole", "--by", "Wido")
	expectOutcome(t, "agent declaration", code, result, intentRefused)
	b.lineage = ""
	owners.status.Status.Prefix = 1
	code, result = b.do("work", "land", "standing-validation", "--whole", "--by", "Wido")
	expectOutcome(t, "human declaration with unread unit", code, result, intentRefused)
	if !strings.Contains(result.Summary, "u2") {
		t.Fatalf("missing read unnamed: %+v", result)
	}
	if lines, err := plain.Entries(install); err != nil || len(lines) != 0 {
		t.Fatalf("refusal queued work: %+v %v", lines, err)
	}
	owners.status.Status.Prefix = 2
	code, result = b.do("work", "land", "standing-validation", "--whole", "--by", "Wido")
	expectOutcome(t, "person declares end", code, result, intentConfirmed)
	data, err := os.ReadFile(filepath.Join(plain.Dir(install), "queue.jsonl"))
	var line plain.Line
	if err != nil || json.Unmarshal(data, &line) != nil || line.WholeBy != "Wido" {
		t.Fatalf("queue lost declarer's name: %s %v", data, err)
	}
}

func TestWorkBuildLastRetainedForCommit(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	brief := b.brief("last.md", "Build the last unit.\n")
	args := append([]string{"work", "build", b.id, "--work", "u2", "--last", "--brief", brief, "--lines", "5"}, workCheck...)
	code, result, _ := b.work(args...)
	expectOutcome(t, "last build", code, result, intentConfirmed)
	plan, err := launch.ReadUnitPlan(resultData(t, result)["plan"].(string))
	if err != nil || !plan.Whole {
		t.Fatalf("last declaration not retained for review: %+v %v", plan, err)
	}
	// A changed boundary is a changed build request and must not rejoin it.
	code, result, _ = b.work(withoutOption(args, "last")...)
	if code == 0 {
		t.Fatalf("changed boundary rejoined last build: %+v", result)
	}
}

func TestGoalNextStepMarkedEnd(t *testing.T) {
	t.Parallel()
	b, inv, state := nextStepBed(t)
	state.Status.Units[1].Whole = true
	next, _ := inv.goalNextStep(b.id)
	if strings.Join(next, " ") != "metasystem work land "+b.id {
		t.Fatalf("marked goal's next step: %q", next)
	}
}

func TestGoalProgressMarkedEndWaivesOnlyReads(t *testing.T) {
	t.Parallel()
	state := intentBranchState{Status: branch.Status{Units: []branch.UnitStatus{
		{Unit: "u1", Commit: "first"}, {Unit: "u2", Commit: "second", Whole: true},
		{Unit: "u3", Commit: "third", Whole: true}, {Unit: "after", Commit: "fourth"},
	}}}
	progress, err := goalProgress(nil, state)
	if err != nil || progress.NoEnd || progress.Unit != "u1" || len(progress.Unread) != 4 {
		t.Fatalf("all units around the newest end need reads: %+v %v", progress, err)
	}
	state.ReadsWaived = true
	progress, err = goalProgress(nil, state)
	if err != nil || progress.NoEnd || progress.Unit != "" || len(progress.Unread) != 0 {
		t.Fatalf("marked goal with waived reads: %+v %v", progress, err)
	}
}

func TestWorkLandWholeCannotSkipDeclaredBuilds(t *testing.T) {
	t.Parallel()
	b, owners, install := admissionBed(t, "lane")
	code, result := b.do("work", "land", "standing-validation", "--whole", "--by", "Wido")
	expectOutcome(t, "declared unit missing", code, result, intentRefused)
	if !strings.Contains(result.Summary, "u3 is not built") {
		t.Fatalf("human end bypassed declared work: %+v", result)
	}
	if entries, err := plain.Entries(install); err != nil || len(entries) != 0 || owners.candidates != 0 {
		t.Fatalf("incomplete declared goal reached delivery: %+v %v", entries, err)
	}
}
