package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
)

func TestElapsedBoundaryClosesFenceAndBatchResumesToFixedPoint(t *testing.T) {
	root := revisionBindingBed(t, 2)
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "local-live.json"), map[string]any{
		"jobId": "local-live", "operationId": "local-live", "goalId": "bounded", "goalRevision": 2,
		"machineId": "bed-m1", "claimEpoch": 7, "capMin": 10, "status": "running",
	})
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "foreign-live.json"), map[string]any{
		"jobId": "foreign-live", "operationId": "foreign-live", "goalId": "bounded", "goalRevision": 2,
		"machineId": "other", "claimEpoch": 7, "capMin": 10, "status": "running",
	})
	now := time.Date(2026, 8, 28, 17, 0, 0, 0, time.UTC)
	admission, err := EvaluateGoalRevisionAdmission(root, "bounded", 2, 5, now)
	if err != nil || !admission.Refused() || admission.LiveStopReason != goal.StopReasonElapsedLimit {
		t.Fatalf("elapsed equality did not become a live stop: %+v %v", admission, err)
	}
	batch, err := EnsureBreachStop(root, "bounded", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if batch.State != goal.StopBatchOpen {
		t.Fatalf("initial batch = %+v", batch)
	}
	binding, err := ResolveGoalBinding(root, "bounded", now)
	if err != nil || binding.Fence == nil || binding.Fence.StopID != batch.StopID {
		t.Fatalf("fence was not closed before scan: %+v %v", binding, err)
	}
	fencedAdmission, err := EvaluateGoalRevisionAdmission(root, "bounded", 2, 5, now)
	if err != nil || !fencedAdmission.Refused() || fencedAdmission.LiveStopReason != goal.StopReasonElapsedLimit {
		t.Fatalf("a closed live-stop fence did not route the retry back through the custodian: %+v %v", fencedAdmission, err)
	}
	batch, err = ReconcileStopBatch(root, batch.StopID, now)
	if err != nil || batch.State != goal.StopBatchOpen || strings.Join(batch.Pending, ",") != "local-live" ||
		strings.Join(batch.Foreign, ",") != "foreign-live" || len(batch.Observed) != 2 ||
		len(batch.CancelOutcomes) != 1 || batch.CancelOutcomes[0].Outcome != stopForeignReportOnly {
		t.Fatalf("ranked scan did not separate custody: %+v %v", batch, err)
	}
	if err := AuthorizeStopCancellation(root, batch.StopID, "local-live"); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "local-live.json"), map[string]any{
		"jobId": "local-live", "operationId": "local-live", "goalId": "bounded", "goalRevision": 2,
		"machineId": "bed-m1", "claimEpoch": 7, "capMin": 10, "status": "cancelled",
	})
	batch, err = ReconcileStopBatch(root, batch.StopID, now.Add(time.Second))
	if err != nil || batch.State != goal.StopBatchComplete || len(batch.Pending) != 0 ||
		len(batch.CancelOutcomes) != 2 || batch.CancelOutcomes[1].Outcome != stopCancelled {
		t.Fatalf("batch did not reach fixed point: %+v %v", batch, err)
	}
	retry, err := EnsureBreachStop(root, "bounded", 2, now.Add(2*time.Second))
	if err != nil || retry.State != goal.StopBatchComplete || retry.StopID != batch.StopID {
		t.Fatalf("completed retry was not idempotent: %+v %v", retry, err)
	}
}

func TestIndeterminateCustodyIsTerminalForMachineryAndRoutesToEscalation(t *testing.T) {
	root := revisionBindingBed(t, 2)
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "unknown.json"), map[string]any{
		"jobId": "unknown", "operationId": "unknown", "goalId": "bounded", "goalRevision": 2,
		"machineId": "bed-m1", "capMin": 10, "status": "running",
	})
	now := time.Date(2026, 8, 28, 17, 0, 0, 0, time.UTC)
	batch, err := EnsureBreachStop(root, "bounded", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	batch, err = ReconcileStopBatch(root, batch.StopID, now)
	if err != nil || batch.State != goal.StopBatchIndeterminate || !strings.Contains(batch.Failure, "unproven custody") {
		t.Fatalf("unknown custody did not become indeterminate: %+v %v", batch, err)
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "unknown.json"), map[string]any{
		"jobId": "unknown", "operationId": "unknown", "goalId": "bounded", "goalRevision": 2,
		"machineId": "bed-m1", "claimEpoch": 7, "capMin": 10, "status": "cancelled",
	})
	retry, err := ReconcileStopBatch(root, batch.StopID, now.Add(time.Second))
	if err != nil || retry.State != goal.StopBatchIndeterminate || retry.Pass != batch.Pass {
		t.Fatalf("machinery retried terminal indeterminate custody: %+v %v", retry, err)
	}
	routes, err := FindBreachStops(root, now.Add(time.Second))
	if err != nil || len(routes) != 1 || routes[0].Failure == "" {
		t.Fatalf("indeterminate batch was not routed to escalation: %+v %v", routes, err)
	}
}

func TestAttemptEqualityRefusesWithoutWindDown(t *testing.T) {
	projection := BudgetProjection{
		Status:  BudgetKnown,
		Limits:  goal.Budget{ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 100, ActiveJobLimit: 3},
		Elapsed: time.Hour, Attempts: 2, ReservedJobMinutes: 20, ActiveJobs: 1,
	}
	if reason := liveStopReason(projection); reason != "" {
		t.Fatalf("attempt equality became live stop %s", reason)
	}
	breaches := budgetAdmissionBreaches(projection)
	if len(breaches) != 1 || breaches[0].Field != "attemptLimit" {
		t.Fatalf("attempt equality did not close admission only: %+v", breaches)
	}
}

func TestBreachStopCannotForgeAReadableGoalLockOwner(t *testing.T) {
	root := revisionBindingBed(t, 2)
	directory, err := GoalRevisionLockDir(root, "bounded", 2)
	if err != nil {
		t.Fatal(err)
	}
	tag := filepath.Base(os.Args[0])
	if err := OwnerLockClaim(directory, int64(os.Getpid()), tag); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = OwnerLockRelease(directory, int64(os.Getpid()), tag) })
	now := time.Date(2026, 8, 28, 17, 0, 0, 0, time.UTC)
	if _, err := EnsureBreachStop(root, "bounded", 2, now); err == nil || !strings.Contains(err.Error(), "LOCK_BUSY") {
		t.Fatalf("readable owner coordinates bypassed acquisition: %v", err)
	}
	binding, err := ResolveGoalBinding(root, "bounded", now)
	if err != nil || binding.Fence != nil {
		t.Fatalf("a refused acquisition changed the fence: binding=%+v err=%v", binding, err)
	}
}

func strandBreachStopJournal(t *testing.T, root string) string {
	t.Helper()
	stopID, ulid := stopIdentity("bounded", 2, 1)
	opid := goal.Opid(ulid, "bed-m1", stopCustodianLineage)
	intent := goal.Intent{Verb: "breach-stop", Targets: []string{"bounded"}, Args: map[string]string{
		"stopId": stopID, "reason": "forged-reason", "goalRevision": "999",
		"capabilityGeneration": "999", "capabilityMachine": "attacker", "claimEpoch": "999", "fenceEpoch": "999",
	}}
	if _, err := goal.CreateEntry(root, opid, "bed-m1", stopCustodianLineage, intent); err != nil {
		t.Fatal(err)
	}
	entry, err := goal.ReadEntry(root, opid)
	if err != nil {
		t.Fatal(err)
	}
	entry.Owner = goal.OwnerIdentity{Pid: 999999999, PidStartedAt: 1}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "goal-transactions", opid+".json"), entry)
	return opid
}

func TestBreachStopRecoveryReprojectsBudgetAndIgnoresJournalAuthorityStrings(t *testing.T) {
	t.Run("under budget escalates without a fence", func(t *testing.T) {
		root := revisionBindingBed(t, 2)
		opid := strandBreachStopJournal(t, root)
		endpoint, err := goal.ResolveEndpoint(root)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
		reports, err := goal.RecoverWithPolicy(endpoint, GoalRecoveryPolicy{Now: now})
		if err != nil {
			t.Fatal(err)
		}
		binding, err := ResolveGoalBinding(root, "bounded", now)
		if err != nil || binding.Fence != nil {
			t.Fatalf("a forged over-limit string closed an under-budget fence: binding=%+v err=%v", binding, err)
		}
		entry, err := goal.ReadEntry(root, opid)
		if err != nil || entry.Outcome != goal.OutcomeRejected || len(reports) == 0 ||
			!strings.Contains(reports[len(reports)-1].Detail, "not over its live budget") {
			t.Fatalf("the rejected recovery did not name its live projection: entry=%+v reports=%+v err=%v", entry, reports, err)
		}
	})

	t.Run("live breach derives the accepted coordinates", func(t *testing.T) {
		root := revisionBindingBed(t, 2)
		strandBreachStopJournal(t, root)
		endpoint, err := goal.ResolveEndpoint(root)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
		if _, err := goal.RecoverWithPolicy(endpoint, GoalRecoveryPolicy{Now: now}); err != nil {
			t.Fatal(err)
		}
		binding, err := ResolveGoalBinding(root, "bounded", now)
		if err != nil || binding.Fence == nil || binding.Fence.Reason != goal.StopReasonElapsedLimit ||
			binding.Fence.Revision != 2 || binding.Fence.CapabilityGeneration != 2 {
			t.Fatalf("recovery did not derive the live fence coordinates: binding=%+v err=%v", binding, err)
		}
	})

	t.Run("ranked lock blocks recovery mutation", func(t *testing.T) {
		root := revisionBindingBed(t, 2)
		strandBreachStopJournal(t, root)
		held, err := goalrevision.Acquire(root, "bounded", 2, "test-holder")
		if err != nil {
			t.Fatal(err)
		}
		defer held.Release()
		endpoint, err := goal.ResolveEndpoint(root)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
		reports, err := goal.RecoverWithPolicy(endpoint, GoalRecoveryPolicy{Now: now})
		if err != nil {
			t.Fatal(err)
		}
		binding, err := ResolveGoalBinding(root, "bounded", now)
		if err != nil || binding.Fence != nil || len(reports) == 0 ||
			!strings.Contains(reports[len(reports)-1].Detail, "LOCK_BUSY") {
			t.Fatalf("recovery mutated without acquiring the ranked lock: binding=%+v reports=%+v err=%v", binding, reports, err)
		}
	})
}
