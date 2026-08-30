package goal

import (
	"reflect"
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

func TestGovernedObligationRoundTripsEveryLawfulState(t *testing.T) {
	for _, state := range []ObligationState{ObligationDraft, ObligationObserve, ObligationLimited, ObligationEnforced} {
		t.Run(string(state), func(t *testing.T) {
			f := claimedGolden()
			f.Id = "governed-" + strings.ToLower(string(state))
			f.Budget = &Budget{ElapsedLimit: "2h", AttemptLimit: 3, ReservedJobMinutesLimit: 90, ActiveJobLimit: 1}
			obligation := &GovernedObligation{
				Revision: 3, BudgetRevision: 2, State: state, Owner: "Wido",
				Effects: []GoverningEffect{EffectAuthorizeSpend, EffectRefuseWork},
				Assumptions: ObligationAssumptions{Recurrence: StandingSharedProcess, Platform: "darwin/arm64",
					ToolchainIdentity: "go1.25.0", SurfaceDigest: strings.Repeat("a", 64), MaxActiveJobs: 1,
					TimingEnvelopeSeconds: 1800, ObservationSource: "run-terminal-record"},
				Triggers: HumanReviewTriggers{ValueJudgment: "yes", Reversibility: "compensable", SevereHarm: "unknown",
					UnfamiliarApproach: "no", TestDiscrimination: "strong", CorrelatedAssumptionRisk: "yes",
					AuthorityScopeChange: "no", DestructiveReach: "none"},
			}
			if state == ObligationLimited || state == ObligationEnforced {
				obligation.AuthorizedBy = "Wido"
				obligation.AuthorizedAt = "2026-08-30T08:00:00Z"
				obligation.AuthorityOperation = "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000002"
				obligation.ReviewPolicy = "C"
				obligation.ReviewOutcome = "human-approved"
				obligation.AuthorizedEffects = append([]GoverningEffect(nil), obligation.Effects...)
			}
			f.Obligation = obligation

			first := RenderFile(f)
			parsed, problems := ParseFile(first)
			if len(problems) != 0 {
				t.Fatalf("lawful %s obligation did not parse: %v", state, problems)
			}
			if second := RenderFile(parsed); string(first) != string(second) {
				t.Fatalf("%s obligation render/parse/render was not a fixed point", state)
			}
			if !reflect.DeepEqual(parsed.Obligation, obligation) {
				t.Fatalf("%s obligation changed in round trip:\nwant %+v\n got %+v", state, obligation, parsed.Obligation)
			}
		})
	}
}
