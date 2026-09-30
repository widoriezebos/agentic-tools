package main

import "fmt"

// The admission refusals in this file are machine protocol as well as text:
// a refused test run exits with proofrun.ExitAdmissionRefused and leaves the
// refusal on its output, where the landing owner reads the leading code
// (internal/landing/batchowner batchAdmissionRefusalCode, laneHold) and, for
// CANDIDATE_GOAL_REFUSED, the "state=" word. That code and word stay first
// and byte-identical, so this file is the one place in the command where a
// code leads a line a person may read; the words after it are plain.

func proofAdmissionMoved(before, after proofAdmissionSnapshots, candidateID, authorityID string) error {
	if before == after {
		return nil
	}
	return fmt.Errorf("CANDIDATE_GOAL_MOVED: goals %s (revision %d to %d) and %s (revision %d to %d) changed while the test run was admitted; start it again",
		candidateID, before.Candidate.Revision, after.Candidate.Revision,
		authorityID, before.Authority.Revision, after.Authority.Revision)
}

func candidateGoalRefusal(id, state, detail string) error {
	if detail != "" {
		detail = " " + detail
	}
	return fmt.Errorf("CANDIDATE_GOAL_REFUSED: candidate goal %s state=%s%s", id, state, detail)
}

func proofAuthorityRefusal(id, detail string) error {
	return fmt.Errorf("PROOF_AUTHORITY_REQUIRED: authority goal %s %s", id, detail)
}

// arcMateRefusal refuses charging a goal's test run to another goal of the
// same arc: the candidate is claimed and charged to itself.
func arcMateRefusal(candidate, authority, arc string) error {
	return fmt.Errorf("PROOF_AUTHORITY_ARC_MATE_REFUSED: goals %s and %s share arc %s; claim %s to charge its test run to itself",
		candidate, authority, arc, candidate)
}

// goalRevisionMoved refuses a test run sealed on goal revisions that have
// moved since.
func goalRevisionMoved(expectedGoal, expectedAccounting, goalRevision, accountingRevision uint64) error {
	return fmt.Errorf("GOAL_REVISION_MOVED: the goal moved since the batch was sealed (revisions %d/%d, now %d/%d)",
		expectedGoal, expectedAccounting, goalRevision, accountingRevision)
}

// batchMemberBudgetRefused refuses a batch member whose budget has no room
// for the diagnostic run beside its own; a person raises the budget.
func batchMemberBudgetRefused(goalID string, minutes uint64) error {
	return fmt.Errorf("BATCH_MEMBER_BUDGET_REFUSED: goal %s needs room for two more test runs and %d reserved minutes, beyond its approved budget; "+
		"a person raises it with metasystem goal budget %s BOX", goalID, minutes, goalID)
}

// laneAccountUnresolved refuses a test run charged to a landing lane that
// cannot be named.
func laneAccountUnresolved(format string, args ...any) error {
	return fmt.Errorf("LANE_ACCOUNT_UNRESOLVED: "+format, args...)
}
