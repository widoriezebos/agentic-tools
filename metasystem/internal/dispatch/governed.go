package dispatch

import (
	"fmt"
	"runtime"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

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
	unavailable := func(field string) run.AssumptionObservation {
		return run.AssumptionObservation{ObservedAt: now.UTC().Format(time.RFC3339), AssumptionState: run.AssumptionUnavailable,
			DriftedFields: []string{field}}
	}
	if record == nil || record.Governed == nil {
		return unavailable("governedAttempt")
	}
	binding, err := ResolveGoalBinding(repoRoot, record.GoalId, now)
	if err != nil || binding.Revision != record.Governed.GoalRevision || binding.File.Obligation == nil ||
		binding.File.Obligation.Revision != record.Governed.ObligationRevision {
		return unavailable("obligationRevision")
	}
	projection := ProjectBudget(repoRoot, binding.File, now)
	if projection.Status != BudgetKnown {
		return unavailable("activeJobs")
	}
	started, err := time.Parse(time.RFC3339, record.StartedAt)
	if err != nil || now.Before(started) {
		return unavailable("durationSeconds")
	}
	return ObserveGovernedAssumptions(repoRoot, record.Governed.ExpectedAssumptions,
		projection.ActiveJobs, uint64(now.Sub(started)/time.Second), now)
}

// EvaluateGovernedRunAdmission binds authorization and the complete existing
// budget projection to the exact obligation revision about to be launched.
func EvaluateGovernedRunAdmission(repoRoot string, request run.GovernedAdmissionRequest, now time.Time) (run.GovernedAdmissionResult, error) {
	binding, err := ResolveGoalBinding(repoRoot, request.GoalID, now)
	if err != nil {
		return run.GovernedAdmissionResult{}, err
	}
	o := binding.File.Obligation
	if o == nil || o.Revision != request.ObligationRevision {
		return run.GovernedAdmissionResult{}, fmt.Errorf("OBLIGATION_REFUSED: goal %s has no accepted obligation revision %d", request.GoalID, request.ObligationRevision)
	}
	if request.StandingShared && o.Assumptions.Recurrence != goal.StandingSharedProcess {
		return run.GovernedAdmissionResult{}, fmt.Errorf("OBLIGATION_REFUSED: revision %d is not authorized as a standing shared process", o.Revision)
	}
	decision := o.Decide(goal.EffectAuthorizeSpend)
	active := o.State == goal.ObligationLimited || o.State == goal.ObligationEnforced
	policy, err := config.CorrelationPolicy(repoRoot)
	if err != nil {
		return run.GovernedAdmissionResult{}, fmt.Errorf("OBLIGATION_REFUSED: correlation policy is unreadable: %w", err)
	}
	if active && policy == "" {
		return run.GovernedAdmissionResult{}, fmt.Errorf("OBLIGATION_REFUSED: correlation policy slot is empty; LIMITED and ENFORCED consequences are not active")
	}
	if active && o.ReviewPolicy != policy {
		return run.GovernedAdmissionResult{}, fmt.Errorf("OBLIGATION_REFUSED: authorization was not recorded under active policy %s", policy)
	}
	if active && !decision.Apply {
		return run.GovernedAdmissionResult{}, fmt.Errorf("OBLIGATION_REFUSED: %s", decision.Reason)
	}
	projection := ProjectBudget(repoRoot, binding.File, now)
	if projection.Status != BudgetKnown {
		return run.GovernedAdmissionResult{}, fmt.Errorf("BUDGET_UNKNOWN record=%s reason=%s", projection.Unknown.Record, projection.Unknown.Reason)
	}
	states, err := obligationstate.LoadGoal(repoRoot, request.GoalID)
	if err != nil {
		return run.GovernedAdmissionResult{}, fmt.Errorf("BUDGET_UNKNOWN record=artifacts/agents/governed-obligations reason=%s", err)
	}
	for _, state := range states {
		if state.GoalRevision != binding.Revision || state.ObligationRevision != request.ObligationRevision {
			continue
		}
		for _, attempt := range state.Attempts {
			inEpoch := sameUint64(attempt.BudgetEpoch, projection.WeightEpoch)
			if active && inEpoch && (attempt.Exhausted || attempt.Breaker == run.BreakerAssumption) {
				return run.GovernedAdmissionResult{}, fmt.Errorf("OBLIGATION_REFUSED: breaker=%s is already terminal on run %s; Wido must choose reduce, redesign, retire, or extend", attempt.Breaker, attempt.RunID)
			}
		}
	}
	weightGeneration, weightUnknown := currentWeightGeneration(repoRoot)
	if weightUnknown != nil {
		return run.GovernedAdmissionResult{}, fmt.Errorf("BUDGET_UNKNOWN record=%s reason=%s", weightUnknown.Record, weightUnknown.Reason)
	}
	cost := (o.Assumptions.TimingEnvelopeSeconds + 59) / 60
	breaches := budgetAdmissionBreaches(projection)
	if projection.ReservedJobMinutes < projection.Limits.ReservedJobMinutesLimit &&
		cost > projection.Limits.ReservedJobMinutesLimit-projection.ReservedJobMinutes {
		breaches = append(breaches, BudgetBreach{Field: "reservedJobMinutesLimit",
			Used: fmt.Sprintf("%d+%d proposed", projection.ReservedJobMinutes, cost), Limit: fmt.Sprint(projection.Limits.ReservedJobMinutesLimit)})
	}
	if active && len(breaches) > 0 {
		return run.GovernedAdmissionResult{}, fmt.Errorf("BUDGET_REFUSED: goal %s revision=%d admission closed", request.GoalID, binding.Revision)
	}
	observation := ObserveGovernedAssumptions(repoRoot, o.Assumptions, projection.ActiveJobs+1, 0, now)
	if active && observation.AssumptionState != run.AssumptionMatch {
		return run.GovernedAdmissionResult{}, fmt.Errorf("OBLIGATION_REFUSED: admission assumptionState=%s fields=%v", observation.AssumptionState, observation.DriftedFields)
	}
	return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{
		GoalRevision: binding.Revision, ObligationRevision: o.Revision, Recurrence: o.Assumptions.Recurrence,
		WeightGeneration: &weightGeneration, BudgetEpoch: projection.WeightEpoch,
		ExecutionCostMinutes: cost, AttemptOrdinal: projection.Attempts + 1, ReservedBefore: projection.ReservedJobMinutes,
		Budget: projection.Limits, BudgetStartedAt: projection.StartedAt.UTC().Format(time.RFC3339), CorrelationPolicy: policy,
		ExpectedAssumptions: o.Assumptions, AdmissionDecision: decision, Observation: &observation, Breaker: run.BreakerClosed,
	}}, nil
}
