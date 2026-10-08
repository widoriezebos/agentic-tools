package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type comparisonAccountingStarter struct {
	unitProofStarter
	dropSupervisor bool
}

func (s *comparisonAccountingStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	ref, err := s.unitProofStarter.StartSupervisor(id, state)
	record, readErr := s.bed.manager.Store.Read(id)
	if err == nil && readErr == nil && record.Kind == "proof" && s.dropSupervisor {
		s.bed.manager.Supervisor = nil
	}
	return ref, err
}

func comparisonAccountingBed(t *testing.T, limit uint64, dropSupervisor bool) (*workBed, []string) {
	t.Helper()
	bed := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.Budget.ReservedJobMinutesLimit = limit
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	bed.manager.Adapters["plain-exec"] = launch.PlainExec{}
	bed.manager.Supervisor = &comparisonAccountingStarter{unitProofStarter: unitProofStarter{bed: bed, t: t}, dropSupervisor: dropSupervisor}
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		units := owners.units
		owners.units = func(layout stateroot.Layout) *launch.UnitRunner {
			runner := units(layout)
			runner.Git = &unitProofBaseGit{workGit: workGit{bed}}
			return runner
		}
	}
	if err := os.WriteFile(filepath.Join(bed.worktree, "red"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	check := filepath.Join(t.TempDir(), "check")
	if err := testexec.WriteFile(check, []byte("#!/bin/sh\nif [ -f red ]; then exit 1; fi\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	brief := bed.brief("comparison.md", "Build the unit.\n")
	bed.declaredCheap = shellCommand([]string{check})
	return bed, []string{"work", "build", bed.id, "comparison", "--brief", brief, "--lines", "5"}
}

func TestIntentUnitComparisonStartRefusalSettlesReservation(t *testing.T) {
	t.Parallel()
	bed, args := comparisonAccountingBed(t, 240, true)
	code, result, _ := bed.work(args...)
	if code != 1 {
		t.Fatalf("comparison start refusal: %d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Rounds) != 1 || len(record.Rounds[0].Steps) < 2 {
		t.Fatalf("comparison was not reached: %+v", record)
	}
	step := record.Rounds[0].Steps[1]
	if step.Comparison == nil || !strings.Contains(step.Reason, "supervisor is unavailable") {
		t.Fatalf("the comparison did not reach the real start refusal: %+v", step)
	}
	if _, err := bed.manager.Store.Read(step.Comparison.LaunchID); !os.IsNotExist(err) {
		t.Fatalf("start refusal invented an execution: %v", err)
	}
	for replay := 0; replay < 2; replay++ {
		projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
		if projection.Status != dispatchcore.BudgetKnown || projection.Attempts != 1 || projection.ActiveJobs != 0 || projection.ReservedJobMinutes != 2 || projection.ObservedJobMinutes != 2 {
			t.Fatalf("unexecuted comparison remains charged: %+v", projection)
		}
		code, result, _ = bed.work("work", "build", "run:"+run)
		if code != 1 || !strings.Contains(resultWords(result), "stopped unclassified") {
			t.Fatalf("advance could not collect the retained stop: %d %+v", code, result)
		}
		after, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
		if err != nil || len(after.Rounds) != 1 || len(after.Revisions) != 0 || after.Rounds[0].Cause != "unclassified" || after.Rounds[0].Stop == nil {
			t.Fatalf("advance spent a correction: %+v %v", after, err)
		}
	}
}

func TestIntentUnitComparisonBudgetHoldResumesAfterPersonBudget(t *testing.T) {
	t.Parallel()
	bed, args := comparisonAccountingBed(t, 121, false)
	code, refused, _ := bed.work(args...)
	if code != 1 || refused.Next == nil || !slices.Equal(refused.Next.Argv, []string{"metasystem", "goal", "budget", bed.id, "BOX"}) || !strings.Contains(resultWords(refused), "BUDGET_REFUSED") {
		t.Fatalf("comparison budget refusal became a correction: %d %+v", code, refused)
	}
	run := resultData(t, refused)["run"].(string)
	before, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil || before.State != "running" || before.Rounds[0].Stop != nil || len(before.Revisions) != 0 || len(bed.starter.launched()) != 2 {
		t.Fatalf("budget hold completed or corrected the round: %+v %v", before, err)
	}
	remedy := append(append([]string(nil), refused.Next.Argv[1:4]...), "4h/4/240m/1/2", "--by", "Wido", "--fixture-human-authority", "--lineage", "m1")
	if code, raised := bed.runJSON(bed.owners(), remedy...); code != 0 {
		t.Fatalf("person's budget remedy failed: %d %+v", code, raised)
	}
	code, result, _ := bed.work("work", "build", "run:"+run)
	if code != 1 || resultData(t, result)["outcome"] != "proof-red" {
		t.Fatalf("comparison did not resume: %d %+v", code, result)
	}
	after, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil || len(after.Rounds) != 1 || len(after.Revisions) != 0 || after.Rounds[0].Cause != "own" || !after.Rounds[0].Steps[1].Comparison.Verified || len(bed.starter.launched()) != 3 {
		t.Fatalf("resumption rebuilt or corrected the unit: %+v %v", after, err)
	}
	projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if projection.Status != dispatchcore.BudgetKnown || projection.Attempts != 1 || projection.ActiveJobs != 0 || projection.ReservedJobMinutes != 1 {
		t.Fatalf("resumed comparison was not settled: %+v", projection)
	}
}
