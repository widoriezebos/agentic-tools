package goal

import (
	"strings"
	"testing"
)

func TestGovernedObligationRoundTripsTypedAssumptionsAndTriggers(t *testing.T) {
	f := claimedGolden()
	f.Id = "governed"
	f.Budget = &Budget{ElapsedLimit: "2h", AttemptLimit: 3, ReservedJobMinutesLimit: 90, ActiveJobLimit: 1}
	f.Obligation = &GovernedObligation{
		Revision: 3, BudgetRevision: 2, State: ObligationDraft, Owner: "Wido",
		Effects: []GoverningEffect{EffectAuthorizeSpend, EffectRefuseWork},
		Assumptions: ObligationAssumptions{Recurrence: StandingSharedProcess, Platform: "darwin/arm64",
			ToolchainIdentity: "go1.25.0", SurfaceDigest: strings.Repeat("a", 64), MaxActiveJobs: 1,
			TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record"},
		Triggers: HumanReviewTriggers{ValueJudgment: "yes", Reversibility: "compensable", SevereHarm: "unknown",
			UnfamiliarApproach: "no", TestDiscrimination: "strong", CorrelatedAssumptionRisk: "yes",
			AuthorityScopeChange: "no", DestructiveReach: "none"},
	}
	parsed, problems := ParseFile(RenderFile(f))
	if len(problems) != 0 {
		t.Fatalf("typed obligation did not parse: %v", problems)
	}
	if parsed.Obligation == nil || parsed.Obligation.Triggers.SevereHarm != "unknown" || parsed.Obligation.BudgetRevision != 2 {
		t.Fatalf("typed obligation changed in round trip: %+v", parsed.Obligation)
	}
}
