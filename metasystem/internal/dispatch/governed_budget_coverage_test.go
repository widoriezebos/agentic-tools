package dispatch

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestBudgetProjectionStartsAtConsumedDurableProofEpoch(t *testing.T) {
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
	for _, job := range []struct {
		id, started, status string
		cap                 uint64
	}{
		{id: "before-proof", started: "2026-08-28T09:00:00Z", status: "completed", cap: 30},
		{id: "after-proof", started: "2026-08-28T09:45:00Z", status: "running", cap: 20},
	} {
		writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", job.id+".json"), map[string]any{
			"jobId": job.id, "operationId": job.id, "goalId": "bounded", "goalRevision": 3,
			"capMin": job.cap, "status": job.status, "startedAt": job.started,
		})
	}

	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.StartedAt.Format(time.RFC3339) != "2026-08-28T09:30:00Z" ||
		projection.WeightEpoch == nil || *projection.WeightEpoch != 1 || projection.Elapsed != 30*time.Minute ||
		projection.Attempts != 1 || projection.ReservedJobMinutes != 20 || projection.ActiveJobs != 1 {
		t.Fatalf("consumed durable proof did not establish the new spending epoch: %+v", projection)
	}
}

func TestBudgetProjectionRefusesMalformedConsumedProofLedger(t *testing.T) {
	completeProof := func() map[string]any {
		return map[string]any{
			"runId": "green-proof", "goalId": "bounded", "goalRevision": 3, "obligationRevision": 6,
			"weightGeneration": 1, "consumedAt": "2026-08-28T09:30:00Z",
			"resetDecision":     map[string]any{"Apply": true, "WouldRefuse": false},
			"dischargeDecision": map[string]any{"Apply": true, "WouldRefuse": false},
		}
	}
	for _, test := range []struct {
		name  string
		state map[string]any
		want  string
	}{
		{name: "invalid generation", state: map[string]any{"schema": 1, "generation": -1}, want: "schema or generation"},
		{name: "non-array ledger", state: map[string]any{"schema": 1, "generation": 2, "consumedProofs": map[string]any{}}, want: "not an array"},
		{name: "untyped entry", state: map[string]any{"schema": 1, "generation": 2, "consumedProofs": []any{"not-an-object"}}, want: "untyped entry"},
		{name: "unauthorized entry", state: map[string]any{"schema": 1, "generation": 2, "consumedProofs": []any{map[string]any{
			"runId": "green-proof", "goalId": "bounded", "goalRevision": 3, "obligationRevision": 6,
			"weightGeneration": 1, "consumedAt": "2026-08-28T09:30:00Z",
			"resetDecision": map[string]any{"apply": false, "wouldRefuse": false}, "dischargeDecision": map[string]any{"apply": true, "wouldRefuse": false},
		}}}, want: "incomplete or unauthorized"},
		{name: "missing durable proof", state: map[string]any{"schema": 1, "generation": 2, "consumedProofs": []any{completeProof()}}, want: "no exact durable green proof"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := budgetProjectionRoot(t)
			file := budgetGoal()
			file.Obligation = &goal.GovernedObligation{Revision: 6}
			writeJSON(t, filepath.Join(root, "artifacts", "agents", "validation-weight.json"), test.state)

			projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			if projection.Status != BudgetUnknown || projection.Unknown == nil || !strings.Contains(projection.Unknown.Reason, test.want) {
				t.Fatalf("malformed consumed-proof ledger did not fail closed with %q: %+v", test.want, projection)
			}
		})
	}
}

func TestObserveGovernedRunUsesAcceptedObligationAndBudgetState(t *testing.T) {
	root := revisionBindingBed(t, 2)
	obligationRevision := installEnforcedObligation(t, root, 5)
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	admission, err := EvaluateGovernedRunAdmission(root, run.GovernedAdmissionRequest{
		GoalID: "bounded", ObligationRevision: obligationRevision, StandingShared: true,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	record := &run.Record{GoalId: "bounded", StartedAt: now.Add(-time.Minute).Format(time.RFC3339), Governed: &admission.Attempt}

	observation := ObserveGovernedRun(root, record, now)
	if observation.AssumptionState != run.AssumptionMatch || observation.ActiveJobs != 0 || observation.DurationSeconds != 60 {
		t.Fatalf("governed observation did not use accepted obligation and current budget state: %+v", observation)
	}

	drift := ObserveGovernedAssumptions(root, goal.ObligationAssumptions{
		Platform: "other/platform", ToolchainIdentity: "other-toolchain", SurfaceDigest: "other-surface",
		MaxActiveJobs: 0, TimingEnvelopeSeconds: 0,
	}, 1, 1, now)
	if drift.AssumptionState != run.AssumptionDrift ||
		strings.Join(drift.DriftedFields, ",") != "activeJobs,durationSeconds,platform,surfaceDigest,toolchainIdentity" ||
		drift.Platform != runtime.GOOS+"/"+runtime.GOARCH || drift.ToolchainIdentity != runtime.Version() {
		t.Fatalf("typed assumption drift was not reported deterministically: %+v", drift)
	}

	if unavailable := ObserveGovernedRun(root, nil, now); unavailable.AssumptionState != run.AssumptionUnavailable ||
		strings.Join(unavailable.DriftedFields, ",") != "governedAttempt" {
		t.Fatalf("missing governed attempt did not fail closed: %+v", unavailable)
	}
	wrongRevision := *record
	wrongAttempt := *record.Governed
	wrongAttempt.GoalRevision++
	wrongRevision.Governed = &wrongAttempt
	if unavailable := ObserveGovernedRun(root, &wrongRevision, now); unavailable.AssumptionState != run.AssumptionUnavailable ||
		strings.Join(unavailable.DriftedFields, ",") != "obligationRevision" {
		t.Fatalf("stale governed binding did not fail closed: %+v", unavailable)
	}
	badStartedAt := *record
	badStartedAt.StartedAt = "not-a-time"
	if unavailable := ObserveGovernedRun(root, &badStartedAt, now); unavailable.AssumptionState != run.AssumptionUnavailable ||
		strings.Join(unavailable.DriftedFields, ",") != "durationSeconds" {
		t.Fatalf("unreadable governed duration did not fail closed: %+v", unavailable)
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "broken.json"), map[string]any{"jobId": "broken"})
	if unavailable := ObserveGovernedRun(root, record, now); unavailable.AssumptionState != run.AssumptionUnavailable ||
		strings.Join(unavailable.DriftedFields, ",") != "activeJobs" {
		t.Fatalf("unknown active-job projection did not fail closed: %+v", unavailable)
	}
}

func TestGovernedAdmissionRefusesDurableAssumptionBreaker(t *testing.T) {
	root := revisionBindingBed(t, 2)
	obligationRevision := installEnforcedObligation(t, root, 5)
	if err := obligationstate.RecordTerminal(root, "bounded", 2, obligationRevision, obligationstate.TerminalAttempt{
		RunID: "assumption-failed", Status: run.StatusRed,
		StartedAt: "2026-08-28T10:20:00Z", EndedAt: "2026-08-28T10:25:00Z", PrunedAt: "2026-08-28T10:26:00Z",
		AttemptOrdinal: 1, ExecutionCostMinutes: 1, ObservedCostMinutes: 1,
		WeightGeneration: 0, Breaker: run.BreakerAssumption,
	}); err != nil {
		t.Fatal(err)
	}

	_, err := EvaluateGovernedRunAdmission(root, run.GovernedAdmissionRequest{
		GoalID: "bounded", ObligationRevision: obligationRevision, StandingShared: true,
	}, time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "breaker=ASSUMPTION_FAILED is already terminal on run assumption-failed") {
		t.Fatalf("durable assumption breaker did not close governed admission: %v", err)
	}
}

func TestGovernedAdmissionRecordsCurrentWeightGenerationWithoutConsumedEpoch(t *testing.T) {
	root := revisionBindingBed(t, 2)
	obligationRevision := installEnforcedObligation(t, root, 5)
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "validation-weight.json"), map[string]any{
		"schema": 1, "generation": 2, "consumedProofs": []any{},
	})

	admission, err := EvaluateGovernedRunAdmission(root, run.GovernedAdmissionRequest{
		GoalID: "bounded", ObligationRevision: obligationRevision, StandingShared: true,
	}, time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	if err != nil || admission.Attempt.WeightGeneration == nil || *admission.Attempt.WeightGeneration != 2 || admission.Attempt.BudgetEpoch != nil {
		t.Fatalf("admission did not bind current generation independently of a consumed epoch: %+v %v", admission, err)
	}
}
