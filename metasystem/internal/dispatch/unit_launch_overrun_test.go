package dispatch

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

func TestCurrentUnitFinishesPastItsBox(t *testing.T) {
	t.Parallel()
	root, file, open := unitRetryGoal(t, 10)
	file.Budget.ElapsedLimit = "1h"
	now := time.Date(2026, 8, 28, 9, 15, 45, 0, time.UTC)
	reserve := func(id, run string, round int, at time.Time, retry bool) error {
		return ReserveUnitLaunch(root, id, run, file, round, 5, at, retry)
	}
	refused := func(err error, field string) {
		t.Helper()
		if refusal.CodeOf(err) != "BUDGET_REFUSED" || !strings.Contains(refusal.DetailOf(err), field) {
			t.Fatalf("want refusal on %s, got %v", field, err)
		}
	}
	_, buildPath, _ := paths(root, "build")
	buildRecord, err := readObject(buildPath)
	if _, present := buildRecord["elapsedOverrunMin"]; err != nil || present {
		t.Fatalf("an open-box launch recorded an overrun: %v %v", buildRecord, err)
	}
	if err := reserve("box-edge", "run", 1, now.Add(-15*time.Minute-45*time.Second), false); err != nil {
		t.Fatal(err)
	}
	_, edgePath, _ := paths(root, "box-edge")
	edge, err := readObject(edgePath)
	if minutes, ok := numInt(edge["elapsedOverrunMin"]); err != nil || !ok || minutes != 0 {
		t.Fatalf("a launch at the box did not record zero overrun: %v %v", edge, err)
	}
	if err := reserve("unexecuted", "R3", 1, open, false); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileUnitLaunch(root, "unexecuted", "R3", file.Id, file.Claimed.Revision, "failed", "environment", open.Format(time.RFC3339Nano), open.Format(time.RFC3339Nano), false); err != nil {
		t.Fatal(err)
	}
	if err := reserve("check", "run", 1, now, false); err != nil {
		t.Fatalf("the current unit's check was refused: %v", err)
	}
	_, path, _ := paths(root, "check")
	record, err := readObject(path)
	minutes, readable := numInt(record["elapsedOverrunMin"])
	if err != nil || !readable || minutes != 15 {
		t.Fatalf("check overrun = %v, %v; want 15 whole minutes", record, err)
	}
	if err := reserve("check", "run", 1, now.Add(time.Minute), false); err != nil {
		t.Fatalf("joining the check was refused: %v", err)
	}
	joined, err := readObject(path)
	if minutes, ok := numInt(joined["elapsedOverrunMin"]); err != nil || !ok || minutes != 15 {
		t.Fatalf("joining changed the recorded overrun: %v %v", joined, err)
	}
	p := ProjectBudget(root, file, now)
	if p.Status != BudgetKnown || p.ElapsedOverrunMinutes != 15 || p.ElapsedOverrunRun != "run" || p.OverrunWords() != "past its box by 15m for run run" {
		t.Fatalf("overrun projection = %+v, words %q", p, p.OverrunWords())
	}
	for _, request := range []struct {
		id    string
		round int
	}{{"read", 1}, {"correction", 2}} {
		if err := reserve(request.id, "run", request.round, now, false); err != nil {
			t.Fatalf("the current unit's %s was refused: %v", request.id, err)
		}
	}
	refused(reserve("new-build", "R2", 1, now, false), "elapsedLimit")
	refused(reserve("unexecuted-check", "R3", 1, now, false), "elapsedLimit")
	file.Budget.AttemptLimit = 2
	err = reserve("retry", "run", 1, now, true)
	refused(err, "attemptLimit")
	if strings.Contains(refusal.DetailOf(err), "elapsedLimit") {
		t.Fatalf("the current run's elapsed limit was not waived: %v", err)
	}
	file.Budget.AttemptLimit = 10
	file.StopFence = &goal.StopFence{StopID: "stopped"}
	refused(reserve("fenced", "run", 1, now, false), "")
	file.StopFence = nil
	late := now.Add(time.Hour)
	if err := reserve("late-check", "run", 1, late, false); err != nil {
		t.Fatalf("the elapsed breach stranded the current unit: %v", err)
	}
	if p := ProjectBudget(root, file, late); liveStopReason(p) != goal.StopReasonElapsedLimit || p.ElapsedOverrunMinutes != 75 || p.ElapsedOverrunRun != "run" {
		t.Fatalf("the elapsed breach no longer stops the goal: %+v", p)
	}
}
