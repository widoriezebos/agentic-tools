// Package governance holds the closed consequence and obligation record
// vocabulary shared by goal and run records. It owns no policy engine.
package governance

import (
	"fmt"
	"time"
)

type ObligationState string

const (
	ObligationDraft    ObligationState = "DRAFT"
	ObligationObserve  ObligationState = "OBSERVE"
	ObligationLimited  ObligationState = "LIMITED"
	ObligationEnforced ObligationState = "ENFORCED"
)

type RecurrenceClass string

const (
	SingleExperiment      RecurrenceClass = "single-experiment"
	StandingSharedProcess RecurrenceClass = "standing-shared-process"
)

type GoverningEffect string

const (
	EffectDischargeObligation GoverningEffect = "discharge-obligation"
	EffectAcceptWork          GoverningEffect = "accept-work"
	EffectResetObligation     GoverningEffect = "reset-obligation"
	EffectResetWeight         GoverningEffect = "reset-weight"
	EffectRefuseWork          GoverningEffect = "refuse-work"
	EffectPromoteAuthority    GoverningEffect = "promote-authority"
	EffectAuthorizeLaunch     GoverningEffect = "authorize-governed-launch"
	EffectAuthorizeSpend      GoverningEffect = "authorize-spend"
)

type ObligationAssumptions struct {
	Recurrence            RecurrenceClass
	Platform              string
	ToolchainIdentity     string
	SurfaceDigest         string
	MaxActiveJobs         uint64
	TimingEnvelopeSeconds uint64
	ObservationSource     string
}

func (a ObligationAssumptions) Validate() error {
	if a.Recurrence != SingleExperiment && a.Recurrence != StandingSharedProcess {
		return fmt.Errorf("recurrence %q is unknown", a.Recurrence)
	}
	if a.Platform == "" || a.ToolchainIdentity == "" || a.SurfaceDigest == "" || a.MaxActiveJobs == 0 ||
		a.TimingEnvelopeSeconds == 0 || a.ObservationSource != "run-terminal-record" {
		return fmt.Errorf("typed assumptions are incomplete or unsupported")
	}
	return nil
}

type HumanReviewTriggers struct {
	ValueJudgment            string
	Reversibility            string
	SevereHarm               string
	UnfamiliarApproach       string
	TestDiscrimination       string
	CorrelatedAssumptionRisk string
	AuthorityScopeChange     string
	DestructiveReach         string
}

type GovernedObligation struct {
	Revision           uint64
	BudgetRevision     uint64
	State              ObligationState
	Owner              string
	AuthorizedBy       string
	AuthorizedAt       string
	AuthorityOperation string
	ReviewPolicy       string
	ReviewOutcome      string
	Effects            []GoverningEffect
	AuthorizedEffects  []GoverningEffect
	Assumptions        ObligationAssumptions
	Triggers           HumanReviewTriggers
}

type ConsequenceDecision struct {
	Apply       bool
	WouldRefuse bool
	Reason      string
}

// ValidateAuthorizationCompleteness protects the authorization tuple required
// before an active obligation can apply a governing effect.
func (o *GovernedObligation) ValidateAuthorizationCompleteness() error {
	if o == nil {
		return fmt.Errorf("authorization record is missing")
	}
	if o.AuthorizedBy == "" {
		return fmt.Errorf("AuthorizedBy is missing")
	}
	if _, err := time.Parse(time.RFC3339, o.AuthorizedAt); err != nil {
		return fmt.Errorf("AuthorizedAt is missing or invalid")
	}
	if o.AuthorityOperation == "" {
		return fmt.Errorf("AuthorityOperation is missing")
	}
	if len(o.AuthorizedEffects) == 0 {
		return fmt.Errorf("AuthorizedEffects are missing")
	}
	if o.ReviewPolicy != "A" && o.ReviewPolicy != "B" && o.ReviewPolicy != "C" {
		return fmt.Errorf("ReviewPolicy is missing or invalid")
	}
	if o.ReviewOutcome != "human-approved" {
		return fmt.Errorf("ReviewOutcome is missing or not human-approved")
	}
	return nil
}

func (o *GovernedObligation) Decide(effect GoverningEffect) ConsequenceDecision {
	if o == nil {
		return ConsequenceDecision{Reason: "no governed obligation is recorded"}
	}
	if o.State == ObligationDraft || o.State == ObligationObserve {
		for _, intended := range o.Effects {
			if intended == effect {
				return ConsequenceDecision{WouldRefuse: true, Reason: string(o.State) + " has no consequence authority"}
			}
		}
		return ConsequenceDecision{Reason: "the obligation does not govern " + string(effect)}
	}
	if o.State != ObligationLimited && o.State != ObligationEnforced {
		return ConsequenceDecision{Reason: "obligation state " + string(o.State) + " has no consequence authority"}
	}
	if err := o.ValidateAuthorizationCompleteness(); err != nil {
		return ConsequenceDecision{Reason: "human authorization is incomplete: " + err.Error()}
	}
	for _, allowed := range o.AuthorizedEffects {
		if allowed == effect {
			return ConsequenceDecision{Apply: true, Reason: "recorded human authorization covers " + string(effect)}
		}
	}
	return ConsequenceDecision{Reason: "recorded human authorization does not cover " + string(effect)}
}
