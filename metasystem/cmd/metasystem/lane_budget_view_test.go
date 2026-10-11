package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestLaneBudgetGoalViewsPrintLocalPauseAndStatusFinding(t *testing.T) {
	t.Parallel()
	paused := time.Date(2026, 10, 11, 10, 15, 0, 0, time.FixedZone("fixture-local", 2*3600))
	view := intentBudgetView{Box: "8h/6/360m/1/2", Lens: "claim", Projection: &dispatchcore.BudgetProjection{Status: dispatchcore.BudgetKnown, LaneBudgetRecord: goal.LaneBudgetRecord{PausedAt: paused, PausedMinutes: 180}, Limits: goal.Budget{ElapsedLimit: "8h"}}}
	want := "in the lane since " + paused.Local().Format("15:04") + ", clock paused (3h so far)"
	if !strings.Contains(strings.Join(view.lines(), "\n"), want) {
		t.Fatalf("goal budget lost pause: %q", view.lines())
	}
	page := textui.New(textui.Env{})
	goalShown{budget: view}.budgetSection(page)
	if !strings.Contains(page.String(), want) {
		t.Fatalf("goal show lost pause: %s", page.String())
	}
	page = textui.New(textui.Env{})
	install := t.TempDir()
	withPlainLane(func(*textui.Page) {}, landingStatusData{View: lane.View{Root: &install}, Findings: []string{"widget: waiting 13 h in the lane"}, Queue: []plain.Entry{}}, install)(page)
	if !strings.Contains(page.String(), "widget: waiting 13 h in the lane") {
		t.Fatalf("landing status hid finding: %s", page.String())
	}
	t.Logf("goal show/budget: %s\nlanding status: %s", want, page.String())
}

func TestLaneBudgetReadsRecordedLedgerAndQueue(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/lane-budget-goal.md")
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 || file.Risk == nil || file.Claimed == nil {
		t.Fatalf("real goal record: %v", problems)
	}
	install, home := t.TempDir(), t.TempDir()
	dir := filepath.Join(install, "artifacts", "agents", "landing")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	// The three lines are byte copies of the landing installation's
	// 2026-10-11 hand-in, delivery update and return, with their full shapes.
	queue, err := os.ReadFile("testdata/lane-budget-queue.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "queue.jsonl"), queue, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(lane.HostDir(home), 0755); err != nil {
		t.Fatal(err)
	}
	registration, err := json.Marshal(lane.Record{Root: install, Install: install, CustodyEpoch: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lane.RecordPath(home), registration, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	start, err := time.Parse(time.RFC3339, file.Claimed.EpisodeAt)
	if err != nil {
		start, err = time.Parse(time.RFC3339, file.Claimed.At)
	}
	if err != nil {
		t.Fatal(err)
	}
	returned := time.Date(2026, 10, 11, 12, 2, 18, 0, time.UTC)
	if err := goal.SaveClosedLaneBudget(home, install, file.Id, returned); err != nil {
		t.Fatal(err)
	}
	projection := dispatchcore.ProjectBudget(install, file, returned.Add(time.Hour), home)
	if projection.Status != dispatchcore.BudgetKnown || !projection.PausedAt.IsZero() || projection.PausedMinutes != (93*time.Minute+2*time.Second).Minutes() {
		t.Fatalf("real ledger/queue projection: %+v", projection)
	}
	record, err := goal.ReadLaneBudget(install, file.Id, start, returned.Add(-time.Second))
	if err != nil || record.PausedAt.IsZero() {
		t.Fatalf("real hand-in was not paused: %+v %v", record, err)
	}
	t.Logf("recorded lane pause: %s; after return %.3f minutes retained", record.Words(), projection.PausedMinutes)
}
