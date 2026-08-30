package run

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
)

func authorityAttempt(now time.Time, attemptLimit uint64) GovernedAttempt {
	weightGeneration := uint64(1)
	return GovernedAttempt{
		GoalRevision: 9, ObligationRevision: 7, Recurrence: governance.StandingSharedProcess,
		WeightGeneration: &weightGeneration, ExecutionCostMinutes: 1, AttemptOrdinal: 1,
		Budget:          goalbudget.Budget{ElapsedLimit: "2h", AttemptLimit: attemptLimit, ReservedJobMinutesLimit: 10, ActiveJobLimit: 1},
		BudgetStartedAt: now.Format(time.RFC3339), ExpectedAssumptions: governance.ObligationAssumptions{
			Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
			SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record"},
		AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: BreakerClosed,
	}
}

func governedAuthorityStore(t *testing.T, attemptLimit uint64) *Store {
	t.Helper()
	s := testStore(t)
	s.AdmitGoverned = func(GovernedAdmissionRequest) (GovernedAdmissionResult, error) {
		return GovernedAdmissionResult{Attempt: authorityAttempt(s.Now(), attemptLimit)}, nil
	}
	return s
}

func TestGovernedLaunchRefusesIncompleteObligationTupleBeforePublishing(t *testing.T) {
	for _, test := range []struct {
		name   string
		params LaunchParams
	}{
		{name: "missing goal", params: LaunchParams{Id: "missing-goal", Kind: "suite", Log: "missing-goal.log", ObligationRevision: 7, StandingShared: true}},
		{name: "missing revision", params: LaunchParams{Id: "missing-revision", Kind: "suite", Log: "missing-revision.log", GoalId: "bounded", StandingShared: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := governedAuthorityStore(t, 2)
			s.AdmitGoverned = func(GovernedAdmissionRequest) (GovernedAdmissionResult, error) {
				t.Fatal("incomplete tuple reached admission")
				return GovernedAdmissionResult{}, nil
			}
			if _, err := s.Launch(mainCaller, test.params); err == nil || !strings.Contains(err.Error(), "requires both goal id and obligation revision") {
				t.Fatalf("incomplete obligation tuple did not refuse: %v", err)
			}
			if record, err := s.Read(test.params.Id); err != nil || record != nil {
				t.Fatalf("refused tuple published a record: %+v %v", record, err)
			}
		})
	}
}

func TestRecordGovernedObservationUpdatesOnlyLiveGovernedAuthority(t *testing.T) {
	s := governedAuthorityStore(t, 2)
	if _, err := s.Launch(mainCaller, governedParams()); err != nil {
		t.Fatal(err)
	}
	observedAt := s.Now().Format(time.RFC3339)
	match := AssumptionObservation{ObservedAt: observedAt, AssumptionState: AssumptionMatch}
	if err := s.RecordGovernedObservation("governed-proof", match); err != nil {
		t.Fatal(err)
	}
	record, err := s.Read("governed-proof")
	if err != nil || record == nil || record.Governed.Observation == nil || record.Governed.Observation.AssumptionState != AssumptionMatch ||
		record.Governed.Breaker != BreakerClosed {
		t.Fatalf("matching observation did not close the live breaker: %+v %v", record, err)
	}
	unavailable := AssumptionObservation{ObservedAt: observedAt, AssumptionState: AssumptionUnavailable, DriftedFields: []string{"surfaceDigest"}}
	if err := s.RecordGovernedObservation("governed-proof", unavailable); err != nil {
		t.Fatal(err)
	}
	record, err = s.Read("governed-proof")
	if err != nil || record.Governed.Breaker != BreakerAssumption || record.Governed.ExhaustionReason != "ASSUMPTION_DRIFT" {
		t.Fatalf("unavailable observation did not fail closed: %+v %v", record, err)
	}
	launchOne(t, s, "ungoverned-observation")
	if err := s.RecordGovernedObservation("ungoverned-observation", unavailable); err != nil {
		t.Fatalf("ungoverned observation was not an inert no-op: %v", err)
	}
	if err := s.RecordGovernedObservation("missing-observation", unavailable); err == nil || !strings.Contains(err.Error(), "no readable run record") {
		t.Fatalf("missing observation target did not refuse: %v", err)
	}
}

func TestUnavailableTerminalObservationFailsClosedAndFencesGovernedID(t *testing.T) {
	s := governedAuthorityStore(t, 2)
	if _, err := s.Launch(mainCaller, governedParams()); err != nil {
		t.Fatal(err)
	}
	if err := s.FailLaunch("governed-proof", "fixture failure"); err != nil {
		t.Fatal(err)
	}
	record, err := s.Read("governed-proof")
	if err != nil || record == nil || record.Governed.Observation == nil ||
		record.Governed.Observation.AssumptionState != AssumptionUnavailable || record.Governed.Breaker != BreakerAssumption ||
		record.Governed.Exhausted || record.Governed.RetroDebtRaised {
		t.Fatalf("terminalization did not record unavailable evidence without inventing exhaustion: %+v %v", record, err)
	}
	state, found, err := obligationstate.Load(s.Root, "bounded", 9, 7)
	if err != nil || !found || len(state.Attempts) != 1 || state.Attempts[0].Breaker != BreakerAssumption {
		t.Fatalf("terminal authority was not durably recorded: %+v found=%t err=%v", state, found, err)
	}
	assertTypedFence := func(err error) {
		t.Helper()
		var typed *TerminalGovernedRunIDReuseError
		if !errors.As(err, &typed) || typed.RunID != "governed-proof" || !strings.Contains(err.Error(), "REFUSED-TERMINAL-GOVERNED-ID") {
			t.Fatalf("governed ID refusal was not typed: %#v", err)
		}
	}
	_, err = s.Launch(mainCaller, LaunchParams{Id: "governed-proof", Kind: "suite", Log: "overlay.log"})
	assertTypedFence(err)
	if err := s.Ack(mainCaller, "governed-proof"); err != nil {
		t.Fatal(err)
	}
	s.Now = func() time.Time { return time.Unix(1786900000, 0).Add(15 * 24 * time.Hour) }
	dropped, err := s.Prune(mainCaller)
	if err != nil || len(dropped) != 1 || !strings.HasPrefix(dropped[0], "governed-proof ") {
		t.Fatalf("governed evidence did not prune behind durable authority: %v %v", dropped, err)
	}
	_, err = s.Launch(mainCaller, LaunchParams{Id: "governed-proof", Kind: "suite", Log: "post-prune-overlay.log"})
	assertTypedFence(err)
}

func TestExhaustedTerminalRepairsFailedRetroDebtPublication(t *testing.T) {
	s := governedAuthorityStore(t, 1)
	s.ObserveGoverned = func(*Record, time.Time) AssumptionObservation {
		return AssumptionObservation{ObservedAt: s.Now().Format(time.RFC3339), AssumptionState: AssumptionMatch}
	}
	if _, err := s.Launch(mainCaller, governedParams()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(retrodebt.Path(s.Root), 0o755); err != nil {
		t.Fatal(err)
	}
	err := s.FailLaunch("governed-proof", "fixture failure")
	if err == nil || !strings.Contains(err.Error(), "retro debt failed") {
		t.Fatalf("failed debt publication did not surface: %v", err)
	}
	record, readErr := s.Read("governed-proof")
	if readErr != nil || record == nil || !record.Governed.Exhausted || record.Governed.Breaker != BreakerExhausted || record.Governed.RetroDebtRaised {
		t.Fatalf("exhaustion was not retained for repair: %+v %v", record, readErr)
	}
	if err := os.Remove(retrodebt.Path(s.Root)); err != nil {
		t.Fatal(err)
	}
	if err := s.RepairGovernedDebt("governed-proof"); err != nil {
		t.Fatalf("repair did not publish the retained debt: %v", err)
	}
	record, readErr = s.Read("governed-proof")
	if readErr != nil || !record.Governed.RetroDebtRaised {
		t.Fatalf("repair was not marked on the terminal record: %+v %v", record, readErr)
	}
	if err := s.RepairGovernedDebt("governed-proof"); err != nil {
		t.Fatalf("completed repair was not idempotent: %v", err)
	}
	if err := s.RepairGovernedDebt("missing-governed-proof"); err == nil || !strings.Contains(err.Error(), "no readable run record") {
		t.Fatalf("missing repair target did not refuse: %v", err)
	}
}
