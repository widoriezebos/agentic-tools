package main

import (
	"fmt"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
)

// The goal family's refusals print their remedy as the public goal budget
// G BOX. Each case below runs a family call that refuses, executes the remedy
// it printed through the public router, and executes the family form the
// remedy named before the object-action grammar (goal budget --id G ... BOX)
// on an identical ledger. Both must land the same act: the public action is
// bound to the owner the replaced spelling used, with no change in effect.

type budgetRemedyCase struct {
	name string
	// stopped starts the goal breach-stopped.
	stopped bool
	amend   func(*goal.GoalFile)
	// refuse is the family call whose refusal prints the remedy.
	refuse func(bed *intentBed, dependencies syncRequestDependencies) int
	// oldBox is the box the remedy named in the family form, given the box
	// the public remedy printed.
	oldBox func(t *testing.T, root, printed string) string
	// wantBox is the box the public remedy names.
	wantBox func(t *testing.T, root string) string
}

// runningWithoutBox is a claimed, running goal that carries no box: a claim
// migrated from before boxes, holding its stop capability.
func runningWithoutBox(file *goal.GoalFile) {
	file.Budget, file.Approved = nil, nil
	file.StopCapability = &goal.StopCapability{Generation: 2, Revision: 2, Machine: "mac-cli", ClaimEpoch: 1}
}

func tierlessRunningWithoutBox(file *goal.GoalFile) {
	runningWithoutBox(file)
	file.Tier, file.Risk = 0, nil
}

func tierlessQueued(file *goal.GoalFile) { makeQueued(file); file.Tier, file.Risk = 0, nil }

func literalBox(box string) func(*testing.T, string) string {
	return func(*testing.T, string) string { return box }
}

func oldLiteralBox(box string) func(*testing.T, string, string) string {
	return func(*testing.T, string, string) string { return box }
}

// samePrintedBox is a remedy whose box was already a complete compact box.
func samePrintedBox(_ *testing.T, _, printed string) string { return printed }

func tierBoxString(tier uint8) func(*testing.T, string) string {
	return func(t *testing.T, root string) string { return goalbudget.FormatBox(tierBox(t, root, tier)) }
}

func budgetRemedyCases() []budgetRemedyCase {
	resume := func(bed *intentBed, dependencies syncRequestDependencies) int {
		args := append(completeResumeArgs(bed.root()), "--fixture-human-authority")
		return runGoalResumeWithInputs(args, fixedFixtureGoalAuthority, bed.commandNow, dependencies, bed.binding)
	}
	approveWithOtherBudget := func(bed *intentBed, dependencies syncRequestDependencies) int {
		args := []string{"--root", bed.root(), "--id", bedGoal, "--budget", "tuple", "--by", "Wido", "--lineage", "m1", "--fixture-human-authority"}
		return runGoalApproveWithInputs(args, fixedFixtureGoalAuthority, bed.commandNow, dependencies, bed.binding)
	}
	budget := func(box string) func(*intentBed, syncRequestDependencies) int {
		return func(bed *intentBed, dependencies syncRequestDependencies) int {
			args := []string{"--root", bed.root(), "--id", bedGoal, "--by", "Wido", "--lineage", "m1", "--fixture-human-authority", box}
			return runGoalBudgetWithInputs(args, fixedFixtureGoalAuthority, bed.commandNow, dependencies, bed.binding)
		}
	}
	return []budgetRemedyCase{
		{
			// The fixture's resume without a fence: a migrated tierless claim.
			name: "resume of an unstopped tierless claim without a box", amend: tierlessRunningWithoutBox,
			refuse: resume, oldBox: oldLiteralBox("norm"), wantBox: tierBoxString(3),
		},
		{
			name: "resume of an unstopped tiered claim without a box", amend: runningWithoutBox,
			refuse: resume, oldBox: oldLiteralBox("norm"), wantBox: literalBox("norm"),
		},
		{
			name: "approve with a budget other than box, tierless queued goal", amend: tierlessQueued,
			refuse: approveWithOtherBudget, oldBox: oldLiteralBox("norm"), wantBox: tierBoxString(3),
		},
		{
			name: "approve with a budget other than box, tiered queued goal", amend: makeQueued,
			refuse: approveWithOtherBudget, oldBox: oldLiteralBox("norm"), wantBox: literalBox("norm"),
		},
		{
			name: "a new box for a breach-stopped goal", stopped: true,
			refuse: budget("8h/6/360m/2/3"), oldBox: oldLiteralBox("keep"), wantBox: literalBox("keep"),
		},
		{
			name: "an incomplete compact box is completed", amend: makeQueued,
			refuse: budget("8h/6"), oldBox: samePrintedBox,
		},
	}
}

func TestIntentBudgetRemediesMatchTheFamilyForm(t *testing.T) {
	t.Parallel()
	for _, test := range budgetRemedyCases() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			refused := newIntentBed(t, test.stopped, test.amend)
			report := &ownerReport{}
			dependencies := refused.dependencies()
			dependencies.report = report
			if code := test.refuse(refused, dependencies); code == 0 || report.refusal == nil || report.refusal.remedy.command == "" {
				t.Fatalf("the family call did not refuse with a command: code=%d report=%+v", code, report.refusal)
			}
			if refused.repo.publications != 0 {
				t.Fatalf("the refusal published %d times", refused.repo.publications)
			}
			printed := shellWords(report.refusal.remedy.command)
			root := refused.root()
			if len(printed) < 5 || !slices.Equal(printed[:4], []string{"metasystem", "goal", "budget", bedGoal}) ||
				test.wantBox != nil && printed[len(printed)-1] != test.wantBox(t, root) {
				t.Fatalf("the remedy is not the public goal budget %s BOX with the expected box: %q", bedGoal, report.refusal.remedy.command)
			}

			// The printed public command on a fresh, identical ledger.
			public := newIntentBed(t, test.stopped, test.amend)
			publicArgs := rehome(printed[1:], root, public.root())
			code, result := public.runJSON(public.owners(), publicArgs...)
			if code != 0 || result.Outcome != intentConfirmed {
				t.Fatalf("the printed remedy %q did not land: code=%d %+v", report.refusal.remedy.command, code, result)
			}

			// The family form the remedy named before the grammar changed.
			family := newIntentBed(t, test.stopped, test.amend)
			familyArgs := append([]string{"--id", bedGoal}, rehome(printed[4:len(printed)-1], root, family.root())...)
			familyArgs = append(familyArgs, test.oldBox(t, root, printed[len(printed)-1]))
			familyReport := &ownerReport{}
			familyDependencies := family.dependencies()
			familyDependencies.report = familyReport
			if code := runGoalBudgetWithInputs(familyArgs, fixedFixtureGoalAuthority, family.commandNow, familyDependencies, family.binding); code != 0 {
				t.Fatalf("the family form goal budget %q did not land: code=%d refusal=%+v failure=%v", familyArgs, code, familyReport.refusal, familyReport.failure)
			}

			if got, want := budgetEffect(public), budgetEffect(family); got != want {
				t.Fatalf("the public remedy and the family form differ:\npublic: %s\nfamily: %s", got, want)
			}
		})
	}
}

// rehome swaps one bed's root for another's in a printed argument vector.
func rehome(args []string, from, to string) []string {
	out := slices.Clone(args)
	for index, arg := range out {
		if arg == from {
			out[index] = to
		}
	}
	return out
}

// budgetEffect is what a budget act leaves on the goal: its state, box,
// approval, fence and the newest history line's verb and actor.
func budgetEffect(bed *intentBed) string {
	bed.t.Helper()
	file := bed.goalFile(bedGoal)
	approved := ""
	if file.Approved != nil {
		approved = file.Approved.By + "/" + file.Approved.Authority
	}
	last := file.History[len(file.History)-1]
	return fmt.Sprintf("state=%s box=%s approved=%s fenced=%t last=%s/%s publications=%d",
		file.State, goalBoxString(file), approved, file.StopFence != nil, last.Verb, last.Actor, bed.repo.publications)
}
