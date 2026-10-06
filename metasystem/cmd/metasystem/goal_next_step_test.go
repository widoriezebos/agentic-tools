package main

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func nextStepBed(t *testing.T) (*workBed, *intentInvocation, *intentBranchState) {
	t.Helper()
	bed := newWorkBed(t)
	owners := bed.workOwners()
	layout, err := owners.resolver.ResolveLayout(bed.root())
	if err != nil {
		t.Fatal(err)
	}
	state := &intentBranchState{Status: branch.Status{Units: []branch.UnitStatus{
		{Unit: "u1", Commit: "first", ReadState: "read clean"},
		{Unit: "u2", Commit: "second", ReadState: "read clean"},
	}}}
	owners.delivery = &intentDeliveryOwners{branchState: func(root, id string) (intentBranchState, error) {
		if root != layout.InstallationRoot.Path() || id != bed.id {
			t.Fatalf("branch read root=%s goal=%s", root, id)
		}
		return *state, nil
	}}
	owners.work.inspectRead = func(string, string, string) (branch.BranchReadResult, error) {
		return branch.BranchReadResult{State: "collected", Published: true}, nil
	}
	return bed, &intentInvocation{owners: owners, layout: layout, stateRoot: bed.root()}, state
}

func nextStepDesign(bed *workBed, names ...string) {
	bed.t.Helper()
	page := "# Next units\n\n- Kind: design\n- Id: next-units\n- Status: accepted\n- Goals: " + bed.id + "\n\n| Unit | Lines |\n| --- | ---: |\n"
	for _, name := range names {
		page += "| " + name + " | 5 |\n"
	}
	(&deliveryBed{intentBed: bed.intentBed}).writeFile(filepath.Join(bed.root(), "plans", "designs", "next-units.md"), page)
}

func TestGoalNextStepContinuations(t *testing.T) {
	t.Parallel()
	bed, inv, state := nextStepBed(t)
	nextStepDesign(bed, "u1", "u2", "u3")
	work := launch.NamedWork{Unit: "u2", Record: &launch.UnitRunRecord{Goal: bed.id, Worktree: bed.worktree, State: "awaiting-judgement",
		Rounds: []launch.UnitRound{{Outcome: "green"}}, Subjects: []launch.UnitSubject{{Round: 1, Commit: "second"}}}}
	for _, step := range []struct {
		name string
		unit string
		verb string
	}{
		{"unbuilt", "u3", "build"},
		{"unread", "u3", "review"},
		{"extra unread", "extra", "review"},
		{"finished", "", "land"},
	} {
		switch step.name {
		case "unread":
			state.Status.Units = append(state.Status.Units, branch.UnitStatus{Unit: "u3", Commit: "third"})
		case "extra unread":
			state.Status.Units[2].ReadState = "read clean"
			state.Status.Units = append(state.Status.Units, branch.UnitStatus{Unit: "extra", Commit: "extra"})
		case "finished":
			state.Status.Units[3].ReadState = "read clean"
		}
		want := inv.publicArgv("work", step.verb, bed.id)
		if step.unit != "" {
			want = append(want, "--work", step.unit)
		}
		if step.verb == "build" {
			want = append(want, "--brief", "FILE", "--check", "COMMAND")
		}
		next, reason := inv.manualContinuation(bed.id, manualWorkItem{Unit: "u2", Commit: "second", Goal: bed.id})
		if !slices.Equal(next, want) || reason == "" || (step.unit != "" && !strings.Contains(reason, step.unit)) {
			t.Fatalf("%s manual next=%q reason=%q want=%q", step.name, next, reason, want)
		}
		next, reason = inv.workContinuation(bed.id, work, true)
		if !slices.Equal(next, want) || reason == "" {
			t.Fatalf("%s built next=%q reason=%q want=%q", step.name, next, reason, want)
		}
	}
}

func TestGoalNextStepNoEndAndReadFailures(t *testing.T) {
	t.Parallel()
	bed, inv, state := nextStepBed(t)
	next, reason := inv.goalNextStep(bed.id)
	want := inv.publicArgv("work", "build", bed.id, "--work", "NAME", "--brief", "FILE", "--check", "COMMAND")
	if !slices.Equal(next, want) || reason != "goal "+bed.id+" has no Units table; this builds its next unit, and `--last` marks its last one" {
		t.Fatalf("no end: next=%q reason=%q", next, reason)
	}
	state.Status.Units[1].ReadState = ""
	next, _ = inv.goalNextStep(bed.id)
	if !slices.Equal(next, inv.publicArgv("work", "review", bed.id, "--work", "u2")) {
		t.Fatalf("pending read without an end: %q", next)
	}
	inv.owners.delivery.branchState = func(string, string) (intentBranchState, error) {
		return intentBranchState{}, errors.New("branch unavailable")
	}
	next, reason = inv.goalNextStep(bed.id)
	if !slices.Equal(next, inv.publicArgv("status", bed.id)) || reason == "" {
		t.Fatalf("unreadable branch: next=%q reason=%q", next, reason)
	}
	inv.owners.delivery.branchState = func(string, string) (intentBranchState, error) { return *state, nil }
	nextStepDesign(bed, "u1", "u2")
	(&deliveryBed{intentBed: bed.intentBed}).writeFile(filepath.Join(bed.root(), "plans", "designs", "next-units.md"), "# Broken\n\n- Kind: design\n")
	next, reason = inv.goalNextStep(bed.id)
	if slices.Contains(next, "land") || reason == "" {
		t.Fatalf("unreadable design: next=%q reason=%q", next, reason)
	}
}

func TestWorkReviewNextDeclaredUnitThenHandIn(t *testing.T) {
	t.Parallel()
	bed, inv, state := nextStepBed(t)
	nextStepDesign(bed, "u1", "u2", "u3")
	inv.owners.delivery.recordWriter = humanRecordWriter
	inv.owners.delivery.branchRead = func([]string) (branch.BranchReadResult, int, error) {
		return branch.BranchReadResult{State: "collected", Published: true}, 0, nil
	}
	inv.owners.delivery.publishRead = func(string, string, string) (branch.PublishReadResult, error) {
		return branch.PublishReadResult{State: "current"}, nil
	}
	for _, unit := range []string{"u2", "u3"} {
		code, built, _ := bed.work(append([]string{"work", "build", bed.id, "--work", unit, "--brief", bed.brief(unit+".md", "Build it.\n")}, workCheck...)...)
		expectOutcome(t, "build", code, built, intentConfirmed)
		run := resultData(t, built)["run"].(string)
		if err := (&launch.UnitRunner{Root: bed.unitRoot}).ReviewSubject(run, func(review launch.UnitReview, retain func(launch.UnitSubject) error) error {
			return retain(launch.UnitSubject{Round: 1, Commit: unit, Tip: unit, Published: unit, DiffDigest: review.DiffDigest})
		}); err != nil {
			t.Fatal(err)
		}
		if unit == "u3" {
			state.Status.Units = append(state.Status.Units, branch.UnitStatus{Unit: unit, Commit: unit, ReadState: "read clean"})
		}
		code, result := bed.runJSON(inv.owners, "work", "review", bed.id, "--work", unit)
		expectOutcome(t, "review", code, result, intentConfirmed)
		want := inv.publicArgv("work", "build", bed.id, "--work", "u3", "--brief", "FILE", "--check", "COMMAND")
		if unit == "u3" {
			want = inv.publicArgv("work", "land", bed.id)
		}
		if result.Next == nil || !slices.Equal(result.Next.Argv, want) {
			t.Fatalf("review %s next=%+v want=%q", unit, result.Next, want)
		}
	}
}
