package steward

// Witnesses of the fourth B3 read (the reader's probes; each failed on
// 5d7057808).

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// U1: a unit's goal is reopened between the plan and the apply of one
// machine pass. Apply "rereads and rejudges", but unitGoalEnded caches
// the ledger view per root for the whole pass (ledgerView.get reads once),
// so the apply judges the reopened goal concluded and removes the unit
// record of an open goal (and its named entry).
func TestB3Read4UnitOfGoalReopenedBetweenPlanAndApplyRemoved(t *testing.T) {
	t.Parallel()
	bed := newStaleBed(t)
	units := filepath.Join(bed.root, "home", "unit")
	id := "unit-1"
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	at := now.Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano)
	record := launch.UnitRunRecord{ID: id, Unit: "u", Goal: "g", Worktree: bed.inst, State: "awaiting-judgement",
		Rounds: []launch.UnitRound{{Number: 1, Steps: []launch.UnitStep{{Name: "read", LaunchID: id + "-r1-s1", State: launch.StepPassed, StartedAt: at, FinishedAt: at}}}}}
	data, _ := json.Marshal(record)
	os.MkdirAll(filepath.Join(units, id), 0o700)
	os.WriteFile(filepath.Join(units, id, "run.json"), data, 0o600)
	os.WriteFile(filepath.Join(units, id, "payload"), make([]byte, 1<<16), 0o600)

	reopened := false
	projections := 0
	ledgerFor := func(string) *ledgerView {
		return &ledgerView{project: func() (goal.Projection, error) {
			projections++
			if reopened {
				return goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{"g": {}}}}, nil
			}
			return goal.Projection{Tree: &goal.TreeGoals{Done: map[string]*goal.GoalFile{"g": {}}}}, nil
		}}
	}
	class := &launch.UnitRetention{Root: units, Target: 1, Keep: 14 * 24 * time.Hour, GoalEnded: unitGoalEnded(ledgerFor)}
	pass := &diskstore.Pass{Now: now}
	items, err := class.Plan(context.Background(), pass)
	if err != nil || len(items) != 1 || items[0].Verdict.Decision != diskstore.Release {
		t.Fatalf("plan: %+v %v", items, err)
	}
	reopened = true // `metasystem goal reopen g` lands between the plan and the apply
	verdict := class.Apply(context.Background(), pass, items[0])
	_, statErr := os.Stat(filepath.Join(units, id))
	t.Logf("apply %+v; ledger projections %d; unit present err=%v", verdict, projections, statErr)
	if statErr != nil {
		t.Fatalf("the unit record of goal g, reopened before the apply, was removed (the apply read the ledger %d time(s) in all)", projections)
	}
}
