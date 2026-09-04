package dispatch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

var AdmissionRefusalCodes = []string{"BUDGET_UNKNOWN", "BUDGET_REFUSED", "HAZARD_REFUSED", "RISK_UNANSWERED"}

func ValidateMisclassificationEvidence(repoRoot, goalID, evidence string) error {
	checkRoot := func(jobID string) (map[string]any, error) {
		if !validJobID.MatchString(jobID) {
			return nil, fmt.Errorf("evidence %s has an invalid job identifier", evidence)
		}
		record, err := readObject(filepath.Join(repoRoot, "artifacts", "agents", "jobs", jobID+".json"))
		if err != nil || asString(record["jobId"]) != jobID || asString(record["goalId"]) != goalID {
			return nil, fmt.Errorf("evidence %s does not name a job bound to goal %s", evidence, goalID)
		}
		return record, nil
	}
	if jobID, ok := strings.CutPrefix(evidence, "root:"); ok {
		_, err := checkRoot(jobID)
		return err
	}
	if ref, ok := strings.CutPrefix(evidence, "finding:"); ok {
		jobID, findingID, found := strings.Cut(ref, "/")
		record, err := checkRoot(jobID)
		if err != nil {
			return err
		}
		if !found || findingID == "" {
			return fmt.Errorf("evidence %s does not name a finding", evidence)
		}
		for _, value := range anySlice(record[findingRegisterField]) {
			if row, ok := value.(map[string]any); ok && asString(row["findingId"]) == findingID {
				return nil
			}
		}
		return fmt.Errorf("evidence %s does not name a finding in job %s", evidence, jobID)
	}
	if code, ok := strings.CutPrefix(evidence, "refusal:"); ok {
		for _, admitted := range AdmissionRefusalCodes {
			if code == admitted {
				return nil
			}
		}
		return fmt.Errorf("evidence refusal:%s is not an admission refusal code; one of: %s", code, strings.Join(AdmissionRefusalCodes, ", "))
	}
	return fmt.Errorf("evidence %s must be root:<jobId>, finding:<jobId>/<id>, or refusal:<code>", evidence)
}

func anySlice(value any) []any {
	if values, ok := value.([]any); ok {
		return values
	}
	return nil
}

// GoalAdmissionRefusal is one claim whose structured budget closes
// the dispatch admission seam. Unknown evidence closes admission without
// inventing counters; known evidence names every exhausted limit.
type GoalAdmissionRefusal struct {
	GoalID         string
	GoalRevision   uint64
	Breaches       []BudgetBreach
	Unknown        *BudgetUnknownEvidence
	LiveStopReason string
}

// GoalAdmissionVerdict contains every refusal owned by the dispatching
// machine and lineage.
type GoalAdmissionVerdict struct {
	Refusals []GoalAdmissionRefusal
}

// Refused reports whether the structured law closes this dispatch round.
func (v GoalAdmissionVerdict) Refused() bool {
	return len(v.Refusals) > 0
}

// EvaluateGoalAdmission applies the four structured limits at the
// pre-reservation seam. It has no side effects: a refusal prevents publication
// but never winds down an existing job.
func EvaluateGoalAdmission(repoRoot, stopLineage string, now time.Time) (GoalAdmissionVerdict, error) {
	var verdict GoalAdmissionVerdict
	if !goal.NewWorld(repoRoot) {
		return verdict, nil
	}
	endpoint, err := goal.ResolveEndpoint(repoRoot)
	if err != nil {
		return verdict, err
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		if unknown, ok := GoalRecordBudgetUnknown(err); ok {
			return verdict, fmt.Errorf("BUDGET_UNKNOWN record=%s reason=%s", unknown.Record, unknown.Reason)
		}
		return verdict, err
	}
	if stopLineage == "" {
		return verdict, nil
	}

	needsMachine := false
	for _, file := range projection.Tree.Live {
		if file.State == goal.StateClaimed && file.Claimed != nil {
			needsMachine = true
			break
		}
	}
	if !needsMachine {
		return verdict, nil
	}
	machine, err := goal.ResolveMachine(repoRoot)
	if err != nil {
		return verdict, err
	}
	ids := make([]string, 0, len(projection.Tree.Live))
	for id := range projection.Tree.Live {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		file := projection.Tree.Live[id]
		if file.State != goal.StateClaimed || file.Claimed == nil ||
			file.Claimed.Machine != machine || file.Claimed.Lineage != stopLineage {
			continue
		}
		if file.StopFence != nil {
			verdict.Refusals = append(verdict.Refusals, GoalAdmissionRefusal{
				GoalID: id, GoalRevision: file.Claimed.Revision,
				Unknown: &BudgetUnknownEvidence{Code: BudgetUnknown, Record: goalRecordPath(id),
					Reason: fmt.Sprintf("launch fence closed by stop batch %s", file.StopFence.StopID)},
				LiveStopReason: file.StopFence.Reason,
			})
			continue
		}
		budget := ProjectBudget(repoRoot, file, now)
		if budget.Status == BudgetUnknown {
			verdict.Refusals = append(verdict.Refusals, GoalAdmissionRefusal{
				GoalID: id, GoalRevision: file.Claimed.Revision, Unknown: budget.Unknown,
			})
			continue
		}
		breaches := budgetAdmissionBreaches(budget)
		if len(breaches) > 0 {
			verdict.Refusals = append(verdict.Refusals, GoalAdmissionRefusal{
				GoalID: id, GoalRevision: budget.GoalRevision, Breaches: breaches,
				LiveStopReason: liveStopReason(budget),
			})
		}
	}
	return verdict, nil
}

// GoalRevisionAdmission is the final decision made while the chain and
// goal-revision locks are held. LiveStopReason is non-empty only at the
// elapsed breach boundary or for corrupt over-limit state; ordinary exhaustion
// closes admission without cancelling already-authorized jobs.
type GoalRevisionAdmission struct {
	GoalID         string
	GoalRevision   uint64
	Refusal        *GoalAdmissionRefusal
	PolicyRefusal  string
	PolicyNotice   string
	LiveStopReason string
}

func (v GoalRevisionAdmission) Refused() bool { return v.Refusal != nil || v.PolicyRefusal != "" }

// EvaluateGoalRevisionAdmission binds the final fence and projected cap
// decision to the exact accepted revision about to publish a reservation.
func EvaluateGoalRevisionAdmission(repoRoot, id string, revision, proposedCap uint64, now time.Time, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	verdict := GoalRevisionAdmission{GoalID: id, GoalRevision: revision}
	if len(hazards) > 1 {
		return verdict, fmt.Errorf("exactly one destructiveReach class may govern goal revision admission")
	}
	hazard := HazardMechanical
	if len(hazards) == 1 {
		hazard = hazards[0]
	}
	if _, err := MinimumHazardConfiguration(hazard); err != nil {
		return verdict, err
	}
	binding, err := ResolveGoalBinding(repoRoot, id, now)
	if err != nil {
		return verdict, err
	}
	if binding.Revision != revision {
		return verdict, fmt.Errorf("goal %s accepted revision moved from %d to %d", id, revision, binding.Revision)
	}
	if binding.File.Risk == nil {
		line := fmt.Sprintf("RISK_UNANSWERED goal=%s tier=%d next: goal edit --risk", id, binding.Tier)
		mode, modeErr := config.RiskGate(filepath.Join(repoRoot, "metasystem.conf"))
		if modeErr != nil {
			return verdict, modeErr
		}
		if mode == config.RiskGateEnforce {
			verdict.PolicyRefusal = line
			return verdict, nil
		}
		verdict.PolicyNotice = line
	}
	if binding.Tier == 1 && hazard != HazardMechanical {
		verdict.PolicyRefusal = "HAZARD_REFUSED: the hazard needs review the tier does not have; goal edit --tier 2"
		return verdict, nil
	}
	if binding.Fence != nil {
		verdict.LiveStopReason = binding.Fence.Reason
		verdict.Refusal = &GoalAdmissionRefusal{
			GoalID: id, GoalRevision: revision,
			Unknown: &BudgetUnknownEvidence{Code: BudgetUnknown, Record: goalRecordPath(id),
				Reason: fmt.Sprintf("launch fence closed by stop batch %s", binding.Fence.StopID)},
			LiveStopReason: binding.Fence.Reason,
		}
		return verdict, nil
	}
	projection := ProjectBudget(repoRoot, binding.File, now)
	if projection.Status != BudgetKnown {
		verdict.Refusal = &GoalAdmissionRefusal{GoalID: id, GoalRevision: revision, Unknown: projection.Unknown}
		return verdict, nil
	}
	if reason := liveStopReason(projection); reason != "" {
		verdict.LiveStopReason = reason
		verdict.Refusal = &GoalAdmissionRefusal{
			GoalID: id, GoalRevision: revision, Breaches: projection.Breaches, LiveStopReason: reason,
		}
		return verdict, nil
	}
	breaches := budgetAdmissionBreaches(projection)
	if proposedCap > 0 && projection.ReservedJobMinutes < projection.Limits.ReservedJobMinutesLimit &&
		proposedCap > projection.Limits.ReservedJobMinutesLimit-projection.ReservedJobMinutes {
		breaches = append(breaches, BudgetBreach{
			Field: "reservedJobMinutesLimit",
			Used:  fmt.Sprintf("%d+%d proposed", projection.ReservedJobMinutes, proposedCap),
			Limit: fmt.Sprintf("%d", projection.Limits.ReservedJobMinutesLimit),
		})
	}
	if len(breaches) > 0 {
		verdict.Refusal = &GoalAdmissionRefusal{GoalID: id, GoalRevision: revision, Breaches: breaches}
	}
	return verdict, nil
}

// budgetAdmissionBreaches uses admission boundaries rather than health's
// corrupt-over-limit boundaries. A proposal may fill attempt or minute
// capacity, and an active set may equal its concurrency limit; once current
// spending is at that boundary, another reservation is refused.
func budgetAdmissionBreaches(projection BudgetProjection) []BudgetBreach {
	var breaches []BudgetBreach
	if projection.Elapsed >= projection.Limits.ElapsedDuration() {
		state := projection.ElapsedState
		if state == "" {
			state = AdmissionClosedElapsed
		}
		limit := projection.Limits.ElapsedLimit
		if state == ElapsedBreach && projection.ElapsedBreachLimit > 0 {
			limit = projection.ElapsedBreachLimit.String()
		}
		breaches = append(breaches, BudgetBreach{
			Field: "elapsedLimit", Used: projection.Elapsed.Round(time.Second).String(), Limit: limit, State: state,
		})
	}
	if projection.Attempts >= projection.Limits.AttemptLimit {
		breaches = append(breaches, budgetIntegerBreach("attemptLimit", projection.Attempts, projection.Limits.AttemptLimit))
	}
	if projection.ReservedJobMinutes >= projection.Limits.ReservedJobMinutesLimit {
		breaches = append(breaches, budgetIntegerBreach("reservedJobMinutesLimit", projection.ReservedJobMinutes, projection.Limits.ReservedJobMinutesLimit))
	}
	if projection.ActiveJobs >= projection.Limits.ActiveJobLimit {
		breaches = append(breaches, budgetIntegerBreach("activeJobLimit", projection.ActiveJobs, projection.Limits.ActiveJobLimit))
	}
	return breaches
}

// FormatGoalAdmission renders stable, human-readable evidence without making
// callers parse prose to decide the exit status.
func FormatGoalAdmission(verdict GoalAdmissionVerdict) []string {
	lines := make([]string, 0, len(verdict.Refusals))
	for _, refusal := range verdict.Refusals {
		if refusal.Unknown != nil {
			lines = append(lines, fmt.Sprintf("BUDGET_UNKNOWN record=%s goal=%s revision=%d reason=%s",
				refusal.Unknown.Record, refusal.GoalID, refusal.GoalRevision, refusal.Unknown.Reason))
			continue
		}
		fields := make([]string, 0, len(refusal.Breaches))
		for _, breach := range refusal.Breaches {
			state := ""
			if breach.State != "" {
				state = " state=" + string(breach.State)
			}
			fields = append(fields, fmt.Sprintf("%s%s used=%s limit=%s", breach.Field, state, breach.Used, breach.Limit))
		}
		lines = append(lines, fmt.Sprintf("BUDGET_REFUSED: goal %s revision=%d admission closed: %s",
			refusal.GoalID, refusal.GoalRevision, strings.Join(fields, ", ")))
	}
	return lines
}
