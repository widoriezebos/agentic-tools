package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegation/fake"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestEngineHostCancelUnitLaunchReturnsTheEndedRecord(t *testing.T) {
	t.Parallel()
	for _, state := range []launch.State{launch.Completed, launch.Running} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()
			now := time.Date(2026, 10, 24, 22, 15, 0, 0, time.UTC)
			ref := identity.Ref{Pid: 71, StartedAtSec: 70}
			store := launch.Store{Root: t.TempDir()}
			record := launch.Record{ID: "unit-reader", State: state, Child: &ref, ProcessGroup: &ref, StartedAt: now.Add(-time.Minute).Format(time.RFC3339Nano)}
			if state.Terminal() {
				record.FinishedAt = now.Add(-time.Second).Format(time.RFC3339Nano)
			}
			if err := store.Create(record); err != nil {
				t.Fatal(err)
			}
			dir, _ := store.StateDir(record.ID)
			before, err := os.ReadFile(filepath.Join(dir, "record.json"))
			if err != nil {
				t.Fatal(err)
			}
			sleeps := 0
			manager := &launch.Manager{Store: store, Now: func() time.Time { return now }, Sleep: func(time.Duration) { sleeps++ }, Prober: workProber{}, Processes: workProcesses{}}
			outcome, err := (engineHost{launches: manager}).CancelUnitLaunch(record.ID)
			wantState, wantEnd := string(state), record.FinishedAt
			if state == launch.Running {
				wantState, wantEnd = "cancelled", now.Format(time.RFC3339Nano)
			}
			if err != nil || outcome.State != wantState || outcome.StartedAt != record.StartedAt || outcome.FinishedAt != wantEnd || !outcome.Executed {
				t.Fatalf("cancel returned %+v, %v", outcome, err)
			}
			after, err := os.ReadFile(filepath.Join(dir, "record.json"))
			if err != nil {
				t.Fatal(err)
			}
			if state.Terminal() && (!bytes.Equal(before, after) || sleeps != 0) {
				t.Fatal("an ended launch was rewritten or waited on")
			}
			if state == launch.Running && sleeps != 1 {
				t.Fatalf("a live launch did not wait for death proof: %d", sleeps)
			}
		})
	}
}

func TestAUnitBuildCollectsAfterItsLaunchWasStopped(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.starter.hold = "build"
	brief := b.brief("build.md", "Build the unit.\n")
	code, result, _ := b.work(append([]string{"work", "build", b.id, "stopped", "--brief", brief, "--lines", "5"}, workCheck...)...)
	if code != 3 {
		t.Fatalf("held build: %d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	unit, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	id := unit.Rounds[0].Steps[0].LaunchID
	execution, err := b.manager.Store.Update(id, func(record *launch.Record) error {
		zero := 0
		record.State, record.ExitCode, record.FinishedAt = launch.Completed, &zero, b.manager.Now().Format(time.RFC3339Nano)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	doubles := fake.NewSet()
	ports := doubles.Ports()
	ports.Host = engineHost{launches: b.manager}
	lifecycle, err := delegation.New(delegation.Config{Root: b.root(), RepoScope: b.root()}, ports)
	if err != nil {
		t.Fatal(err)
	}
	var diagnostic bytes.Buffer
	cancelled := lifecycle.Run(context.Background(), delegation.Request{Env: delegation.Env{DelegateInternal: true}, Stderr: &diagnostic}, []string{"cancel", "--job", id})
	if cancelled.ExitCode != 0 {
		t.Fatalf("cancel: %d %s", cancelled.ExitCode, &diagnostic)
	}
	reservation, err := dispatchcore.ReadRecordObject(filepath.Join(b.root(), "artifacts/agents/jobs", id+".json"))
	if err != nil || reservation["status"] != "completed" || reservation["unitExecuted"] != true || reservation["startedAt"] != execution.StartedAt || reservation["endedAt"] != execution.FinishedAt {
		t.Fatalf("cancel did not retain the build outcome: %+v %v", reservation, err)
	}
	b.starter.hold = ""
	code, collected, _ := b.work("work", "wait", b.id, "--work", "stopped")
	if code != 0 {
		t.Fatalf("collection after cancel: %d %+v", code, collected)
	}
	projection := dispatchcore.ProjectBudget(b.root(), b.goalFile(b.id), b.manager.Now())
	if projection.Status != dispatchcore.BudgetKnown {
		t.Fatalf("collection left spending unknown: %+v", projection)
	}
}
