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
	AuthorityOutcome   string
	AuthorityReviewBy  string
	AuthorityRuling    string
	TemporaryHumanWord string
	Effects            []GoverningEffect
	AuthorizedEffects  []GoverningEffect
	Assumptions        ObligationAssumptions
	Triggers           HumanReviewTriggers
}

const (
	ReviewOutcomeHumanApproved               = "human-approved"
	ReviewOutcomeRecordedRelay               = "recorded-relay"
	AuthorizedByRecordedRelay                = "recorded-relay"
	AuthorityOutcomeTemporaryHumanWord       = "TEMPORARY_HUMAN_WORD"
	AuthorityOutcomeAuthenticatedChannelWord = "AUTHENTICATED_CHANNEL_WORD"
	AuthorityOutcomeVerifiedChannelAnswer    = "VERIFIED_CHANNEL_ANSWER"
	TemporaryGoalAuthorityRuling             = "R-32-m1"
	TemporaryGoalAuthorityHorizon            = "2026-09-06"
	authorityReviewByDateLayout              = "2006-01-02"
)

type RecordedChannelAuthority struct {
	Outcome, Provider, UserID, MessageRef, ContextID string
	Step                                             int64
}

func (a RecordedChannelAuthority) ValidateRecorded() error {
	if (a.Outcome != AuthorityOutcomeAuthenticatedChannelWord && a.Outcome != AuthorityOutcomeVerifiedChannelAnswer) ||
		a.Provider == "" || a.UserID == "" || a.MessageRef == "" || a.Step < 1 ||
		(a.Outcome == AuthorityOutcomeVerifiedChannelAnswer && a.ContextID == "") {
		return fmt.Errorf("authenticated channel authority proof is incomplete")
	}
	return nil
}

// RecordedTemporaryAuthority is the durable tuple copied from a successful
// relayed grant. It records the supplied words and ruling; it does not prove
// who supplied the words.
type RecordedTemporaryAuthority struct {
	Outcome   string
	ReviewBy  string
	Ruling    string
	HumanWord string
}

func (a RecordedTemporaryAuthority) empty() bool {
	return a.Outcome == "" && a.ReviewBy == "" && a.Ruling == "" && a.HumanWord == ""
}

// ValidateRecorded checks only the stable wire shape of a landed fact. Grant
// policy belongs to the authority boundary: a later ruling or horizon must
// never make an already-landed ledger unreadable.
func (a RecordedTemporaryAuthority) ValidateRecorded() error {
	if a.empty() {
		return nil
	}
	if a.Outcome != AuthorityOutcomeTemporaryHumanWord {
		return fmt.Errorf("AuthorityOutcome is missing or invalid")
	}
	_, err := time.Parse(authorityReviewByDateLayout, a.ReviewBy)
	if err != nil {
		return fmt.Errorf("AuthorityReviewBy is missing or invalid")
	}
	// The first landed form predated ruling/word fields and carried exactly
	// outcome+reviewBy. It remains a readable historical fact; new grants use
	// the complete four-field form below.
	if a.Ruling == "" && a.HumanWord == "" {
		return nil
	}
	if a.Ruling == "" {
		return fmt.Errorf("AuthorityRuling is missing")
	}
	if a.HumanWord == "" {
		return fmt.Errorf("TemporaryHumanWord is missing")
	}
	return nil
}

type ConsequenceDecision struct {
	Apply       bool
	WouldRefuse bool
	Reason      string
}

// ValidateRecordedAuthority protects the fixed shape of a landed temporary
// marker without treating its recorded word as proof of human provenance.
func (o *GovernedObligation) ValidateRecordedAuthority() error {
	if o == nil {
		return nil
	}
	return (RecordedTemporaryAuthority{
		Outcome: o.AuthorityOutcome, ReviewBy: o.AuthorityReviewBy,
		Ruling: o.AuthorityRuling, HumanWord: o.TemporaryHumanWord,
	}).ValidateRecorded()
}

// ValidateAuthorizationCompleteness protects the authorization tuple required
// before an active obligation can apply a governing effect.
func (o *GovernedObligation) ValidateAuthorizationCompleteness() error {
	if o == nil {
		return fmt.Errorf("authorization record is missing")
	}
	if err := o.ValidateRecordedAuthority(); err != nil {
		return err
	}
	recorded := RecordedTemporaryAuthority{
		Outcome: o.AuthorityOutcome, ReviewBy: o.AuthorityReviewBy,
		Ruling: o.AuthorityRuling, HumanWord: o.TemporaryHumanWord,
	}
	recordedRelay := !recorded.empty()
	legacyRelay := recordedRelay && recorded.Ruling == "" && recorded.HumanWord == ""
	if o.AuthorizedBy == "" {
		return fmt.Errorf("AuthorizedBy is missing")
	}
	if recordedRelay && !legacyRelay && o.AuthorizedBy != AuthorizedByRecordedRelay {
		return fmt.Errorf("AuthorizedBy must identify recorded-relay authority, not a verified person")
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
	if recordedRelay {
		if legacyRelay && o.ReviewOutcome != ReviewOutcomeHumanApproved {
			return fmt.Errorf("ReviewOutcome is missing or invalid for the legacy recorded relay")
		}
		if !legacyRelay && o.ReviewOutcome != ReviewOutcomeRecordedRelay {
			return fmt.Errorf("ReviewOutcome is missing or not a recorded relay")
		}
	} else if o.ReviewOutcome != ReviewOutcomeHumanApproved {
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
	authority := "recorded enrolled-human authorization"
	if o.AuthorityOutcome != "" || o.AuthorityReviewBy != "" || o.AuthorityRuling != "" || o.TemporaryHumanWord != "" {
		authority = "recorded relayed authority (human provenance not verified)"
	}
	for _, allowed := range o.AuthorizedEffects {
		if allowed == effect {
			return ConsequenceDecision{Apply: true, Reason: authority + " covers " + string(effect)}
		}
	}
	return ConsequenceDecision{Reason: authority + " does not cover " + string(effect)}
}
