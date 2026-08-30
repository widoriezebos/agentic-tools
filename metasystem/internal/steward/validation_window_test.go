package steward

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func writeValidationRun(t *testing.T, root, id, status, log string, seq int64) {
	t.Helper()
	ended := "2026-08-30T11:00:00Z"
	observed := uint64(1)
	weightGeneration := uint64(3)
	record := run.Record{SchemaVersion: 1, RunId: id, Kind: "suite", Display: "weight-triggered direct validation",
		Custody: run.CustodyWrapped, Generation: 1, LaunchNonce: strings.Repeat("a", 32), Log: log,
		StartedAt: "2026-08-30T10:59:00Z", GoalId: "bounded", StaleAfterMin: 30, WindDownMin: 10,
		Evidence: run.Evidence{Mode: run.EvidenceSidecar}, Status: status, TerminalSeq: &seq, EndedAt: &ended,
		Governed: &run.GovernedAttempt{GoalRevision: 2, ObligationRevision: 3, WeightGeneration: &weightGeneration,
			Recurrence: governance.StandingSharedProcess, ExecutionCostMinutes: 1, ObservedCostMinutes: &observed,
			AttemptOrdinal: uint64(seq), Budget: goalbudget.Budget{ElapsedLimit: "1h", AttemptLimit: 4,
				ReservedJobMinutesLimit: 10, ActiveJobLimit: 1}, BudgetStartedAt: "2026-08-30T10:00:00Z",
			ExpectedAssumptions: governance.ObligationAssumptions{Recurrence: governance.StandingSharedProcess,
				Platform: "fixture/os", ToolchainIdentity: "fixture-go", SurfaceDigest: "fixture-digest",
				MaxActiveJobs: 1, TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record"},
			AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Observation: &run.AssumptionObservation{
				ObservedAt: ended, AssumptionState: run.AssumptionMatch}, Breaker: run.BreakerClosed}}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := run.RecordPath(root, id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidationWindowObservesOnlyClassifiableRunsWithReadableRealLedgers(t *testing.T) {
	root := t.TempDir()
	ledger := filepath.Join(root, "artifacts", "agents", "validation-stage-results", "direct.tsv")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	var rows strings.Builder
	for _, section := range retiredCatchClassSections[1:] {
		rows.WriteString("section\t" + section + "\tpass\t0\t1\n")
	}
	if err := os.WriteFile(ledger, []byte(rows.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "artifacts", "agents", "runs", "direct.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("stage results: "+ledger+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeValidationRun(t, root, "aborted-proof", run.StatusLaunchFailed, "artifacts/missing-aborted.log", 1)
	writeValidationRun(t, root, "unclassifiable-proof", run.StatusGreen, "artifacts/missing-green.log", 2)
	writeValidationRun(t, root, "measured-proof", run.StatusRed, filepath.Join("artifacts", "agents", "runs", "direct.log"), 3)
	if err := observeDirectValidationWindow(root, time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	state, err := loadValidationWindow(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Observations) != 1 || state.Observations[0].RunID != "measured-proof" ||
		strings.Join(state.Observations[0].Missing, ",") != retiredCatchClassSections[0] || len(state.Observations[0].NonGreen) != 0 {
		t.Fatalf("real observer advanced on aborted/unclassifiable evidence or lost the readable diff: %+v", state)
	}
}
