package gaterun

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/retrodebt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestDraftWouldRefuseIsRecordedButCannotResetWeight(t *testing.T) {
	prior := WeightState{Schema: 1, Generation: 4, Accumulated: 73, Landings: 6, SinceUTC: "2026-08-30T00:00:00Z"}
	decision := WeightDecision{RunID: "draft", ResetDecision: governance.ConsequenceDecision{
		WouldRefuse: true, Reason: "DRAFT has no consequence authority"}, DischargeDecision: governance.ConsequenceDecision{
		WouldRefuse: true, Reason: "DRAFT has no consequence authority"}, PriorWeight: prior.Accumulated, PriorLandings: prior.Landings}
	got, recorded := recordInertWeightDecision(prior, decision)
	if !recorded || got.LastDecision == nil || !got.LastDecision.ResetDecision.WouldRefuse || !got.LastDecision.DischargeDecision.WouldRefuse {
		t.Fatalf("would-refuse was not recorded: %+v", got)
	}
	if got.Accumulated != prior.Accumulated || got.Landings != prior.Landings || got.LastDecision.Applied {
		t.Fatalf("DRAFT changed the governing state: before=%+v after=%+v", prior, got)
	}
}

type proofProber struct {
	alive   bool
	started time.Time
}

func (prober *proofProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if !prober.alive {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: prober.started}, identity.Alive, nil
}

func governedWeightBed(t *testing.T, now time.Time) (string, *goal.GoalFile) {
	t.Helper()
	root := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("init", "-q", "-b", "main")
	runGit("config", "user.name", "fixture")
	runGit("config", "user.email", "fixture@example.invalid")
	runGit("config", "goal.sync-remote", "local")
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	file := &goal.GoalFile{Id: "bounded", State: goal.StateClaimed, Intent: "Bound direct validation", Origin: goal.OriginMain,
		NextStep: "Run direct validation.", OpenedAt: now.Add(-time.Hour).Format(time.RFC3339), Revision: 3,
		Claimed:        &goal.ClaimRecord{Machine: "bed-m1", Lineage: "coordinator", At: now.Add(-30 * time.Minute).Format(time.RFC3339), Revision: 2},
		Budget:         &goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 30, ActiveJobLimit: 1},
		StopCapability: &goal.StopCapability{Generation: 2, Revision: 2, Machine: "bed-m1", ClaimEpoch: 7},
		History: []goal.HistoryLine{
			{At: now.Add(-time.Hour).Format(time.RFC3339), Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAV-bed-m1-00000000", Verb: "open", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
			{At: now.Add(-30 * time.Minute).Format(time.RFC3339), Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-bed-m1-00000001", Verb: "claim", Actor: "bed-m1+coordinator", Targets: []string{"bounded"}, Keep: -1},
			{At: now.Add(-20 * time.Minute).Format(time.RFC3339), Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000002", Verb: "set-obligation", Actor: "human:Wido", Targets: []string{"bounded"}, Keep: -1},
		},
	}
	effects := []goal.GoverningEffect{goal.EffectAuthorizeSpend, goal.EffectResetWeight, goal.EffectDischargeObligation}
	file.Obligation = &goal.GovernedObligation{Revision: 3, BudgetRevision: 2, State: goal.ObligationEnforced,
		Owner: "Wido", AuthorizedBy: "Wido", AuthorizedAt: now.Add(-20 * time.Minute).Format(time.RFC3339),
		AuthorityOperation: "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000002", ReviewPolicy: "C", ReviewOutcome: "human-approved",
		Effects: effects, AuthorizedEffects: effects,
		Assumptions: goal.ObligationAssumptions{Recurrence: goal.StandingSharedProcess, Platform: runtime.GOOS + "/" + runtime.GOARCH,
			ToolchainIdentity: runtime.Version(), SurfaceDigest: digest, MaxActiveJobs: 1, TimingEnvelopeSeconds: 60,
			ObservationSource: "run-terminal-record"},
		Triggers: goal.HumanReviewTriggers{ValueJudgment: "no", Reversibility: "reversible", SevereHarm: "no",
			UnfamiliarApproach: "no", TestDiscrimination: "strong", CorrelatedAssumptionRisk: "no",
			AuthorityScopeChange: "no", DestructiveReach: "none"}}
	goalPath := filepath.Join(root, "plans", "goals", "bounded.md")
	if err := os.MkdirAll(filepath.Dir(goalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "backlog.md"), goal.RenderRoot(&goal.RootRecord{
		Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
	}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goalPath, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "metasystem.conf", "plans/goals")
	runGit("commit", "-q", "-m", "governed weight bed")
	runGit("update-ref", goal.AcceptedRef, "HEAD")
	return root, file
}

func completeGreenProof(t *testing.T, root, id string, now *time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	prober := &proofProber{alive: true, started: *now}
	store := &run.Store{Root: root, Now: func() time.Time { return *now }, Prober: prober,
		Getpgid: func(pid int64) (int64, error) { return pid, nil }, AllPids: func() ([]int64, error) { return nil, nil }}
	store.AdmitGoverned = func(request run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		return dispatch.EvaluateGovernedRunAdmission(root, request, *now)
	}
	store.ObserveGoverned = func(record *run.Record, ended time.Time) run.AssumptionObservation {
		return dispatch.ObserveGovernedRun(root, record, ended)
	}
	nonce, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: id, Kind: "suite",
		Display: "weight-triggered direct validation", Log: filepath.Join("artifacts", id+".log"), GoalId: "bounded",
		ObligationRevision: 3, StandingShared: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind(id, nonce, 4242, 4242); err != nil {
		t.Fatal(err)
	}
	*now = (*now).Add(time.Minute)
	if err := store.WriteSidecar(id, 1, nonce, 0); err != nil {
		t.Fatal(err)
	}
	prober.alive = false
	result, err := store.Assess(id)
	if err != nil || !result.Transitioned || result.To != run.StatusGreen {
		t.Fatalf("green proof did not terminalize through real run files: %+v %v", result, err)
	}
}

func TestWeightDischargeConsumesExactFreshProofRaisesRetroAndReplayChangesNothing(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	root, file := governedWeightBed(t, now)
	priorNow := weightNow
	weightNow = func() time.Time { return now }
	t.Cleanup(func() { weightNow = priorNow })
	if _, _, err := WeightAdd(root, "landing-one", []byte("1\t0\tdirect.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	completeGreenProof(t, root, "green-g1", &now)
	now = now.Add(time.Minute)
	result, err := WeightDischarge(root, "bounded", 3, "green-g1")
	if err != nil || !result.Decision.Applied || result.Decision.WeightGeneration != 1 || len(result.State.ConsumedProofs) != 1 {
		t.Fatalf("fresh real proof did not discharge exactly once: %+v %v", result, err)
	}
	open, err := retrodebt.Open(root)
	if err != nil || len(open) != 1 || open[0].Kind != retrodebt.KindObligation || !strings.Contains(open[0].Source, "weight-g1-green-g1") {
		t.Fatalf("green discharge did not raise its retained retro obligation: %+v %v", open, err)
	}
	projection := dispatch.ProjectBudget(root, file, now.Add(time.Minute))
	if projection.Status != dispatch.BudgetKnown || !projection.StartedAt.Equal(now) || projection.WeightEpoch == nil ||
		*projection.WeightEpoch != 1 || projection.Attempts != 0 {
		t.Fatalf("real consumed proof did not become the budget epoch: %+v unknown=%+v", projection, projection.Unknown)
	}
	completeGreenProof(t, root, "same-second-g2", &now)
	projection = dispatch.ProjectBudget(root, file, now)
	if projection.Status != dispatch.BudgetKnown || projection.Attempts != 1 {
		t.Fatalf("exact generation failed to count a post-discharge proof launched in the discharge second: %+v", projection)
	}
	before, err := loadWeight(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := WeightAdd(root, "landing-two", []byte("1\t0\tnext.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := WeightDischarge(root, "bounded", 3, "green-g1"); err == nil || !strings.Contains(err.Error(), "REFUSED-PROOF-CONSUMED") {
		t.Fatalf("consumed proof replay was not typed refusal: %v", err)
	}
	after, err := loadWeight(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if after.LastDecision == nil || before.LastDecision == nil || after.LastDecision.DecidedAt != before.LastDecision.DecidedAt ||
		after.Generation != before.Generation+1 || after.Accumulated == 0 {
		t.Fatalf("refused replay changed the discharge decision or reset weight: before=%+v after=%+v", before, after)
	}
}

func TestWeightDischargeRefusesProofOlderThanCurrentWeightEpoch(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	root, _ := governedWeightBed(t, now)
	priorNow := weightNow
	weightNow = func() time.Time { return now }
	t.Cleanup(func() { weightNow = priorNow })
	if _, _, err := WeightAdd(root, "landing-one", []byte("1\t0\tdirect.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	completeGreenProof(t, root, "green-old", &now)
	if _, _, err := WeightAdd(root, "landing-two", []byte("1\t0\tchanged.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	stateBefore, err := loadWeight(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WeightDischarge(root, "bounded", 3, "green-old"); err == nil || !strings.Contains(err.Error(), "REFUSED-PROOF-STALE") {
		t.Fatalf("stale proof did not receive a typed refusal: %v", err)
	}
	stateAfter, err := loadWeight(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if stateAfter.Generation != stateBefore.Generation || stateAfter.Accumulated != stateBefore.Accumulated || !reflect.DeepEqual(stateAfter.LastDecision, stateBefore.LastDecision) {
		t.Fatalf("stale refusal changed weight state: before=%+v after=%+v", stateBefore, stateAfter)
	}
}
