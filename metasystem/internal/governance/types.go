// Package governance holds the closed consequence and obligation record
// vocabulary shared by goal and run records. It owns no policy engine.
package governance

import "fmt"

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
	if o.AuthorizedBy == "" || o.AuthorizedAt == "" || o.AuthorityOperation == "" {
		return ConsequenceDecision{Reason: "the human authorization record is incomplete"}
	}
	for _, allowed := range o.AuthorizedEffects {
		if allowed == effect {
			return ConsequenceDecision{Apply: true, Reason: "recorded human authorization covers " + string(effect)}
		}
	}
	return ConsequenceDecision{Reason: "recorded human authorization does not cover " + string(effect)}
}
