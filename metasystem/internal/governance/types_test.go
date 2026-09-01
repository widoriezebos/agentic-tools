package governance_test

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
)

func authorizedObligation(state governance.ObligationState) governance.GovernedObligation {
	effect := governance.EffectAuthorizeSpend
	return governance.GovernedObligation{
		State:              state,
		AuthorizedBy:       "Wido",
		AuthorizedAt:       "2026-08-30T08:00:00Z",
		AuthorityOperation: "01ARZ3NDEKTSV4RRFFQ69G5FAX-bed-m1-00000002",
		ReviewPolicy:       "C",
		ReviewOutcome:      "human-approved",
		Effects:            []governance.GoverningEffect{effect},
		AuthorizedEffects:  []governance.GoverningEffect{effect},
	}
}

func TestActiveObligationRefusesIncompleteHumanAuthorization(t *testing.T) {
	for _, state := range []governance.ObligationState{governance.ObligationLimited, governance.ObligationEnforced} {
		t.Run(string(state), func(t *testing.T) {
			tests := []struct {
				name       string
				fieldClass string
				remove     func(*governance.GovernedObligation)
			}{
				{name: "authorized by", fieldClass: "AuthorizedBy", remove: func(o *governance.GovernedObligation) { o.AuthorizedBy = "" }},
				{name: "authorized at", fieldClass: "AuthorizedAt", remove: func(o *governance.GovernedObligation) { o.AuthorizedAt = "not-a-timestamp" }},
				{name: "authority operation", fieldClass: "AuthorityOperation", remove: func(o *governance.GovernedObligation) { o.AuthorityOperation = "" }},
				{name: "authorized effects", fieldClass: "AuthorizedEffects", remove: func(o *governance.GovernedObligation) { o.AuthorizedEffects = nil }},
				{name: "review policy", fieldClass: "ReviewPolicy", remove: func(o *governance.GovernedObligation) { o.ReviewPolicy = "D" }},
				{name: "review outcome", fieldClass: "ReviewOutcome", remove: func(o *governance.GovernedObligation) { o.ReviewOutcome = "machine-approved" }},
			}
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					obligation := authorizedObligation(state)
					test.remove(&obligation)
					validationErr := obligation.ValidateAuthorizationCompleteness()
					if validationErr == nil || !strings.Contains(validationErr.Error(), test.fieldClass) {
						t.Fatalf("authorization validation did not name %s: %v", test.fieldClass, validationErr)
					}
					decision := obligation.Decide(governance.EffectAuthorizeSpend)
					if decision.Apply || decision.WouldRefuse {
						t.Fatalf("incomplete human authorization was not refused: %+v", decision)
					}
					if !strings.Contains(decision.Reason, test.fieldClass) || !strings.Contains(decision.Reason, validationErr.Error()) {
						t.Fatalf("refusal did not preserve the shared %s validation reason: %+v", test.fieldClass, decision)
					}
				})
			}
		})
	}
}

func TestDecideCoversEveryObligationState(t *testing.T) {
	effect := governance.EffectAuthorizeSpend
	for _, state := range []governance.ObligationState{governance.ObligationDraft, governance.ObligationObserve} {
		t.Run(string(state), func(t *testing.T) {
			obligation := governance.GovernedObligation{State: state, Effects: []governance.GoverningEffect{effect}}
			decision := obligation.Decide(effect)
			if decision.Apply || !decision.WouldRefuse || !strings.Contains(decision.Reason, "no consequence authority") {
				t.Fatalf("observation-only state gained consequence authority: %+v", decision)
			}
			decision = obligation.Decide(governance.EffectAcceptWork)
			if decision.Apply || decision.WouldRefuse || !strings.Contains(decision.Reason, "does not govern") {
				t.Fatalf("unintended effect received an inert refusal: %+v", decision)
			}
		})
	}
	for _, state := range []governance.ObligationState{governance.ObligationLimited, governance.ObligationEnforced} {
		t.Run(string(state), func(t *testing.T) {
			obligation := authorizedObligation(state)
			decision := obligation.Decide(effect)
			if !decision.Apply || decision.WouldRefuse || !strings.Contains(decision.Reason, "covers authorize-spend") {
				t.Fatalf("complete active authorization did not apply: %+v", decision)
			}
			decision = obligation.Decide(governance.EffectAcceptWork)
			if decision.Apply || decision.WouldRefuse || !strings.Contains(decision.Reason, "does not cover accept-work") {
				t.Fatalf("unauthorized effect was not refused: %+v", decision)
			}
		})
	}
}

func TestDecideFailsClosedWithoutALawfulState(t *testing.T) {
	if decision := (*governance.GovernedObligation)(nil).Decide(governance.EffectAuthorizeSpend); decision.Apply || decision.WouldRefuse || !strings.Contains(decision.Reason, "no governed obligation") {
		t.Fatalf("missing obligation did not fail closed: %+v", decision)
	}
	obligation := authorizedObligation(governance.ObligationState("UNKNOWN"))
	if decision := obligation.Decide(governance.EffectAuthorizeSpend); decision.Apply || decision.WouldRefuse || !strings.Contains(decision.Reason, "UNKNOWN") {
		t.Fatalf("unknown obligation state did not fail closed: %+v", decision)
	}
	if err := (*governance.GovernedObligation)(nil).ValidateAuthorizationCompleteness(); err == nil || !strings.Contains(err.Error(), "authorization record") {
		t.Fatalf("missing authorization record validated: %v", err)
	}
}

func TestRecordedTemporaryAuthorityRequiresACompleteWireTuple(t *testing.T) {
	valid := governance.GovernedObligation{
		AuthorityOutcome:   governance.AuthorityOutcomeTemporaryHumanWord,
		AuthorityReviewBy:  "2026-09-06",
		AuthorityRuling:    governance.TemporaryGoalAuthorityRuling,
		TemporaryHumanWord: "Wido authorizes this obligation",
	}
	if err := valid.ValidateRecordedAuthority(); err != nil {
		t.Fatalf("valid temporary authority provenance was refused: %v", err)
	}
	historical := valid
	historical.AuthorityReviewBy = "2026-09-07"
	historical.AuthorityRuling = "R-33-m1"
	if err := historical.ValidateRecordedAuthority(); err != nil {
		t.Fatalf("a landed authority fact was re-judged against today's grant policy: %v", err)
	}
	legacy := valid
	legacy.AuthorityRuling, legacy.TemporaryHumanWord = "", ""
	if err := legacy.ValidateRecordedAuthority(); err != nil {
		t.Fatalf("a landed two-field authority fact became unreadable: %v", err)
	}
	historicalActive := authorizedObligation(governance.ObligationEnforced)
	historicalActive.AuthorizedBy = governance.AuthorizedByRecordedRelay
	historicalActive.ReviewOutcome = governance.ReviewOutcomeRecordedRelay
	historicalActive.AuthorityOutcome = historical.AuthorityOutcome
	historicalActive.AuthorityReviewBy = historical.AuthorityReviewBy
	historicalActive.AuthorityRuling = historical.AuthorityRuling
	historicalActive.TemporaryHumanWord = historical.TemporaryHumanWord
	if decision := historicalActive.Decide(governance.EffectAuthorizeSpend); !decision.Apply ||
		!strings.Contains(decision.Reason, "human provenance not verified") {
		t.Fatalf("a renewed historical marker disabled an already-authorized active consequence: %+v", decision)
	}
	legacyActive := authorizedObligation(governance.ObligationEnforced)
	legacyActive.AuthorityOutcome = legacy.AuthorityOutcome
	legacyActive.AuthorityReviewBy = legacy.AuthorityReviewBy
	if decision := legacyActive.Decide(governance.EffectAuthorizeSpend); !decision.Apply ||
		!strings.Contains(decision.Reason, "human provenance not verified") {
		t.Fatalf("a legacy two-field marker disabled an already-authorized active consequence: %+v", decision)
	}
	invalidLegacy := legacyActive
	invalidLegacy.ReviewOutcome = governance.ReviewOutcomeRecordedRelay
	if decision := invalidLegacy.Decide(governance.EffectAuthorizeSpend); decision.Apply ||
		!strings.Contains(decision.Reason, "invalid for the legacy recorded relay") {
		t.Fatalf("a legacy marker accepted a review outcome it never carried: %+v", decision)
	}
	overclaimed := historicalActive
	overclaimed.ReviewOutcome = governance.ReviewOutcomeHumanApproved
	if decision := overclaimed.Decide(governance.EffectAuthorizeSpend); decision.Apply ||
		!strings.Contains(decision.Reason, "not a recorded relay") {
		t.Fatalf("a complete relayed marker still claimed human-approved provenance: %+v", decision)
	}
	overclaimed = historicalActive
	overclaimed.AuthorizedBy = "Wido"
	if decision := overclaimed.Decide(governance.EffectAuthorizeSpend); decision.Apply ||
		!strings.Contains(decision.Reason, "not a verified person") {
		t.Fatalf("a complete relayed marker still claimed a verified person: %+v", decision)
	}
	var absent *governance.GovernedObligation
	if err := absent.ValidateRecordedAuthority(); err != nil {
		t.Fatalf("nil obligation carries no provenance to refuse: %v", err)
	}
	for _, test := range []struct {
		name string
		edit func(*governance.GovernedObligation)
		want string
	}{
		{name: "missing outcome", edit: func(o *governance.GovernedObligation) { o.AuthorityOutcome = "" }, want: "AuthorityOutcome"},
		{name: "unknown outcome", edit: func(o *governance.GovernedObligation) { o.AuthorityOutcome = "HUMAN_AUTHORITY_PROVEN" }, want: "AuthorityOutcome"},
		{name: "missing review date", edit: func(o *governance.GovernedObligation) { o.AuthorityReviewBy = "" }, want: "AuthorityReviewBy"},
		{name: "non-date review", edit: func(o *governance.GovernedObligation) { o.AuthorityReviewBy = "whenever" }, want: "AuthorityReviewBy"},
		{name: "missing ruling", edit: func(o *governance.GovernedObligation) { o.AuthorityRuling = "" }, want: "AuthorityRuling"},
		{name: "missing word", edit: func(o *governance.GovernedObligation) { o.TemporaryHumanWord = "" }, want: "TemporaryHumanWord"},
	} {
		t.Run(test.name, func(t *testing.T) {
			obligation := valid
			test.edit(&obligation)
			if err := obligation.ValidateRecordedAuthority(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid authority provenance did not refuse by %s: %v", test.want, err)
			}
		})
	}
	active := authorizedObligation(governance.ObligationEnforced)
	active.AuthorityOutcome = "HUMAN_AUTHORITY_PROVEN"
	if decision := active.Decide(governance.EffectAuthorizeSpend); decision.Apply || !strings.Contains(decision.Reason, "AuthorityOutcome") {
		t.Fatalf("active obligation with malformed authority provenance did not fail closed: %+v", decision)
	}
}

func TestObligationAssumptionsValidateClosedVocabulary(t *testing.T) {
	valid := governance.ObligationAssumptions{
		Recurrence: governance.SingleExperiment, Platform: "fixture/os", ToolchainIdentity: "fixture-go",
		SurfaceDigest: "fixture-digest", MaxActiveJobs: 1, TimingEnvelopeSeconds: 60,
		ObservationSource: "run-terminal-record",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("complete typed assumptions were refused: %v", err)
	}
	unknown := valid
	unknown.Recurrence = "sometimes"
	if err := unknown.Validate(); err == nil || !strings.Contains(err.Error(), "recurrence") {
		t.Fatalf("unknown recurrence was accepted: %v", err)
	}
	incomplete := valid
	incomplete.Platform = ""
	if err := incomplete.Validate(); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete assumptions were accepted: %v", err)
	}
}
