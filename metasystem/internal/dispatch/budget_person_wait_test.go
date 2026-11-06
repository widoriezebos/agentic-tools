package dispatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func personWaitBed(t *testing.T) (*goalAdmissionBed, *goal.GoalFile, time.Time) {
	t.Helper()
	bed := newGoalAdmissionBed(t, 2)
	templateAdmissionBed(t, bed)
	data, err := os.ReadFile(filepath.Join(bed.root, "plans/goals/bounded.md"))
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.budget.elapsed-grace-percent=0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	bed.reads.BudgetHome = testprovider.Register(t, bed.root)
	return bed, file, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
}

// These names mirror the channel's JSON; the external tests also write its types.
func writePersonQuestion(t *testing.T, root, id, goalID string, opened time.Time, fields map[string]any) {
	t.Helper()
	record := map[string]any{"id": id, "goal": goalID, "openedAt": opened, "state": "open"}
	for key, value := range fields {
		record[key] = value
	}
	writeJSON(t, filepath.Join(root, "artifacts/agents/channel/questions", id+".json"), record)
}

func TestPersonWaitOpenQuestion(t *testing.T) {
	t.Parallel()
	bed, file, start := personWaitBed(t)
	opened := start.Add(2 * time.Hour)
	writePersonQuestion(t, bed.root, "open", file.Id, opened, nil)
	now := start.Add(10 * time.Hour)
	p := ProjectBudget(bed.root, file, now, bed.reads.BudgetHome)
	if p.Status != BudgetKnown || p.Elapsed != 2*time.Hour || !p.PersonWaitSince.Equal(opened) || p.PersonPaused != 8*time.Hour || p.ElapsedState != "" {
		t.Fatalf("open wait spent elapsed: %+v", p)
	}
	admission, err := bed.revisionAdmission(file.Id, 2, 5, now)
	if err != nil || admission.Refused() || admission.LiveStopReason != "" {
		t.Fatalf("waiting goal refused: %+v %v", admission, err)
	}
	if routes, err := bed.stops(now); err != nil || len(routes) != 0 {
		t.Fatalf("waiting goal stopped: %+v %v", routes, err)
	}
}

func TestPersonWaitAnsweredQuestion(t *testing.T) {
	t.Parallel()
	bed, file, start := personWaitBed(t)
	writePersonQuestion(t, bed.root, "answered", file.Id, start.Add(2*time.Hour), map[string]any{"state": "answered", "answer": map[string]any{"at": start.Add(5 * time.Hour)}})
	now := start.Add(11 * time.Hour)
	p := ProjectBudget(bed.root, file, now, bed.reads.BudgetHome)
	if p.Status != BudgetKnown || p.Elapsed != 8*time.Hour || !p.PersonWaitSince.IsZero() {
		t.Fatalf("answer failed to resume elapsed: %+v", p)
	}
	admission, err := bed.revisionAdmission(file.Id, 2, 5, now)
	if err != nil || !admission.Refused() || admission.Refusal == nil || len(admission.Refusal.Breaches) != 1 || admission.Refusal.Breaches[0].Field != "elapsedLimit" {
		t.Fatalf("elapsed admission remained open: %+v %v", admission, err)
	}
}

func TestPersonWaitOtherMachineKind(t *testing.T) {
	t.Parallel()
	bed, file, start := personWaitBed(t)
	writePersonQuestion(t, bed.root, "budget", "", start.Add(2*time.Hour), map[string]any{"about": "machine", "decision": map[string]string{"Kind": "budget-closed", "Subject": file.Id}})
	now := start.Add(9 * time.Hour)
	p := ProjectBudget(bed.root, file, now, bed.reads.BudgetHome)
	if p.Status != BudgetKnown || p.Elapsed != 9*time.Hour || p.PersonPaused != 0 {
		t.Fatalf("budget condition paused itself: %+v", p)
	}
	if routes, err := bed.stops(now); err != nil || len(routes) != 1 || routes[0].Reason != goal.StopReasonElapsedLimit {
		t.Fatalf("real breach was not named: %+v %v", routes, err)
	}
}

func TestPersonWaitPendingJob(t *testing.T) {
	t.Parallel()
	bed, file, start := personWaitBed(t)
	writePersonQuestion(t, bed.root, "pending", file.Id, start.Add(2*time.Hour), nil)
	writeBudgetJob(t, bed.root, "pending", "pending", 2, 1, "pending", budgetJobLife{createdAt: start.Add(time.Hour).Format(time.RFC3339)})
	p := ProjectBudget(bed.root, file, start.Add(6*time.Hour), bed.reads.BudgetHome)
	if p.Status != BudgetKnown || p.Elapsed != 2*time.Hour {
		t.Fatalf("pending setup counted as executed work: %+v", p)
	}
}

func TestPersonWaitExecutedWork(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"launch", "proof", "obligation"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			bed, file, start := personWaitBed(t)
			writePersonQuestion(t, bed.root, "work", file.Id, start.Add(time.Hour), map[string]any{"state": "closed", "closedAt": start.Add(9 * time.Hour)})
			store := launch.Store{Root: filepath.Join(bed.reads.BudgetHome, "launch")}
			if err := store.Create(launch.Record{ID: "work", Goal: file.Id, Adapter: "local", State: launch.Completed, StartedAt: start.Add(3 * time.Hour).Format(time.RFC3339), FinishedAt: start.Add(3*time.Hour + 30*time.Minute).Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			want := 2*time.Hour + 30*time.Minute
			if row != "launch" {
				if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.budget.elapsed-grace-percent=0\n"), 0644); err != nil {
					t.Fatal(err)
				}
				launcher, err := proofrun.CurrentProcessIdentity(nil)
				if err != nil {
					t.Fatal(err)
				}
				_, identity := dispatchProofFixture(t, "wait-proof")
				_, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{ControlRoot: bed.root, ExecutionRoot: bed.root, GoalID: file.Id, GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 1, Identity: identity, Launcher: launcher, Now: start.Add(4 * time.Hour), AttemptID: "wait-proof"}))
				if err != nil {
					t.Fatal(err)
				}
				requireProofReservationNotAdmissionRefused(t, decision)
				launchBudgetProof(t, bed.root, "wait-proof", start.Add(4*time.Hour))
				if _, err := proofrun.FinalizeAttempt(bed.root, "wait-proof", proofrun.TerminalCancelled, 1, "fixture completed", nil, start.Add(4*time.Hour+30*time.Minute)); err != nil {
					t.Fatal(err)
				}
				want += 30 * time.Minute
			}
			if row == "obligation" {
				if err := obligationstate.RecordTerminal(bed.root, file.Id, 2, 1, obligationstate.TerminalAttempt{RunID: "wait-obligation", Status: run.StatusGreen, StartedAt: start.Add(5 * time.Hour).Format(time.RFC3339), EndedAt: start.Add(5*time.Hour + 30*time.Minute).Format(time.RFC3339), PrunedAt: start.Add(6 * time.Hour).Format(time.RFC3339), AttemptOrdinal: 1, ExecutionCostMinutes: 1, ObservedCostMinutes: 1, WeightGeneration: 1, Breaker: run.BreakerClosed}); err != nil {
					t.Fatal(err)
				}
				want += 30 * time.Minute
			}
			p := ProjectBudget(bed.root, file, start.Add(10*time.Hour), bed.reads.BudgetHome)
			if p.Status != BudgetKnown || p.Elapsed != want {
				t.Fatalf("%s work disappeared inside wait: want %s, %+v", row, want, p)
			}
		})
	}
}

func TestPersonWaitOverlap(t *testing.T) {
	t.Parallel()
	bed, file, start := personWaitBed(t)
	opened := start.Add(2 * time.Hour)
	writePersonQuestion(t, bed.root, "goal", file.Id, opened, nil)
	writePersonQuestion(t, bed.root, "hold", "", opened, map[string]any{"about": "machine", "decision": map[string]string{"Kind": "steward-hold", "Subject": file.Id}})
	install := t.TempDir()
	writeJSON(t, lane.RecordPath(bed.reads.BudgetHome), lane.Record{Root: install, Install: install, CustodyEpoch: 1})
	writeJSON(t, filepath.Join(install, "artifacts/agents/landing/queue.jsonl"), map[string]string{"goal": file.Id, "sha": "unit", "at": opened.Format(time.RFC3339)})
	p := ProjectBudget(bed.root, file, start.Add(10*time.Hour), bed.reads.BudgetHome)
	if p.Status != BudgetKnown || p.Elapsed != 2*time.Hour || p.Wait != 8*time.Hour || p.PersonPaused != 8*time.Hour {
		t.Fatalf("overlapping waits counted twice: %+v", p)
	}
}

func TestPersonWaitWorkCutKeepsOpenWaitStart(t *testing.T) {
	t.Parallel()
	bed, file, start := personWaitBed(t)
	// The question predates this claim; a completed job splits the clipped wait.
	writePersonQuestion(t, bed.root, "early", file.Id, start.Add(-time.Hour), nil)
	writeBudgetJob(t, bed.root, "worked", "worked", 2, 1, "completed", budgetJobLife{startedAt: start.Add(time.Hour).Format(time.RFC3339), endedAt: start.Add(2 * time.Hour).Format(time.RFC3339)})
	p := ProjectBudget(bed.root, file, start.Add(4*time.Hour), bed.reads.BudgetHome)
	if p.Status != BudgetKnown || p.Elapsed != time.Hour || !p.PersonWaitSince.Equal(start) || p.PersonPaused != 3*time.Hour {
		t.Fatalf("work changed the wait's beginning: %+v", p)
	}
}

func TestPersonWaitLiveWorkCountsToNow(t *testing.T) {
	t.Parallel()
	bed, file, start := personWaitBed(t)
	writePersonQuestion(t, bed.root, "live", file.Id, start.Add(time.Hour), nil)
	store := launch.Store{Root: filepath.Join(bed.reads.BudgetHome, "launch")}
	if err := store.Create(launch.Record{ID: "live", Goal: file.Id, Adapter: "local", State: launch.Running, StartedAt: start.Add(2 * time.Hour).Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	p := ProjectBudget(bed.root, file, start.Add(5*time.Hour), bed.reads.BudgetHome)
	if p.Status != BudgetKnown || p.Elapsed != 4*time.Hour || p.PersonPaused != time.Hour || !p.PersonWaitSince.IsZero() {
		t.Fatalf("live launch was paused or showed an idle wait: %+v", p)
	}
}
