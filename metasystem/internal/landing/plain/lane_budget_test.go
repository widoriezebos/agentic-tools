package plain

import (
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestLaneBudgetHandInReturnAndPushUseAcceptedRecords(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	home := t.TempDir()
	sha := b.seat("seat-a", "goal-a")
	if _, _, err := HandIn(b.install, Line{Goal: "goal-a", SHA: sha, At: bedNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	read := func(paused bool, minutes float64) {
		t.Helper()
		record, err := goal.ReadLaneBudget(b.install, "goal-a", bedNow.Add(-7*time.Hour), bedNow.Add(3*time.Hour))
		if err != nil || record.PausedAt.IsZero() == paused || record.PausedMinutes != minutes {
			t.Fatalf("accepted lane records: %+v %v", record, err)
		}
	}
	read(true, 180)
	if _, changed, err := ReturnProven(b.install, "goal-a", "other", "back to seat", true, "fixture", bedNow.Add(2*time.Hour), ProveSeams{BudgetHome: home, Person: &ActProvenance{Kind: "return", Person: "fixture"}}); err != nil || !changed {
		t.Fatalf("return: %t %v", changed, err)
	}
	read(false, 120)
	assertClosedLaneBudget(t, home, "goal-a", 120)
	if _, _, err := HandIn(b.install, Line{Goal: "goal-a", SHA: sha, At: bedNow.Add(2*time.Hour + 30*time.Minute).Format(time.RFC3339), Again: true}); err != nil {
		t.Fatal(err)
	}
	b.merge("goal-a")
	b.prove(b.greenScript)
	// Push without a selected batch must record its goal members too.
	if _, err := PushChecked(b.install, b.checkout, bedNow.Add(3*time.Hour), nil, ProveSeams{BudgetHome: home}); err != nil {
		t.Fatal(err)
	}
	read(false, 150)
	assertClosedLaneBudget(t, home, "goal-a", 150)
	// A retry after publication joins the same closed stays.
	if _, err := PushChecked(b.install, b.checkout, bedNow.Add(4*time.Hour), nil, ProveSeams{BudgetHome: home}); err != nil {
		t.Fatal(err)
	}
	assertClosedLaneBudget(t, home, "goal-a", 150)
	layout, err := lane.NewLayout(b.checkout)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(home, layout, "fixture", bedNow); err != nil {
		t.Fatal(err)
	}
	unset, err := lane.Unset(home, "fixture", bedNow.Add(4*time.Hour), false, lane.UnsetSeams{Settle: func(lane.Layout) (lane.Settlement, error) { return lane.Settlement{}, nil }})
	if err != nil || !unset.Unregistered {
		t.Fatalf("unset: %+v %v", unset, err)
	}
	assertClosedLaneBudget(t, home, "goal-a", 150)
	other := newBed(t)
	layout, err = lane.NewLayout(other.checkout)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := lane.Register(home, layout, "fixture", bedNow.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(b.install, "artifacts", "agents", "landing")); err != nil {
		t.Fatal(err)
	}
	assertClosedLaneBudget(t, home, "goal-a", 150)
}

func TestLaneBudgetOlderPushUsesThePublishedCommit(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	old := b.originMain()
	sha := b.seat("seat-a", "goal-a")
	if _, _, err := HandIn(b.install, Line{Goal: "goal-a", SHA: sha, At: bedNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	head := b.merge("goal-a")
	// Byte copy of the landing installation's first push, whose shape
	// predates batch-members. Only its commit coordinates and clock are
	// rebound to this test's real Git graph.
	data, err := os.ReadFile("testdata/lane-budget-legacy-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var push Pushed
	if err := json.Unmarshal(data, &push); err != nil {
		t.Fatal(err)
	}
	if len(push.BatchMembers) != 0 {
		t.Fatal("legacy fixture unexpectedly names its members")
	}
	push.Old, push.Commit, push.At = old, old, bedNow.Add(time.Hour).Format(time.RFC3339)
	if err := appendLine(pushesPath(b.install), push); err != nil {
		t.Fatal(err)
	}
	now := bedNow.Add(3 * time.Hour)
	record, err := goal.ReadLaneBudget(b.install, "goal-a", bedNow.Add(-7*time.Hour), now)
	if err != nil || record.PausedAt.IsZero() || record.PausedMinutes != 180 {
		t.Fatalf("unrelated legacy push ended pause: %+v %v", record, err)
	}
	push.Commit, push.At = head, bedNow.Add(2*time.Hour).Format(time.RFC3339)
	if err := appendLine(pushesPath(b.install), push); err != nil {
		t.Fatal(err)
	}
	record, err = goal.ReadLaneBudget(b.install, "goal-a", bedNow.Add(-7*time.Hour), now)
	if err != nil || !record.PausedAt.IsZero() || record.PausedMinutes != 120 {
		t.Fatalf("legacy push failed to end pause: %+v %v", record, err)
	}
}

func TestLaneWaitBoundIsAFindingAndNeverAStop(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	b.lines("queue.jsonl", Line{Goal: "waiting", SHA: "a", At: bedNow.Format(time.RFC3339)}, Line{Goal: "landed", SHA: "b", At: bedNow.Format(time.RFC3339)})
	for _, hours := range []int{12, 13} {
		now := bedNow.Add(time.Duration(hours) * time.Hour)
		status := b.read(ProveSeams{Now: func() time.Time { return now }}, b.git([]string{"b"}, nil))
		if status.Stop != nil || status.Paused || status.Queue[0].State != StateWaiting || len(status.Problems) != 0 {
			t.Fatalf("waiting bound stopped lane: %+v", status)
		}
		if hours == 12 && len(status.Findings) != 0 || hours == 13 && (len(status.Findings) != 1 || status.Findings[0] != "waiting: waiting 13 h in the lane") {
			t.Fatalf("wrong finding at %d hours: %q", hours, status.Findings)
		}
		data, err := json.Marshal(status)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("landing status: %s", data)
	}
}

func assertClosedLaneBudget(t *testing.T, home, id string, minutes float64) {
	t.Helper()
	record, err := goal.ReadClosedLaneBudget(home, id, bedNow.Add(-7*time.Hour), bedNow.Add(6*time.Hour))
	if err != nil || record.PausedMinutes != minutes || !record.PausedAt.IsZero() {
		t.Fatalf("closed stays: %+v %v", record, err)
	}
	data, err := os.ReadFile(filepath.Join(home, "host", "goal-budgets", id, "lane.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["lane_paused_minutes"]; !ok {
		t.Fatalf("closed minutes missing: %s", data)
	}
}

func TestLaneBudgetReturnRetriesClosedRecordWrite(t *testing.T) {
	t.Parallel()
	b := newBed(t)
	home := t.TempDir()
	sha := b.seat("seat-a", "goal-a")
	if _, _, err := HandIn(b.install, Line{Goal: "goal-a", SHA: sha, At: bedNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "host", "goal-budgets", "goal-a", "lane.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{damaged"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := appendLine(stopsPath(b.install), Stop{Loop: "lane-return", Subject: "goal-a", Tree: sha, Decision: "stop", At: bedNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	seams := ProveSeams{BudgetHome: home, Person: &ActProvenance{Kind: "return", Person: "fixture"}}
	returned, changed, err := ReturnProven(b.install, "goal-a", "other", "back to seat", true, "fixture", bedNow.Add(2*time.Hour), seams)
	if err == nil || !changed || returned.State != StateReturned {
		t.Fatalf("record failure hid the accepted return: %+v %t %v", returned, changed, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	returned, changed, err = ReturnProven(b.install, "goal-a", "other", "back to seat", true, "fixture", bedNow.Add(3*time.Hour), seams)
	if err != nil || changed || returned.State != StateReturned {
		t.Fatalf("return recovery: %+v %t %v", returned, changed, err)
	}
	if stop, err := NewestStop(b.install); err != nil || stop != nil {
		t.Fatalf("return recovery left its stop open: %+v %v", stop, err)
	}
	assertClosedLaneBudget(t, home, "goal-a", 120)
	lines, err := readLines[Line](queuePath(b.install))
	if err != nil || len(lines) != 2 {
		t.Fatalf("recovery repeated queue transition: %+v %v", lines, err)
	}
}
