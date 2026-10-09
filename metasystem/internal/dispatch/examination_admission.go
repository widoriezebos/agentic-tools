package dispatch

import (
	"fmt"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ReservedUnknownExaminationRetry checks the exact terminal examination
// whose owner reserved the chain's single fresh read. The exemption belongs
// to that examination, never to an arbitrary follow-up in the chain.
func ReservedUnknownExaminationRetry(repoRoot, jobID string) error {
	_, err := reservedUnknownExaminationRetry(repoRoot, jobID)
	return err
}

func reservedUnknownExaminationRetry(repoRoot, jobID string) (map[string]any, error) {
	state := loadCritiqueState(repoRoot)
	prior := state.records[jobID]
	root := state.records[state.chainRoot(jobID)]
	role := asString(prior["role"])
	if prior == nil || asString(prior["jobId"]) != jobID || (role != "code-critic" && role != "design-critic") || role != asString(root["role"]) ||
		(asString(prior["status"]) != "completed" && !(role == "design-critic" && asString(prior["status"]) == "failed")) || asString(root["unknownExaminationRetryFrom"]) != jobID {
		return nil, fmt.Errorf("the fresh examination has no matching reserved source read")
	}
	if asString(prior["goalId"]) != asString(root["goalId"]) || fmt.Sprint(prior["goalRevision"]) != fmt.Sprint(root["goalRevision"]) ||
		asString(prior["reviews"]) != asString(root["reviews"]) {
		return nil, fmt.Errorf("the reserved examination does not match its chain's goal and subject")
	}
	if _, err := CollectExamination(repoRoot, jobID); err == nil {
		return nil, fmt.Errorf("the reserved source has readable stop inputs and must be decided")
	}
	if prior["pid"] != nil && asString(prior["groupDeathProvenAt"]) == "" &&
		ProveCustodyDeath(repoRoot, prior, CustodyDeathDependencies{}).Outcome != CustodyDeathProven {
		return nil, fmt.Errorf("the reserved source examination is not proven quiescent")
	}
	return prior, nil
}

// EvaluateUnknownExaminationRetryAdmission retains the exact goal fence,
// claim clock, active-job ceiling and hazard checks while admitting the
// reserved read without another work attempt or minute reservation.
func EvaluateUnknownExaminationRetryAdmission(repoRoot, jobID, id string, revision, proposedCap uint64,
	now time.Time, reads ProofAdmissionReads, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	if err := reads.Validate(); err != nil {
		return GoalRevisionAdmission{}, err
	}
	prior, err := reservedUnknownExaminationRetry(repoRoot, jobID)
	if err != nil {
		return GoalRevisionAdmission{}, err
	}
	priorRevision, _ := numInt(prior["goalRevision"])
	if asString(prior["goalId"]) != id || priorRevision < 1 || uint64(priorRevision) != revision {
		return GoalRevisionAdmission{}, fmt.Errorf("the fresh examination does not belong to this goal revision")
	}
	return evaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, id, revision, proposedCap, now,
		asString(prior["role"]), "follow-up", authorityBudgetMembers, reads.private(), hazards...)
}

// EvaluateGoalAdmissionForUnknownExaminationRetry preserves every other
// claimed goal's admission and the retry goal's clock and active-job limits.
func EvaluateGoalAdmissionForUnknownExaminationRetry(repoRoot, stopLineage, jobID string, now time.Time) (GoalAdmissionVerdict, error) {
	return evaluateGoalAdmissionForUnknownExaminationRetryWithReads(repoRoot, stopLineage, jobID, now, concreteGoalAdmissionReads())
}

func evaluateGoalAdmissionForUnknownExaminationRetryWithReads(repoRoot, stopLineage, jobID string, now time.Time, reads goalAdmissionReads) (GoalAdmissionVerdict, error) {
	prior, err := reservedUnknownExaminationRetry(repoRoot, jobID)
	if err != nil {
		return GoalAdmissionVerdict{}, err
	}
	verdict, err := evaluateGoalAdmissionWithReads(repoRoot, stopLineage, now, reads)
	if err != nil {
		return verdict, err
	}
	revision, _ := numInt(prior["goalRevision"])
	kept := verdict.Refusals[:0]
	for _, refusal := range verdict.Refusals {
		if refusal.GoalID == asString(prior["goalId"]) && revision > 0 && refusal.GoalRevision == uint64(revision) && refusal.Unknown == nil {
			refusal.Breaches = budgetBreachesForLens(refusal.Breaches, authorityBudgetMembers)
			if len(refusal.Breaches) == 0 {
				continue
			}
			refusal.LiveStopReason = ""
			for _, breach := range refusal.Breaches {
				if breach.Field == "elapsedLimit" && breach.State == ElapsedBreach {
					refusal.LiveStopReason = goal.StopReasonElapsedLimit
				} else if used, err := strconv.ParseUint(breach.Used, 10, 64); err == nil {
					limit, _ := strconv.ParseUint(breach.Limit, 10, 64)
					if used > limit {
						refusal.LiveStopReason = goal.StopReasonCorruptOverLimit
					}
				}
			}
		}
		kept = append(kept, refusal)
	}
	verdict.Refusals = kept
	return verdict, nil
}
