package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func goalBudgetSeat(t *testing.T, root string, file *goal.GoalFile) {
	t.Helper()
	fleetWriteJSON(t, filepath.Join(root, "artifacts/agents/steward/seats", "budget-"+file.Id+".json"), steward.SeatRecord{
		Schema: 1, LaunchID: "budget-seat", Goal: file.Id, StartedAt: file.Claimed.At,
	})
}

type ownWorkClockStarter struct {
	unitProofStarter
	clock *workClock
}

func (s ownWorkClockStarter) StartSupervisor(id, state string) (identity.Ref, error) {
	s.clock.mu.Lock()
	s.clock.now = s.clock.now.Add(time.Minute)
	s.clock.mu.Unlock()
	return s.unitProofStarter.StartSupervisor(id, state)
}

func TestGoalStatusElapsedIsTheGoalsOwnWork(t *testing.T) {
	t.Parallel()
	bed := newWorkBedWith(t, workApprovedBox)
	clock := &workClock{now: bed.manager.Now()}
	bed.manager.Now = clock.Now
	bed.manager.Prober = unitProofProber{exact: identity.Exact{Pid: int64(os.Getpid()), StartedAt: clock.Now()}}
	bed.manager.Supervisor = ownWorkClockStarter{unitProofStarter{bed: bed, t: t}, clock}
	owners := bed.workOwners()
	owners.commandNow = func(string) (time.Time, error) { return clock.Now(), nil }
	owners.lookupEnv = func(key string) (string, bool) { return bed.root(), key == "METASYSTEM_SUPERVISION_REGISTRY_HOME" }
	// The runner and goal status read the same fixture launch store.
	bed.manager.Store.Root = filepath.Join(testprovider.Register(t, bed.root()), "launch")
	check := filepath.Join(t.TempDir(), "check")
	if err := testexec.WriteFile(check, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	bed.declaredCheap = shellCommand([]string{check})
	brief := bed.brief("own-work.md", "Build the unit.\n\nRead each round: yes\n")
	code, built := bed.runJSON(owners, "work", "build", bed.id, "current", "--brief", brief, "--lines", "5")
	bed.recordReadDirs(built)
	if code != 0 {
		t.Fatalf("build: %d %+v", code, built)
	}
	unit, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(resultData(t, built)["run"].(string))
	if err != nil || len(unit.Rounds) != 1 || len(unit.Rounds[0].Steps) != 3 {
		t.Fatalf("unit did not collect build/check/read: %+v %v", unit, err)
	}
	for _, step := range unit.Rounds[0].Steps {
		record, err := bed.manager.Store.Read(step.LaunchID)
		if err != nil || !record.State.Terminal() || record.Goal != bed.id {
			t.Fatalf("uncollected goal launch: %+v %v", record, err)
		}
	}
	projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), clock.Now(), filepath.Dir(bed.manager.Store.Root))
	if projection.Status != dispatchcore.BudgetKnown || projection.ElapsedLegacy {
		t.Fatalf("status fixture has no own-work projection: %+v", projection)
	}
	spent := func() string {
		t.Helper()
		code, text, stderr := bed.run(owners, "goal", "status", bed.id)
		match := regexp.MustCompile(`spent: elapsed ([^ ]+) of`).FindStringSubmatch(text)
		if code != 0 || len(match) != 2 || match[1] == "0s" || strings.Contains(text, "elapsed counts wall time") {
			t.Fatalf("no own work in status: %d %s %s", code, text, stderr)
		}
		t.Log(text)
		return match[1]
	}
	before := spent()
	clock.mu.Lock()
	clock.now = clock.now.Add(14 * time.Hour)
	clock.mu.Unlock()
	if after := spent(); after != before {
		t.Fatalf("idle night spent the box: before=%s after=%s", before, after)
	}
}
