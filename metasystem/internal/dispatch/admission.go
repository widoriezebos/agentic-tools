package dispatch

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// GoalAdmissionRefusal is one claim whose structured budget closes
// the dispatch admission seam. Unknown evidence closes admission without
// inventing counters; known evidence names every exhausted limit.
type GoalAdmissionRefusal struct {
	GoalID       string
	GoalRevision uint64
	Breaches     []BudgetBreach
	Unknown      *BudgetUnknownEvidence
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
			})
		}
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
		breaches = append(breaches, BudgetBreach{
			Field: "elapsedLimit", Used: projection.Elapsed.Round(time.Second).String(), Limit: projection.Limits.ElapsedLimit,
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
			fields = append(fields, fmt.Sprintf("%s used=%s limit=%s", breach.Field, breach.Used, breach.Limit))
		}
		lines = append(lines, fmt.Sprintf("BUDGET_REFUSED: goal %s revision=%d admission closed: %s",
			refusal.GoalID, refusal.GoalRevision, strings.Join(fields, ", ")))
	}
	return lines
}
