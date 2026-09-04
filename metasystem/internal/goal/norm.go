package goal

import (
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
	return 0, 0, 0, false, false, nil
}

func refuseGoalNorm(id string, budget, box Budget) error {
	return fmt.Errorf("GOAL_NORM_REFUSED: goal %s tuple minutes=%d reviewRounds=%d exceeds its tier box minutes=%d reviewRounds=%d; split it into an arc of members within the box, or record the human word and pass --approved-ref (strict form: goal=%s minutes=%d reviewRounds=%d goalRevision=<r>)",
		id, budget.ReservedJobMinutesLimit, budget.ReviewRoundLimit, box.ReservedJobMinutesLimit, box.ReviewRoundLimit, id, budget.ReservedJobMinutesLimit, budget.ReviewRoundLimit)
}

func goalNormApproval(repoRoot string, tree *TreeGoals, file *GoalFile, budget Budget, approvedRef string, proof *humanauthority.Proof) (*GoalNormApprovalClaim, error) {
	tier := file.Tier
	if tier == 0 {
		tier = 3
	}
	box, err := config.TierBox(filepath.Join(repoRoot, "metasystem.conf"), tier)
	if err != nil {
		return nil, err
	}
	if approvedRef != strings.TrimSpace(approvedRef) {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: --approved-ref must exactly name a rulings-register row or human goal-history operation")
	}
	if approvedRef == "" {
		if budget.ReservedJobMinutesLimit > box.ReservedJobMinutesLimit || budget.ReviewRoundLimit > box.ReviewRoundLimit {
			if proof != nil && proof.ChannelWordFor(repoRoot) && proof.Outcome == humanauthority.OutcomeVerifiedChannel {
				return &GoalNormApprovalClaim{ApprovedRef: proof.ChannelContext, Minutes: budget.ReservedJobMinutesLimit, ReviewRounds: budget.ReviewRoundLimit, GoalRevision: file.Revision}, nil
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
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: --approved-ref %s does not name a rulings-register row or human goal-history operation", approvedRef)
	}
	if !proven {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: --approved-ref %s must approve this goal with the exact token form goal=<id> minutes=<n> reviewRounds=<n> goalRevision=<r>", approvedRef)
	}
	if revision != file.Revision {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: --approved-ref %s covers goal %s revision %d, not current revision %d; re-approval is required", approvedRef, file.Id, revision, file.Revision)
	}
	if minutes < budget.ReservedJobMinutesLimit {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: reservedJobMinutesLimit %dm exceeds --approved-ref %s's proven %dm", budget.ReservedJobMinutesLimit, approvedRef, minutes)
	}
	if rounds < budget.ReviewRoundLimit {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: reviewRoundLimit %d exceeds --approved-ref %s's proven %d", budget.ReviewRoundLimit, approvedRef, rounds)
	}
	if budget.ReservedJobMinutesLimit <= box.ReservedJobMinutesLimit && budget.ReviewRoundLimit <= box.ReviewRoundLimit {
		return nil, nil
	}
	return &GoalNormApprovalClaim{ApprovedRef: approvedRef, Minutes: minutes, ReviewRounds: rounds, GoalRevision: revision}, nil
}

func requireWithinGoalNorm(repoRoot string, tier uint8, budget Budget, id, context string) error {
	if tier == 0 {
		tier = 3
	}
	box, err := config.TierBox(filepath.Join(repoRoot, "metasystem.conf"), tier)
	if err != nil {
		return err
	}
	if budget.ReservedJobMinutesLimit <= box.ReservedJobMinutesLimit && budget.ReviewRoundLimit <= box.ReviewRoundLimit {
		return nil
	}
	if context != "" {
		return fmt.Errorf("GOAL_NORM_REFUSED: goal %s %s with an over-norm tuple; set-budget it within the norm first, or rejoin after release", id, context)
	}
	return refuseGoalNorm(id, budget, box)
}

func sameGoalNormApproval(left, right *GoalNormApprovalClaim) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
