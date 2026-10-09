package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalrevision"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func TestElapsedBudgetThreeBandsAndBreachStopFixedPoint(t *testing.T) {
	bed := newGoalMutationBed(t)
	root := bed.root
	underLimit := time.Date(2026, 8, 28, 16, 59, 59, 0, time.UTC)
	admission, err := bed.admission("bounded", 2, 5, underLimit)
	if err != nil || admission.Refused() || admission.LiveStopReason != "" {
		t.Fatalf("elapsed below the limit was not admitted: %+v %v", admission, err)
	}

	betweenThresholds := time.Date(2026, 8, 28, 17, 0, 0, 0, time.UTC)
	admission, err = bed.admission("bounded", 2, 5, betweenThresholds)
	if err != nil || !admission.Refused() || admission.LiveStopReason != "" || admission.Refusal == nil ||
		len(admission.Refusal.Breaches) != 1 || admission.Refusal.Breaches[0].State != AdmissionClosedElapsed {
		t.Fatalf("elapsed equality must close admission without stopping: %+v %v", admission, err)
	}
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*admission.Refusal}})
	if len(lines) != 1 || !strings.Contains(lines[0], "state=ADMISSION_CLOSED_ELAPSED") {
		t.Fatalf("the admission refusal lost its typed elapsed evidence: %v", lines)
	}
	routes, err := bed.stops(betweenThresholds)
	if err != nil || len(routes) != 0 {
		t.Fatalf("the grace band produced a stop route: %+v %v", routes, err)
	}
	if _, err := bed.stop("bounded", 2, betweenThresholds); err == nil || !strings.Contains(err.Error(), "no overspend to stop") {
		t.Fatalf("the grace band allowed direct stop custody: %v", err)
	}
	binding, err := bed.binding("bounded", betweenThresholds)
	if err != nil || binding.Fence != nil {
		t.Fatalf("the refused grace-band stop changed the launch fence: %+v %v", binding, err)
	}

	atGraceBoundary := time.Date(2026, 8, 28, 21, 0, 0, 0, time.UTC)
	admission, err = bed.admission("bounded", 2, 5, atGraceBoundary)
	if err != nil || !admission.Refused() || admission.LiveStopReason != goal.StopReasonElapsedLimit || admission.Refusal == nil ||
		len(admission.Refusal.Breaches) != 1 || admission.Refusal.Breaches[0].State != ElapsedBreach {
		t.Fatalf("the grace boundary did not become a live stop: %+v %v", admission, err)
	}
	lines = FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*admission.Refusal}})
	if len(lines) != 1 || !strings.Contains(lines[0], "state=ELAPSED_BREACH") {
		t.Fatalf("the breach refusal lost its typed elapsed evidence: %v", lines)
	}
	pastGraceBoundary := atGraceBoundary.Add(time.Second)
	admission, err = bed.admission("bounded", 2, 5, pastGraceBoundary)
	if err != nil || !admission.Refused() || admission.LiveStopReason != goal.StopReasonElapsedLimit {
		t.Fatalf("elapsed past the grace boundary did not keep the live stop armed: %+v %v", admission, err)
	}

	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "local-live.json"), map[string]any{
		"runtime": "local", "jobId": "local-live", "operationId": "local-live", "goalId": "bounded", "goalRevision": 2,
		"machineId": "bed-m1", "claimEpoch": 7, "capMin": 10, "status": "running",
	})
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "foreign-live.json"), map[string]any{
		"runtime": "local", "jobId": "foreign-live", "operationId": "foreign-live", "goalId": "bounded", "goalRevision": 2,
		"machineId": "other", "claimEpoch": 7, "capMin": 10, "status": "running",
	})
	batch, err := bed.stop("bounded", 2, pastGraceBoundary)
	if err != nil {
		t.Fatal(err)
	}
	if batch.State != goal.StopBatchOpen || batch.FiringEvidence == nil ||
		batch.FiringEvidence.ElapsedUsed != "12h0m1s" || batch.FiringEvidence.AdmissionLimit != "1d" ||
		batch.FiringEvidence.BreachBoundary != "12h0m0s" || batch.FiringEvidence.GracePercent != 50 {
		t.Fatalf("initial batch = %+v", batch)
	}
	changedEvidence := batch
	copyEvidence := *batch.FiringEvidence
	changedEvidence.FiringEvidence = &copyEvidence
	changedEvidence.FiringEvidence.ElapsedUsed = "12h0m2s"
	if err := goal.WriteStopBatch(root, changedEvidence); err == nil || !strings.Contains(err.Error(), "firing evidence is immutable") {
		t.Fatalf("open batch accepted changed firing evidence: %v", err)
	}
	binding, err = bed.binding("bounded", pastGraceBoundary)
	if err != nil || binding.Fence == nil || binding.Fence.StopID != batch.StopID {
		t.Fatalf("fence was not closed before scan: %+v %v", binding, err)
	}
	fencedAdmission, err := bed.admission("bounded", 2, 5, pastGraceBoundary)
	if err != nil || !fencedAdmission.Refused() || fencedAdmission.LiveStopReason != goal.StopReasonElapsedLimit {
		t.Fatalf("a closed live-stop fence did not route the retry back through the custodian: %+v %v", fencedAdmission, err)
	}
	batch, err = ReconcileStopBatch(root, batch.StopID, pastGraceBoundary)
	if err != nil || batch.State != goal.StopBatchOpen || strings.Join(batch.Pending, ",") != "local-live" ||
		strings.Join(batch.Foreign, ",") != "foreign-live" || len(batch.Observed) != 2 ||
		len(batch.CancelOutcomes) != 1 || batch.CancelOutcomes[0].Outcome != stopForeignReportOnly {
		t.Fatalf("ranked scan did not separate custody: %+v %v", batch, err)
	}
	if err := AuthorizeStopCancellation(root, batch.StopID, "local-live"); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "local-live.json"), map[string]any{
		"runtime": "local", "jobId": "local-live", "operationId": "local-live", "goalId": "bounded", "goalRevision": 2,
		"machineId": "bed-m1", "claimEpoch": 7, "capMin": 10, "status": "cancelled",
	})
	batch, err = ReconcileStopBatch(root, batch.StopID, pastGraceBoundary.Add(time.Second))
	if err != nil || batch.State != goal.StopBatchComplete || len(batch.Pending) != 0 ||
		len(batch.CancelOutcomes) != 2 || batch.CancelOutcomes[1].Outcome != stopCancelled {
		t.Fatalf("batch did not reach fixed point: %+v %v", batch, err)
	}
	retry, err := bed.stop("bounded", 2, pastGraceBoundary.Add(2*time.Second))
	if err != nil || retry.State != goal.StopBatchComplete || retry.StopID != batch.StopID {
		t.Fatalf("completed retry was not idempotent: %+v %v", retry, err)
	}
	if accepted := bed.parsedAcceptedGoal(t, "bounded"); accepted.StopFence == nil || accepted.StopFence.StopID != batch.StopID {
		t.Fatalf("accepted goal lost the stop fence: %+v", accepted.StopFence)
	}
}

func TestIndeterminateCustodyIsTerminalForMachineryAndRoutesToEscalation(t *testing.T) {
	bed := newGoalMutationBed(t)
	root := bed.root
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "unknown.json"), map[string]any{
		"runtime": "local", "jobId": "unknown", "operationId": "unknown", "goalId": "bounded", "goalRevision": 2,
		"machineId": "bed-m1", "capMin": 10, "status": "running",
	})
	now := time.Date(2026, 8, 28, 21, 0, 0, 0, time.UTC)
	batch, err := bed.stop("bounded", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	batch, err = ReconcileStopBatch(root, batch.StopID, now)
	if err != nil || batch.State != goal.StopBatchIndeterminate || !strings.Contains(batch.Failure, "unproven custody") {
		t.Fatalf("unknown custody did not become indeterminate: %+v %v", batch, err)
	}
	writeJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", "unknown.json"), map[string]any{
		"runtime": "local", "jobId": "unknown", "operationId": "unknown", "goalId": "bounded", "goalRevision": 2,
		"machineId": "bed-m1", "claimEpoch": 7, "capMin": 10, "status": "cancelled",
	})
	retry, err := ReconcileStopBatch(root, batch.StopID, now.Add(time.Second))
	if err != nil || retry.State != goal.StopBatchIndeterminate || retry.Pass != batch.Pass {
		t.Fatalf("machinery retried terminal indeterminate custody: %+v %v", retry, err)
	}
	routes, err := bed.stops(now.Add(time.Second))
	if err != nil || len(routes) != 1 || routes[0].Condition != StopRouteIndeterminate || routes[0].Failure == "" {
		t.Fatalf("indeterminate batch was not routed to escalation: %+v %v", routes, err)
	}
	if accepted := bed.parsedAcceptedGoal(t, "bounded"); accepted.StopFence == nil || accepted.StopFence.StopID != batch.StopID {
		t.Fatalf("accepted goal lost the indeterminate stop fence: %+v", accepted.StopFence)
	}
}

func TestCorruptGraceAfterLaunchRefusesAdmissionAndRoutesIndeterminateStop(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	root := bed.root
	before := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	admission, err := bed.revisionAdmission("bounded", 2, 5, before)
	if err != nil || admission.Refused() {
		t.Fatalf("valid launch admission: %+v %v", admission, err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"),
		[]byte("metasystem.budget.elapsed-grace-percent=broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	routes, err := bed.stops(before.Add(time.Minute))
	if err != nil || len(routes) != 1 || routes[0].Condition != StopRouteIndeterminate ||
		!strings.Contains(routes[0].Failure, "BUDGET_UNKNOWN") || routes[0].StopID != "" {
		t.Fatalf("corrupt post-launch grace did not produce one typed indeterminate route: %+v %v", routes, err)
	}
	refused, err := bed.revisionAdmission("bounded", 2, 5, before.Add(time.Minute))
	if err != nil || !refused.Refused() || refused.Refusal == nil || refused.Refusal.Unknown == nil {
		t.Fatalf("corrupt post-launch grace admitted new work: %+v %v", refused, err)
	}
	binding, err := bed.binding("bounded", before.Add(time.Minute))
	if err != nil || binding.Fence != nil {
		t.Fatalf("indeterminate budget cancelled or fenced lawful work: %+v %v", binding, err)
	}
}

func TestUnrepresentableGraceBoundaryRoutesIndeterminateStop(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	root := bed.root
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(content)
	if len(problems) != 0 {
		t.Fatalf("parse accepted goal: %v", problems)
	}
	file.Budget.ElapsedLimit = "2562047h"
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)

	now := time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC)
	routes, err := bed.stops(now)
	if err != nil || len(routes) != 1 || routes[0].Condition != StopRouteIndeterminate ||
		!strings.Contains(routes[0].Failure, "duration range") {
		t.Fatalf("unrepresentable grace did not produce a typed indeterminate route: %+v %v", routes, err)
	}
	admission, err := bed.revisionAdmission("bounded", 2, 5, now)
	if err != nil || !admission.Refused() || admission.Refusal == nil || admission.Refusal.Unknown == nil {
		t.Fatalf("unrepresentable grace admitted new work: %+v %v", admission, err)
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
	bed := newGoalMutationBed(t)
	root := bed.root
	directory, err := GoalRevisionLockDir(root, "bounded", 2)
	if err != nil {
		t.Fatal(err)
	}
	tag := filepath.Base(os.Args[0])
	if err := OwnerLockClaim(directory, int64(os.Getpid()), tag); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = OwnerLockRelease(directory, int64(os.Getpid()), tag) })
	now := time.Date(2026, 8, 28, 21, 0, 0, 0, time.UTC)
	if _, err := bed.stop("bounded", 2, now); err == nil || goal.RefusalCode(err) != goalrevision.BusyCode {
		t.Fatalf("readable owner coordinates bypassed acquisition: %v", err)
	}
	binding, err := bed.binding("bounded", now)
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
		bed := newGoalMutationBed(t)
		root := bed.root
		opid := strandBreachStopJournal(t, root)
		endpoint := bed.endpoint()
		now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
		reports, err := goal.RecoverWithPolicy(endpoint, goalRecoveryPolicyWithReads{GoalRecoveryPolicy: GoalRecoveryPolicy{Now: now}, reads: bed.reads})
		if err != nil {
			t.Fatal(err)
		}
		binding, err := bed.binding("bounded", now)
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
		bed := newGoalMutationBed(t)
		root := bed.root
		strandBreachStopJournal(t, root)
		endpoint := bed.endpoint()
		now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
		if _, err := goal.RecoverWithPolicy(endpoint, goalRecoveryPolicyWithReads{GoalRecoveryPolicy: GoalRecoveryPolicy{Now: now}, reads: bed.reads}); err != nil {
			t.Fatal(err)
		}
		binding, err := bed.binding("bounded", now)
		if err != nil || binding.Fence == nil || binding.Fence.Reason != goal.StopReasonElapsedLimit ||
			binding.Fence.Revision != 2 || binding.Fence.CapabilityGeneration != 2 {
			t.Fatalf("recovery did not derive the live fence coordinates: binding=%+v err=%v", binding, err)
		}
		if accepted := bed.parsedAcceptedGoal(t, "bounded"); accepted.StopFence == nil || accepted.StopFence.Reason != goal.StopReasonElapsedLimit {
			t.Fatalf("accepted recovery fence: %+v", accepted.StopFence)
		}
	})

	t.Run("ranked lock blocks recovery mutation", func(t *testing.T) {
		bed := newGoalMutationBed(t)
		root := bed.root
		strandBreachStopJournal(t, root)
		held, err := goalrevision.Acquire(root, "bounded", 2, "test-holder")
		if err != nil {
			t.Fatal(err)
		}
		defer held.Release()
		endpoint := bed.endpoint()
		now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
		reports, err := goal.RecoverWithPolicy(endpoint, goalRecoveryPolicyWithReads{GoalRecoveryPolicy: GoalRecoveryPolicy{Now: now}, reads: bed.reads})
		if err != nil {
			t.Fatal(err)
		}
		binding, err := bed.binding("bounded", now)
		if err != nil || binding.Fence != nil || len(reports) == 0 ||
			!strings.Contains(reports[len(reports)-1].Detail, "LOCK_BUSY") {
			t.Fatalf("recovery mutated without acquiring the ranked lock: binding=%+v reports=%+v err=%v", binding, reports, err)
		}
	})
}

// markedAdmissionBed is the bounded goal, claimed at 09:00 with a box of one
// working day (eight hours) and a breach limit of twelve hours, marked as
// waiting to land at mark.
func markedAdmissionBed(t *testing.T, mark string) (*goalAdmissionBed, *goal.GoalFile) {
	t.Helper()
	bed := newGoalAdmissionBed(t, 2)
	templateAdmissionBed(t, bed)
	path := filepath.Join(bed.root, "plans", "goals", "bounded.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(content)
	if len(problems) != 0 {
		t.Fatalf("parse accepted goal: %v", problems)
	}
	file.Landing = &goal.LandingRecord{At: mark, Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-bed-m1-00000004"}
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.accept(t)
	return bed, file
}

func TestAWaitDoesNotGrowElapsed(t *testing.T) {
	t.Parallel()
	bed, file := markedAdmissionBed(t, "2026-08-28T12:00:00Z")
	// Marked for forty-two hours, far past the breach limit, with no job or
	// proof of its own since the mark: the clock still reads the three hours
	// it had at the mark.
	now := time.Date(2026, 8, 30, 6, 0, 0, 0, time.UTC)
	projection := ProjectBudget(bed.root, file, now)
	if projection.Status != BudgetKnown || projection.Elapsed != 3*time.Hour || projection.Wait != 42*time.Hour || projection.ElapsedState != "" {
		t.Fatalf("the wait grew elapsed: %+v", projection)
	}
	admission, err := bed.revisionAdmission("bounded", 2, 5, now)
	if err != nil || admission.Refused() || admission.LiveStopReason != "" {
		t.Fatalf("a goal that only waited was refused or stopped: %+v %v", admission, err)
	}
	routes, err := bed.stops(now)
	if err != nil || len(routes) != 0 {
		t.Fatalf("a goal that only waited was named for a breach stop: %+v %v", routes, err)
	}
}

func TestAJobThatNeverRanDoesNotEndTheWait(t *testing.T) {
	t.Parallel()
	// Marked at 09:00, the goal holds a reservation from 08:50 still in setup
	// and one from 10:00 cancelled in setup. Neither ran, so a day after the
	// mark, past the twelve-hour breach limit, the whole day is wait.
	bed, file := markedAdmissionBed(t, "2026-08-28T09:00:00Z")
	writeBudgetJob(t, bed.root, "in-setup", "reserve-in-setup", 2, 10, "pending-setup", budgetJobLife{createdAt: "2026-08-28T08:50:00Z"})
	writeBudgetJob(t, bed.root, "cancelled", "reserve-cancelled", 2, 10, "cancelled", budgetJobLife{
		createdAt: "2026-08-28T10:00:00Z", endedAt: "2026-08-28T10:05:00Z",
	})
	now := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	projection := ProjectBudget(bed.root, file, now)
	if projection.Status != BudgetKnown || projection.Wait != 24*time.Hour || projection.Elapsed != 0 || projection.ElapsedState != "" {
		t.Fatalf("a job that never ran ended the wait: %+v", projection)
	}
	if routes, err := bed.stops(now); err != nil || len(routes) != 0 {
		t.Fatalf("a goal whose jobs never ran was named for a breach stop: %+v %v", routes, err)
	}
	// A job created at 10:50 that reached running at 11:00 ends the wait at
	// its start.
	writeBudgetJob(t, bed.root, "ran", "reserve-ran", 2, 10, "running", budgetJobLife{
		createdAt: "2026-08-28T10:50:00Z", startedAt: "2026-08-28T11:00:00Z",
	})
	projection = ProjectBudget(bed.root, file, now)
	if projection.Status != BudgetKnown || projection.Wait != 2*time.Hour || projection.Elapsed != 22*time.Hour || projection.ElapsedState != ElapsedBreach {
		t.Fatalf("the job that ran did not end the wait at its start: %+v", projection)
	}
	if routes, err := bed.stops(now); err != nil || len(routes) != 1 || routes[0].GoalID != "bounded" || routes[0].Reason != goal.StopReasonElapsedLimit {
		t.Fatalf("a goal that worked past its breach limit was not named for a breach stop: %+v %v", routes, err)
	}
}

func TestAProofThatNeverLaunchedDoesNotEndTheWait(t *testing.T) {
	t.Parallel()
	// Marked at 09:00, the goal holds a proof reserved at 10:00 that has not
	// launched and one reserved at 10:30 that was cancelled before it
	// launched. Neither ran, so a day after the mark, past the twelve-hour
	// breach limit, the whole day is wait.
	bed, file := markedAdmissionBed(t, "2026-08-28T09:00:00Z")
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, proof := range []struct {
		id string
		at time.Time
	}{
		{"waiting-proof", time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)},
		{"cancelled-proof", time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)},
	} {
		_, identity := dispatchProofFixture(t, proof.id)
		_, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
			ControlRoot: bed.root, ExecutionRoot: bed.root, GoalID: "bounded", GoalRevision: 2, AccountingRevision: 2,
			ReservedMinutes: 10, Identity: identity, Launcher: launcher, Now: proof.at, AttemptID: proof.id,
		}))
		if err != nil {
			t.Fatal(err)
		}
		requireProofReservationNotAdmissionRefused(t, decision)
	}
	if _, err := proofrun.FinalizeAttempt(bed.root, "cancelled-proof", proofrun.TerminalCancelled, 1, "cancelled before its launch", nil,
		time.Date(2026, 8, 28, 10, 35, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	projection := ProjectBudget(bed.root, file, now)
	if projection.Status != BudgetKnown || projection.Wait != 24*time.Hour || projection.Elapsed != 0 || projection.ElapsedState != "" {
		t.Fatalf("a proof that never launched ended the wait: %+v", projection)
	}
	if routes, err := bed.stops(now); err != nil || len(routes) != 0 {
		t.Fatalf("a goal whose proofs never launched was named for a breach stop: %+v %v", routes, err)
	}
	// The proof reserved at 10:00 launched its suite at 11:00: the wait ends
	// at its launch, not at its reservation.
	launchBudgetProof(t, bed.root, "waiting-proof", time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC))
	projection = ProjectBudget(bed.root, file, now)
	if projection.Status != BudgetKnown || projection.Wait != 2*time.Hour || projection.Elapsed != 22*time.Hour || projection.ElapsedState != ElapsedBreach {
		t.Fatalf("the proof that launched did not end the wait at its launch: %+v", projection)
	}
	if routes, err := bed.stops(now); err != nil || len(routes) != 1 || routes[0].GoalID != "bounded" || routes[0].Reason != goal.StopReasonElapsedLimit {
		t.Fatalf("a goal that worked past its breach limit was not named for a breach stop: %+v %v", routes, err)
	}
}

func TestAnUndatableProofLaunchMakesTheBudgetUnknown(t *testing.T) {
	t.Parallel()
	// The goal's proof launched, but its process record cannot be parsed.
	bed, file, path := launchedMarkedProofBed(t)
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	requireUndatableLaunch(t, bed, file)
}

func TestAProcessRecordWithoutALaunchTimeMakesTheBudgetUnknown(t *testing.T) {
	t.Parallel()
	// The goal's proof launched, and its process record names the suite
	// process by its start in microseconds but leaves out pidStartedAt, the
	// launch time. The record is well-formed, yet nothing in it dates the
	// launch: a missing launch time is not a launch at the start of 1970.
	bed, file, path := launchedMarkedProofBed(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	suite := record["suiteProcess"].(map[string]any)
	delete(suite, "pidStartedAt")
	suite["pidStartedAtMicro"] = time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC).UnixMicro()
	writeJSON(t, path, record)
	if _, err := proofrun.ReadProcessRecord(bed.root, "undatable-proof-launch-1"); err != nil {
		t.Fatalf("the process record without a launch time is not well-formed: %v", err)
	}
	requireUndatableLaunch(t, bed, file)
}

// launchedMarkedProofBed is a goal marked at 09:00 with a proof of its own
// reserved at 10:00 that launched its suite at 13:00. It returns the path of
// that launch's process record.
func launchedMarkedProofBed(t *testing.T) (*goalAdmissionBed, *goal.GoalFile, string) {
	t.Helper()
	bed, file := markedAdmissionBed(t, "2026-08-28T09:00:00Z")
	if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, identity := dispatchProofFixture(t, "undatable-proof")
	_, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: bed.root, ExecutionRoot: bed.root, GoalID: "bounded", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 10, Identity: identity, Launcher: launcher,
		Now: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC), AttemptID: "undatable-proof",
	}))
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	launchBudgetProof(t, bed.root, "undatable-proof", time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC))
	path, err := proofrun.ProcessRecordPath(bed.root, "undatable-proof", "launch-1")
	if err != nil {
		t.Fatal(err)
	}
	return bed, file, path
}

// requireUndatableLaunch checks the bed's goal once its proof's launch
// cannot be dated. Nothing then dates the end of the wait, so the budget is
// unknown and names the launch's process record. A day after the mark, past
// the breach limit on any guess that the goal worked, the goal is neither
// stopped nor admitted.
func requireUndatableLaunch(t *testing.T, bed *goalAdmissionBed, file *goal.GoalFile) {
	t.Helper()
	const record = "artifacts/agents/proof-runs/processes/undatable-proof-launch-1.json"
	now := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)
	projection := ProjectBudget(bed.root, file, now)
	if projection.Status != BudgetUnknown || projection.Unknown == nil || projection.Unknown.Record != record ||
		!strings.Contains(projection.Unknown.Reason, "cannot be dated") {
		t.Fatalf("an undatable launch did not make the budget unknown, naming its record: %+v", projection)
	}
	routes, err := bed.stops(now)
	if err != nil || len(routes) != 1 || routes[0].GoalID != "bounded" || routes[0].Condition != StopRouteIndeterminate ||
		routes[0].StopID != "" || !strings.Contains(routes[0].Failure, "BUDGET_UNKNOWN record="+record) {
		t.Fatalf("an undatable launch did not produce one indeterminate route naming its record: %+v %v", routes, err)
	}
	refused, err := bed.revisionAdmission("bounded", 2, 5, now)
	if err != nil || !refused.Refused() || refused.LiveStopReason != "" || refused.Refusal == nil ||
		refused.Refusal.Unknown == nil || refused.Refusal.Unknown.Record != record {
		t.Fatalf("an undatable launch admitted new work or stopped the goal: %+v %v", refused, err)
	}
	binding, err := bed.binding("bounded", now)
	if err != nil || binding.Fence != nil {
		t.Fatalf("an undatable launch fenced the goal: %+v %v", binding, err)
	}
}

func TestAMarkedGoalStillObeysItsAttemptLimit(t *testing.T) {
	t.Parallel()
	// Marked at 12:00 and observed the next evening, the goal only waited
	// since the mark, so its elapsed is the three hours it had then, inside
	// the box. Two attempts before the mark fill its attempt limit of two:
	// the next reservation is refused for attempts alone.
	bed, _ := markedAdmissionBed(t, "2026-08-28T12:00:00Z")
	receipt := newStrictBudgetReceipt(t, bed.root, bed.root, "memory/receipts.log")
	receipt.want = 1
	bed.reads.Receipt = receipt.reads()
	for _, name := range []string{"first", "second"} {
		writeBudgetJob(t, bed.root, name, "reserve-"+name, 2, 10, "completed", budgetJobLife{
			createdAt: "2026-08-28T10:00:00Z", startedAt: "2026-08-28T10:00:00Z", endedAt: "2026-08-28T10:10:00Z",
		})
	}
	now := time.Date(2026, 8, 29, 22, 0, 0, 0, time.UTC)
	refused, err := bed.revisionAdmission("bounded", 2, 5, now)
	if err != nil || !refused.Refused() || refused.LiveStopReason != "" || refused.Refusal == nil ||
		len(refused.Refusal.Breaches) != 1 || refused.Refusal.Breaches[0].Field != "attemptLimit" {
		t.Fatalf("a marked goal at its attempt limit was not refused for attempts: %+v %v", refused, err)
	}
	if routes, err := bed.stops(now); err != nil || len(routes) != 0 {
		t.Fatalf("a marked goal at its attempt limit was named for a stop: %+v %v", routes, err)
	}
	// A third attempt puts it over its limit: it is stopped as corrupt over
	// its limit.
	writeBudgetJob(t, bed.root, "third", "reserve-third", 2, 10, "completed", budgetJobLife{
		createdAt: "2026-08-28T10:20:00Z", startedAt: "2026-08-28T10:20:00Z", endedAt: "2026-08-28T10:30:00Z",
	})
	routes, err := bed.stops(now)
	if err != nil || len(routes) != 1 || routes[0].GoalID != "bounded" || routes[0].Reason != goal.StopReasonCorruptOverLimit ||
		routes[0].Condition != StopRouteBreach {
		t.Fatalf("a marked goal over its attempt limit was not named for a breach stop: %+v %v", routes, err)
	}
	stopped, err := bed.revisionAdmission("bounded", 2, 5, now)
	if err != nil || !stopped.Refused() || stopped.LiveStopReason != goal.StopReasonCorruptOverLimit {
		t.Fatalf("a marked goal over its attempt limit was not stopped as corrupt over its limit: %+v %v", stopped, err)
	}
}

func TestAMarkedGoalThatWorksPastItsLimitIsRefusedAndStopped(t *testing.T) {
	t.Parallel()
	bed, _ := markedAdmissionBed(t, "2026-08-28T12:00:00Z")
	// Its own job started at 13:00, after the mark: the clock runs again from
	// then, three hours at the mark plus the time since the job started, and
	// reaches the eight-hour box at 18:00 and the breach limit at 22:00.
	writeBudgetJob(t, bed.root, "after-mark", "reserve-after-mark", 2, 10, "completed", budgetJobLife{
		startedAt: "2026-08-28T13:00:00Z", endedAt: "2026-08-28T13:10:00Z", pid: 4242,
	})
	atLimit := time.Date(2026, 8, 28, 18, 0, 0, 0, time.UTC)
	refused, err := bed.revisionAdmission("bounded", 2, 5, atLimit)
	if err != nil || !refused.Refused() || refused.LiveStopReason != "" || refused.Refusal == nil ||
		len(refused.Refusal.Breaches) != 1 || refused.Refusal.Breaches[0].Field != "elapsedLimit" ||
		refused.Refusal.Breaches[0].State != AdmissionClosedElapsed {
		t.Fatalf("a marked goal that worked to its elapsed limit was not refused with the elapsed breach: %+v %v", refused, err)
	}
	routes, err := bed.stops(atLimit)
	if err != nil || len(routes) != 0 {
		t.Fatalf("the grace band produced a stop route: %+v %v", routes, err)
	}
	pastBreach := time.Date(2026, 8, 28, 22, 0, 0, 0, time.UTC)
	stopped, err := bed.revisionAdmission("bounded", 2, 5, pastBreach)
	if err != nil || !stopped.Refused() || stopped.LiveStopReason != goal.StopReasonElapsedLimit {
		t.Fatalf("a marked goal past its breach limit was not stopped: %+v %v", stopped, err)
	}
	routes, err = bed.stops(pastBreach)
	if err != nil || len(routes) != 1 || routes[0].GoalID != "bounded" || routes[0].Reason != goal.StopReasonElapsedLimit ||
		routes[0].Condition != StopRouteBreach {
		t.Fatalf("a marked goal past its breach limit was not named for a breach stop: %+v %v", routes, err)
	}
}

func TestAProofThatDischargesAfterTheMarkEndsTheWait(t *testing.T) {
	t.Parallel()
	// Marked at 09:00, the goal ran a proof of its own from 10:00 that
	// discharged at 11:00 and opened a new spending epoch. Neither the proof's
	// reservation, bound to the old epoch, nor its retained governed record,
	// which starts at 10:30, counts toward the new one, yet either ends the
	// wait: at 23:00 the clock reads the twelve hours since 11:00, the breach
	// limit.
	for _, reserved := range []bool{true, false} {
		t.Run(fmt.Sprint("reserved=", reserved), func(t *testing.T) {
			bed, _ := markedAdmissionBed(t, "2026-08-28T09:00:00Z")
			obligationRevision := installAcceptedEnforcedObligation(t, bed, 5)
			writeConsumedBudgetProof(t, bed.root, "discharging-run", 2, obligationRevision, time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC))
			if reserved {
				conf := []byte("metasystem.governance.correlation-policy=C\nmetasystem.runtimes=fake\n")
				if err := os.WriteFile(filepath.Join(bed.root, "metasystem.conf"), conf, 0o644); err != nil {
					t.Fatal(err)
				}
				_, identity := dispatchProofFixture(t, "gate")
				launcher, err := proofrun.CurrentProcessIdentity(nil)
				if err != nil {
					t.Fatal(err)
				}
				oldEpoch := uint64(0)
				_, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
					ControlRoot: bed.root, ExecutionRoot: bed.root, GoalID: "bounded", GoalRevision: 2, AccountingRevision: 2,
					BudgetEpoch: &oldEpoch, ReservedMinutes: 60, Identity: identity, Launcher: launcher,
					Now: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC), AttemptID: "discharging-proof",
				}))
				if err != nil {
					t.Fatal(err)
				}
				requireProofReservationNotAdmissionRefused(t, decision)
				launchBudgetProof(t, bed.root, "discharging-proof", time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
			}
			now := time.Date(2026, 8, 28, 23, 0, 0, 0, time.UTC)
			binding, err := bed.binding("bounded", now)
			if err != nil {
				t.Fatal(err)
			}
			projection := ProjectBudget(bed.root, binding.File, now)
			if projection.Status != BudgetKnown || projection.Elapsed != 12*time.Hour || projection.ElapsedState != ElapsedBreach {
				t.Fatalf("the proof after the mark did not end the wait: %+v", projection)
			}
			stopped, err := bed.revisionAdmission("bounded", 2, 5, now)
			if err != nil || !stopped.Refused() || stopped.LiveStopReason != goal.StopReasonElapsedLimit || stopped.Refusal == nil ||
				len(stopped.Refusal.Breaches) != 1 || stopped.Refusal.Breaches[0].Field != "elapsedLimit" {
				t.Fatalf("new work was not refused with the elapsed breach: %+v %v", stopped, err)
			}
			routes, err := bed.stops(now)
			if err != nil || len(routes) != 1 || routes[0].GoalID != "bounded" || routes[0].Reason != goal.StopReasonElapsedLimit {
				t.Fatalf("the goal was not named for a breach stop: %+v %v", routes, err)
			}
		})
	}
}
