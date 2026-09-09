package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestConclusionIsTheArchiveAct(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	e := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	line := func(at time.Time, opid, verb, actor string, targets []string, reason string) goal.HistoryLine {
		return goal.HistoryLine{
			At: at.Format(time.RFC3339), Opid: opid, Verb: verb, Actor: actor,
			Targets: targets, Keep: -1, Reason: reason,
		}
	}
	c := &goal.GoalFile{
		Id: "c", State: goal.StateClaimed, Priority: 1, Sequence: 2,
		Claimed: &goal.ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: t2.Format(time.RFC3339)},
		History: []goal.HistoryLine{
			line(t0, "c-open", "open", "mac-a+lin-1", nil, ""),
			line(t1, "c-approve", "approve", "human:wido", nil, ""),
			line(t2, "c-claim", "claim", "mac-a+lin-1", nil, ""),
			line(t3, "b-done", "done", "human:wido", []string{"b", "c"}, "priority-order from=1:3 to=1:2"),
		},
	}
	d := &goal.GoalFile{
		Id: "d", State: goal.StateApproved,
		History: []goal.HistoryLine{
			line(t0, "d-open", "open", "mac-a+lin-1", nil, ""),
			line(t1, "d-claim", "claim", "mac-a+lin-1", nil, ""),
			line(t2, "d-done", "done", "human:wido", []string{"d"}, ""),
			line(t2.Add(time.Hour), "d-reopen", "reopen", "human:wido", nil, ""),
			line(t3, "b-done-d", "done", "human:wido", []string{"b", "d"}, "priority-order from=1:3 to=1:2"),
		},
	}
	b := &goal.GoalFile{
		Id: "b", State: goal.StateDone,
		History: []goal.HistoryLine{
			line(t0, "b-open", "open", "mac-a+lin-1", nil, ""),
			line(t2, "b-claim", "claim", "mac-a+lin-1", nil, ""),
			line(t3, "b-done", "done", "human:wido", []string{"b", "c"}, ""),
		},
	}
	splitParent := &goal.GoalFile{
		Id: "e", State: goal.StateDone, Priority: 1, Sequence: 2,
		OpenedAt: e.Format(time.RFC3339), Conclude: "decomposed into arc e: e-one, e-two",
		History: []goal.HistoryLine{
			line(e, "e-open", "open", "mac-a+lin-1", nil, ""),
			line(t0, "e-claim", "claim", "mac-a+lin-1", nil, ""),
			line(t2, "b-done-e", "done", "human:wido", []string{"b", "e"}, "priority-order from=1:3 to=1:2"),
			line(t3, "e-split", "split", "mac-a+lin-1", []string{"e", "e-one", "e-two", "g"}, ""),
		},
	}
	splitSurvivor := &goal.GoalFile{
		Id: "g", State: goal.StateQueued, Priority: 1, Sequence: 2,
		History: []goal.HistoryLine{
			line(t0, "g-open", "open", "mac-a+lin-1", nil, ""),
			line(t3, "e-split", "split", "mac-a+lin-1", []string{"e", "e-one", "e-two", "g"}, "priority-order from=1:3 to=1:2"),
		},
	}
	w := world{
		Goals: map[string]goalRecord{
			"b": {File: b}, "c": {File: c}, "d": {File: d}, "e": {File: splitParent}, "g": {File: splitSurvivor},
		},
		Landings: []landingCommit{
			{At: t1.Add(12 * time.Hour), Goals: map[string]bool{"d": true}},
			{At: t2.Add(12 * time.Hour), Goals: map[string]bool{"b": true}},
			{At: t0.Add(12 * time.Hour), Goals: map[string]bool{"e": true}},
			{At: t2.Add(12 * time.Hour), Goals: map[string]bool{"c": true}},
		},
		GoalCoverage: Coverage{Source: "goals", Found: 5}, LandingCoverage: Coverage{Source: "landings", Found: 4},
	}
	period := Period{Instant: t3.AddDate(0, 0, 1)}
	limits := thresholds{Waiting: shareLimit{Raw: "0.5", Value: 0.5}}
	hasExactDetail := func(row metricRow, want string) bool {
		for _, item := range row.Details {
			if item.Text == want {
				return true
			}
		}
		return false
	}

	t.Run("waiting-row", func(t *testing.T) {
		row := computeWaiting(w, period, "c", limits)
		if row.Value != "unavailable" {
			t.Fatalf("Value = %q; want unavailable", row.Value)
		}
		if !hasExactDetail(row, "lifecycle incomplete: goal=c epochs=0") {
			t.Fatalf("Details = %+v; want exact incomplete lifecycle detail", row.Details)
		}
	})

	t.Run("reopened-then-survived", func(t *testing.T) {
		row := computeWaiting(w, period, "d", limits)
		if row.Value != "unavailable" {
			t.Fatalf("Value = %q; want unavailable", row.Value)
		}
		if !hasExactDetail(row, "lifecycle incomplete: goal=d epochs=0") {
			t.Fatalf("Details = %+v; want exact incomplete lifecycle detail", row.Details)
		}
	})

	t.Run("concluded-in-window", func(t *testing.T) {
		if ConcludedInWindow(c, t3.Add(-time.Hour), t3.Add(time.Hour)) {
			t.Fatal("live survivor was reported concluded in the window")
		}
	})

	t.Run("split-parent", func(t *testing.T) {
		row := computeWaiting(w, period, "e", limits)
		want := "e building_hours=12.000 proving_hours=36.000 waiting_share=0.750 epochs=1"
		if row.Value != want {
			t.Fatalf("Value = %q; want %q", row.Value, want)
		}
	})

	t.Run("split-parent-in-window", func(t *testing.T) {
		if ConcludedInWindow(splitParent, t2.Add(-time.Hour), t2.Add(time.Hour)) {
			t.Fatal("split parent was reported concluded at its neighbour's conclusion")
		}
		if !ConcludedInWindow(splitParent, t3.Add(-time.Hour), t3.Add(time.Hour)) {
			t.Fatal("split parent was not reported concluded at its split")
		}
	})

	t.Run("split-parent-selected-by-its-split", func(t *testing.T) {
		window := Period{Start: t3.Add(-time.Hour), End: t3.Add(time.Hour), Instant: t3.AddDate(0, 0, 1)}
		row := computeWaiting(w, window, "", limits)
		want := "e building_hours=12.000 proving_hours=36.000 waiting_share=0.750 epochs=1"
		if !strings.Contains(row.Value, want) {
			t.Fatalf("Value = %q; want it to contain %q", row.Value, want)
		}
	})

	t.Run("survivor-of-a-split", func(t *testing.T) {
		row := computeWaiting(w, period, "g", limits)
		if row.Value != "unavailable" {
			t.Fatalf("Value = %q; want unavailable", row.Value)
		}
		if !hasExactDetail(row, "lifecycle incomplete: goal=g epochs=0") {
			t.Fatalf("Details = %+v; want exact incomplete lifecycle detail", row.Details)
		}
	})

	t.Run("departed-neighbour-still-concludes", func(t *testing.T) {
		row := computeWaiting(w, period, "b", limits)
		want := "b building_hours=12.000 proving_hours=12.000 waiting_share=0.500 epochs=1"
		if row.Value != want {
			t.Fatalf("Value = %q; want %q", row.Value, want)
		}
	})
}
