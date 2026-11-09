package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

type boxOverrunStarter struct {
	unitProofStarter
	clock *workClock
	past  time.Time
}

func (s boxOverrunStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	record, err := s.bed.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind == "build" {
		s.clock.mu.Lock()
		s.clock.now = s.past
		s.clock.mu.Unlock()
	}
	return s.unitProofStarter.StartSupervisor(id, state)
}

func TestUnitPastItsBoxGetsItsCheckAndRead(t *testing.T) {
	t.Parallel()
	bed := newWorkBedWith(t, func(file *goal.GoalFile) {
		workApprovedBox(file)
		file.Budget.ElapsedLimit = "1h"
		file.Approved.Digest = goal.ApprovalDigest(file.Intent, file.Tier, *file.Budget, file.Risk)
	})
	// The seat keeps working as its build crosses the box.
	goalBudgetSeat(t, bed.root(), bed.goalFile(bed.id))
	clock := &workClock{now: bed.manager.Now()}
	claim, err := time.Parse(time.RFC3339, bed.goalFile(bed.id).Claimed.At)
	if err != nil {
		t.Fatal(err)
	}
	bed.manager.Now = clock.Now
	bed.manager.Prober = unitProofProber{exact: identity.Exact{Pid: int64(os.Getpid()), StartedAt: clock.Now()}}
	bed.manager.Supervisor = boxOverrunStarter{unitProofStarter{bed: bed, t: t}, clock, claim.Add(75 * time.Minute)}
	owners := bed.workOwners()
	owners.commandNow = func(string) (time.Time, error) { return clock.Now(), nil }
	check := filepath.Join(t.TempDir(), "check")
	if err := testexec.WriteFile(check, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	bed.declaredCheap = shellCommand([]string{check})
	brief := bed.brief("box.md", "Build the unit.\n\nRead each round: yes\n")
	code, result := bed.runJSON(owners, "work", "build", bed.id, "current", "--brief", brief, "--lines", "5")
	bed.recordReadDirs(result)
	if code != 0 || strings.Contains(resultWords(result), "BUDGET_REFUSED") || strings.Contains(resultWords(result), "a person sets the goal's budget") {
		t.Fatalf("the current unit was held for its budget: %d %s %v", code, result.Summary, result.Details)
	}
	run := resultData(t, result)["run"].(string)
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil || len(record.Rounds) != 1 || len(record.Rounds[0].Steps) != 3 || record.Rounds[0].Steps[2].State != launch.StepPassed || len(record.Rounds[0].Reads) != 1 {
		t.Fatalf("the unit did not collect its check and read: state=%s rounds=%d %v", record.State, len(record.Rounds), err)
	}
	checkID := record.Rounds[0].Steps[1].LaunchID
	body, err := os.ReadFile(filepath.Join(bed.root(), "artifacts", "agents", "jobs", checkID+".json"))
	var reservation map[string]any
	if err != nil || json.Unmarshal(body, &reservation) != nil || reservation["elapsedOverrunMin"] != float64(15) {
		t.Fatalf("the check did not record its overrun: %s %v", body, err)
	}
	code, text, stderr := bed.run(owners, "goal", "show", bed.id)
	if code != 0 || !strings.Contains(text, "past its box by 15m for run "+run) {
		t.Fatalf("goal show lost the overrun: %d %s %s", code, text, stderr)
	}
	code, result = bed.runJSON(owners, "work", "build", bed.id, "other", "--brief", brief, "--lines", "5")
	bed.recordReadDirs(result)
	if code != 3 || !strings.Contains(result.Summary, "the worktree belongs to run "+run) {
		t.Fatalf("a second unit did not retain the current run's worktree: %d %s %v", code, result.Summary, result.Details)
	}
}
