package obligationstate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

const (
	goalID             = "bounded"
	goalRevision       = 3
	obligationRevision = 1
	startedAt          = "2026-08-28T08:30:00Z"
	endedAt            = "2026-08-28T08:35:00Z"
)

func terminalAttempt(runID string) obligationstate.TerminalAttempt {
	return obligationstate.TerminalAttempt{
		RunID:                runID,
		Status:               run.StatusGreen,
		StartedAt:            startedAt,
		EndedAt:              endedAt,
		AttemptOrdinal:       1,
		ExecutionCostMinutes: 5,
		ObservedCostMinutes:  5,
		WeightGeneration:     1,
		Breaker:              run.BreakerClosed,
	}
}

func budgetGoal() *goal.GoalFile {
	return &goal.GoalFile{
		Id: goalID, State: goal.StateClaimed, Revision: 3,
		Claimed: &goal.ClaimRecord{
			Machine: "bed-m1", Lineage: "coordinator", At: "2026-08-28T08:00:00Z", Revision: goalRevision,
		},
		Budget: &goal.Budget{
			ElapsedLimit: "4h", AttemptLimit: 3, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1,
		},
		History: []goal.HistoryLine{
			{At: "2026-08-28T06:00:00Z"},
			{At: "2026-08-28T07:00:00Z"},
			{At: "2026-08-28T08:00:00Z"},
		},
	}
}

func budgetRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeTerminalRun(t *testing.T, root, runID string, executionCost uint64) {
	t.Helper()
	weightGeneration := uint64(1)
	observedCost := uint64(5)
	terminalSequence := int64(1)
	ended := endedAt
	record := run.Record{
		SchemaVersion: 1,
		RunId:         runID,
		Kind:          "suite",
		Display:       "governed terminal fixture",
		Custody:       run.CustodyWrapped,
		Generation:    1,
		LaunchNonce:   "0123456789abcdef0123456789abcdef",
		Log:           "artifacts/terminal.log",
		StartedAt:     startedAt,
		GoalId:        goalID,
		Governed: &run.GovernedAttempt{
			GoalRevision:         goalRevision,
			ObligationRevision:   obligationRevision,
			WeightGeneration:     &weightGeneration,
			Recurrence:           governance.SingleExperiment,
			ExecutionCostMinutes: executionCost,
			ObservedCostMinutes:  &observedCost,
			AttemptOrdinal:       1,
			Budget: goalbudget.Budget{
				ElapsedLimit: "4h", AttemptLimit: 3, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1,
			},
			BudgetStartedAt: startedAt,
			ExpectedAssumptions: governance.ObligationAssumptions{
				Recurrence: governance.SingleExperiment, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
				SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 300,
				ObservationSource: "run-terminal-record",
			},
			Observation: &run.AssumptionObservation{
				ObservedAt: endedAt, AssumptionState: run.AssumptionMatch,
			},
			Breaker: run.BreakerClosed,
		},
		StaleAfterMin: 30,
		WindDownMin:   10,
		Evidence:      run.Evidence{Mode: run.EvidenceNone},
		Status:        run.StatusGreen,
		TerminalSeq:   &terminalSequence,
		EndedAt:       &ended,
	}
	if problems := run.Validate(&record); len(problems) != 0 {
		t.Fatalf("terminal run fixture is invalid: %v", problems)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := run.RecordPath(root, runID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireBudgetUnknown(t *testing.T, projection dispatch.BudgetProjection, detail string) {
	t.Helper()
	if projection.Status != dispatch.BudgetUnknown || projection.Unknown == nil ||
		projection.Unknown.Code != dispatch.BudgetUnknown || !strings.Contains(projection.Unknown.Reason, detail) {
		t.Fatalf("projection did not refuse BUDGET_UNKNOWN with %q: %+v", detail, projection)
	}
}

func TestTerminalStateContradictionRefusesBudgetProjection(t *testing.T) {
	root := budgetRoot(t)
	attempt := terminalAttempt("cost-conflict")
	attempt.ExecutionCostMinutes = 6
	if err := obligationstate.RecordTerminal(root, goalID, goalRevision, obligationRevision, attempt); err != nil {
		t.Fatal(err)
	}
	writeTerminalRun(t, root, attempt.RunID, 5)

	projection := dispatch.ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	requireBudgetUnknown(t, projection, "terminal run evidence contradicts its durable obligation state")
}

func TestMissingDurableOwnerOrRunEvidenceRefusesBudgetProjection(t *testing.T) {
	t.Run("terminal run has no durable owner", func(t *testing.T) {
		root := budgetRoot(t)
		writeTerminalRun(t, root, "missing-owner", 5)

		projection := dispatch.ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
		requireBudgetUnknown(t, projection, "terminal governed spend has no durable obligation-state owner")
	})
	t.Run("unpruned durable owner has no run evidence", func(t *testing.T) {
		root := budgetRoot(t)
		attempt := terminalAttempt("missing-evidence")
		if err := obligationstate.RecordTerminal(root, goalID, goalRevision, obligationRevision, attempt); err != nil {
			t.Fatal(err)
		}

		projection := dispatch.ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
		requireBudgetUnknown(t, projection, "durable obligation state claims unpruned spend for missing run missing-evidence")
	})
}

func TestRecordTerminalRefusesConflictingRunIDReuse(t *testing.T) {
	root := t.TempDir()
	original := terminalAttempt("reused-run")
	if err := obligationstate.RecordTerminal(root, goalID, goalRevision, obligationRevision, original); err != nil {
		t.Fatal(err)
	}
	conflicting := original
	conflicting.ObservedCostMinutes++
	err := obligationstate.RecordTerminal(root, goalID, goalRevision, obligationRevision, conflicting)
	if err == nil || err.Error() != "governed run id reused-run already owns different terminal obligation state" {
		t.Fatalf("conflicting terminal run reuse was not refused: %v", err)
	}
	state, found, loadErr := obligationstate.Load(root, goalID, goalRevision, obligationRevision)
	if loadErr != nil || !found || len(state.Attempts) != 1 || !reflect.DeepEqual(state.Attempts[0], original) {
		t.Fatalf("conflicting reuse changed the original owner: %+v found=%t err=%v", state, found, loadErr)
	}
}

func TestFindRunRefusesDuplicateDurableClaims(t *testing.T) {
	root := t.TempDir()
	attempt := terminalAttempt("duplicate-run")
	if err := obligationstate.RecordTerminal(root, goalID, goalRevision, 1, attempt); err != nil {
		t.Fatal(err)
	}
	if err := obligationstate.RecordTerminal(root, goalID, goalRevision, 2, attempt); err != nil {
		t.Fatal(err)
	}

	found, record, err := obligationstate.FindRun(root, attempt.RunID)
	if err == nil || found != nil || record != "artifacts/agents/governed-obligations/bounded.g3.o2.json" ||
		err.Error() != "governed run id duplicate-run is claimed by multiple obligation records" {
		t.Fatalf("duplicate durable claim did not refuse exactly: found=%+v record=%q err=%v", found, record, err)
	}
}

func TestLoadRefusesMalformedState(t *testing.T) {
	root := t.TempDir()
	path := obligationstate.Path(root, goalID, goalRevision, obligationRevision)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"schema":1,"generation":`), 0o644); err != nil {
		t.Fatal(err)
	}

	state, found, err := obligationstate.Load(root, goalID, goalRevision, obligationRevision)
	if err == nil || found || !reflect.DeepEqual(state, obligationstate.State{}) ||
		!strings.Contains(err.Error(), "malformed obligation execution record") {
		t.Fatalf("malformed state did not fail closed: state=%+v found=%t err=%v", state, found, err)
	}
}

func TestTerminalStateRoundTripRetainsFieldsAndPruneMarker(t *testing.T) {
	root := budgetRoot(t)
	attempt := terminalAttempt("round-trip")
	if err := obligationstate.RecordTerminal(root, goalID, goalRevision, obligationRevision, attempt); err != nil {
		t.Fatal(err)
	}
	state, found, err := obligationstate.Load(root, goalID, goalRevision, obligationRevision)
	if err != nil || !found || state.Schema != obligationstate.Schema || state.Generation != 1 ||
		state.GoalID != goalID || state.GoalRevision != goalRevision || state.ObligationRevision != obligationRevision ||
		len(state.Attempts) != 1 || !reflect.DeepEqual(state.Attempts[0], attempt) {
		t.Fatalf("terminal state did not round-trip intact: %+v found=%t err=%v", state, found, err)
	}

	prunedAt := time.Date(2026, 8, 29, 8, 35, 0, 0, time.UTC)
	if err := obligationstate.MarkPruned(root, goalID, goalRevision, obligationRevision, attempt.RunID, prunedAt); err != nil {
		t.Fatal(err)
	}
	state, found, err = obligationstate.Load(root, goalID, goalRevision, obligationRevision)
	want := attempt
	want.PrunedAt = prunedAt.Format(time.RFC3339)
	if err != nil || !found || state.Generation != 2 || len(state.Attempts) != 1 || !reflect.DeepEqual(state.Attempts[0], want) {
		t.Fatalf("prune retention marker did not round-trip: %+v found=%t err=%v", state, found, err)
	}
	projection := dispatch.ProjectBudget(root, budgetGoal(), time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC))
	if projection.Status != dispatch.BudgetKnown || projection.Attempts != 1 || projection.ReservedJobMinutes != attempt.ObservedCostMinutes {
		t.Fatalf("pruned run evidence did not retain durable spend: %+v", projection)
	}
}

func TestRetroDebtMarkerRoundTripsAndIsIdempotent(t *testing.T) {
	root := budgetRoot(t)
	attempt := terminalAttempt("retro-round-trip")
	if err := obligationstate.RecordTerminal(root, goalID, goalRevision, obligationRevision, attempt); err != nil {
		t.Fatal(err)
	}
	if err := obligationstate.MarkRetroDebt(root, goalID, goalRevision, obligationRevision, attempt.RunID); err != nil {
		t.Fatal(err)
	}
	state, found, err := obligationstate.Load(root, goalID, goalRevision, obligationRevision)
	if err != nil || !found || state.Generation != 2 || len(state.Attempts) != 1 || !state.Attempts[0].RetroDebtRaised {
		t.Fatalf("retro debt marker did not round-trip: %+v found=%t err=%v", state, found, err)
	}

	if err := obligationstate.MarkRetroDebt(root, goalID, goalRevision, obligationRevision, attempt.RunID); err != nil {
		t.Fatal(err)
	}
	state, found, err = obligationstate.Load(root, goalID, goalRevision, obligationRevision)
	if err != nil || !found || state.Generation != 2 || !state.Attempts[0].RetroDebtRaised {
		t.Fatalf("repeating the retro debt marker was not idempotent: %+v found=%t err=%v", state, found, err)
	}
}
