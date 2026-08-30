package run

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
)

func governedParams() LaunchParams {
	return LaunchParams{Id: "governed-proof", Kind: "suite", Display: "governed proof",
		Log: "artifacts/governed-proof.log", GoalId: "bounded", ObligationRevision: 7, StandingShared: true}
}

func TestGovernedAdmissionRefusesBeforeRecordWhenTupleIsUnavailable(t *testing.T) {
	s := testStore(t)
	_, err := s.Launch(mainCaller, governedParams())
	if err == nil || !strings.Contains(err.Error(), "admission is unavailable") {
		t.Fatalf("missing tuple did not refuse at admission: %v", err)
	}
	if record, readErr := s.Read("governed-proof"); readErr != nil || record != nil {
		t.Fatalf("refused admission published a run: %+v %v", record, readErr)
	}
}

func TestFailingTerminalAttemptOwnsExhaustionAndRaisesDebt(t *testing.T) {
	s := testStore(t)
	now := s.Now()
	weightGeneration := uint64(0)
	s.AdmitGoverned = func(GovernedAdmissionRequest) (GovernedAdmissionResult, error) {
		return GovernedAdmissionResult{Attempt: GovernedAttempt{
			GoalRevision: 9, ObligationRevision: 7, Recurrence: governance.StandingSharedProcess,
			WeightGeneration:     &weightGeneration,
			ExecutionCostMinutes: 1, AttemptOrdinal: 1,
			Budget:          goalbudget.Budget{ElapsedLimit: "2h", AttemptLimit: 1, ReservedJobMinutesLimit: 10, ActiveJobLimit: 1},
			BudgetStartedAt: now.Format(time.RFC3339), ExpectedAssumptions: governance.ObligationAssumptions{
				Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
				SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record"},
			AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: BreakerClosed,
		}}, nil
	}
	s.ObserveGoverned = func(*Record, time.Time) AssumptionObservation {
		return AssumptionObservation{ObservedAt: now.Format(time.RFC3339), AssumptionState: AssumptionMatch}
	}
	if _, err := s.Launch(mainCaller, governedParams()); err != nil {
		t.Fatal(err)
	}
	if err := s.FailLaunch("governed-proof", "fixture failure"); err != nil {
		t.Fatal(err)
	}
	record, err := s.Read("governed-proof")
	if err != nil || record == nil || !record.Governed.Exhausted || record.Governed.Breaker != BreakerExhausted || !record.Governed.RetroDebtRaised {
		t.Fatalf("terminalization did not own exhaustion and debt: %+v %v", record, err)
	}
	open, err := retrodebt.Open(s.Root)
	if err != nil || len(open) != 1 || open[0].Kind != retrodebt.KindObligation {
		t.Fatalf("debt was not raised by the failing terminal attempt: %+v %v", open, err)
	}
}
