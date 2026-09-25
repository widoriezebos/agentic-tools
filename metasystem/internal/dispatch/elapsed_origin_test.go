package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func TestElapsedOriginRaiseCanary(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
	bed := newGoalMutationBed(t)
	root, endpoint := bed.root, bed.endpoint()
	machine := "bed-m2"
	bed.reads.ResolveMachine = func(got string) (string, error) {
		if got != root {
			return "", fmt.Errorf("undeclared goal machine root %q", got)
		}
		return machine, nil
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\nmetasystem.budget.elapsed-grace-percent=25\nmetasystem.governance.correlation-policy=C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	authorization, err := fixtureauth.New(root)
	if err != nil {
		t.Fatal(err)
	}
	value, err := humanauthority.FixtureGoalProof(root, authorization.GoalHumanAuthority(), t0)
	if err != nil {
		t.Fatal(err)
	}
	proof := &value
	if _, err := goal.Project(endpoint, false, t0); err != nil {
		t.Fatalf("writer bed is not readable before the first verb: %v", err)
	}
	request := func(ulid string, at time.Time) goal.VerbRequest {
		return goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: machine, Lineage: "coordinator"},
			Ulid: ulid, Now: at, ClaimEpoch: 7}
	}
	if result, err := goal.Open(request("01J5X00000000000000000E000", t0.Add(-2*time.Minute)), "elapsed-canary", "Preserve elapsed origin.", goal.OriginMain, "Exercise the breach clock."); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	risk := goal.RiskRecord{Severity: 3, Novelty: 3, Exposure: 1, Accumulation: 1, Basis: "The fixture exercises a tier-three elapsed budget."}
	if result, err := goal.Edit(request("01J5X00000000000000000E005", t0.Add(-90*time.Second)), "elapsed-canary", goal.EditFields{Risk: &risk}); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("answer risk: %+v %v", result, err)
	}
	initial := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 240, ActiveJobLimit: 2}
	human := request("01J5X00000000000000000E001", t0.Add(-time.Minute))
	human.Actor.Human = "Wido"
	if result, err := goal.Approve(human, []string{"elapsed-canary"}, &initial, proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("approve: %+v %v", result, err)
	}
	if result, err := goal.Claim(request("01J5X00000000000000000E002", t0), "elapsed-canary"); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}
	raised := initial
	raised.ElapsedLimit = "6h"
	human = request("01J5X00000000000000000E003", t0.Add(3*time.Hour))
	human.Actor.Human = "Wido"
	if result, err := goal.SetBudgetApproved(human, "elapsed-canary", raised, proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("raise: %+v %v", result, err)
	}

	atAdmissionLimit := t0.Add(6 * time.Hour)
	projection, err := goal.Project(endpoint, false, atAdmissionLimit)
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live["elapsed-canary"]
	binding, err := bed.binding(file.Id, atAdmissionLimit)
	if err != nil || binding.Revision != file.Claimed.Revision {
		t.Fatalf("accepted binding did not match the raised claim: %+v %v", binding, err)
	}
	verdict, err := bed.admission(file.Id, file.Claimed.Revision, 1, atAdmissionLimit)
	if err != nil || verdict.Refusal == nil || len(verdict.Refusal.Breaches) != 1 || verdict.Refusal.Breaches[0].State != AdmissionClosedElapsed {
		t.Fatalf("the raised limit did not close admission from the original claim time: verdict=%+v err=%v", verdict, err)
	}
	projected := ProjectBudget(root, file, atAdmissionLimit)
	if projected.Status != BudgetKnown || !projected.StartedAt.Equal(t0) || projected.Elapsed != 6*time.Hour {
		t.Fatalf("the accepted raised claim reset its elapsed origin: %+v unknown=%+v", projected, projected.Unknown)
	}
	tip, present, err := bed.repository.Accepted()
	if err != nil || !present {
		t.Fatalf("accepted tip: %q present=%t err=%v", tip, present, err)
	}
	files, err := bed.repository.Files(tip, "plans/goals/elapsed-canary.md")
	accepted := files["plans/goals/elapsed-canary.md"]
	if err != nil || !strings.Contains(string(accepted), "episodeAt="+t0.Format(time.RFC3339)+" episodeRevision=") ||
		file.Claimed.AccountingRevision != file.Claimed.Revision {
		t.Fatalf("the accepted claim did not preserve an explicit episode while advancing accounting: claim=%+v bytes=%s err=%v", file.Claimed, accepted, err)
	}

	pastGrace := t0.Add(7*time.Hour + 31*time.Minute)
	batch, err := bed.stop(file.Id, file.Claimed.Revision, pastGrace)
	if err != nil {
		t.Fatal(err)
	}
	if batch.FiringEvidence == nil || batch.FiringEvidence.ElapsedUsed != "7h31m0s" || batch.FiringEvidence.AdmissionLimit != "6h" ||
		batch.FiringEvidence.BreachBoundary != "7h30m0s" || batch.FiringEvidence.GracePercent != 25 {
		t.Fatalf("breach stop lost the preserved elapsed evidence: %+v", batch)
	}
	afterStop, err := goal.Project(endpoint, false, pastGrace)
	if err != nil {
		t.Fatal(err)
	}
	stopped := afterStop.Tree.Live[file.Id]
	acceptedStop := bed.parsedAcceptedGoal(t, file.Id)
	stoppedProjection := ProjectBudget(root, stopped, pastGrace)
	if stopped.StopFence == nil || stopped.StopFence.Revision != file.Claimed.Revision || acceptedStop.StopFence == nil ||
		acceptedStop.StopFence.Revision != stopped.StopFence.Revision || stoppedProjection.Status != BudgetKnown || !stoppedProjection.StartedAt.Equal(t0) {
		t.Fatalf("the durable stop fence or preserved projection is missing: goal=%+v projection=%+v", stopped, stoppedProjection)
	}
}
