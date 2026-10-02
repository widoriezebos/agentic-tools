package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestRunPassCarriesGovernedSpendProjection(t *testing.T) {
	deny, err := filepath.Abs(filepath.Join("..", "..", "internal", "testgit", "testdata", "deny-bin"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", deny+string(os.PathListSeparator)+os.Getenv("PATH"))
	deniedLog := filepath.Join(t.TempDir(), "git-denied.log")
	t.Setenv("METASYSTEM_TEST_GIT_DENIED_LOG", deniedLog)
	t.Cleanup(func() {
		calls, err := os.ReadFile(deniedLog)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if len(calls) != 0 {
			t.Fatalf("run pass test invoked physical Git: %s", calls)
		}
	})
	now := time.Now().UTC()
	repository := newProofAdmissionRepositoryFixture(t, now, false)
	root := repository.root
	seedClaimLaunchGoalFiles(t, root)
	conf := []byte("metasystem.governance.correlation-policy=C\n")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), conf, 0o644); err != nil {
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
	backlog, err := os.ReadFile(filepath.Join(root, "plans", "goals", "backlog.md"))
	if err != nil {
		t.Fatal(err)
	}
	repository.seed(map[string][]byte{
		"metasystem/metasystem.conf":        conf,
		"metasystem/plans/goals/backlog.md": backlog,
		"metasystem/plans/goals/goal-a.md":  data,
	})
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
	accepted := repository.amend(t, "goal-a", func(accepted *goal.GoalFile) {
		accepted.Revision = file.Revision
		accepted.Obligation = file.Obligation
	})
	acceptedBytes := repository.rawFile(t, "metasystem/plans/goals/goal-a.md")
	localBytes, err := os.ReadFile(goalPath)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Obligation == nil || accepted.Obligation.State != goal.ObligationEnforced || string(acceptedBytes) != string(localBytes) {
		t.Fatal("accepted goal-a bytes differ from the enforced local obligation")
	}
	started := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
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
	concludingStore, err := dispatchcore.NewConcludingRunStoreWithReads(root, nil, repository.reads())
	if err != nil {
		t.Fatal(err)
	}
	if err := runPassWithStore(t.Output(), root, identity.Ref{Pid: 71, StartedAtSec: 72}, concludingStore); err != nil {
		t.Fatal(err)
	}
	concluded, err := store.Read("governed-pass")
	state, found, stateErr := obligationstate.Load(root, "goal-a", 3, file.Revision)
	if err != nil || stateErr != nil || concluded == nil || concluded.Status != run.StatusLaunchFailed || !found || len(state.Attempts) != 1 ||
		concluded.Governed.Exhausted || concluded.Governed.ExhaustionReason != "" {
		t.Fatalf("watcher run pass did not durably carry its projection: run=%+v state=%+v found=%t err=%v stateErr=%v", concluded, state, found, err, stateErr)
	}
	passBytes, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "supervision", "runs-pass.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pass struct {
		ScannedRuns []struct {
			ID string `json:"id"`
		} `json:"scannedRuns"`
	}
	if err := json.Unmarshal(passBytes, &pass); err != nil || len(pass.ScannedRuns) != 1 || pass.ScannedRuns[0].ID != "governed-pass" {
		t.Fatalf("run pass attestation=%s error=%v", passBytes, err)
	}
}
