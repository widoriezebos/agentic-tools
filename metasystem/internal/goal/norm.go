package goal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

var goalNormRulingRef = regexp.MustCompile(`^R-[0-9]+(?:[a-z]|-[a-z0-9][a-z0-9-]*)?$`)

// StrictApprovalQuadruple recognizes exactly one goal budget approval token.
func StrictApprovalQuadruple(text, goalID string) (uint64, int64, uint64, bool) {
	type coordinates struct {
		minutes  uint64
		rounds   int64
		revision uint64
	}
	var match coordinates
	matchCount := 0
	fields := strings.Fields(text)
	for index := 0; index+3 < len(fields); index++ {
		goal, found := strings.CutPrefix(fields[index], "goal=")
		if !found || goal != goalID {
			continue
		}
		minutesText, minutesFound := strings.CutPrefix(fields[index+1], "minutes=")
		roundsText, roundsFound := strings.CutPrefix(fields[index+2], "reviewRounds=")
		revisionText, revisionFound := strings.CutPrefix(fields[index+3], "goalRevision=")
		if !minutesFound || !roundsFound || !revisionFound {
			continue
		}
		minutes, minutesErr := strconv.ParseUint(minutesText, 10, 64)
		rounds, roundsErr := strconv.ParseInt(roundsText, 10, 64)
		revision, revisionErr := strconv.ParseUint(revisionText, 10, 64)
		if minutesErr != nil || roundsErr != nil || revisionErr != nil || minutes == 0 || rounds < 0 || revision == 0 {
			continue
		}
		match = coordinates{minutes: minutes, rounds: rounds, revision: revision}
		matchCount++
	}
	if matchCount != 1 {
		return 0, 0, 0, false
	}
	return match.minutes, match.rounds, match.revision, true
}

// RecordedNormApproval searches the two durable human-word channels against
// the transaction tip that is about to be changed.
func RecordedNormApproval(repoRoot string, tree *TreeGoals, ref, goalID string) (minutes uint64, rounds int64, revision uint64, exists, proven bool, err error) {
	if goalNormRulingRef.MatchString(ref) {
		data, readErr := os.ReadFile(filepath.Join(repoRoot, "memory", "rulings.md"))
		if os.IsNotExist(readErr) {
			return 0, 0, 0, false, false, nil
		}
		if readErr != nil {
			return 0, 0, 0, false, false, fmt.Errorf("read rulings register for --approved-ref: %w", readErr)
		}
		needle := "| " + ref + " |"
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), needle) {
				minutes, rounds, revision, proven = StrictApprovalQuadruple(line, goalID)
				return minutes, rounds, revision, true, proven, nil
			}
		}
		return 0, 0, 0, false, false, nil
	}
	contains := func(file *GoalFile) (uint64, int64, uint64, bool, bool) {
		for _, line := range file.History {
			if line.Opid == ref && strings.HasPrefix(line.Actor, "human:") {
				minutes, rounds, revision, proven := StrictApprovalQuadruple(line.Reason, goalID)
				return minutes, rounds, revision, true, proven
			}
		}
		return 0, 0, 0, false, false
	}
	for _, file := range tree.Live {
		if minutes, rounds, revision, exists, proven := contains(file); exists {
			return minutes, rounds, revision, true, proven, nil
		}
	}
	for _, file := range tree.Done {
		if minutes, rounds, revision, exists, proven := contains(file); exists {
			return minutes, rounds, revision, true, proven, nil
		}
	}
	for _, file := range tree.Abandoned {
		if minutes, rounds, revision, exists, proven := contains(file); exists {
			return minutes, rounds, revision, true, proven, nil
		}
	}
	return 0, 0, 0, false, false, nil
}

func refuseGoalNorm(id string, budget, box Budget) error {
	return coded("GOAL_NORM_REFUSED", fmt.Errorf("goal %s asks for %dm and %d review rounds, over its tier's %dm and %d; split it, or pass --approved-ref",
		id, budget.ReservedJobMinutesLimit, budget.ReviewRoundLimit, box.ReservedJobMinutesLimit, box.ReviewRoundLimit))
}

func goalNormApproval(repoRoot string, tree *TreeGoals, file *GoalFile, budget Budget, approvedRef, operationID string, proof *humanauthority.Proof) (*GoalNormApprovalClaim, error) {
	tier := file.Tier
	if tier == 0 {
		tier = 3
	}
	box, err := config.TierBox(filepath.Join(repoRoot, "metasystem.conf"), tier)
	if err != nil {
		return nil, err
	}
	if approvedRef != strings.TrimSpace(approvedRef) {
		return nil, coded("GOAL_NORM_REFUSED", errors.New("--approved-ref must name a ruling or a person's goal act exactly, without spaces"))
	}
	if approvedRef == "" {
		if budget.ReservedJobMinutesLimit > box.ReservedJobMinutesLimit || budget.ReviewRoundLimit > box.ReviewRoundLimit {
			if proof != nil && proof.ChannelWordFor(repoRoot) && proof.Outcome == humanauthority.OutcomeVerifiedChannel {
				return &GoalNormApprovalClaim{ApprovedRef: proof.ChannelContext, Minutes: budget.ReservedJobMinutesLimit, ReviewRounds: budget.ReviewRoundLimit, GoalRevision: file.Revision}, nil
			}
			if proof != nil && proof.EnrolledTerminalFor(repoRoot) && operationID != "" {
				return &GoalNormApprovalClaim{ApprovedRef: operationID, Minutes: budget.ReservedJobMinutesLimit, ReviewRounds: budget.ReviewRoundLimit, GoalRevision: file.Revision}, nil
			}
			return nil, refuseGoalNorm(file.Id, budget, box)
		}
		return nil, nil
	}
	minutes, rounds, revision, exists, proven, err := RecordedNormApproval(repoRoot, tree, approvedRef, file.Id)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, coded("GOAL_NORM_REFUSED", fmt.Errorf("--approved-ref %s names no ruling or person's goal act", approvedRef))
	}
	if !proven {
		return nil, coded("GOAL_NORM_REFUSED", fmt.Errorf("--approved-ref %s doesn't say goal=<id> minutes=<n> reviewRounds=<n> goalRevision=<r>", approvedRef))
	}
	if revision != file.Revision {
		return nil, coded("GOAL_NORM_REFUSED", fmt.Errorf("--approved-ref %s covers goal %s at revision %d, but it is now %d; approve it again", approvedRef, file.Id, revision, file.Revision))
	}
	if minutes < budget.ReservedJobMinutesLimit {
		return nil, coded("GOAL_NORM_REFUSED", fmt.Errorf("%dm of job time is more than --approved-ref %s approved (%dm)", budget.ReservedJobMinutesLimit, approvedRef, minutes))
	}
	if rounds < budget.ReviewRoundLimit {
		return nil, coded("GOAL_NORM_REFUSED", fmt.Errorf("%d review rounds are more than --approved-ref %s approved (%d)", budget.ReviewRoundLimit, approvedRef, rounds))
	}
	if budget.ReservedJobMinutesLimit <= box.ReservedJobMinutesLimit && budget.ReviewRoundLimit <= box.ReviewRoundLimit {
		return nil, nil
	}
	return &GoalNormApprovalClaim{ApprovedRef: approvedRef, Minutes: minutes, ReviewRounds: rounds, GoalRevision: revision}, nil
}

func budgetExceedsBox(budget, box Budget) bool {
	return budget.ElapsedDuration() > box.ElapsedDuration() || budget.AttemptLimit > box.AttemptLimit ||
		budget.ReservedJobMinutesLimit > box.ReservedJobMinutesLimit || budget.ActiveJobLimit > box.ActiveJobLimit ||
		budget.ReviewRoundLimit > box.ReviewRoundLimit
}

func sameGoalNormApproval(left, right *GoalNormApprovalClaim) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
