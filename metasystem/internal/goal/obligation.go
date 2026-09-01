package goal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
)

// ObligationState separates observation from authority. Draft and observe
// records can explain what they would have stopped, but cannot cause an
// external consequence.
type ObligationState = governance.ObligationState

const (
	ObligationDraft    = governance.ObligationDraft
	ObligationObserve  = governance.ObligationObserve
	ObligationLimited  = governance.ObligationLimited
	ObligationEnforced = governance.ObligationEnforced
)

// RecurrenceClass distinguishes a private experiment from a standing shared
// process. Repeating an experiment does not turn it into governance.
type RecurrenceClass = governance.RecurrenceClass

const (
	SingleExperiment      = governance.SingleExperiment
	StandingSharedProcess = governance.StandingSharedProcess
)

// GoverningEffect is the closed set of consequences that require recorded
// human authority at the action boundary.
type GoverningEffect = governance.GoverningEffect

const (
	EffectDischargeObligation = governance.EffectDischargeObligation
	EffectAcceptWork          = governance.EffectAcceptWork
	EffectResetObligation     = governance.EffectResetObligation
	EffectResetWeight         = governance.EffectResetWeight
	EffectRefuseWork          = governance.EffectRefuseWork
	EffectPromoteAuthority    = governance.EffectPromoteAuthority
	EffectAuthorizeLaunch     = governance.EffectAuthorizeLaunch
	EffectAuthorizeSpend      = governance.EffectAuthorizeSpend
)

// ObligationAssumptions are executable facts, not prose. The steward can
// observe every one from a terminal run record and treats an unavailable
// observation as a failed assumption.
type ObligationAssumptions = governance.ObligationAssumptions

// HumanReviewTriggers records the complete review vocabulary even while the
// correlation-policy slot is empty. Values are closed so policy activation
// cannot silently miss a spelling.
type HumanReviewTriggers = governance.HumanReviewTriggers

// GovernedObligation binds one immutable revision to the existing budget
// tuple. A replacement is a new goal revision; earlier bytes remain in Git.
type GovernedObligation = governance.GovernedObligation

const (
	ReviewOutcomeHumanApproved         = governance.ReviewOutcomeHumanApproved
	ReviewOutcomeRecordedRelay         = governance.ReviewOutcomeRecordedRelay
	AuthorizedByRecordedRelay          = governance.AuthorizedByRecordedRelay
	AuthorityOutcomeTemporaryHumanWord = governance.AuthorityOutcomeTemporaryHumanWord
	TemporaryGoalAuthorityRuling       = governance.TemporaryGoalAuthorityRuling
)

func validateRecordedTemporaryAuthority(outcome, reviewBy, ruling, humanWord string) error {
	return (governance.RecordedTemporaryAuthority{
		Outcome: outcome, ReviewBy: reviewBy, Ruling: ruling, HumanWord: humanWord,
	}).ValidateRecorded()
}

// ConsequenceDecision is recorded at the base action boundary. WouldRefuse is
// deliberately inert in DRAFT and OBSERVE.
type ConsequenceDecision = governance.ConsequenceDecision

func validObligationState(value ObligationState) bool {
	switch value {
	case ObligationDraft, ObligationObserve, ObligationLimited, ObligationEnforced:
		return true
	}
	return false
}

func validEffect(value GoverningEffect) bool {
	switch value {
	case EffectDischargeObligation, EffectAcceptWork, EffectResetObligation, EffectResetWeight,
		EffectRefuseWork, EffectPromoteAuthority, EffectAuthorizeLaunch, EffectAuthorizeSpend:
		return true
	}
	return false
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func validateGovernedObligation(o *GovernedObligation, fileRevision uint64, claim *ClaimRecord, budget *Budget) error {
	if o == nil {
		return nil
	}
	if o.Revision == 0 || o.Revision > fileRevision {
		return fmt.Errorf("obligation revision=%d is outside goal Revision=%d", o.Revision, fileRevision)
	}
	if claim == nil || budget == nil || o.BudgetRevision == 0 || o.BudgetRevision != claim.Revision {
		return fmt.Errorf("obligation budgetRevision=%d does not bind the claimed budget revision", o.BudgetRevision)
	}
	if !validObligationState(o.State) || strings.TrimSpace(o.Owner) == "" {
		return fmt.Errorf("obligation requires a known state and owner")
	}
	if err := o.ValidateRecordedAuthority(); err != nil {
		return fmt.Errorf("obligation recorded authority: %w", err)
	}
	if err := o.Assumptions.Validate(); err != nil {
		return fmt.Errorf("obligationAssumptions: %v", err)
	}
	seen := map[GoverningEffect]bool{}
	for _, effect := range o.Effects {
		if !validEffect(effect) || seen[effect] {
			return fmt.Errorf("obligation has invalid or duplicate effect %q", effect)
		}
		seen[effect] = true
	}
	if len(o.Effects) == 0 {
		return fmt.Errorf("obligation requires at least one governed effect")
	}
	if o.Assumptions.Recurrence == StandingSharedProcess && !seen[EffectAuthorizeSpend] {
		return fmt.Errorf("a standing shared process must record authorize-spend as a governed effect")
	}
	authorized := map[GoverningEffect]bool{}
	for _, effect := range o.AuthorizedEffects {
		if !validEffect(effect) || authorized[effect] || !seen[effect] {
			return fmt.Errorf("obligation has invalid, duplicate, or unintended authorized effect %q", effect)
		}
		authorized[effect] = true
	}
	if o.State == ObligationLimited || o.State == ObligationEnforced {
		if err := o.ValidateAuthorizationCompleteness(); err != nil {
			return fmt.Errorf("obligation %s requires a complete human authorization: %w", o.State, err)
		}
	}
	if (o.State == ObligationDraft || o.State == ObligationObserve) &&
		(o.AuthorizedBy != "" || o.AuthorizedAt != "" || o.AuthorityOperation != "" || len(o.AuthorizedEffects) != 0 ||
			o.ReviewPolicy != "" || o.ReviewOutcome != "") {
		return fmt.Errorf("obligation %s cannot carry consequence authority", o.State)
	}
	t := o.Triggers
	if !oneOf(t.ValueJudgment, "yes", "no", "unknown") ||
		!oneOf(t.Reversibility, "reversible", "compensable", "irreversible", "unknown") ||
		!oneOf(t.SevereHarm, "yes", "no", "unknown") ||
		!oneOf(t.UnfamiliarApproach, "yes", "no", "unknown") ||
		!oneOf(t.TestDiscrimination, "strong", "weak", "unknown") ||
		!oneOf(t.CorrelatedAssumptionRisk, "yes", "no", "unknown") ||
		!oneOf(t.AuthorityScopeChange, "yes", "no", "unknown") ||
		!oneOf(t.DestructiveReach, "none", "reversible-local", "destructive", "unknown") {
		return fmt.Errorf("obligationTriggers contains an unknown enum value")
	}
	return nil
}

func renderEffects(values []GoverningEffect) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = string(value)
	}
	sort.Strings(parts)
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

func parseEffects(value string) ([]GoverningEffect, error) {
	if value == "-" {
		return nil, nil
	}
	var effects []GoverningEffect
	for _, raw := range strings.Split(value, ",") {
		effect := GoverningEffect(raw)
		if !validEffect(effect) {
			return nil, fmt.Errorf("unknown effect %q", raw)
		}
		effects = append(effects, effect)
	}
	return effects, nil
}

func parseObligationRecord(value string) (*GovernedObligation, error) {
	withoutWord, humanWord, _, err := cutQuotedRecordField(value, "temporaryHumanWord")
	if err != nil {
		return nil, err
	}
	rec, err := parseKVRecord(withoutWord,
		[]string{"revision", "budgetRevision", "state", "owner", "authorizedBy", "authorizedAt", "authorityOperation", "reviewPolicy", "reviewOutcome", "effects", "authorizedEffects"},
		[]string{"authorityOutcome", "authorityReviewBy", "authorityRuling"}, "")
	if err != nil {
		return nil, err
	}
	revision, revisionErr := strconv.ParseUint(rec["revision"], 10, 64)
	budgetRevision, budgetErr := strconv.ParseUint(rec["budgetRevision"], 10, 64)
	effects, effectsErr := parseEffects(rec["effects"])
	authorizedEffects, authorizedEffectsErr := parseEffects(rec["authorizedEffects"])
	if revisionErr != nil || budgetErr != nil || effectsErr != nil || authorizedEffectsErr != nil {
		return nil, fmt.Errorf("invalid revision, budget revision, or effects")
	}
	normalize := func(value string) string {
		if value == "-" {
			return ""
		}
		return value
	}
	return &GovernedObligation{
		Revision: revision, BudgetRevision: budgetRevision, State: ObligationState(rec["state"]), Owner: rec["owner"],
		AuthorizedBy: normalize(rec["authorizedBy"]), AuthorizedAt: normalize(rec["authorizedAt"]),
		AuthorityOperation: normalize(rec["authorityOperation"]), ReviewPolicy: normalize(rec["reviewPolicy"]),
		ReviewOutcome: normalize(rec["reviewOutcome"]), AuthorityOutcome: normalize(rec["authorityOutcome"]),
		AuthorityReviewBy: normalize(rec["authorityReviewBy"]), AuthorityRuling: normalize(rec["authorityRuling"]),
		TemporaryHumanWord: humanWord, Effects: effects, AuthorizedEffects: authorizedEffects,
	}, nil
}

func parseObligationAssumptions(value string) (ObligationAssumptions, error) {
	rec, err := parseKVRecord(value,
		[]string{"recurrence", "platform", "toolchainIdentity", "surfaceDigest", "maxActiveJobs", "timingEnvelopeSeconds", "observationSource"}, nil, "")
	if err != nil {
		return ObligationAssumptions{}, err
	}
	maxActive, activeErr := strconv.ParseUint(rec["maxActiveJobs"], 10, 64)
	timing, timingErr := strconv.ParseUint(rec["timingEnvelopeSeconds"], 10, 64)
	if activeErr != nil || timingErr != nil {
		return ObligationAssumptions{}, fmt.Errorf("maxActiveJobs and timingEnvelopeSeconds must be unsigned integers")
	}
	return ObligationAssumptions{Recurrence: RecurrenceClass(rec["recurrence"]), Platform: rec["platform"],
		ToolchainIdentity: rec["toolchainIdentity"], SurfaceDigest: rec["surfaceDigest"], MaxActiveJobs: maxActive,
		TimingEnvelopeSeconds: timing, ObservationSource: rec["observationSource"]}, nil
}

func parseObligationTriggers(value string) (HumanReviewTriggers, error) {
	rec, err := parseKVRecord(value,
		[]string{"valueJudgment", "reversibility", "severeHarm", "unfamiliarApproach", "testDiscrimination", "correlatedAssumptionRisk", "authorityScopeChange", "destructiveReach"}, nil, "")
	if err != nil {
		return HumanReviewTriggers{}, err
	}
	return HumanReviewTriggers{ValueJudgment: rec["valueJudgment"], Reversibility: rec["reversibility"],
		SevereHarm: rec["severeHarm"], UnfamiliarApproach: rec["unfamiliarApproach"], TestDiscrimination: rec["testDiscrimination"],
		CorrelatedAssumptionRisk: rec["correlatedAssumptionRisk"], AuthorityScopeChange: rec["authorityScopeChange"], DestructiveReach: rec["destructiveReach"]}, nil
}
