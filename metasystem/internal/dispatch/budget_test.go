package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func budgetGoal() *goal.GoalFile {
	return &goal.GoalFile{
		Id: "bounded", State: goal.StateClaimed, Revision: 5,
		Claimed: &goal.ClaimRecord{
			Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-28T08:00:00Z", Revision: 3,
		},
		Budget: &goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 75, ActiveJobLimit: 1,
		},
		History: []goal.HistoryLine{
			{At: "2026-08-28T06:00:00Z"},
			{At: "2026-08-28T07:00:00Z"},
			{At: "2026-08-28T08:00:00Z"},
			{At: "2026-08-28T08:30:00Z"},
			{At: "2026-08-28T09:00:00Z"},
		},
	}
}

func budgetProjectionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

type budgetJobLife struct {
	startedAt string
	endedAt   string
	createdAt string
	pid       int
	provenAt  string
}

func writeBudgetJob(t *testing.T, root, name, operation string, revision, cap uint64, status string, life budgetJobLife) {
	t.Helper()
	record := map[string]any{
		"jobId": name, "operationId": operation, "goalId": "bounded",
		"goalRevision": revision, "capMin": cap, "status": status,
	}
	if life.startedAt != "" {
		record["startedAt"] = life.startedAt
	}
	if life.endedAt != "" {
		record["endedAt"] = life.endedAt
	}
	if life.createdAt != "" {
		record["createdAt"] = life.createdAt
	}
	if life.pid != 0 {
		record["pid"] = life.pid
	}
	if life.provenAt != "" {
		record["ownershipProof"] = map[string]any{"provenAt": life.provenAt, "source": "trusted-launcher"}
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", name+".json"), record)
}

func TestBudgetProjectionUsesJobRecordsForTheBoundRevision(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "done", "reserve-a", 3, 30, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:19:30Z", pid: 4242,
	})
	writeBudgetJob(t, root, "live", "reserve-b", 3, 45, "running", budgetJobLife{})
	writeBudgetJob(t, root, "old-revision", "reserve-old", 2, 60, "completed", budgetJobLife{})
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "unrelated.json"), map[string]any{
		"jobId": "unrelated", "goalId": nil, "status": "completed",
	})

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.GoalRevision != 3 || projection.Attempts != 2 ||
		projection.ReservedJobMinutes != 55 || projection.ObservedJobMinutes != 10 || projection.OpenCapMinutes != 45 ||
		projection.ActiveJobs != 1 || projection.Elapsed != 2*time.Hour ||
		projection.ElapsedGracePercent != 50 || projection.ElapsedBreachLimit != 6*time.Hour ||
		projection.ElapsedState != "" || len(projection.Breaches) != 0 {
		t.Fatalf("projection did not use the sole reservation facts: %+v", projection)
	}
}

func TestCompletedJobChargesObservedMinutesNotItsCap(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "completed", "reserve-completed", 3, 120, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242,
	})

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 10 || projection.ObservedJobMinutes != 10 ||
		projection.OpenCapMinutes != 0 || projection.ActiveJobs != 0 || projection.Attempts != 1 {
		t.Fatalf("completed job did not settle to its observed runtime: %+v", projection)
	}
}

func TestRunningJobChargesItsCap(t *testing.T) {
	for _, status := range []string{"running", "pending", "pending-setup"} {
		t.Run(status, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			writeBudgetJob(t, root, status, "reserve-"+status, 3, 45, status, budgetJobLife{})
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 45 || projection.ObservedJobMinutes != 0 ||
				projection.OpenCapMinutes != 45 || projection.ActiveJobs != 1 || projection.Attempts != 1 {
				t.Fatalf("open %s job did not retain its cap reservation: %+v", status, projection)
			}
		})
	}
}

func TestFailedJobThatEndedSecondsAfterStartChargesOneMinute(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "failed", "reserve-failed", 3, 120, "failed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:10:12Z", pid: 4242,
	})
	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 1 || projection.ObservedJobMinutes != 1 {
		t.Fatalf("seconds-long failed job did not charge the one-minute floor: %+v", projection)
	}
}

func TestTimeoutAtTheCapChargesTheCapNeverMore(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "timeout", "reserve-timeout", 3, 120, "timeout", budgetJobLife{
		startedAt: "2026-08-28T08:00:00Z", endedAt: "2026-08-28T10:01:30Z", pid: 4242,
	})
	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 2, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 120 || projection.ObservedJobMinutes != 120 {
		t.Fatalf("timeout charge exceeded or missed its cap: %+v", projection)
	}
}

func TestSettledMinutesRoundUpLikeTheGovernedPath(t *testing.T) {
	start := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		seconds int
		want    uint64
	}{
		{seconds: 0, want: 1}, {seconds: 1, want: 1}, {seconds: 60, want: 1},
		{seconds: 61, want: 2}, {seconds: 544, want: 10}, {seconds: 711, want: 12},
		{seconds: 7260, want: 120},
	} {
		if got := settledJobMinutes(start, start.Add(time.Duration(test.seconds)*time.Second), 120); got != test.want {
			t.Errorf("settledJobMinutes for %d seconds = %d, want %d", test.seconds, got, test.want)
		}
	}
}

func TestNeverLaunchedTerminalRecordChargesZero(t *testing.T) {
	t.Run("pending setup husk", func(t *testing.T) {
		root := budgetProjectionRoot(t)
		writeBudgetJob(t, root, "husk", "reserve-husk", 3, 120, "cancelled", budgetJobLife{
			createdAt: "2026-08-28T08:05:00Z", endedAt: "2026-08-28T08:12:00Z",
		})
		projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 0 || projection.ObservedJobMinutes != 0 || projection.Attempts != 1 {
			t.Fatalf("never-launched setup husk consumed minutes: %+v", projection)
		}
	})
	t.Run("pending launch failure", func(t *testing.T) {
		root := budgetProjectionRoot(t)
		writeBudgetJob(t, root, "failed", "reserve-failed", 3, 120, "failed", budgetJobLife{
			startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z",
		})
		projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.ReservedJobMinutes != 0 || projection.ObservedJobMinutes != 0 || projection.Attempts != 1 {
			t.Fatalf("never-launched pending record consumed minutes: %+v", projection)
		}
	})
	t.Run("post discharge uses createdAt", func(t *testing.T) {
		root := budgetProjectionRoot(t)
		file := budgetGoal()
		file.Obligation = &goal.GovernedObligation{Revision: 6}
		zero := uint64(0)
		if err := obligationstate.RecordTerminal(root, "bounded", 3, 6, obligationstate.TerminalAttempt{
			RunID: "green-proof", Status: run.StatusGreen,
			StartedAt: "2026-08-28T09:00:00Z", EndedAt: "2026-08-28T09:25:00Z", PrunedAt: "2026-08-28T09:31:00Z",
			AttemptOrdinal: 1, ExecutionCostMinutes: 30, ObservedCostMinutes: 25,
			WeightGeneration: 1, BudgetEpoch: &zero, Breaker: run.BreakerClosed,
		}); err != nil {
			t.Fatal(err)
		}
		writeJSON(t, filepath.Join(root, "artifacts", "agents", "validation-weight.json"), map[string]any{
			"schema": 1, "generation": 2,
			"consumedProofs": []any{map[string]any{
				"runId": "green-proof", "goalId": "bounded", "goalRevision": 3, "obligationRevision": 6,
				"weightGeneration": 1, "consumedAt": "2026-08-28T09:30:00Z",
				"resetDecision":     map[string]any{"apply": true, "wouldRefuse": false},
				"dischargeDecision": map[string]any{"apply": true, "wouldRefuse": false},
			}},
		})
		writeBudgetJob(t, root, "after", "reserve-after", 3, 120, "cancelled", budgetJobLife{
			createdAt: "2026-08-28T09:45:00Z", endedAt: "2026-08-28T09:46:00Z",
		})
		writeBudgetJob(t, root, "before", "reserve-before", 3, 120, "cancelled", budgetJobLife{
			createdAt: "2026-08-28T09:00:00Z", endedAt: "2026-08-28T09:01:00Z",
		})
		projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 0 {
			t.Fatalf("post-discharge husks were not filtered by createdAt: %+v", projection)
		}
		writeBudgetJob(t, root, "stampless", "reserve-stampless", 3, 120, "cancelled", budgetJobLife{endedAt: "2026-08-28T09:46:00Z"})
		projection = ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
		if projection.Status != BudgetUnknown || projection.Unknown == nil || projection.Unknown.Record != "artifacts/agents/jobs/stampless.json" ||
			!strings.Contains(projection.Unknown.Reason, "startedAt or createdAt") {
			t.Fatalf("stampless post-discharge husk did not fail closed: %+v", projection)
		}
	})
}

func TestLaunchedTerminalRecordWithoutReadableTimestampsIsUnknown(t *testing.T) {
	for _, test := range []struct {
		name string
		life budgetJobLife
		want string
	}{
		{name: "missing start", life: budgetJobLife{endedAt: "2026-08-28T08:20:00Z", pid: 4242}, want: "startedAt"},
		{name: "missing end", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", pid: 4242}, want: "endedAt"},
		{name: "unreadable end", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "yesterday", pid: 4242}, want: "endedAt"},
		{name: "unreadable ownership proof", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", provenAt: "yesterday", endedAt: "2026-08-28T08:20:00Z", pid: 4242}, want: "ownershipProof.provenAt"},
		{name: "clock regressed", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:09:00Z", pid: 4242}, want: "CLOCK_REGRESSED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			writeBudgetJob(t, root, "bad-time", "reserve-bad-time", 3, 120, "completed", test.life)
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetUnknown || projection.Unknown == nil ||
				projection.Unknown.Record != "artifacts/agents/jobs/bad-time.json" || !strings.Contains(projection.Unknown.Reason, test.want) {
				t.Fatalf("bad terminal timestamp did not fail closed with %q: %+v", test.want, projection)
			}
		})
	}
}

func TestReservedJobMinutesIsSumOfNamedComponents(t *testing.T) {
	writeDelegated := func(t *testing.T, root string) {
		t.Helper()
		writeBudgetJob(t, root, "completed", "reserve-completed", 3, 120, "completed", budgetJobLife{
			startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242,
		})
		writeBudgetJob(t, root, "running", "reserve-running", 3, 45, "running", budgetJobLife{})
		writeBudgetJob(t, root, "never-launched", "reserve-never-launched", 3, 120, "failed", budgetJobLife{
			startedAt: "2026-08-28T08:30:00Z", endedAt: "2026-08-28T08:31:00Z",
		})
	}
	writeGoverned := func(t *testing.T, root string) {
		t.Helper()
		if err := obligationstate.RecordTerminal(root, "bounded", 3, 6, obligationstate.TerminalAttempt{
			RunID: "settled-run", Status: run.StatusGreen,
			StartedAt: "2026-08-28T08:10:00Z", EndedAt: "2026-08-28T08:35:00Z", PrunedAt: "2026-08-28T08:40:00Z",
			AttemptOrdinal: 1, ExecutionCostMinutes: 30, ObservedCostMinutes: 25,
			WeightGeneration: 1, Breaker: run.BreakerClosed,
		}); err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
		weightGeneration := uint64(1)
		store := &run.Store{Root: root, Now: func() time.Time { return now }}
		store.AdmitGoverned = func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
			return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{
				GoalRevision: 3, ObligationRevision: 6, Recurrence: governance.StandingSharedProcess,
				WeightGeneration: &weightGeneration, ExecutionCostMinutes: 30, AttemptOrdinal: 2,
				Budget: *budgetGoal().Budget, BudgetStartedAt: "2026-08-28T08:00:00Z",
				ExpectedAssumptions: governance.ObligationAssumptions{
					Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
					SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record",
				},
				AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: run.BreakerClosed,
			}}, nil
		}
		if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{
			Id: "live-run", Kind: "suite", Display: "live governed run", Log: "artifacts/live-run.log",
			GoalId: "bounded", ObligationRevision: 6,
		}); err != nil {
			t.Fatal(err)
		}
	}
	writeProofs := func(t *testing.T, root string, terminalIdentity proofrun.ProofIdentity) {
		t.Helper()
		liveIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "sum-live-proof",
			[]string{"gate"}, behaviorsurface.SupportedVersion)
		if err != nil {
			t.Fatal(err)
		}
		launcher, err := proofrun.CurrentProcessIdentity(nil)
		if err != nil {
			t.Fatal(err)
		}
		terminalStarted := time.Date(2026, 8, 28, 8, 0, 0, 0, time.UTC)
		terminal, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{
			ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
			ReservedMinutes: 20, Identity: terminalIdentity, Launcher: launcher, Now: terminalStarted, AttemptID: "sum-terminal-proof",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := proofrun.FinalizeAttempt(root, terminal.AttemptID, proofrun.TerminalFailed, 1, "controlled proof failure", nil,
			terminalStarted.Add(5*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{
			ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
			ReservedMinutes: 15, Identity: liveIdentity, Launcher: launcher,
			Now: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC), AttemptID: "sum-live-proof",
		}); err != nil {
			t.Fatal(err)
		}
	}

	for _, test := range []struct {
		name                        string
		delegated, governed, proofs bool
		observed, open, proof       uint64
		reserved, attempts          uint64
	}{
		{name: "delegated", delegated: true, observed: 10, open: 45, reserved: 55, attempts: 3},
		{name: "governed", governed: true, observed: 25, open: 30, reserved: 55, attempts: 2},
		{name: "mixed", delegated: true, governed: true, observed: 35, open: 75, reserved: 110, attempts: 5},
		{name: "proof reservations", proofs: true, observed: 10, proof: 35, reserved: 45, attempts: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := ""
			var terminalIdentity proofrun.ProofIdentity
			if test.proofs {
				root, terminalIdentity = dispatchProofFixture(t, "sum-terminal-proof")
			} else {
				root = budgetProjectionRoot(t)
			}
			if test.delegated {
				writeDelegated(t, root)
			}
			if test.governed {
				writeGoverned(t, root)
			}
			if test.proofs {
				writeBudgetJob(t, root, "completed", "reserve-completed", 3, 120, "completed", budgetJobLife{
					startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242,
				})
				writeProofs(t, root, terminalIdentity)
			}
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetKnown || projection.ObservedJobMinutes != test.observed || projection.OpenCapMinutes != test.open ||
				projection.ProofReservationMinutes != test.proof ||
				projection.ReservedJobMinutes != test.reserved || projection.Attempts != test.attempts ||
				projection.ReservedJobMinutes != projection.ObservedJobMinutes+projection.OpenCapMinutes+projection.ProofReservationMinutes {
				t.Fatalf("reserved-minute components do not add up: %+v", projection)
			}
			if test.name == "governed" && projection.ActiveJobs != 1 {
				t.Fatalf("live governed run counted as %d active jobs, want 1: %+v", projection.ActiveJobs, projection)
			}
			if test.proofs {
				lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{{
					GoalID: "bounded", GoalRevision: 3, Breaches: budgetAdmissionBreaches(projection),
					Reserved: reservedMinutesEvidence(projection),
				}}})
				want := "BUDGET_REFUSED: goal bounded revision=3 admission closed: attemptLimit used=3 limit=2, activeJobLimit used=1 limit=1; reserved observed=10 open-caps=0 proof=35 limit=75; rule=setup-refusal-release: terminal records with phase=setup and refusalClass=setup count as neither attempts nor reserved job minutes"
				if len(lines) != 1 || lines[0] != want {
					t.Fatalf("proof reservation refusal line = %v, want %q", lines, want)
				}
			}
		})
	}
}

func TestTwoBarsForChangesSpecimenSettlesToObservedMinutes(t *testing.T) {
	records := []struct {
		name                   string
		revision               uint64
		started, proven, ended string
	}{
		{name: "r26-a", revision: 26, started: "2026-09-02T11:48:39Z", proven: "2026-09-02T11:48:40Z", ended: "2026-09-02T11:48:51Z"},
		{name: "r26-b", revision: 26, started: "2026-09-02T11:34:35Z", proven: "2026-09-02T11:34:36Z", ended: "2026-09-02T11:46:27Z"},
		{name: "r26-c", revision: 26, started: "2026-09-02T16:01:59Z", proven: "2026-09-02T16:02:00Z", ended: "2026-09-02T16:15:41Z"},
		{name: "r26-d", revision: 26, started: "2026-09-02T11:52:22Z", proven: "2026-09-02T11:52:23Z", ended: "2026-09-02T11:52:34Z"},
		{name: "r26-e", revision: 26, started: "2026-09-02T15:49:32Z", proven: "2026-09-02T15:49:33Z", ended: "2026-09-02T15:58:37Z"},
		{name: "r26-f", revision: 26, started: "2026-09-02T16:18:10Z", proven: "2026-09-02T16:18:11Z", ended: "2026-09-02T16:29:46Z"},
		{name: "r28-a", revision: 28, started: "2026-09-02T16:35:27Z", proven: "2026-09-02T16:35:28Z", ended: "2026-09-02T16:46:05Z"},
		{name: "r28-b", revision: 28, started: "2026-09-02T16:48:04Z", proven: "2026-09-02T16:48:05Z", ended: "2026-09-02T16:56:49Z"},
	}
	for _, test := range []struct {
		revision uint64
		want     uint64
	}{{revision: 26, want: 50}, {revision: 28, want: 20}} {
		root := budgetProjectionRoot(t)
		for _, record := range records {
			if record.revision != test.revision {
				continue
			}
			writeBudgetJob(t, root, record.name, "reserve-"+record.name, record.revision, 120, "completed", budgetJobLife{
				startedAt: record.started, endedAt: record.ended, pid: 4242, provenAt: record.proven,
			})
		}
		file := budgetGoal()
		file.Revision = 28
		file.History = make([]goal.HistoryLine, 28)
		for index := range file.History {
			file.History[index].At = file.Claimed.At
		}
		file.Budget.AttemptLimit = 20
		file.Budget.ReservedJobMinutesLimit = 2000
		file.Budget.ActiveJobLimit = 20
		file.Claimed.Revision = test.revision
		file.Claimed.AccountingRevision = test.revision
		projection := ProjectBudget(root, file, time.Date(2026, 9, 2, 18, 0, 0, 0, time.UTC))
		if projection.Status != BudgetKnown || projection.ReservedJobMinutes != test.want || projection.ObservedJobMinutes != test.want || projection.OpenCapMinutes != 0 {
			t.Fatalf("specimen revision %d settled to %+v, want %d observed minutes", test.revision, projection, test.want)
		}
	}
}

func TestObservedMinutesRunFromTheOwnershipStamp(t *testing.T) {
	for _, test := range []struct {
		name       string
		life       budgetJobLife
		pidStarted bool
		want       uint64
	}{
		{name: "ownership proof", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", provenAt: "2026-08-28T08:12:00Z", endedAt: "2026-08-28T08:22:00Z", pid: 4242}, want: 10},
		{name: "absent proof falls back", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242}, want: 10},
		{name: "empty proof falls back", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:20:00Z", pid: 4242}, want: 10},
		{name: "pid start is ignored", life: budgetJobLife{startedAt: "2026-08-28T08:10:00Z", provenAt: "2026-08-28T08:12:00Z", endedAt: "2026-08-28T08:22:00Z", pid: 4242}, pidStarted: true, want: 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			writeBudgetJob(t, root, "ownership", "reserve-ownership", 3, 120, "completed", test.life)
			path := filepath.Join(root, "artifacts", "agents", "jobs", "ownership.json")
			if test.name == "empty proof falls back" || test.pidStarted {
				record, err := readObject(path)
				if err != nil {
					t.Fatal(err)
				}
				if test.name == "empty proof falls back" {
					record["ownershipProof"] = map[string]any{"provenAt": "", "source": "trusted-launcher"}
				}
				if test.pidStarted {
					record["pidStartedAt"] = time.Date(2026, 8, 28, 7, 0, 0, 0, time.UTC).Unix()
				}
				writeJSON(t, path, record)
			}
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetKnown || projection.ReservedJobMinutes != test.want {
				t.Fatalf("ownership-stamp settlement = %+v, want %d minutes", projection, test.want)
			}
		})
	}
}

func TestSTR2P2A01AccountingRevisionPreservesRaisedSpendAndSetBudgetResetsIt(t *testing.T) {
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	file.Claimed.Revision = 5
	file.Claimed.AccountingRevision = 3
	file.History[4].Reason = "Misclassified: from=1 to=3 evidence=refusal:BUDGET_REFUSED"
	writeBudgetJob(t, root, "root-before-raise-one", "reserve-before-one", 3, 20, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:10:00Z", endedAt: "2026-08-28T08:30:00Z", pid: 4242,
	})
	writeBudgetJob(t, root, "root-before-raise-two", "reserve-before-two", 4, 30, "running", budgetJobLife{})

	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 2 || projection.ReservedJobMinutes != 50 || projection.ActiveJobs != 1 {
		t.Fatalf("risk raise erased spend from the accounting interval: %+v", projection)
	}
	file.Claimed.AccountingRevision = file.Claimed.Revision
	reset := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if reset.Status != BudgetKnown || reset.Attempts != 0 || reset.ReservedJobMinutes != 0 || reset.ActiveJobs != 0 {
		t.Fatalf("human set-budget boundary did not reset the tally: %+v", reset)
	}
}

func TestUnconsumedDischargeJSONCannotResetTheBudgetProjection(t *testing.T) {
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	file.Obligation = &goal.GovernedObligation{Revision: 6}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "validation-weight.json"), map[string]any{
		"schema": 1, "generation": 2, "accumulated": 0, "landings": 0, "sinceUtc": "2026-08-28T09:30:00Z", "lastCommit": "landed",
		"lastDecision": map[string]any{"runId": "green-proof", "goalId": "bounded", "obligationRevision": 6,
			"decidedAt": "2026-08-28T09:30:00Z", "applied": true,
			"resetDecision":     map[string]any{"apply": true, "wouldRefuse": false, "reason": "authorized"},
			"dischargeDecision": map[string]any{"apply": true, "wouldRefuse": false, "reason": "authorized"}},
	})
	for _, job := range []struct {
		id, operation, started, status string
		cap                            int
	}{
		{id: "before", operation: "before", started: "2026-08-28T09:00:00Z", status: "completed", cap: 30},
		{id: "after", operation: "after", started: "2026-08-28T09:45:00Z", status: "running", cap: 20},
	} {
		writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", job.id+".json"), map[string]any{
			"jobId": job.id, "operationId": job.operation, "goalId": "bounded", "goalRevision": 3,
			"capMin": job.cap, "status": job.status, "startedAt": job.started,
		})
	}
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetUnknown || projection.Unknown == nil ||
		!strings.Contains(projection.Unknown.Reason, "consumed") {
		t.Fatalf("forged discharge JSON reset the projection without consuming a proof: %+v", projection)
	}
}

func TestPublishedSetupRetainsAttemptAndReservedMinutes(t *testing.T) {
	root := budgetProjectionRoot(t)
	stage := t.TempDir()
	capFile := writeJSON(t, filepath.Join(stage, "cap.json"), map[string]any{
		"capMin": 30, "capDeadline": "2026-08-28T10:00:00Z",
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	setup := filepath.Join(stage, "setup.json")
	if err := BuildSetup(root, setup, "reserved", "implementer", "", "main-1", "5", "bounded", 3, 3, capFile, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := RecordCreate(root, "reserved", setup); err != nil {
		t.Fatal(err)
	}

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 30 || projection.ActiveJobs != 1 {
		t.Fatalf("a crash after setup publication lost reservation spend: %+v", projection)
	}
}

func TestSetupRefusalsReleaseAttemptAndMinuteReservations(t *testing.T) {
	root := budgetProjectionRoot(t)
	file := budgetGoal()
	file.Budget.AttemptLimit = 3
	file.Budget.ReservedJobMinutesLimit = 120
	for _, name := range []string{"setup-refused-one", "setup-refused-two"} {
		writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", name+".json"), map[string]any{
			"jobId": name, "operationId": name, "goalId": "bounded", "goalRevision": 3,
			"capMin": 30, "status": "failed", "phase": "setup", "refusalClass": "setup",
		})
	}
	writeBudgetJob(t, root, "completed", "completed", 3, 30, "completed", budgetJobLife{
		startedAt: "2026-08-28T08:00:00Z", endedAt: "2026-08-28T08:30:00Z", pid: 4242,
	})

	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != 30 {
		t.Fatalf("two setup refusals and one completion did not leave two of three attempts free: %+v", projection)
	}

	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "protocol-error.json"), map[string]any{
		"jobId": "protocol-error", "operationId": "protocol-error", "goalId": "bounded", "goalRevision": 3,
		"capMin": 30, "status": "failed", "phase": "validation",
		"pid": 4242, "startedAt": "2026-08-28T08:30:00Z", "endedAt": "2026-08-28T09:00:00Z",
		"protocolError": map[string]any{"key": "invalid-return", "violation": "malformed implementer return"},
	})
	projection = ProjectBudget(root, file, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 2 || projection.ReservedJobMinutes != 60 {
		t.Fatalf("a protocol error after the agent started did not consume its reservation: %+v", projection)
	}
}

func TestBudgetProjectionReportsExactUnknownRecord(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(root string)
		want   string
	}{
		{
			name: "revisionless",
			mutate: func(root string) {
				writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "lost.json"), map[string]any{
					"jobId": "lost", "operationId": "reserve-lost", "goalId": "bounded", "capMin": 10, "status": "running",
				})
			},
			want: "artifacts/agents/jobs/lost.json",
		},
		{
			name: "duplicate operation",
			mutate: func(root string) {
				writeBudgetJob(t, root, "first", "reserve-same", 3, 10, "completed", budgetJobLife{})
				writeBudgetJob(t, root, "second", "reserve-same", 3, 10, "running", budgetJobLife{})
			},
			want: "artifacts/agents/jobs/second.json",
		},
		{
			name: "contradictory unbound revision",
			mutate: func(root string) {
				writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "contradictory.json"), map[string]any{
					"jobId": "contradictory", "operationId": "reserve-contradictory", "goalId": nil,
					"goalRevision": 3, "capMin": 10, "status": "running",
				})
			},
			want: "artifacts/agents/jobs/contradictory.json",
		},
		{
			name: "unreadable",
			mutate: func(root string) {
				path := filepath.Join(root, "artifacts", "agents", "jobs", "broken.json")
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				// Duplicate keys are unreadable under the authoritative wire grammar.
				data := []byte("{\"goalId\":\"bounded\",\"goalId\":\"bounded\"}\n")
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "artifacts/agents/jobs/broken.json",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			test.mutate(root)
			projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
			if projection.Status != BudgetUnknown || projection.Unknown == nil || projection.Unknown.Code != BudgetUnknown ||
				projection.Unknown.Record != test.want {
				t.Fatalf("unknown evidence did not name the exact record: %+v", projection)
			}
		})
	}
}

func TestBudgetProjectionSurfacesBreachesWithoutEnforcement(t *testing.T) {
	root := budgetProjectionRoot(t)
	writeBudgetJob(t, root, "one", "reserve-one", 3, 50, "running", budgetJobLife{})
	writeBudgetJob(t, root, "two", "reserve-two", 3, 50, "pending", budgetJobLife{})
	writeBudgetJob(t, root, "three", "reserve-three", 3, 10, "completed", budgetJobLife{})

	projection := ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.ElapsedState != ElapsedBreach || len(projection.Breaches) != 4 {
		t.Fatalf("all four breaches are health evidence, not a projection refusal: %+v", projection)
	}
	var fields []string
	for _, breach := range projection.Breaches {
		fields = append(fields, breach.Field)
	}
	if strings.Join(fields, ",") != "elapsedLimit,attemptLimit,reservedJobMinutesLimit,activeJobLimit" {
		t.Fatalf("breach fields = %v", fields)
	}
}

func TestBudgetAdmissionClosesAtEveryCurrentEqualityBoundary(t *testing.T) {
	projection := BudgetProjection{
		Status: BudgetKnown,
		Limits: goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 75, ActiveJobLimit: 1,
		},
		Elapsed: 4 * time.Hour, Attempts: 2, ReservedJobMinutes: 75, ActiveJobs: 1,
	}
	breaches := budgetAdmissionBreaches(projection)
	var fields []string
	for _, breach := range breaches {
		fields = append(fields, breach.Field)
	}
	if strings.Join(fields, ",") != "elapsedLimit,attemptLimit,reservedJobMinutesLimit,activeJobLimit" {
		t.Fatalf("admission equality boundaries = %v", fields)
	}
}

func TestBudgetAdmissionRefusalNamesSetupRefusalReleaseRule(t *testing.T) {
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{{
		GoalID: "bounded", GoalRevision: 3,
		Breaches: []BudgetBreach{budgetIntegerBreach("attemptLimit", 3, 3)},
	}}})
	if len(lines) != 1 || !strings.Contains(lines[0], "rule=setup-refusal-release") ||
		!strings.Contains(lines[0], "count as neither attempts nor reserved job minutes") {
		t.Fatalf("attempt refusal did not explain the setup-refusal release rule: %v", lines)
	}
}
