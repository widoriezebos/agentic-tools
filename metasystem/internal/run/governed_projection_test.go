package run

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
)

func projectionAttempt(now time.Time, reservedLimit uint64) GovernedAttempt {
	weightGeneration := uint64(0)
	return GovernedAttempt{
		GoalRevision: 9, ObligationRevision: 7, Recurrence: governance.StandingSharedProcess,
		WeightGeneration: &weightGeneration, ExecutionCostMinutes: 2, AttemptOrdinal: 1,
		Budget:          goalbudget.Budget{ElapsedLimit: "2h", AttemptLimit: 2, ReservedJobMinutesLimit: reservedLimit, ActiveJobLimit: 1},
		BudgetStartedAt: now.Format(time.RFC3339), ExpectedAssumptions: governance.ObligationAssumptions{
			Recurrence: governance.StandingSharedProcess, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
			SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 7200, ObservationSource: "run-terminal-record",
		},
		AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: BreakerClosed,
	}
}

func launchProjectionRun(t *testing.T, store *Store, id string, reservedLimit uint64) {
	t.Helper()
	now := store.Now()
	store.AdmitGoverned = func(GovernedAdmissionRequest) (GovernedAdmissionResult, error) {
		return GovernedAdmissionResult{Attempt: projectionAttempt(now, reservedLimit)}, nil
	}
	store.ObserveGoverned = func(*Record, time.Time) AssumptionObservation {
		return AssumptionObservation{ObservedAt: store.Now().UTC().Format(time.RFC3339), AssumptionState: AssumptionMatch}
	}
	if _, err := store.Launch(mainCaller, LaunchParams{Id: id, Kind: "suite", Display: id,
		Log: "artifacts/" + id + ".log", GoalId: "bounded", ObligationRevision: 7, StandingShared: true}); err != nil {
		t.Fatal(err)
	}
}

func TestBareStoreRefusesEveryGovernedConclusionPath(t *testing.T) {
	for _, test := range []struct {
		name     string
		prepare  func(*Store)
		conclude func(*Store) error
	}{
		{name: "FailLaunch", conclude: func(store *Store) error { return store.FailLaunch("bare", "fixture") }},
		{name: "Stop", conclude: func(store *Store) error { _, err := store.Stop("bare"); return err }},
		{name: "Assess", prepare: func(store *Store) {
			started := store.Now()
			store.Now = func() time.Time { return started.Add(2 * time.Minute) }
		}, conclude: func(store *Store) error { _, err := store.Assess("bare"); return err }},
		{name: "SweepStale", prepare: func(store *Store) {
			pid := int64(74)
			started := store.Now()
			store.Prober = fakeProber{verdicts: map[int64]identity.Liveness{pid: identity.Alive}, starts: map[int64]int64{pid: started.Unix()}}
			record, err := store.Read("bare")
			if err != nil || record == nil {
				t.Fatalf("read sweep fixture: %+v %v", record, err)
			}
			if err := store.Bind("bare", record.LaunchNonce, pid, pid); err != nil {
				t.Fatal(err)
			}
			record, err = store.Read("bare")
			if err != nil {
				t.Fatal(err)
			}
			epoch := int64(1)
			record.ClaimEpoch = &epoch
			if err := store.write(record); err != nil {
				t.Fatal(err)
			}
		}, conclude: func(store *Store) error {
			return store.SweepStale(2, func(int64, string) (bool, bool) { return true, true }, func(int64) error { return nil })
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := testStore(t)
			launchProjectionRun(t, store, "bare", 10)
			if test.prepare != nil {
				test.prepare(store)
			}
			store.ProjectSpend = nil
			before, err := store.Read("bare")
			if err != nil {
				t.Fatal(err)
			}
			err = test.conclude(store)
			if !errors.Is(err, ErrNoSpendProjection) {
				t.Fatalf("bare governed conclusion error=%v, want ErrNoSpendProjection", err)
			}
			after, readErr := store.Read("bare")
			if readErr != nil || after == nil || after.Status != before.Status || after.Generation != before.Generation {
				t.Fatalf("refused conclusion changed the run: before=%+v after=%+v err=%v", before, after, readErr)
			}
			if attempt, path, findErr := obligationstate.FindRun(store.Root, "bare"); findErr != nil || attempt != nil {
				t.Fatalf("refused conclusion wrote durable state: attempt=%+v path=%s err=%v", attempt, path, findErr)
			}
		})
	}
}

func TestGovernedExhaustionReservedMinuteComparisonCannotWrap(t *testing.T) {
	store := testStore(t)
	start := store.Now()
	launchProjectionRun(t, store, "overflow", math.MaxUint64)
	store.ProjectSpend = func(*Record, time.Time) (SpendSnapshot, string) {
		return SpendSnapshot{ObservedMinutes: math.MaxUint64 - 1}, ""
	}
	store.Now = func() time.Time { return start.Add(2 * time.Minute) }
	if err := store.FailLaunch("overflow", "fixture"); err != nil {
		t.Fatal(err)
	}
	record, err := store.Read("overflow")
	if err != nil || record == nil || record.Governed == nil || !record.Governed.Exhausted || record.Governed.Breaker != BreakerExhausted {
		t.Fatalf("overflow-safe comparison did not exhaust: record=%+v err=%v", record, err)
	}
}

func TestGovernedSpendProjectionUsesConclusionClockNotEndedAt(t *testing.T) {
	store := testStore(t)
	start := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	store.Now = func() time.Time { return start }
	pid := int64(73)
	store.Prober = fakeProber{verdicts: map[int64]identity.Liveness{pid: identity.Alive}, starts: map[int64]int64{pid: start.Unix()}}
	launchProjectionRun(t, store, "clock", 60)
	record, err := store.Read("clock")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("clock", record.LaunchNonce, pid, pid); err != nil {
		t.Fatal(err)
	}
	record, err = store.Read("clock")
	if err != nil {
		t.Fatal(err)
	}
	endedAt := start.Format(time.RFC3339)
	record.Status = StatusDraining
	record.EndedAt = &endedAt
	verdict := StatusRed
	record.ProvisionalVerdict = &verdict
	if err := store.write(record); err != nil {
		t.Fatal(err)
	}
	conclusion := start.Add(time.Hour)
	store.Now = func() time.Time { return conclusion }
	var projectedAt time.Time
	store.ProjectSpend = func(_ *Record, now time.Time) (SpendSnapshot, string) {
		projectedAt = now
		return SpendSnapshot{}, ""
	}
	if _, err := store.Assess("clock"); err != nil {
		t.Fatal(err)
	}
	if !projectedAt.Equal(conclusion) {
		t.Fatalf("projection time=%s, want conclusion clock %s", projectedAt, conclusion)
	}
}
