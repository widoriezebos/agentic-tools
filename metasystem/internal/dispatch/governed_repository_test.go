package dispatch

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// installAcceptedEnforcedObligation writes the goal before accepting its new snapshot.
func installAcceptedEnforcedObligation(t *testing.T, bed *goalAdmissionBed, attemptLimit uint64) uint64 {
	t.Helper()
	root := bed.root
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
	bed.accept(t)
	return file.Revision
}
