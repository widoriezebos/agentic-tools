package dispatch

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"time"

	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

const governedObligationRevisionMismatch = "obligationRevision"

// ObserveGovernedAssumptions evaluates only the five typed fields declared by
// the obligation. There is intentionally no expression or plug-in language.
func ObserveGovernedAssumptions(repoRoot string, expected goal.ObligationAssumptions, activeJobs, durationSeconds uint64, now time.Time) run.AssumptionObservation {
	observation := run.AssumptionObservation{ObservedAt: now.UTC().Format(time.RFC3339),
		Platform: runtime.GOOS + "/" + runtime.GOARCH, ToolchainIdentity: runtime.Version(),
		ActiveJobs: activeJobs, DurationSeconds: durationSeconds, AssumptionState: run.AssumptionMatch}
	policy, err := behaviorsurface.Load()
	if err == nil {
		observation.SurfaceDigest, err = policy.Digest(repoRoot, behaviorsurface.Engine)
	}
	if err != nil {
		observation.AssumptionState = run.AssumptionUnavailable
		observation.DriftedFields = []string{"surfaceDigest"}
		return observation
	}
	if observation.Platform != expected.Platform {
		observation.DriftedFields = append(observation.DriftedFields, "platform")
	}
	if observation.ToolchainIdentity != expected.ToolchainIdentity {
		observation.DriftedFields = append(observation.DriftedFields, "toolchainIdentity")
	}
	if observation.SurfaceDigest != expected.SurfaceDigest {
		observation.DriftedFields = append(observation.DriftedFields, "surfaceDigest")
	}
	if activeJobs > expected.MaxActiveJobs {
		observation.DriftedFields = append(observation.DriftedFields, "activeJobs")
	}
	if durationSeconds > expected.TimingEnvelopeSeconds {
		observation.DriftedFields = append(observation.DriftedFields, "durationSeconds")
	}
	if len(observation.DriftedFields) > 0 {
		observation.AssumptionState = run.AssumptionDrift
	}
	sort.Strings(observation.DriftedFields)
	return observation
}

// ObserveGovernedRun resolves active executions from the same budget
// projection used by admission. Any unreadable observation fails closed.
func ObserveGovernedRun(repoRoot string, record *run.Record, now time.Time) run.AssumptionObservation {
	return observeGovernedRun(repoRoot, record, now, "")
}

func observeGovernedRun(repoRoot string, record *run.Record, now time.Time, excludeRunID string) run.AssumptionObservation {
	return observeGovernedRunWithReads(repoRoot, record, now, excludeRunID, concreteGoalAdmissionReads())
}

func observeGovernedRunWithReads(repoRoot string, record *run.Record, now time.Time, excludeRunID string, reads goalAdmissionReads) run.AssumptionObservation {
	unavailable := func(field string) run.AssumptionObservation {
		return run.AssumptionObservation{ObservedAt: now.UTC().Format(time.RFC3339), AssumptionState: run.AssumptionUnavailable,
			DriftedFields: []string{field}}
	}
	if record == nil || record.Governed == nil {
		return unavailable("governedAttempt")
	}
	binding, err := resolveGoalBindingWithReads(repoRoot, record.GoalId, now, reads)
	if err != nil || binding.Revision != record.Governed.GoalRevision || binding.File.Obligation == nil ||
		binding.File.Obligation.Revision != record.Governed.ObligationRevision {
		return unavailable(governedObligationRevisionMismatch)
	}
	projection := ProjectBudgetWithoutRun(repoRoot, binding.File, now, excludeRunID)
	if projection.Status != BudgetKnown {
		return unavailable("activeJobs")
	}
	started, err := time.Parse(time.RFC3339, record.StartedAt)
	if err != nil || now.Before(started) {
		return unavailable("durationSeconds")
	}
	activeJobs := projection.ActiveJobs
	if excludeRunID != "" {
		if activeJobs == math.MaxUint64 {
			return unavailable("activeJobs")
		}
		activeJobs++
	}
	return ObserveGovernedAssumptions(repoRoot, record.Governed.ExpectedAssumptions,
		activeJobs, uint64(now.Sub(started)/time.Second), now)
}

func settledSpendAtConclusionWithReads(repoRoot string, record *run.Record, now time.Time, reads goalAdmissionReads) (run.SpendSnapshot, string) {
	binding, err := resolveGoalBindingWithReads(repoRoot, record.GoalId, now, reads)
	if err != nil {
		return run.SpendSnapshot{}, fmt.Sprintf("record=%s reason=%s", record.RunId, err)
	}
	if record.Governed == nil || binding.Revision != record.Governed.GoalRevision || binding.File.Obligation == nil ||
		binding.File.Obligation.Revision != record.Governed.ObligationRevision {
		return run.SpendSnapshot{}, fmt.Sprintf("record=%s reason=%s", record.RunId, governedObligationRevisionMismatch)
	}
	projection := ProjectBudgetWithoutRun(repoRoot, binding.File, now, record.RunId)
	if projection.Status != BudgetKnown {
		return run.SpendSnapshot{}, fmt.Sprintf("record=%s reason=%s", projection.Unknown.Record, projection.Unknown.Reason)
	}
	return run.SpendSnapshot{ObservedMinutes: projection.ObservedJobMinutes, OpenCapMinutes: projection.OpenCapMinutes,
		ProofReservationMinutes: projection.ProofReservationMinutes}, ""
}

// NewConcludingRunStore is the production constructor for a run store that
// may terminalize a governed run.
func NewConcludingRunStore(root string, currentEpoch func() (*int64, bool)) *run.Store {
	return newConcludingRunStoreWithReads(root, currentEpoch, concreteGoalAdmissionReads())
}

// NewConcludingRunStoreWithReads binds concluding observations to one validated
// source of accepted goal and receipt facts.
func NewConcludingRunStoreWithReads(root string, currentEpoch func() (*int64, bool), reads ProofAdmissionReads) (*run.Store, error) {
	if err := reads.Validate(); err != nil {
		return nil, err
	}
	return newConcludingRunStoreWithReads(root, currentEpoch, reads.private()), nil
}

func newConcludingRunStoreWithReads(root string, currentEpoch func() (*int64, bool), reads goalAdmissionReads) *run.Store {
	return &run.Store{Root: root, CurrentEpoch: currentEpoch,
		AdmitGoverned: func(request run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
			return evaluateGovernedRunAdmissionWithReads(root, request, time.Now().UTC(), reads)
		},
		ObserveGoverned: func(record *run.Record, now time.Time) run.AssumptionObservation {
			return observeGovernedRunWithReads(root, record, now, record.RunId, reads)
		},
		ProjectSpend: func(record *run.Record, now time.Time) (run.SpendSnapshot, string) {
			return settledSpendAtConclusionWithReads(root, record, now, reads)
		},
	}
}

func evaluateGovernedRunAdmissionWithReads(repoRoot string, request run.GovernedAdmissionRequest, now time.Time, reads goalAdmissionReads) (run.GovernedAdmissionResult, error) {
	binding, err := resolveGoalBindingWithReads(repoRoot, request.GoalID, now, reads)
	if err != nil {
		return run.GovernedAdmissionResult{}, err
	}
	o := binding.File.Obligation
	if o == nil || o.Revision != request.ObligationRevision {
		return run.GovernedAdmissionResult{}, refusal.New("OBLIGATION_REFUSED", fmt.Sprintf("goal=%s obligation=%d", request.GoalID, request.ObligationRevision),
			fmt.Errorf("goal %s has no accepted obligation %d, so this run would spend without approval\na person sets one: metasystem goal edit %s --obligation STATE --owner <name>", request.GoalID, request.ObligationRevision, request.GoalID))
	}
	if request.StandingShared && o.Assumptions.Recurrence != goal.StandingSharedProcess {
		return run.GovernedAdmissionResult{}, refusal.New("OBLIGATION_REFUSED", fmt.Sprintf("revision=%d", o.Revision), fmt.Errorf("obligation %d is not approved to run as a standing shared process", o.Revision))
	}
	decision := o.Decide(goal.EffectAuthorizeSpend)
	active := o.State == goal.ObligationLimited || o.State == goal.ObligationEnforced
	policy, err := config.CorrelationPolicy(repoRoot)
	if err != nil {
		return run.GovernedAdmissionResult{}, refusal.New("OBLIGATION_REFUSED", "policy=unreadable", fmt.Errorf("the review policy setting cannot be read: %w", err))
	}
	if active && policy == "" {
		return run.GovernedAdmissionResult{}, refusal.New("OBLIGATION_REFUSED", "policy=empty", errors.New("no review policy is set, so limited and enforced obligations cannot run"))
	}
	if active && o.ReviewPolicy != policy {
		return run.GovernedAdmissionResult{}, refusal.New("OBLIGATION_REFUSED", "policy="+policy, fmt.Errorf("the obligation was approved under another review policy than the current one, %s", policy))
	}
	if active && !decision.Apply {
		return run.GovernedAdmissionResult{}, refusal.New("OBLIGATION_REFUSED", "", fmt.Errorf("the obligation does not allow this spend: %s", decision.Reason))
	}
	projection := ProjectBudget(repoRoot, binding.File, now)
	if projection.Status != BudgetKnown {
		return run.GovernedAdmissionResult{}, refusal.New("BUDGET_UNKNOWN", "record="+projection.Unknown.Record+" reason="+projection.Unknown.Reason,
			fmt.Errorf("the goal's budget cannot be worked out: %s cannot be read", projection.Unknown.Record))
	}
	states, err := obligationstate.LoadGoal(repoRoot, request.GoalID)
	if err != nil {
		return run.GovernedAdmissionResult{}, refusal.New("BUDGET_UNKNOWN", "record=artifacts/agents/governed-obligations reason="+err.Error(), fmt.Errorf("the record of earlier obligation runs cannot be read: %w", err))
	}
	for _, state := range states {
		if state.GoalRevision != binding.Revision || state.ObligationRevision != request.ObligationRevision {
			continue
		}
		for _, attempt := range state.Attempts {
			inEpoch := sameUint64(attempt.BudgetEpoch, projection.WeightEpoch)
			if active && inEpoch && (attempt.Exhausted || attempt.Breaker == run.BreakerAssumption) {
				return run.GovernedAdmissionResult{}, refusal.New("OBLIGATION_REFUSED", "breaker="+string(attempt.Breaker)+" run="+attempt.RunID,
					fmt.Errorf("run %s already stopped this obligation (%s); a person decides to reduce, redesign, retire or extend it", attempt.RunID, attempt.Breaker))
			}
		}
	}
	weightGeneration, weightUnknown := currentWeightGeneration(repoRoot)
	if weightUnknown != nil {
		return run.GovernedAdmissionResult{}, refusal.New("BUDGET_UNKNOWN", "record="+weightUnknown.Record+" reason="+weightUnknown.Reason, fmt.Errorf("the goal's budget cannot be worked out: %s cannot be read", weightUnknown.Record))
	}
	cost := (o.Assumptions.TimingEnvelopeSeconds + 59) / 60
	breaches := budgetAdmissionBreaches(projection)
	if projection.ReservedJobMinutes < projection.Limits.ReservedJobMinutesLimit &&
		cost > projection.Limits.ReservedJobMinutesLimit-projection.ReservedJobMinutes {
		breaches = append(breaches, BudgetBreach{Field: "reservedJobMinutesLimit",
			Used: fmt.Sprintf("%d+%d proposed", projection.ReservedJobMinutes, cost), Limit: fmt.Sprint(projection.Limits.ReservedJobMinutesLimit)})
	}
	if active && len(breaches) > 0 {
		return run.GovernedAdmissionResult{}, refusal.New("BUDGET_REFUSED", fmt.Sprintf("goal=%s revision=%d %s", request.GoalID, binding.Revision, formatRefusalDetail(breaches, reservedMinutesEvidence(projection))),
			fmt.Errorf("goal %s has used its budget, so this run does not start\na person raises it: metasystem goal budget %s", request.GoalID, request.GoalID))
	}
	observation := ObserveGovernedAssumptions(repoRoot, o.Assumptions, projection.ActiveJobs+1, 0, now)
	if active && observation.AssumptionState != run.AssumptionMatch {
		return run.GovernedAdmissionResult{}, refusal.New("OBLIGATION_REFUSED", fmt.Sprintf("assumptionState=%s fields=%v", observation.AssumptionState, observation.DriftedFields),
			fmt.Errorf("the obligation's assumptions no longer hold (%v changed), so this run does not start", observation.DriftedFields))
	}
	return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{
		GoalRevision: binding.Revision, ObligationRevision: o.Revision, Recurrence: o.Assumptions.Recurrence,
		WeightGeneration: &weightGeneration, BudgetEpoch: projection.WeightEpoch,
		ExecutionCostMinutes: cost, AttemptOrdinal: projection.Attempts + 1, ReservedBefore: projection.ReservedJobMinutes,
		Budget: projection.Limits, BudgetStartedAt: projection.StartedAt.UTC().Format(time.RFC3339), CorrelationPolicy: policy,
		ExpectedAssumptions: o.Assumptions, AdmissionDecision: decision, Observation: &observation, Breaker: run.BreakerClosed,
	}}, nil
}
