package main

import (
	"os"
	"path/filepath"
	stdruntime "runtime"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestRunPassCarriesGovernedSpendProjection(t *testing.T) {
	root := t.TempDir()
	seedClaimLaunchGoal(t, root)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goalPath := filepath.Join(root, "plans", "goals", "goal-a.md")
	data, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("parse fixture goal: %v", problems)
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	file.Revision++
	effects := []goal.GoverningEffect{goal.EffectAuthorizeSpend}
	file.Obligation = &goal.GovernedObligation{Revision: file.Revision, BudgetRevision: file.Claimed.Revision,
		State: goal.ObligationEnforced, Owner: "fixture", AuthorizedBy: "fixture", AuthorizedAt: time.Now().UTC().Format(time.RFC3339),
		AuthorityOperation: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-m-test-run-pass", ReviewPolicy: "C", ReviewOutcome: "human-approved",
		Effects: effects, AuthorizedEffects: effects,
		Assumptions: goal.ObligationAssumptions{Recurrence: goal.StandingSharedProcess,
			Platform: stdruntime.GOOS + "/" + stdruntime.GOARCH, ToolchainIdentity: stdruntime.Version(), SurfaceDigest: digest,
			MaxActiveJobs: 1, TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record"},
		Triggers: goal.HumanReviewTriggers{ValueJudgment: "no", Reversibility: "reversible", SevereHarm: "no",
			UnfamiliarApproach: "no", TestDiscrimination: "strong", CorrelatedAssumptionRisk: "no",
			AuthorityScopeChange: "no", DestructiveReach: "none"}}
	if err := os.WriteFile(goalPath, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	goalSyncMutationGit(t, root, "add", "metasystem.conf", "plans/goals/goal-a.md")
	goalSyncMutationGit(t, root, "commit", "-qm", "enforce run-pass obligation")
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, "HEAD")
	started := time.Now().UTC().Add(-3 * time.Minute)
	weightGeneration := uint64(0)
	store := &run.Store{Root: root, Now: func() time.Time { return started },
		AdmitGoverned: func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
			return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{GoalRevision: 3, ObligationRevision: file.Revision,
				WeightGeneration: &weightGeneration, Recurrence: governance.StandingSharedProcess,
				ExecutionCostMinutes: 30, AttemptOrdinal: 1,
				Budget:          goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 60, ActiveJobLimit: 1},
				BudgetStartedAt: started.Format(time.RFC3339), ExpectedAssumptions: governance.ObligationAssumptions{
					Recurrence: governance.StandingSharedProcess, Platform: stdruntime.GOOS + "/" + stdruntime.GOARCH,
					ToolchainIdentity: stdruntime.Version(), SurfaceDigest: digest, MaxActiveJobs: 1,
					TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record",
				}, AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: run.BreakerClosed}}, nil
		}}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "governed-pass", Kind: "suite",
		Display: "governed pass", Log: "artifacts/governed-pass.log", GoalId: "goal-a", ObligationRevision: file.Revision, StandingShared: true}); err != nil {
		t.Fatal(err)
	}
	if err := runPass(root, identity.Ref{Pid: 71, StartedAtSec: 72}); err != nil {
		t.Fatal(err)
	}
	concluded, err := store.Read("governed-pass")
	state, found, stateErr := obligationstate.Load(root, "goal-a", 3, file.Revision)
	if err != nil || stateErr != nil || concluded == nil || concluded.Status != run.StatusLaunchFailed || !found || len(state.Attempts) != 1 ||
		concluded.Governed.Exhausted || concluded.Governed.ExhaustionReason != "" {
		t.Fatalf("watcher run pass did not durably carry its projection: run=%+v state=%+v found=%t err=%v stateErr=%v", concluded, state, found, err, stateErr)
	}
}
