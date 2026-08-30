package steward

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestStewardTickFailsClosedWhenAssumptionObservationIsUnavailable(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	weightGeneration := uint64(0)
	assumptions := governance.ObligationAssumptions{Recurrence: governance.StandingSharedProcess,
		Platform: "fixture/os", ToolchainIdentity: "fixture-go", SurfaceDigest: "fixture-digest",
		MaxActiveJobs: 1, TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record"}
	store := &run.Store{Root: root, Now: func() time.Time { return now },
		AdmitGoverned: func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
			return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{GoalRevision: 2, ObligationRevision: 3,
				WeightGeneration: &weightGeneration,
				Recurrence:       governance.StandingSharedProcess, ExecutionCostMinutes: 1, AttemptOrdinal: 1,
				Budget:          goalbudget.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 10, ActiveJobLimit: 1},
				BudgetStartedAt: now.Format(time.RFC3339), ExpectedAssumptions: assumptions,
				AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Observation: &run.AssumptionObservation{
					ObservedAt: now.Format(time.RFC3339), AssumptionState: run.AssumptionMatch}, Breaker: run.BreakerClosed}}, nil
		}}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "unavailable-observation", Kind: "suite",
		Display: "unavailable observation", Log: "artifacts/unavailable.log", GoalId: "bounded",
		ObligationRevision: 3, StandingShared: true}); err != nil {
		t.Fatal(err)
	}
	if failures := refreshGovernedObligations(root, now.Add(time.Minute)); len(failures) != 0 {
		t.Fatalf("steward could not record the unavailable observation: %v", failures)
	}
	verdict := checkGovernedObligations(root)
	if verdict.Status != HealthDead || !verdict.NoAutomaticRemedy || !strings.Contains(verdict.Reason, "observation unavailable") {
		t.Fatalf("unavailable evidence did not fail closed: %+v", verdict)
	}
}

func TestStandingWatchFlagsOnlyTheNamedUngovernedProblemClass(t *testing.T) {
	root := t.TempDir()
	store := &run.Store{Root: root}
	caller := run.Caller{Class: "HUMAN"}
	if _, err := store.Launch(caller, run.LaunchParams{Id: "ordinary-goal-run", Kind: "suite",
		Display: "ordinary goal validation", Log: "artifacts/ordinary.log", GoalId: "ordinary"}); err != nil {
		t.Fatal(err)
	}
	if verdict := checkGovernedObligations(root); verdict.Status != HealthAlive {
		t.Fatalf("ordinary goal work was mistaken for a governing mechanism: %+v", verdict)
	}
	if _, err := store.Launch(caller, run.LaunchParams{Id: "ungoverned-direct-validator", Kind: "suite",
		Display: "weight-triggered direct validation", Log: "artifacts/direct.log", GoalId: "validator"}); err != nil {
		t.Fatal(err)
	}
	verdict := checkGovernedObligations(root)
	if verdict.Status != HealthDead || !strings.Contains(verdict.Reason, "recurring ungoverned execution") {
		t.Fatalf("the named ungoverned problem class was not flagged: %+v", verdict)
	}
}
