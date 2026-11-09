package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// writeBudgetSeat supplies occupancy, which never ends a person or landing wait.
func writeBudgetSeat(t *testing.T, root, goalID, started string) {
	t.Helper()
	writeJSON(t, filepath.Join(root, "artifacts/agents/steward/seats", "budget-"+goalID+".json"), map[string]any{
		"launchId": "budget-seat", "goal": goalID, "startedAt": started,
	})
}

func ownWorkBed(t *testing.T, hour int) (*goalAdmissionBed, *goal.GoalFile, time.Time) {
	t.Helper()
	bed := newGoalAdmissionBed(t, 2, true)
	templateAdmissionBed(t, bed)
	data, err := os.ReadFile(filepath.Join(bed.root, "plans/goals/bounded.md"))
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	start := time.Date(2026, 8, 28, hour, 0, 0, 0, time.UTC)
	shift := start.Sub(time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	for i := range file.History {
		at, _ := time.Parse(time.RFC3339, file.History[i].At)
		file.History[i].At = at.Add(shift).Format(time.RFC3339)
	}
	file.Claimed.At = start.Format(time.RFC3339)
	file.Approved.At = file.History[0].At
	file.Budget.ElapsedLimit = "4h"
	file.Budget.AttemptLimit, file.Budget.ReservedJobMinutesLimit, file.Budget.ActiveJobLimit = 100, 100000, 10
	file.Approved.Digest = legacyBudgetApprovalDigest(file.Intent, *file.Budget)
	if err := os.WriteFile(filepath.Join(bed.root, "plans/goals/bounded.md"), goal.RenderFile(file), 0600); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
	bed.reads.BudgetHome = testprovider.Register(t, bed.root)
	return bed, file, start
}

func ownWorkLaunch(t *testing.T, bed *goalAdmissionBed, goalID, id string, start, end time.Time) {
	t.Helper()
	if err := (launch.Store{Root: filepath.Join(bed.reads.BudgetHome, "launch")}).Create(launch.Record{
		ID: id, Goal: goalID, Kind: "build", Adapter: "local", State: launch.Completed,
		StartedAt: start.Format(time.RFC3339Nano), FinishedAt: end.Format(time.RFC3339Nano),
	}); err != nil {
		t.Fatal(err)
	}
}

func assertOwnElapsed(t *testing.T, bed *goalAdmissionBed, file *goal.GoalFile, now time.Time, want time.Duration) BudgetProjection {
	t.Helper()
	p := ProjectBudget(bed.root, file, now, bed.reads.BudgetHome)
	if p.Status != BudgetKnown || p.ElapsedLegacy || p.Elapsed != want {
		t.Fatalf("want own elapsed %s: %+v unknown=%+v", want, p, p.Unknown)
	}
	return p
}

func TestOwnWorkNight(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 18)
	ownWorkLaunch(t, bed, file.Id, "build", start, start.Add(time.Hour))
	now := start.Add(14 * time.Hour)
	p := assertOwnElapsed(t, bed, file, now, time.Hour)
	if p.ElapsedState != "" || len(p.Breaches) != 0 {
		t.Fatalf("night closed admission: %+v", p)
	}
	v, err := bed.admission("coordinator", now)
	if err != nil || v.Refused() {
		t.Fatalf("night refused: %+v %v", v, err)
	}
}

func TestOwnWorkConcurrent(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 10)
	ownWorkLaunch(t, bed, file.Id, "first", start, start.Add(2*time.Hour))
	ownWorkLaunch(t, bed, file.Id, "second", start.Add(time.Hour), start.Add(3*time.Hour))
	assertOwnElapsed(t, bed, file, start.Add(4*time.Hour), 3*time.Hour)
}

func ownWorkJob(t *testing.T, root, id, status, created, started string) {
	t.Helper()
	record := map[string]any{"jobId": id, "operationId": id, "goalId": "bounded", "goalRevision": 2, "capMin": 1, "runtime": "local", "mainId": "fixture", "claimEpoch": 1, "status": "pending-setup"}
	if created != "" {
		record["createdAt"] = created
	}
	source := filepath.Join(t.TempDir(), "job.json")
	writeJSON(t, source, record)
	if err := RecordCreate(root, id, source); err != nil {
		t.Fatal(err)
	}
	record["status"] = "pending"
	if started != "" {
		record["startedAt"] = started
	}
	writeJSON(t, source, record)
	if err := RecordSetup(root, id, source); err != nil {
		t.Fatal(err)
	}
	if status == "running" {
		patch := writeJSON(t, filepath.Join(t.TempDir(), "patch.json"), map[string]any{})
		if _, err := RecordCAS(root, id, "pending", "running", patch); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOwnWorkPending(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 10)
	ownWorkJob(t, bed.root, "pending", "pending", start.Format(time.RFC3339Nano), "")
	assertOwnElapsed(t, bed, file, start.Add(time.Hour), 0)
}

func ownEnvironmentLaunch(t *testing.T, bed *goalAdmissionBed, file *goal.GoalFile, start time.Time) {
	t.Helper()
	if err := ReserveUnitLaunch(bed.root, "environment", "unit", file, 1, 40, start, false); err != nil {
		t.Fatal(err)
	}
	end := start.Add(32 * time.Minute)
	if err := ReconcileUnitLaunch(bed.root, "environment", "unit", file.Id, file.Claimed.Revision, "failed", "environment", start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), true); err != nil {
		t.Fatal(err)
	}
	ownWorkLaunch(t, bed, file.Id, "environment", start, end)
}

func TestOwnWorkEnvironment(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 10)
	ownEnvironmentLaunch(t, bed, file, start)
	assertOwnElapsed(t, bed, file, start.Add(time.Hour), 0)
	retry, end := start.Add(time.Hour), start.Add(90*time.Minute)
	if err := ReserveUnitLaunch(bed.root, "retry", "unit", file, 1, 40, retry, true); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileUnitLaunch(bed.root, "retry", "unit", file.Id, file.Claimed.Revision, "completed", "", retry.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), true); err != nil {
		t.Fatal(err)
	}
	ownWorkLaunch(t, bed, file.Id, "retry", retry, end)
	assertOwnElapsed(t, bed, file, start.Add(2*time.Hour), 30*time.Minute)
}

func TestOwnWorkPauseOutsideWork(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 9)
	writePersonQuestion(t, bed.root, "person", file.Id, start, map[string]any{"state": "closed", "closedAt": start.Add(8 * time.Hour)})
	ownWorkLaunch(t, bed, file.Id, "build", start.Add(time.Hour), start.Add(2*time.Hour))
	assertOwnElapsed(t, bed, file, start.Add(8*time.Hour), time.Hour)
}

func TestOwnWorkFallback(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"job", "seat"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			bed, file, start := ownWorkBed(t, 9)
			name := "undated"
			if source == "job" {
				ownWorkJob(t, bed.root, name, "running", "", "")
			} else {
				name = "seat-broken.json"
				path := filepath.Join(bed.root, "artifacts/agents/steward/seats", name)
				writeJSON(t, path, nil)
				if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p := ProjectBudget(bed.root, file, start.Add(8*time.Hour), bed.reads.BudgetHome)
			if p.Status != BudgetKnown || !p.ElapsedLegacy || p.Elapsed != 8*time.Hour || len(p.Problems) != 1 || !strings.Contains(p.Problems[0], "elapsed counts wall time less pauses: ") || !strings.Contains(p.Problems[0], name) {
				t.Fatalf("optional failure hid elapsed or its reason: %+v", p)
			}
		})
	}
}

func TestOwnWorkRequiredLaunchStore(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 9)
	writePersonQuestion(t, bed.root, "person", file.Id, start, nil)
	ownWorkJob(t, bed.root, "running", "running", start.Format(time.RFC3339Nano), start.Format(time.RFC3339Nano))
	if err := os.WriteFile(filepath.Join(bed.reads.BudgetHome, "launch"), []byte("unreadable directory"), 0600); err != nil {
		t.Fatal(err)
	}
	p := ProjectBudget(bed.root, file, start.Add(time.Hour), bed.reads.BudgetHome)
	if p.Status != BudgetUnknown || p.ElapsedLegacy {
		t.Fatalf("required failure fell back: %+v", p)
	}
	if err := ReserveUnitLaunch(bed.root, "build", "unit", file, 1, 1, start.Add(time.Hour), false, bed.reads.BudgetHome); refusal.CodeOf(err) != "BUDGET_UNKNOWN" {
		t.Fatalf("unknown admitted: %v", err)
	}
}

func TestOwnWorkBreachStillStops(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 9)
	ownWorkLaunch(t, bed, file.Id, "build-one", start, start.Add(2*time.Hour))
	ownWorkLaunch(t, bed, file.Id, "build-two", start.Add(5*time.Hour), start.Add(7*time.Hour))
	p := assertOwnElapsed(t, bed, file, start.Add(7*time.Hour), 4*time.Hour)
	v, err := bed.admission("coordinator", start.Add(7*time.Hour))
	if err != nil || !v.Refused() || len(v.Refusals) != 1 || len(v.Refusals[0].Breaches) != 1 || v.Refusals[0].Breaches[0].Field != "elapsedLimit" || p.ElapsedState != AdmissionClosedElapsed {
		t.Fatalf("box stayed open: %+v %+v %v", p, v, err)
	}
	ownWorkLaunch(t, bed, file.Id, "build-three", start.Add(8*time.Hour), start.Add(10*time.Hour))
	p = assertOwnElapsed(t, bed, file, start.Add(10*time.Hour), 6*time.Hour)
	if p.ElapsedState != ElapsedBreach || liveStopReason(p) != goal.StopReasonElapsedLimit {
		t.Fatalf("build breach lost stop: %+v", p)
	}
}

func TestOwnWorkSession(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		state launch.State
		reap  string
		want  time.Duration
	}{
		{"terminal", launch.Completed, "2026-08-28T11:00:00Z", 3 * time.Hour},
		{"reaped", launch.Running, "2026-08-28T11:00:00Z", 2 * time.Hour},
		{"missing launch reaped", "", "2026-08-28T11:00:00Z", 2 * time.Hour},
		{"running", launch.Running, "", 4 * time.Hour},
		{"missing launch live", "", "", 4 * time.Hour},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			bed, file, start := ownWorkBed(t, 9)
			writeJSON(t, filepath.Join(bed.root, "artifacts/agents/steward/seats/seat.json"), map[string]any{"launchId": "seat", "goal": file.Id, "startedAt": start.Format("2006-01-02T15:04:05.000000000Z"), "reapedAt": row.reap})
			if row.state != "" {
				if err := (launch.Store{Root: filepath.Join(bed.reads.BudgetHome, "launch")}).Create(launch.Record{ID: "seat", State: row.state, StartedAt: start.Format(time.RFC3339Nano), FinishedAt: start.Add(3 * time.Hour).Format(time.RFC3339Nano)}); err != nil {
					t.Fatal(err)
				}
			}
			assertOwnElapsed(t, bed, file, start.Add(4*time.Hour), row.want)
		})
	}
}

func TestOwnWorkOtherGoalsSession(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 9)
	writeBudgetSeat(t, bed.root, "another", start.Format(time.RFC3339Nano))
	assertOwnElapsed(t, bed, file, start.Add(4*time.Hour), 0)
}

func TestOwnWorkSeatOverEnvironment(t *testing.T) {
	t.Parallel()
	for _, minute := range []int{0, 40, 10} {
		t.Run((time.Duration(minute) * time.Minute).String(), func(t *testing.T) {
			t.Parallel()
			bed, file, start := ownWorkBed(t, 10)
			writeBudgetSeat(t, bed.root, file.Id, start.Format(time.RFC3339Nano))
			ownEnvironmentLaunch(t, bed, file, start)
			want := 28 * time.Minute
			if minute > 0 {
				at := start.Add(time.Duration(minute) * time.Minute)
				ownWorkLaunch(t, bed, file.Id, "build", at, at.Add(10*time.Minute))
				if minute == 10 {
					want += 10 * time.Minute
				}
			}
			assertOwnElapsed(t, bed, file, start.Add(time.Hour), want)
		})
	}
}

func TestOwnWorkSeatUnderPersonWait(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 9)
	writeBudgetSeat(t, bed.root, file.Id, start.Format(time.RFC3339Nano))
	writePersonQuestion(t, bed.root, "act", file.Id, start.Add(2*time.Hour), map[string]any{"state": "answered", "unitStop": map[string]any{"loop": "unit-round", "subject": "bounded/unit/run", "attempt": 1, "finding": "read:1", "acceptableActs": []string{"work-revise"}}, "answer": map[string]any{"at": start.Add(3 * time.Hour), "phase": "recorded"}})
	p := assertOwnElapsed(t, bed, file, start.Add(6*time.Hour), 2*time.Hour)
	if p.PersonPaused != 4*time.Hour {
		t.Fatalf("seat ended person wait: %+v", p)
	}
}

func ownWorkGovernedRun(t *testing.T, bed *goalAdmissionBed, file *goal.GoalFile, start time.Time) {
	t.Helper()
	generation := uint64(1)
	store := &run.Store{Root: bed.root, Prober: &governedProofProber{alive: true, started: start}, Getpgid: func(pid int64) (int64, error) { return pid, nil }, Now: func() time.Time { return start }, AdmitGoverned: func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{GoalRevision: 2, ObligationRevision: 1, Recurrence: governance.StandingSharedProcess, WeightGeneration: &generation, Breaker: run.BreakerClosed, ExpectedAssumptions: governance.ObligationAssumptions{Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go", SurfaceDigest: "fixture-digest", MaxActiveJobs: 10, TimingEnvelopeSeconds: 600, ObservationSource: "run-terminal-record"}, ExecutionCostMinutes: 10, AttemptOrdinal: 1, Budget: *file.Budget, BudgetStartedAt: file.Claimed.At, AdmissionDecision: governance.ConsequenceDecision{Apply: true}}}, nil
	}}
	nonce, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "governed", Kind: "suite", Display: "governed work", Log: "artifacts/governed.log", GoalId: file.Id, ObligationRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("governed", nonce, 101, 101); err != nil {
		t.Fatal(err)
	}
}

func TestOwnWorkLiveGovernedRun(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 9)
	ownWorkGovernedRun(t, bed, file, start)
	now := start.Add(4 * time.Hour)
	assertOwnElapsed(t, bed, file, now, 4*time.Hour)
	v, err := bed.admission("coordinator", now)
	if err != nil || !v.Refused() || len(v.Refusals) != 1 || len(v.Refusals[0].Breaches) != 1 || v.Refusals[0].Breaches[0].Field != "elapsedLimit" {
		t.Fatalf("governed run left box open: %+v %v", v, err)
	}
	p := ProjectBudgetWithoutRun(bed.root, file, now, "governed")
	if p.Status != BudgetKnown || p.ElapsedLegacy || p.Elapsed != 0 {
		t.Fatalf("excluded run spent: %+v", p)
	}
}

func TestOwnWorkSeatFailuresKeepTheLegacyClock(t *testing.T) {
	t.Parallel()
	for _, damage := range []string{"directory", "record", "startedAt", "reapedAt", "launch", "finishedAt"} {
		t.Run(damage, func(t *testing.T) {
			t.Parallel()
			bed, file, start := ownWorkBed(t, 9)
			writePersonQuestion(t, bed.root, "person", file.Id, start.Add(2*time.Hour), map[string]any{"state": "closed", "closedAt": start.Add(4 * time.Hour)})
			path := filepath.Join(bed.root, "artifacts/agents/steward/seats/seat.json")
			seat := map[string]any{"launchId": "seat", "goal": file.Id, "startedAt": start.Format(time.RFC3339Nano)}
			want := "seat record seat.json"
			switch damage {
			case "directory":
				if err := os.MkdirAll(filepath.Dir(filepath.Dir(path)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Dir(path), nil, 0600); err != nil {
					t.Fatal(err)
				}
				want = "seat directory"
			case "record":
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "startedAt":
				seat["startedAt"] = "bad"
			case "reapedAt":
				seat["reapedAt"] = "bad"
			case "launch":
				seat["launchId"] = "INVALID"
			case "finishedAt":
				if err := (launch.Store{Root: filepath.Join(bed.reads.BudgetHome, "launch")}).Create(launch.Record{ID: "seat", State: launch.Completed, StartedAt: start.Format(time.RFC3339Nano), FinishedAt: "bad"}); err != nil {
					t.Fatal(err)
				}
			}
			if damage != "directory" && damage != "record" {
				writeJSON(t, path, seat)
			}
			p := ProjectBudget(bed.root, file, start.Add(6*time.Hour), bed.reads.BudgetHome)
			if p.Status != BudgetKnown || !p.ElapsedLegacy || p.Elapsed != 4*time.Hour || p.Wait != 2*time.Hour || len(p.Problems) != 1 || !strings.Contains(p.Problems[0], want) {
				t.Fatalf("seat failure changed legacy clock: %+v", p)
			}
		})
	}
}

func TestOwnWorkClipsToTheBudgetWindow(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 9)
	ownWorkLaunch(t, bed, file.Id, "before", start.Add(-2*time.Hour), start.Add(time.Hour))
	ownWorkLaunch(t, bed, file.Id, "after", start.Add(2*time.Hour), start.Add(6*time.Hour))
	ownWorkLaunch(t, bed, file.Id, "empty", start.Add(5*time.Hour), start.Add(5*time.Hour))
	// Brief files and unrelated seats carry no occupancy for this goal.
	writeBudgetSeat(t, bed.root, "another", "bad")
	path := filepath.Join(bed.root, "artifacts/agents/steward/seats/seat.brief.md")
	if err := os.WriteFile(path, []byte("not JSON"), 0600); err != nil {
		t.Fatal(err)
	}
	assertOwnElapsed(t, bed, file, start.Add(4*time.Hour), 3*time.Hour)
}

func TestOwnWorkUndatedGovernedRunKeepsTheLegacyClock(t *testing.T) {
	t.Parallel()
	bed, file, start := ownWorkBed(t, 9)
	ownWorkGovernedRun(t, bed, file, start)
	path := run.RecordPath(bed.root, "governed")
	record, err := readObject(path)
	if err != nil {
		t.Fatal(err)
	}
	// Damaged optional start evidence retains the wall-time clock.
	record["startedAt"] = "bad"
	writeJSON(t, path, record)
	p := ProjectBudget(bed.root, file, start.Add(4*time.Hour), bed.reads.BudgetHome)
	if p.Status != BudgetKnown || !p.ElapsedLegacy || p.Elapsed != 4*time.Hour || len(p.Problems) != 1 || !strings.Contains(p.Problems[0], "governed run governed has no readable startedAt") {
		t.Fatalf("undated governed run hid time: %+v", p)
	}
}
