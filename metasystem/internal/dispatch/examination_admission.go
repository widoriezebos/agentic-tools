package dispatch

import (
	"fmt"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// ReservedUnknownExaminationRetry checks the exact completed examination
// whose owner reserved the chain's single fresh read. The exemption belongs
// to that examination, never to an arbitrary follow-up in the chain.
func ReservedUnknownExaminationRetry(repoRoot, jobID string, custody CustodyDeathDependencies) error {
	_, err := reservedUnknownExaminationRetry(repoRoot, jobID, custody)
	return err
}

func reservedUnknownExaminationRetry(repoRoot, jobID string, custody CustodyDeathDependencies) (map[string]any, error) {
	state := loadCritiqueState(repoRoot)
	prior := state.records[jobID]
	root := state.records[state.chainRoot(jobID)]
	if prior == nil || asString(prior["jobId"]) != jobID || asString(prior["role"]) != "code-critic" ||
		asString(prior["status"]) != "completed" || asString(root["unknownExaminationRetryFrom"]) != jobID {
		return nil, fmt.Errorf("the fresh examination has no matching reserved source read")
	}
	if asString(prior["goalId"]) != asString(root["goalId"]) || fmt.Sprint(prior["goalRevision"]) != fmt.Sprint(root["goalRevision"]) ||
		asString(prior["reviews"]) != asString(root["reviews"]) {
		return nil, fmt.Errorf("the reserved examination does not match its chain's goal and subject")
	}
	if _, err := CollectExamination(repoRoot, jobID); err == nil {
		return nil, fmt.Errorf("the reserved source has readable stop inputs and must be decided")
	}
	if prior["pid"] != nil && asString(prior["groupDeathProvenAt"]) == "" {
		if death := ProveCustodyDeath(repoRoot, prior, custody); death.Outcome != CustodyDeathProven {
			return nil, quiescenceRefusal("the reserved source examination", prior, death)
		}
	}
	return prior, nil
}

// quiescenceRefusal names the process and group to end. No metasystem
// command ends them: a stop of a completed job changes nothing.
func quiescenceRefusal(subject string, record map[string]any, death CustodyDeathResult) error {
	pgid, grouped := numInt(record["pgid"])
	grouped = grouped && pgid > 1
	if death.Outcome == CustodyDeathAlive && death.Process != nil {
		end := fmt.Sprintf("end process %d", death.Process.Pid)
		if grouped {
			end += fmt.Sprintf(" and its process group %d", pgid)
		}
		return fmt.Errorf("%s is not proven quiescent: the examination's process %d is still running (%s);\n%s, then retry", subject, death.Process.Pid, death.Reason, end)
	}
	if grouped {
		return fmt.Errorf("%s is not proven quiescent (%s); if its process group %d still runs, end it, then retry", subject, death.Reason, pgid)
	}
	return fmt.Errorf("%s is not proven quiescent (%s)", subject, death.Reason)
}

// EvaluateUnknownExaminationRetryAdmission retains the exact goal fence,
// claim clock, active-job ceiling and hazard checks while admitting the
// reserved read without another work attempt or minute reservation.
func EvaluateUnknownExaminationRetryAdmission(repoRoot, jobID, id string, revision, proposedCap uint64,
	now time.Time, reads ProofAdmissionReads, custody CustodyDeathDependencies, hazards ...HazardClass) (GoalRevisionAdmission, error) {
	if err := reads.Validate(); err != nil {
		return GoalRevisionAdmission{}, err
	}
	prior, err := reservedUnknownExaminationRetry(repoRoot, jobID, custody)
	if err != nil {
		return GoalRevisionAdmission{}, err
	}
	priorRevision, _ := numInt(prior["goalRevision"])
	if asString(prior["goalId"]) != id || priorRevision < 1 || uint64(priorRevision) != revision {
		return GoalRevisionAdmission{}, fmt.Errorf("the fresh examination does not belong to this goal revision")
	}
	return evaluateGoalRevisionAdmissionForDispatchWithReads(repoRoot, id, revision, proposedCap, now,
		"code-critic", "follow-up", authorityBudgetMembers, reads.private(), hazards...)
}

// EvaluateGoalAdmissionForUnknownExaminationRetry preserves every other
// claimed goal's admission and the retry goal's clock and active-job limits.
func EvaluateGoalAdmissionForUnknownExaminationRetry(repoRoot, stopLineage, jobID string, now time.Time, custody CustodyDeathDependencies) (GoalAdmissionVerdict, error) {
	return evaluateGoalAdmissionForUnknownExaminationRetryWithReads(repoRoot, stopLineage, jobID, now, concreteGoalAdmissionReads(), custody)
}

func evaluateGoalAdmissionForUnknownExaminationRetryWithReads(repoRoot, stopLineage, jobID string, now time.Time, reads goalAdmissionReads, custody CustodyDeathDependencies) (GoalAdmissionVerdict, error) {
	prior, err := reservedUnknownExaminationRetry(repoRoot, jobID, custody)
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
