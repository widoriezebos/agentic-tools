package dispatch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func governedConclusionBed(t *testing.T, limit, attempts uint64, timingSeconds uint64) (string, uint64) {
	t.Helper()
	for _, name := range []string{"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		value, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, value)
			} else {
				_ = os.Unsetenv(name)
			}
		})
	}
	root := revisionBindingBed(t, 2)
	revision := installEnforcedObligation(t, root, attempts)
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 || file.Obligation == nil || file.Budget == nil {
		t.Fatalf("conclusion fixture goal did not parse: %v", problems)
	}
	file.Budget.ReservedJobMinutesLimit = limit
	file.Budget.AttemptLimit = attempts
	file.Budget.ActiveJobLimit = 3
	file.Obligation.Assumptions.TimingEnvelopeSeconds = timingSeconds
	file.Obligation.Assumptions.MaxActiveJobs = 3
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "plans/goals/bounded.md"}, {"commit", "-q", "-m", "conclusion fixture"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if output, runErr := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, output)
		}
	}
	return root, revision
}

func launchConclusionAttempt(t *testing.T, root string, revision uint64, now *time.Time) (*run.Store, *governedProofProber, string) {
	t.Helper()
	prober := &governedProofProber{alive: true, started: *now}
	store := NewConcludingRunStore(root, nil)
	store.Now = func() time.Time { return *now }
	store.Prober = prober
	store.Getpgid = func(pid int64) (int64, error) { return pid, nil }
	store.AllPids = func() ([]int64, error) { return nil, nil }
	store.AdmitGoverned = func(request run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		return EvaluateGovernedRunAdmission(root, request, *now)
	}
	nonce, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "concluding-run", Kind: "suite",
		Display: "concluding run", Log: "artifacts/concluding-run.log", GoalId: "bounded", ObligationRevision: revision, StandingShared: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("concluding-run", nonce, 5151, 5151); err != nil {
		t.Fatal(err)
	}
	return store, prober, nonce
}

func concludeAttemptRed(t *testing.T, store *run.Store, prober *governedProofProber, nonce string, now *time.Time) *run.Record {
	t.Helper()
	*now = now.Add(30 * time.Minute)
	if err := store.WriteSidecar("concluding-run", 1, nonce, 1); err != nil {
		t.Fatal(err)
	}
	prober.alive = false
	if result, err := store.Assess("concluding-run"); err != nil || result.To != run.StatusRed {
		t.Fatalf("governed run did not conclude red: %+v %v", result, err)
	}
	record, err := store.Read("concluding-run")
	if err != nil || record == nil || record.Governed == nil {
		t.Fatalf("read concluded run: %+v %v", record, err)
	}
	return record
}

func TestGovernedExhaustionReprojectsSettledSpendAtConclusion(t *testing.T) {
	t.Run("conclusion-observation-counts-itself", func(t *testing.T) {
		root, revision := governedConclusionBed(t, 240, 4, 1800)
		now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
		weightGeneration := uint64(0)
		others := &run.Store{Root: root, Now: func() time.Time { return now }, AdmitGoverned: func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
			return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{GoalRevision: 2, ObligationRevision: revision,
				WeightGeneration: &weightGeneration, Recurrence: governance.StandingSharedProcess, ExecutionCostMinutes: 30,
				AttemptOrdinal: 1, Budget: goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 3},
				BudgetStartedAt: now.Format(time.RFC3339), ExpectedAssumptions: governance.ObligationAssumptions{
					Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
					SurfaceDigest: "fixture-digest", MaxActiveJobs: 3, TimingEnvelopeSeconds: 1800,
					ObservationSource: "run-terminal-record",
				}, AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: run.BreakerClosed}}, nil
		}}
		for _, id := range []string{"other-one", "other-two"} {
			if _, err := others.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: id, Kind: "suite", Log: "artifacts/" + id + ".log",
				GoalId: "bounded", ObligationRevision: revision, StandingShared: true}); err != nil {
				t.Fatal(err)
			}
		}
		store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
		record, err := store.Read("concluding-run")
		if err != nil {
			t.Fatal(err)
		}
		record.Governed.ExpectedAssumptions.MaxActiveJobs = 2
		data, err := json.MarshalIndent(record, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(run.RecordPath(root, record.RunId), append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		concluded := concludeAttemptRed(t, store, prober, nonce, &now)
		if concluded.Governed.Observation == nil || concluded.Governed.Observation.ActiveJobs != 3 ||
			concluded.Governed.Observation.AssumptionState != run.AssumptionDrift ||
			!strings.Contains(strings.Join(concluded.Governed.Observation.DriftedFields, ","), "activeJobs") ||
			concluded.Governed.Breaker != run.BreakerAssumption {
			t.Fatalf("conclusion omitted itself from the assumption count: %+v", concluded.Governed)
		}
	})

	t.Run("foreign-malformed-obligation-state", func(t *testing.T) {
		root, revision := governedConclusionBed(t, 60, 3, 1800)
		now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
		store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
		foreign := filepath.Join(root, "artifacts", "agents", "governed-obligations", "foreign.g1.o1.json")
		if err := os.MkdirAll(filepath.Dir(foreign), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(foreign, []byte("not-json\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if record := concludeAttemptRed(t, store, prober, nonce, &now); record.Status != run.StatusRed {
			t.Fatalf("foreign malformed state blocked conclusion: %+v", record)
		}
	})

	t.Run("settled-cap", func(t *testing.T) {
		root, revision := governedConclusionBed(t, 150, 3, 1800)
		now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
		writeBudgetJob(t, root, "settling", "settling", 2, 120, "running", budgetJobLife{})
		store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
		admitted, err := store.Read("concluding-run")
		if err != nil || admitted == nil || admitted.Governed == nil || admitted.Governed.ReservedBefore != 120 {
			t.Fatalf("fixture was not admitted against the open cap: run=%+v err=%v", admitted, err)
		}
		writeBudgetJob(t, root, "settling", "settling", 2, 120, "completed", budgetJobLife{
			startedAt: "2026-08-28T08:00:00Z", endedAt: "2026-08-28T08:10:00Z", pid: 4242,
		})
		record := concludeAttemptRed(t, store, prober, nonce, &now)
		if record.Governed.Exhausted || record.Governed.Breaker == run.BreakerExhausted || record.Governed.ExhaustionReason != "" {
			t.Fatalf("a settled cap caused false exhaustion: %+v", record.Governed)
		}
	})

	t.Run("real-exhaustion", func(t *testing.T) {
		root, revision := governedConclusionBed(t, 150, 3, 1800)
		now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
		writeBudgetJob(t, root, "running", "running", 2, 120, "running", budgetJobLife{})
		store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
		record := concludeAttemptRed(t, store, prober, nonce, &now)
		want := "terminal non-green attempt reached the human-set tuple: observed=0 open-caps=120 attempt=30 limit=150"
		if !record.Governed.Exhausted || record.Governed.ExhaustionReason != want {
			t.Fatalf("real open-cap exhaustion was missed: %+v", record.Governed)
		}
	})

	t.Run("excludes-concluding-run", func(t *testing.T) {
		root, revision := governedConclusionBed(t, 60, 3, 1800)
		now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
		store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
		record := concludeAttemptRed(t, store, prober, nonce, &now)
		if record.Governed.Exhausted {
			t.Fatalf("concluding run charged itself twice: %+v", record.Governed)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		root, revision := governedConclusionBed(t, 150, 3, 1800)
		now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
		store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
		badPath := filepath.Join(root, "artifacts", "agents", "jobs", "duplicate.json")
		if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(badPath, []byte(`{"jobId":"first","jobId":"second","status":"running"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		record := concludeAttemptRed(t, store, prober, nonce, &now)
		if !record.Governed.Exhausted || !strings.Contains(record.Governed.ExhaustionReason,
			"BUDGET_UNKNOWN at conclusion: record=artifacts/agents/jobs/duplicate.json reason=") {
			t.Fatalf("unknown conclusion projection did not fail closed: %+v", record.Governed)
		}
	})

	t.Run("proof-reservation-after-admission", func(t *testing.T) {
		root, revision := governedConclusionBed(t, 60, 3, 1800)
		now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
		store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
		identity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "late-proof",
			[]string{"gate"}, behaviorsurface.SupportedVersion)
		if err != nil {
			t.Fatal(err)
		}
		launcher, err := proofrun.CurrentProcessIdentity(nil)
		if err != nil {
			t.Fatal(err)
		}
		proof, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{ControlRoot: root, ExecutionRoot: root,
			GoalID: "bounded", GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 30,
			Identity: identity, Launcher: launcher, Now: now.Add(time.Minute), AttemptID: "late-proof"})
		if err != nil {
			t.Fatal(err)
		}
		// A terminal proof is charged what it used (decision 3 of the
		// hang-detection design): this one runs its whole thirty-minute
		// reservation before it fails, so it counts for thirty.
		if _, err := proofrun.FinalizeAttempt(root, proof.AttemptID, proofrun.TerminalFailed, 1, "fixture", nil, now.Add(31*time.Minute)); err != nil {
			t.Fatal(err)
		}
		record := concludeAttemptRed(t, store, prober, nonce, &now)
		if !record.Governed.Exhausted || record.Governed.ExhaustionReason != "terminal non-green attempt reached the human-set tuple: observed=0 open-caps=0 proof=30 attempt=30 limit=60" {
			t.Fatalf("late proof reservation was absent from exhaustion: %+v", record.Governed)
		}
	})
}

func TestGovernedExhaustionRetryRaisesMissingDebtImmediately(t *testing.T) {
	root, revision := governedConclusionBed(t, 30, 2, 1800)
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
	before, err := os.ReadFile(run.RecordPath(root, "concluding-run"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.Read("concluding-run")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Minute)
	if err := store.WriteSidecar(record.RunId, record.Generation, nonce, 1); err != nil {
		t.Fatal(err)
	}
	prober.alive = false
	reason := "terminal non-green attempt reached the human-set tuple: observed=0 open-caps=0 attempt=30 limit=30"
	if err := obligationstate.RecordTerminal(root, "bounded", record.Governed.GoalRevision, revision, obligationstate.TerminalAttempt{
		RunID: record.RunId, Status: run.StatusRed, StartedAt: record.StartedAt, EndedAt: now.Format(time.RFC3339),
		AttemptOrdinal: record.Governed.AttemptOrdinal, ExecutionCostMinutes: 30, ObservedCostMinutes: 30,
		Breaker: run.BreakerExhausted, Exhausted: true, ExhaustionReason: reason,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Assess(record.RunId); err != nil {
		t.Fatal(err)
	}
	retried, readErr := store.Read(record.RunId)
	open, err := retrodebt.Open(root)
	state, _, stateErr := obligationstate.Load(root, "bounded", record.Governed.GoalRevision, revision)
	debtBytes, debtReadErr := os.ReadFile(retrodebt.Path(root))
	var firstDebtState struct {
		Generation uint64 `json:"generation"`
	}
	debtJSONErr := json.Unmarshal(debtBytes, &firstDebtState)
	if err != nil || stateErr != nil || readErr != nil || debtReadErr != nil || debtJSONErr != nil || len(open) != 1 ||
		state.Generation != 2 || !state.Attempts[0].RetroDebtRaised || retried == nil || retried.Governed == nil ||
		!retried.Governed.RetroDebtRaised || retried.Governed.Breaker != run.BreakerExhausted {
		t.Fatalf("retry did not repair missing debt once: run=%+v open=%+v state=%+v debt=%+v err=%v stateErr=%v", retried, open, state, firstDebtState, err, stateErr)
	}
	if err := os.WriteFile(run.RecordPath(root, record.RunId), before, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Assess(record.RunId); err != nil {
		t.Fatal(err)
	}
	open, err = retrodebt.Open(root)
	state, _, stateErr = obligationstate.Load(root, "bounded", record.Governed.GoalRevision, revision)
	debtBytes, debtReadErr = os.ReadFile(retrodebt.Path(root))
	var secondDebtState struct {
		Generation uint64 `json:"generation"`
	}
	debtJSONErr = json.Unmarshal(debtBytes, &secondDebtState)
	if err != nil || stateErr != nil || debtReadErr != nil || debtJSONErr != nil || len(open) != 1 || state.Generation != 2 ||
		secondDebtState.Generation != firstDebtState.Generation {
		t.Fatalf("second retry duplicated debt: open=%+v state=%+v firstDebt=%+v secondDebt=%+v err=%v stateErr=%v", open, state, firstDebtState, secondDebtState, err, stateErr)
	}
}

func TestGovernedExhaustionRetryConvergesAfterDebtBeforeRunWrite(t *testing.T) {
	root, revision := governedConclusionBed(t, 30, 2, 1800)
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
	before, err := os.ReadFile(run.RecordPath(root, "concluding-run"))
	if err != nil {
		t.Fatal(err)
	}
	first := concludeAttemptRed(t, store, prober, nonce, &now)
	if !first.Governed.Exhausted || !first.Governed.RetroDebtRaised {
		t.Fatalf("fixture did not reach post-debt state: %+v", first.Governed)
	}
	state, found, err := obligationstate.Load(root, "bounded", first.Governed.GoalRevision, revision)
	if err != nil || !found || state.Generation != 2 || len(state.Attempts) != 1 || !state.Attempts[0].RetroDebtRaised {
		t.Fatalf("durable post-debt fixture is wrong: %+v found=%t err=%v", state, found, err)
	}
	open, err := retrodebt.Open(root)
	if err != nil || len(open) != 1 {
		t.Fatalf("post-debt fixture has %d open debts: %v", len(open), err)
	}
	debtBytes, err := os.ReadFile(retrodebt.Path(root))
	var debtState struct {
		Generation uint64 `json:"generation"`
	}
	if err != nil || json.Unmarshal(debtBytes, &debtState) != nil || debtState.Generation != 1 {
		t.Fatalf("post-debt fixture generation is not one: state=%+v err=%v", debtState, err)
	}
	if err := os.WriteFile(run.RecordPath(root, "concluding-run"), before, 0o644); err != nil {
		t.Fatal(err)
	}
	binding, err := ResolveGoalBinding(root, "bounded", now)
	if err != nil {
		t.Fatal(err)
	}
	partial := ProjectBudget(root, binding.File, now)
	if partial.Status != BudgetUnknown || partial.Unknown == nil ||
		!strings.Contains(partial.Unknown.Reason, "durable obligation state claims unpruned spend for missing run concluding-run") {
		t.Fatalf("ordinary projection trusted the partial-commit window: %+v", partial)
	}
	if result, err := store.Assess("concluding-run"); err != nil || result.To != run.StatusRed {
		t.Fatalf("post-debt retry did not converge: %+v %v", result, err)
	}
	retried, readErr := store.Read("concluding-run")
	if readErr != nil || retried == nil || retried.Governed == nil || retried.Status != first.Status ||
		retried.EndedAt == nil || first.EndedAt == nil || *retried.EndedAt != *first.EndedAt ||
		retried.Governed.Breaker != first.Governed.Breaker ||
		retried.Governed.Exhausted != first.Governed.Exhausted || retried.Governed.ExhaustionReason != first.Governed.ExhaustionReason ||
		retried.Governed.RetroDebtRaised != first.Governed.RetroDebtRaised {
		t.Fatalf("retry did not adopt durable terminal fields: first=%+v retried=%+v err=%v", first, retried, readErr)
	}
	stateAfter, _, err := obligationstate.Load(root, "bounded", first.Governed.GoalRevision, revision)
	openAfter, debtErr := retrodebt.Open(root)
	debtBytesAfter, debtReadErr := os.ReadFile(retrodebt.Path(root))
	var debtStateAfter struct {
		Generation uint64 `json:"generation"`
	}
	debtJSONErr := json.Unmarshal(debtBytesAfter, &debtStateAfter)
	if err != nil || debtErr != nil || debtReadErr != nil || debtJSONErr != nil || debtStateAfter.Generation != debtState.Generation ||
		stateAfter.Generation != 2 || len(stateAfter.Attempts) != 1 || len(openAfter) != 1 {
		t.Fatalf("retry duplicated durable effects: state=%+v debt=%+v err=%v debtErr=%v", stateAfter, openAfter, err, debtErr)
	}
}

func TestGovernedNonExhaustedRetryIgnoresItsOwnPartialDurableAttempt(t *testing.T) {
	root, revision := governedConclusionBed(t, 60, 2, 1800)
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	store, prober, nonce := launchConclusionAttempt(t, root, revision, &now)
	before, err := os.ReadFile(run.RecordPath(root, "concluding-run"))
	if err != nil {
		t.Fatal(err)
	}
	first := concludeAttemptRed(t, store, prober, nonce, &now)
	if first.Governed.Exhausted || first.Governed.Breaker != run.BreakerClosed || first.Governed.ExhaustionReason != "" {
		t.Fatalf("fixture did not establish a non-exhausted terminal state: %+v", first.Governed)
	}
	firstObservation := *first.Governed.Observation
	firstObservation.DriftedFields = append([]string(nil), first.Governed.Observation.DriftedFields...)
	if firstObservation.DurationSeconds != 1800 || firstObservation.AssumptionState != run.AssumptionMatch {
		t.Fatalf("fixture first observation is not the intended thirty-minute match: %+v", firstObservation)
	}
	if err := os.WriteFile(run.RecordPath(root, "concluding-run"), before, 0o644); err != nil {
		t.Fatal(err)
	}
	interferenceAttempt := *first.Governed
	interferenceAttempt.ObservedCostMinutes = nil
	interferenceAttempt.Observation = nil
	interferenceAttempt.AttemptOrdinal++
	interferenceAttempt.Exhausted = false
	interferenceAttempt.ExhaustionReason = ""
	interferenceAttempt.RetroDebtRaised = false
	interferenceAttempt.Breaker = run.BreakerClosed
	interference := &run.Store{Root: root, Now: func() time.Time { return now }, AdmitGoverned: func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		return run.GovernedAdmissionResult{Attempt: interferenceAttempt}, nil
	}}
	if _, err := interference.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "retry-interference", Kind: "suite",
		Display: "retry interference", Log: "artifacts/retry-interference.log", GoalId: "bounded",
		ObligationRevision: revision, StandingShared: true}); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Read("concluding-run")
	if err != nil || restored == nil || first.EndedAt == nil {
		t.Fatalf("read restored retry fixture: run=%+v err=%v", restored, err)
	}
	adoptedEnd, err := time.Parse(time.RFC3339, *first.EndedAt)
	if err != nil {
		t.Fatal(err)
	}
	wouldRecompute := store.ObserveGoverned(restored, adoptedEnd)
	if wouldRecompute.ActiveJobs == firstObservation.ActiveJobs {
		t.Fatalf("retry fixture did not change the current observation: first=%+v current=%+v", firstObservation, wouldRecompute)
	}
	now = now.Add(10 * time.Minute)
	if _, err := store.Assess("concluding-run"); err != nil {
		t.Fatal(err)
	}
	retried, err := store.Read("concluding-run")
	if err != nil || retried == nil || retried.Governed == nil || retried.Governed.Observation == nil ||
		!reflect.DeepEqual(*retried.Governed.Observation, firstObservation) || retried.Status != first.Status ||
		retried.EndedAt == nil || first.EndedAt == nil || *retried.EndedAt != *first.EndedAt ||
		retried.Governed.Exhausted != first.Governed.Exhausted || retried.Governed.RetroDebtRaised != first.Governed.RetroDebtRaised ||
		retried.Governed.Breaker != first.Governed.Breaker || retried.Governed.ExhaustionReason != first.Governed.ExhaustionReason {
		t.Fatalf("retry changed its first observation or breaker: first=%+v retried=%+v err=%v", first.Governed, retried, err)
	}
}

func TestSettledSpendAtConclusionUsesStableBindingDiagnostics(t *testing.T) {
	root, revision := governedConclusionBed(t, 60, 2, 1800)
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	record := &run.Record{RunId: "diagnostic-run", GoalId: "bounded", Governed: &run.GovernedAttempt{GoalRevision: 99, ObligationRevision: revision}}
	observation := ObserveGovernedRun(root, record, now)
	_, unknown := SettledSpendAtConclusion(root, record, now)
	wantMismatch := "record=diagnostic-run reason=" + strings.Join(observation.DriftedFields, ",")
	if unknown != wantMismatch {
		t.Fatalf("revision mismatch diagnostic=%q, want observation diagnostic %q", unknown, wantMismatch)
	}
	record.GoalId = "missing"
	_, bindingErr := ResolveGoalBinding(root, "missing", now)
	_, unknown = SettledSpendAtConclusion(root, record, now)
	if bindingErr == nil || unknown != "record=diagnostic-run reason="+bindingErr.Error() {
		t.Fatalf("binding failure diagnostic=%q error=%v", unknown, bindingErr)
	}
}
