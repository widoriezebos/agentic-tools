package gaterun

import (
	"os"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
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
	bed, file := governedWeightFixture(t, now)
	return bed.root, file
}

func governedWeightFixture(t *testing.T, now time.Time) (*goalRepositoryFixture, *goal.GoalFile) {
	t.Helper()
	bed := newGoalRepositoryFixture(t, now)
	root := bed.root
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
	bed.seed(t, file, now)
	return bed, file
}

func completeGreenProofForGoal(t *testing.T, bed *goalRepositoryFixture, id, goalID string, obligationRevision uint64, now *time.Time) {
	t.Helper()
	root := bed.root
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	prober := &proofProber{alive: true, started: *now}
	store := &run.Store{Root: root, Now: func() time.Time { return *now }, Prober: prober,
		Getpgid: func(pid int64) (int64, error) { return pid, nil }, AllPids: func() ([]int64, error) { return nil, nil }}
	store.AdmitGoverned = func(request run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		return bed.admittedAttempt(request, *now)
	}
	store.ObserveGoverned = bed.terminalObservation
	store.ProjectSpend = func(*run.Record, time.Time) (run.SpendSnapshot, string) { return run.SpendSnapshot{}, "" }
	nonce, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: id, Kind: "suite",
		Display: "weight-triggered direct validation", Log: filepath.Join("artifacts", id+".log"), GoalId: goalID,
		ObligationRevision: obligationRevision, StandingShared: true})
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

func completeGreenProof(t *testing.T, bed *goalRepositoryFixture, id string, now *time.Time) {
	t.Helper()
	completeGreenProofForGoal(t, bed, id, "bounded", 3, now)
}

func elapsedWriterRequest(endpoint goal.Endpoint, ulid, machine string, at time.Time) goal.VerbRequest {
	return goal.VerbRequest{Endpoint: endpoint, Actor: goal.Actor{Machine: machine, Lineage: "coordinator"},
		Ulid: ulid, Now: at, ClaimEpoch: 7}
}

func acceptedWriterGoal(t *testing.T, endpoint goal.Endpoint, id string, at time.Time) *goal.GoalFile {
	t.Helper()
	projection, err := goal.Project(endpoint, false, at)
	if err != nil {
		t.Fatal(err)
	}
	file := projection.Tree.Live[id]
	if file == nil {
		t.Fatalf("accepted goal %s is missing", id)
	}
	return file
}

func realWriterObligation(t *testing.T, root string) goal.GovernedObligation {
	t.Helper()
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	digest, err := policy.Digest(root, behaviorsurface.Engine)
	if err != nil {
		t.Fatal(err)
	}
	effects := []goal.GoverningEffect{goal.EffectAuthorizeSpend, goal.EffectResetWeight, goal.EffectDischargeObligation}
	return goal.GovernedObligation{State: goal.ObligationEnforced, Owner: "Wido", Effects: effects,
		Assumptions: goal.ObligationAssumptions{Recurrence: goal.StandingSharedProcess, Platform: runtime.GOOS + "/" + runtime.GOARCH,
			ToolchainIdentity: runtime.Version(), SurfaceDigest: digest, MaxActiveJobs: 1, TimingEnvelopeSeconds: 60,
			ObservationSource: "run-terminal-record"},
		Triggers: goal.HumanReviewTriggers{ValueJudgment: "no", Reversibility: "reversible", SevereHarm: "no",
			UnfamiliarApproach: "no", TestDiscrimination: "strong", CorrelatedAssumptionRisk: "no",
			AuthorityScopeChange: "no", DestructiveReach: "none"}}
}

func openApprovedClaimedWriterGoal(t *testing.T, endpoint goal.Endpoint, proof *humanauthority.Proof, t0 time.Time, risk *goal.RiskRecord) *goal.GoalFile {
	t.Helper()
	budget := goal.Budget{ElapsedLimit: "4h", AttemptLimit: 4, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1}
	open := elapsedWriterRequest(endpoint, "01J5X00000000000000000EB00", "bed-m1", t0.Add(-2*time.Minute))
	var result goal.PublishResult
	var err error
	if risk == nil {
		result, err = goal.Open(open, "bounded", "Preserve the episode.", goal.OriginMain, "Exercise real writers.")
	} else {
		budget = goal.Budget{ElapsedLimit: "1h", AttemptLimit: 3, ReservedJobMinutesLimit: 360, ActiveJobLimit: 1}
		result, err = goal.OpenRisked(open, "bounded", "Preserve the legacy episode.", goal.OriginHuman, "Exercise real writers.", "", *risk, 0, "", &budget, nil)
	}
	if err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("open writer goal: %+v %v", result, err)
	}
	human := elapsedWriterRequest(endpoint, "01J5X00000000000000000EB01", "bed-m1", t0.Add(-time.Minute))
	human.Actor.Human = "Wido"
	if result, err = goal.Approve(human, []string{"bounded"}, &budget, proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("approve writer goal: %+v %v", result, err)
	}
	claim := elapsedWriterRequest(endpoint, "01J5X00000000000000000EB02", "bed-m1", t0)
	if result, err = goal.Claim(claim, "bounded"); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("claim writer goal: %+v %v", result, err)
	}
	return acceptedWriterGoal(t, endpoint, "bounded", t0)
}

func installLegacyEpisodeShape(t *testing.T, bed *goalRepositoryFixture, file *goal.GoalFile) {
	t.Helper()
	file.Claimed.EpisodeAt = ""
	file.Claimed.EpisodeRevision = 0
	file.Claimed.EpisodeObligationRevision = 0
	bed.publishFile(t, file, "fixture-legacy-episode")
}

func TestSetBudgetInheritsEpisodeObligationRevision(t *testing.T) {
	t0 := time.Date(2026, 9, 11, 8, 0, 0, 0, time.UTC)
	bed := newGoalRepositoryFixture(t, t0)
	bed.seed(t, nil, t0)
	root, endpoint, proof := bed.root, bed.endpoint, bed.proof
	file := openApprovedClaimedWriterGoal(t, endpoint, proof, t0, nil)
	human := elapsedWriterRequest(endpoint, "01J5X00000000000000000EB03", "bed-m1", t0.Add(time.Minute))
	human.Actor.Human = "Wido"
	if result, err := goal.SetObligation(human, file.Id, realWriterObligation(t, root), proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("set obligation: %+v %v", result, err)
	}
	file = acceptedWriterGoal(t, endpoint, file.Id, human.Now)
	obligationRevision := file.Obligation.Revision
	now := t0.Add(2 * time.Minute)
	priorNow := weightNow
	weightNow = func() time.Time { return now }
	t.Cleanup(func() { weightNow = priorNow })
	if _, _, err := WeightAdd(root, "episode-landing", []byte("1\t0\tepisode.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	completeGreenProofForGoal(t, bed, "episode-green", file.Id, obligationRevision, &now)
	now = now.Add(time.Minute)
	dischargeAt := now
	if result, err := bed.discharge(file.Id, obligationRevision, "episode-green", now); err != nil || !result.Decision.Applied {
		t.Fatalf("weight discharge: %+v %v", result, err)
	}
	file = acceptedWriterGoal(t, endpoint, file.Id, now)
	beforeRaise := dispatch.ProjectBudget(root, file, now)
	if beforeRaise.Status != dispatch.BudgetKnown || !beforeRaise.StartedAt.Equal(dischargeAt) {
		t.Fatalf("real discharge did not move the start before a raise: %+v", beforeRaise)
	}
	firstBudget := *file.Budget
	firstBudget.ElapsedLimit = "6h"
	first := elapsedWriterRequest(endpoint, "01J5X00000000000000000EB04", "bed-m1", now.Add(time.Minute))
	first.Actor.Human = "Wido"
	if result, err := goal.SetBudgetApproved(first, file.Id, firstBudget, proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("first budget raise: %+v %v", result, err)
	}
	file = acceptedWriterGoal(t, endpoint, file.Id, first.Now)
	firstProjection := dispatch.ProjectBudget(root, file, first.Now)
	if file.Obligation != nil || file.Claimed.EpisodeObligationRevision != obligationRevision || firstProjection.Status != dispatch.BudgetKnown || !firstProjection.StartedAt.Equal(dischargeAt) {
		t.Fatalf("first raise did not capture the live discharge identity: goal=%+v projection=%+v", file, firstProjection)
	}
	secondBudget := *file.Budget
	secondBudget.ElapsedLimit = "8h"
	second := elapsedWriterRequest(endpoint, "01J5X00000000000000000EB05", "bed-m1", first.Now.Add(time.Minute))
	second.Actor.Human = "Wido"
	if result, err := goal.SetBudgetApproved(second, file.Id, secondBudget, proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
		t.Fatalf("second budget raise: %+v %v", result, err)
	}
	file = acceptedWriterGoal(t, endpoint, file.Id, second.Now)
	secondProjection := dispatch.ProjectBudget(root, file, second.Now)
	if file.Obligation != nil || file.Claimed.EpisodeObligationRevision != obligationRevision || secondProjection.Status != dispatch.BudgetKnown || !secondProjection.StartedAt.Equal(dischargeAt) ||
		firstProjection.WeightEpoch == nil || secondProjection.WeightEpoch == nil || *firstProjection.WeightEpoch != *secondProjection.WeightEpoch {
		t.Fatalf("second raise did not inherit the cleared discharge identity: goal=%+v first=%+v second=%+v", file, firstProjection, secondProjection)
	}
}

func TestLegacyRiskRaiseThenBudgetRaiseKeepsOriginAndProof(t *testing.T) {
	for _, withDischarge := range []bool{false, true} {
		t.Run(map[bool]string{false: "without discharge", true: "with authorized discharge"}[withDischarge], func(t *testing.T) {
			t0 := time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)
			bed := newGoalRepositoryFixture(t, t0)
			bed.seed(t, nil, t0)
			root, endpoint, proof := bed.root, bed.endpoint, bed.proof
			low := goal.RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "landed precedent"}
			file := openApprovedClaimedWriterGoal(t, endpoint, proof, t0, &low)
			high := goal.RiskRecord{Severity: 2, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "moderate consequence discovered"}
			tier := uint8(3)
			raiseRisk := elapsedWriterRequest(endpoint, "01J5X00000000000000000EK00", "bed-m1", t0.Add(time.Minute))
			if result, err := goal.Edit(raiseRisk, file.Id, goal.EditFields{Risk: &high, Tier: &tier, Why: "retain full review", Evidence: "refusal:BUDGET_REFUSED"}); err != nil || result.Outcome != goal.OutcomeConfirmed {
				t.Fatalf("risk raise: %+v %v", result, err)
			}
			file = acceptedWriterGoal(t, endpoint, file.Id, raiseRisk.Now)
			preRaiseAccounting := file.Claimed.AccountingRevision
			installLegacyEpisodeShape(t, bed, file)
			file = acceptedWriterGoal(t, endpoint, file.Id, raiseRisk.Now)
			wantStart := t0
			if withDischarge {
				set := elapsedWriterRequest(endpoint, "01J5X00000000000000000EK01", "bed-m1", t0.Add(2*time.Minute))
				set.Actor.Human = "Wido"
				if result, err := goal.SetObligation(set, file.Id, realWriterObligation(t, root), proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
					t.Fatalf("set obligation: %+v %v", result, err)
				}
				file = acceptedWriterGoal(t, endpoint, file.Id, set.Now)
				now := t0.Add(3 * time.Minute)
				priorNow := weightNow
				weightNow = func() time.Time { return now }
				t.Cleanup(func() { weightNow = priorNow })
				if _, _, err := WeightAdd(root, "legacy-landing", []byte("1\t0\tlegacy.go\n"), "", 1); err != nil {
					t.Fatal(err)
				}
				completeGreenProofForGoal(t, bed, "legacy-green", file.Id, file.Obligation.Revision, &now)
				now = now.Add(time.Minute)
				if result, err := bed.discharge(file.Id, file.Obligation.Revision, "legacy-green", now); err != nil || !result.Decision.Applied {
					t.Fatalf("weight discharge: %+v %v", result, err)
				}
				wantStart = now
				file = acceptedWriterGoal(t, endpoint, file.Id, now)
			}
			before := dispatch.ProjectBudget(root, file, wantStart)
			if before.Status != dispatch.BudgetKnown || !before.StartedAt.Equal(wantStart) {
				t.Fatalf("legacy risk-raised projection lost its origin before budget raise: %+v", before)
			}
			next := *file.Budget
			next.ElapsedLimit = "2h"
			setBudget := elapsedWriterRequest(endpoint, "01J5X00000000000000000EK02", "bed-m1", wantStart.Add(time.Minute))
			setBudget.Actor.Human = "Wido"
			if result, err := goal.SetBudgetApproved(setBudget, file.Id, next, proof); err != nil || result.Outcome != goal.OutcomeConfirmed {
				t.Fatalf("budget raise: %+v %v", result, err)
			}
			file = acceptedWriterGoal(t, endpoint, file.Id, setBudget.Now)
			after := dispatch.ProjectBudget(root, file, setBudget.Now)
			if file.Claimed.EpisodeAt != t0.Format(time.RFC3339) || file.Claimed.EpisodeRevision != preRaiseAccounting ||
				after.Status != dispatch.BudgetKnown || !after.StartedAt.Equal(wantStart) {
				t.Fatalf("legacy risk/budget raise did not materialize the accounting origin and preserve proof eligibility: goal=%+v before=%+v after=%+v", file, before, after)
			}
			if withDischarge {
				if before.WeightEpoch == nil || after.WeightEpoch == nil || *before.WeightEpoch != *after.WeightEpoch || file.Claimed.EpisodeObligationRevision == 0 {
					t.Fatalf("authorized legacy discharge identity did not survive the budget raise: goal=%+v before=%+v after=%+v", file, before, after)
				}
			} else if before.WeightEpoch != nil || after.WeightEpoch != nil || file.Claimed.EpisodeObligationRevision != 0 {
				t.Fatalf("no-discharge legacy case invented an epoch: goal=%+v before=%+v after=%+v", file, before, after)
			}
		})
	}
}

func TestWeightDischargeConsumesExactFreshProofRaisesRetroAndReplayChangesNothing(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	bed, file := governedWeightFixture(t, now)
	root := bed.root
	priorNow := weightNow
	weightNow = func() time.Time { return now }
	t.Cleanup(func() { weightNow = priorNow })
	if _, _, err := WeightAdd(root, "landing-one", []byte("1\t0\tdirect.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	completeGreenProof(t, bed, "green-g1", &now)
	now = now.Add(time.Minute)
	result, err := bed.discharge("bounded", 3, "green-g1", now)
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
	completeGreenProof(t, bed, "same-second-g2", &now)
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
	if _, err := bed.discharge("bounded", 3, "green-g1", now); err == nil || !strings.Contains(err.Error(), "REFUSED-PROOF-CONSUMED") {
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

func TestFocusedProofCannotDischargeCadence(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	bed, _ := governedWeightFixture(t, now)
	root := bed.root
	priorNow := weightNow
	weightNow = func() time.Time { return now }
	t.Cleanup(func() { weightNow = priorNow })
	conf := filepath.Join(root, "metasystem.conf")
	data, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(conf, append(data, []byte("testing.contract=testing.json\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WeightAdd(root, "landing-one", []byte("1\t0\tdirect.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	completeGreenProof(t, bed, "focused-green", &now)
	if _, err := bed.discharge("bounded", 3, "focused-green", now); err == nil || !strings.Contains(err.Error(), "lacks exact sufficient deep cadence evidence") {
		t.Fatalf("focused outer green discharged cadence weight: %v", err)
	}
}

func TestWeightDischargeAcceptsRecordedRelayReviewOutcome(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	bed, file := governedWeightFixture(t, now)
	root := bed.root
	file.Obligation.AuthorizedBy = "recorded-relay"
	file.Obligation.ReviewOutcome = goal.ReviewOutcomeRecordedRelay
	file.Obligation.AuthorityOutcome = goal.AuthorityOutcomeTemporaryHumanWord
	file.Obligation.AuthorityReviewBy = "2026-09-06"
	file.Obligation.AuthorityRuling = goal.TemporaryGoalAuthorityRuling
	file.Obligation.TemporaryHumanWord = "Wido authorizes weight discharge"
	last := &file.History[len(file.History)-1]
	last.AuthorityOutcome = goal.AuthorityOutcomeTemporaryHumanWord
	last.AuthorityReviewBy = "2026-09-06"
	last.AuthorityRuling = goal.TemporaryGoalAuthorityRuling
	last.TemporaryHumanWord = "Wido authorizes weight discharge"
	bed.publishFile(t, file, "fixture-recorded-relay")
	priorNow := weightNow
	weightNow = func() time.Time { return now }
	t.Cleanup(func() { weightNow = priorNow })
	if _, _, err := WeightAdd(root, "relay-landing", []byte("1\t0\tdirect.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	completeGreenProof(t, bed, "relay-green", &now)
	now = now.Add(time.Minute)
	result, err := bed.discharge("bounded", 3, "relay-green", now)
	if err != nil || !result.Decision.Applied ||
		!strings.Contains(result.Decision.ResetDecision.Reason, "human provenance not verified") {
		t.Fatalf("recorded relay became inert or overstated at weight discharge: result=%+v err=%v", result, err)
	}
}

func TestWeightDischargeRefusesProofOlderThanCurrentWeightEpoch(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	bed, _ := governedWeightFixture(t, now)
	root := bed.root
	priorNow := weightNow
	weightNow = func() time.Time { return now }
	t.Cleanup(func() { weightNow = priorNow })
	if _, _, err := WeightAdd(root, "landing-one", []byte("1\t0\tdirect.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	completeGreenProof(t, bed, "green-old", &now)
	if _, _, err := WeightAdd(root, "landing-two", []byte("1\t0\tchanged.go\n"), "", 1); err != nil {
		t.Fatal(err)
	}
	stateBefore, err := loadWeight(root, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bed.discharge("bounded", 3, "green-old", now); err == nil || !strings.Contains(err.Error(), "REFUSED-PROOF-STALE") {
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
