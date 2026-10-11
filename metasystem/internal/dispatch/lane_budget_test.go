package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLaneBudgetWaitDoesNotBreachAndReturnResumesCustody(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"no job", "lane fix"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			bed, file := markedAdmissionBed(t, "2026-08-28T16:00:00Z")
			if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.budget.elapsed-grace-percent=0\n"), 0644); err != nil {
				t.Fatal(err)
			}
			home, install := t.TempDir(), t.TempDir()
			bed.reads.BudgetHome = home
			writeJSON(t, lane.RecordPath(home), lane.Record{Root: install, Install: install, CustodyEpoch: 1})
			handIn := time.Date(2026, 8, 28, 16, 0, 0, 0, time.UTC)
			queuePath := filepath.Join(install, "artifacts", "agents", "landing", "queue.jsonl")
			writeJSON(t, queuePath, map[string]string{"goal": file.Id, "sha": "unit-sha", "at": handIn.Format(time.RFC3339)})
			now := handIn.Add(3 * time.Hour)
			// A lane fix's work does not terminate the lane's pause.
			if scenario == "lane fix" {
				writeBudgetJob(t, bed.root, "lane-fix", "lane-fix", 2, 1, "completed", budgetJobLife{startedAt: handIn.Add(time.Hour).Format(time.RFC3339), endedAt: handIn.Add(time.Hour + time.Minute).Format(time.RFC3339), pid: 41})
			}
			projection := ProjectBudget(bed.root, file, now, home)
			if projection.Status != BudgetKnown || projection.Elapsed != 7*time.Hour || projection.PausedMinutes != 180 || !projection.PausedAt.Equal(handIn) || projection.ElapsedState != "" {
				t.Fatalf("lane wait spent the goal's last hour: %+v", projection)
			}
			if routes, err := bed.stops(now); err != nil || len(routes) != 0 {
				t.Fatalf("custodian stopped lane wait: %+v %v", routes, err)
			}
			if _, err := ensureBreachStopWithReads(bed.root, file.Id, 2, now, bed.reads, ""); err == nil || !strings.Contains(err.Error(), "no overspend to stop") {
				t.Fatalf("direct custodian accepted a lane breach: %v", err)
			}
			queue, err := os.OpenFile(queuePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := queue.WriteString("\n"); err != nil {
				t.Fatal(err)
			}
			if err := json.NewEncoder(queue).Encode(map[string]string{"goal": file.Id, "sha": "unit-sha", "at": now.Format(time.RFC3339), "outcome": "returned"}); err != nil {
				t.Fatal(err)
			}
			queue.Close()
			if err := goal.SaveClosedLaneBudget(home, install, file.Id, now); err != nil {
				t.Fatal(err)
			}
			resumed := ProjectBudget(bed.root, file, now.Add(time.Hour), home)
			if resumed.Status != BudgetKnown || resumed.Elapsed != 8*time.Hour || !resumed.PausedAt.IsZero() || resumed.PausedMinutes != 180 {
				t.Fatalf("return failed to resume clock: %+v", resumed)
			}
			if routes, err := bed.stops(now.Add(5 * time.Hour)); err != nil || len(routes) != 1 || routes[0].Reason != goal.StopReasonElapsedLimit {
				t.Fatalf("resumed clock failed to breach: %+v %v", routes, err)
			}
		})
	}
}

func TestLaneBudgetCustodianAcceptsRealStopRecordShape(t *testing.T) {
	t.Parallel()
	// Copied byte for byte from the landing installation's 2026-10-11
	// stop-lane-proves-at-the-batch-risk-r3-f1.json, including firing evidence.
	data, err := os.ReadFile("testdata/lane-budget-stop.json")
	if err != nil {
		t.Fatal(err)
	}
	var recorded goal.StopBatch
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "goal-stops", recorded.StopID+".json"), recorded)
	read, err := goal.ReadStopBatch(root, recorded.StopID)
	if err != nil || read.State != goal.StopBatchComplete || read.FiringEvidence == nil || read.FiringEvidence.ElapsedUsed != "12h5m22.19788s" || read.FiringEvidence.GracePercent != 50 {
		t.Fatalf("real custodian evidence lost: %+v %v", read, err)
	}
}

func TestLaneBudgetUnrelatedDamageKeepsBudgetKnown(t *testing.T) {
	t.Parallel()
	for _, damage := range []string{"torn queue", "incomplete registration"} {
		t.Run(damage, func(t *testing.T) {
			t.Parallel()
			bed, file := markedAdmissionBed(t, "2026-08-28T16:00:00Z")
			home, install := t.TempDir(), t.TempDir()
			record := lane.Record{Root: install, Install: install, CustodyEpoch: 1}
			if damage == "incomplete registration" {
				record.CustodyEpoch = 0
			}
			writeJSON(t, lane.RecordPath(home), record)
			path := filepath.Join(install, "artifacts", "agents", "landing", "queue.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{torn"), 0644); err != nil {
				t.Fatal(err)
			}
			projection := ProjectBudget(bed.root, file, time.Date(2026, 8, 28, 16, 0, 0, 0, time.UTC), home)
			if projection.Status != BudgetKnown || len(projection.Problems) == 0 {
				t.Fatalf("unrelated lane damage must stay known and visible: %+v", projection)
			}
		})
	}
}

func TestLaneBudgetClosedPauseSurvivesRegistrationChange(t *testing.T) {
	t.Parallel()
	bed, file := markedAdmissionBed(t, "2026-08-28T16:00:00Z")
	home, install := t.TempDir(), t.TempDir()
	at := time.Date(2026, 8, 28, 16, 0, 0, 0, time.UTC)
	path := filepath.Join(install, "artifacts", "agents", "landing", "queue.jsonl")
	writeJSON(t, path, map[string]string{"goal": file.Id, "sha": "a", "at": at.Format(time.RFC3339)})
	queue, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	err = json.NewEncoder(queue).Encode(map[string]string{"goal": file.Id, "sha": "a", "at": at.Add(2 * time.Hour).Format(time.RFC3339), "outcome": "returned"})
	queue.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := goal.SaveClosedLaneBudget(home, install, file.Id, at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	check := func(minutes float64, paused bool) {
		t.Helper()
		projection := ProjectBudget(bed.root, file, at.Add(4*time.Hour), home)
		if len(projection.ClosedSpans) != 0 || projection.Status != BudgetKnown || projection.PausedMinutes != minutes || projection.Elapsed != 11*time.Hour-time.Duration(minutes*float64(time.Minute)) || projection.PausedAt.IsZero() == paused {
			t.Fatalf("closed budget after registration change: %+v", projection)
		}
	}
	writeJSON(t, lane.RecordPath(home), lane.Record{Root: install, Install: install, CustodyEpoch: 1})
	check(120, false)
	if err := os.Remove(lane.RecordPath(home)); err != nil {
		t.Fatal(err)
	}
	check(120, false)
	other := t.TempDir()
	writeJSON(t, lane.RecordPath(home), lane.Record{Root: other, Install: other, CustodyEpoch: 2})
	check(120, false)
	writeJSON(t, filepath.Join(other, "artifacts", "agents", "landing", "queue.jsonl"), map[string]string{"goal": file.Id, "sha": "b", "at": at.Add(3 * time.Hour).Format(time.RFC3339)})
	check(180, true)
	// Closed stays are clipped when a budget episode starts inside one.
	clipped, err := goal.ReadClosedLaneBudget(home, file.Id, at.Add(time.Hour), at.Add(4*time.Hour))
	if err != nil || clipped.PausedMinutes != 60 {
		t.Fatalf("new episode kept prior pause: %+v %v", clipped, err)
	}
}
