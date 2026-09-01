package dispatch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

type governedProofProber struct {
	alive   bool
	started time.Time
}

func (prober *governedProofProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if !prober.alive {
		return identity.Exact{}, identity.Dead, nil
	}
	return identity.Exact{Pid: pid, StartedAt: prober.started}, identity.Alive, nil
}

func installEnforcedObligation(t *testing.T, root string, attemptLimit uint64) uint64 {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=C\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("fixture goal did not parse: %v", problems)
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
	file.Budget.AttemptLimit = attemptLimit
	file.History = append(file.History, goal.HistoryLine{At: "2026-08-28T10:15:00Z",
		Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-bed-m1-00000004", Verb: "set-obligation",
		Actor: "human:Wido", Targets: []string{"bounded"}, Keep: -1})
	effects := []goal.GoverningEffect{goal.EffectAuthorizeSpend}
	file.Obligation = &goal.GovernedObligation{Revision: file.Revision, BudgetRevision: file.Claimed.Revision,
		State: goal.ObligationEnforced, Owner: "Wido", AuthorizedBy: "Wido", AuthorizedAt: "2026-08-28T10:15:00Z",
		AuthorityOperation: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-bed-m1-00000004", ReviewPolicy: "C", ReviewOutcome: "human-approved",
		Effects: effects, AuthorizedEffects: effects,
		Assumptions: goal.ObligationAssumptions{Recurrence: goal.StandingSharedProcess,
			Platform: runtime.GOOS + "/" + runtime.GOARCH, ToolchainIdentity: runtime.Version(), SurfaceDigest: digest,
			MaxActiveJobs: 1, TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record"},
		Triggers: goal.HumanReviewTriggers{ValueJudgment: "no", Reversibility: "reversible", SevereHarm: "no",
			UnfamiliarApproach: "no", TestDiscrimination: "strong", CorrelatedAssumptionRisk: "no",
			AuthorityScopeChange: "no", DestructiveReach: "none"}}
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "metasystem.conf", "plans/goals/bounded.md"}, {"commit", "-q", "-m", "enforced obligation"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if output, runErr := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, output)
		}
	}
	return file.Revision
}

func TestGovernedAdmissionRefusesAcceptedGoalWithoutObligationTuple(t *testing.T) {
	root := revisionBindingBed(t, 2)
	_, err := EvaluateGovernedRunAdmission(root, run.GovernedAdmissionRequest{
		GoalID: "bounded", ObligationRevision: 1, StandingShared: true,
	}, time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "has no accepted obligation revision 1") {
		t.Fatalf("accepted goal without its obligation tuple was admitted: %v", err)
	}
}

func TestGovernedAdmissionKeepsRecordedRelayConsequencesActive(t *testing.T) {
	root := revisionBindingBed(t, 2)
	revision := installEnforcedObligation(t, root, 4)
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 || file.Obligation == nil {
		t.Fatalf("enforced fixture did not parse: %v", problems)
	}
	file.Obligation.AuthorizedBy = "recorded-relay"
	file.Obligation.ReviewOutcome = goal.ReviewOutcomeRecordedRelay
	file.Obligation.AuthorityOutcome = goal.AuthorityOutcomeTemporaryHumanWord
	file.Obligation.AuthorityReviewBy = "2026-09-06"
	file.Obligation.AuthorityRuling = goal.TemporaryGoalAuthorityRuling
	file.Obligation.TemporaryHumanWord = "Wido authorizes governed admission"
	last := &file.History[len(file.History)-1]
	last.AuthorityOutcome = goal.AuthorityOutcomeTemporaryHumanWord
	last.AuthorityReviewBy = "2026-09-06"
	last.AuthorityRuling = goal.TemporaryGoalAuthorityRuling
	last.TemporaryHumanWord = "Wido authorizes governed admission"
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "plans/goals/bounded.md"}, {"commit", "-q", "-m", "recorded relay obligation"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if output, runErr := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, output)
		}
	}
	admission, err := EvaluateGovernedRunAdmission(root, run.GovernedAdmissionRequest{
		GoalID: "bounded", ObligationRevision: revision, StandingShared: true,
	}, time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC))
	if err != nil || !admission.Attempt.AdmissionDecision.Apply ||
		!strings.Contains(admission.Attempt.AdmissionDecision.Reason, "human provenance not verified") {
		t.Fatalf("recorded relay became inert or overstated at governed admission: admission=%+v err=%v", admission, err)
	}
}

func TestDraftAdmissionRecordsWouldRefuseButDoesNotRefuseTheRun(t *testing.T) {
	root := revisionBindingBed(t, 2)
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.governance.correlation-policy=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "plans", "goals", "bounded.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 {
		t.Fatalf("fixture goal did not parse: %v", problems)
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
	file.History = append(file.History, goal.HistoryLine{At: "2026-08-28T10:15:00Z",
		Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAZ-bed-m1-00000004", Verb: "set-obligation",
		Actor: "human:wido", Targets: []string{"bounded"}, Keep: -1})
	file.Obligation = &goal.GovernedObligation{Revision: file.Revision, BudgetRevision: 2,
		State: goal.ObligationDraft, Owner: "Wido", Effects: []goal.GoverningEffect{goal.EffectAuthorizeSpend},
		Assumptions: goal.ObligationAssumptions{Recurrence: goal.StandingSharedProcess,
			Platform: runtime.GOOS + "/" + runtime.GOARCH, ToolchainIdentity: runtime.Version(), SurfaceDigest: digest,
			MaxActiveJobs: 1, TimingEnvelopeSeconds: 60, ObservationSource: "run-terminal-record"},
		Triggers: goal.HumanReviewTriggers{ValueJudgment: "unknown", Reversibility: "unknown", SevereHarm: "unknown",
			UnfamiliarApproach: "unknown", TestDiscrimination: "unknown", CorrelatedAssumptionRisk: "unknown",
			AuthorityScopeChange: "unknown", DestructiveReach: "unknown"}}
	if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "plans/goals/bounded.md"}, {"commit", "-q", "-m", "draft obligation"}, {"update-ref", goal.AcceptedRef, "HEAD"}} {
		if output, runErr := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); runErr != nil {
			t.Fatalf("git %v: %v: %s", args, runErr, output)
		}
	}
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	admission, err := EvaluateGovernedRunAdmission(root, run.GovernedAdmissionRequest{
		GoalID: "bounded", ObligationRevision: file.Revision, StandingShared: true}, now)
	if err != nil || !admission.Attempt.AdmissionDecision.WouldRefuse || admission.Attempt.AdmissionDecision.Apply {
		t.Fatalf("DRAFT did not produce an inert would-refuse: %+v %v", admission, err)
	}
	store := &run.Store{Root: root, Now: func() time.Time { return now },
		AdmitGoverned: func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) { return admission, nil }}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "draft-observer", Kind: "suite",
		Display: "draft observer", Log: "artifacts/draft-observer.log", GoalId: "bounded",
		ObligationRevision: file.Revision, StandingShared: true}); err != nil {
		t.Fatalf("DRAFT would-refuse became a refusal: %v", err)
	}
	record, err := store.Read("draft-observer")
	if err != nil || record == nil || record.Governed == nil || !record.Governed.AdmissionDecision.WouldRefuse {
		t.Fatalf("DRAFT would-refuse was not recorded on the admitted run: %+v %v", record, err)
	}
}

func TestExhaustedObligationSurvivesIDOverlayAttemptAndGovernedRunPrune(t *testing.T) {
	root := revisionBindingBed(t, 2)
	obligationRevision := installEnforcedObligation(t, root, 1)
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	prober := &governedProofProber{alive: true, started: now}
	store := &run.Store{Root: root, Now: func() time.Time { return now }, Prober: prober,
		Getpgid: func(pid int64) (int64, error) { return pid, nil }, AllPids: func() ([]int64, error) { return nil, nil }}
	store.AdmitGoverned = func(request run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		return EvaluateGovernedRunAdmission(root, request, now)
	}
	store.ObserveGoverned = func(*run.Record, time.Time) run.AssumptionObservation {
		return run.AssumptionObservation{ObservedAt: now.UTC().Format(time.RFC3339), AssumptionState: run.AssumptionMatch}
	}
	params := run.LaunchParams{Id: "red-n", Kind: "suite", Display: "governed red N", Log: "artifacts/red-n.log",
		GoalId: "bounded", ObligationRevision: obligationRevision, StandingShared: true}
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	nonce, err := store.Launch(run.Caller{Class: "HUMAN"}, params)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Bind("red-n", nonce, 5151, 5151); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	if err := store.WriteSidecar("red-n", 1, nonce, 1); err != nil {
		t.Fatal(err)
	}
	prober.alive = false
	if result, err := store.Assess("red-n"); err != nil || result.To != run.StatusRed {
		t.Fatalf("red N did not terminalize: %+v %v", result, err)
	}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "red-n", Kind: "suite", Display: "overlay", Log: "artifacts/overlay.log"}); err == nil {
		t.Fatal("ungoverned overlay reused the terminal governed ID")
	} else {
		var typed *run.TerminalGovernedRunIDReuseError
		if !errors.As(err, &typed) {
			t.Fatalf("terminal governed ID refusal was not typed: %v", err)
		}
	}
	if err := store.Ack(run.Caller{Class: "HUMAN"}, "red-n"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(15 * 24 * time.Hour)
	dropped, err := store.Prune(run.Caller{Class: "HUMAN"})
	if err != nil || len(dropped) != 1 {
		t.Fatalf("governed evidence did not prune with durable state retained: %v %v", dropped, err)
	}
	if _, err := os.Stat(run.RecordPath(root, "red-n")); !os.IsNotExist(err) {
		t.Fatalf("run evidence survived prune unexpectedly: %v", err)
	}
	states, err := obligationstate.LoadGoal(root, "bounded")
	if err != nil || len(states) != 1 || len(states[0].Attempts) != 1 || states[0].Attempts[0].PrunedAt == "" || !states[0].Attempts[0].Exhausted {
		t.Fatalf("prune forgot durable exhaustion: %+v %v", states, err)
	}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "red-n", Kind: "suite", Display: "post-prune overlay", Log: "artifacts/overlay.log"}); err == nil {
		t.Fatal("pruning made a terminal governed ID reusable")
	}
	params.Id = "attempt-n-plus-one"
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, params); err == nil || !strings.Contains(err.Error(), "OBLIGATION_REFUSED") {
		t.Fatalf("N+1 was not refused after terminal evidence pruning: %v", err)
	}
}

func TestMissingUnprunedRunEvidenceMakesDurableSpendFailClosed(t *testing.T) {
	root := revisionBindingBed(t, 2)
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	obligationRevision := installEnforcedObligation(t, root, 2)
	now := time.Date(2026, 8, 28, 10, 30, 0, 0, time.UTC)
	store := &run.Store{Root: root, Now: func() time.Time { return now },
		AdmitGoverned: func(request run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
			return EvaluateGovernedRunAdmission(root, request, now)
		}, ObserveGoverned: func(*run.Record, time.Time) run.AssumptionObservation {
			return run.AssumptionObservation{ObservedAt: now.UTC().Format(time.RFC3339), AssumptionState: run.AssumptionMatch}
		}}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, run.LaunchParams{Id: "lost-evidence", Kind: "suite",
		Display: "lost evidence", Log: "artifacts/lost.log", GoalId: "bounded", ObligationRevision: obligationRevision,
		StandingShared: true}); err != nil {
		t.Fatal(err)
	}
	if err := store.FailLaunch("lost-evidence", "fixture terminal"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(run.RecordPath(root, "lost-evidence")); err != nil {
		t.Fatal(err)
	}
	params := run.LaunchParams{Id: "after-loss", Kind: "suite", Display: "after loss", Log: "artifacts/after.log",
		GoalId: "bounded", ObligationRevision: obligationRevision, StandingShared: true}
	if _, err := store.Launch(run.Caller{Class: "HUMAN"}, params); err == nil || !strings.Contains(err.Error(), "BUDGET_UNKNOWN") {
		t.Fatalf("durable spend exceeding surviving unpruned evidence did not fail closed: %v", err)
	}
}
