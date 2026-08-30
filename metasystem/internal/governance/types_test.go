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
