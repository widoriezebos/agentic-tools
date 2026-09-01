package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

var goalNormRulingRef = regexp.MustCompile(`^R-[0-9]+(?:[a-z]|-[a-z0-9][a-z0-9-]*)?$`)

// StrictApprovalTriple recognizes exactly one positive approval triple for a
// named goal. The token includes its equals sign, for example "minutes=";
// this keeps goal-scope approval distinct from the slice capMin law.
func StrictApprovalTriple(text, goalID, token string) (uint64, uint64, bool) {
	type coordinates struct {
		value    uint64
		revision uint64
	}
	var match coordinates
	matchCount := 0
	fields := strings.Fields(text)
	for index := 0; index+2 < len(fields); index++ {
		goal, found := strings.CutPrefix(fields[index], "goal=")
		if !found || goal != goalID {
			continue
		}
		valueText, valueFound := strings.CutPrefix(fields[index+1], token)
		revisionText, revisionFound := strings.CutPrefix(fields[index+2], "goalRevision=")
		if !valueFound || !revisionFound {
			continue
		}
		value, valueErr := strconv.ParseUint(valueText, 10, 64)
		revision, revisionErr := strconv.ParseUint(revisionText, 10, 64)
		if valueErr != nil || revisionErr != nil || value == 0 || revision == 0 {
			continue
		}
		match = coordinates{value: value, revision: revision}
		matchCount++
	}
	if matchCount != 1 {
		return 0, 0, false
	}
	return match.value, match.revision, true
}

// RecordedNormApproval searches the two durable human-word channels against
// the transaction tip that is about to be changed.
func RecordedNormApproval(repoRoot string, tree *TreeGoals, ref, goalID string) (minutes, revision uint64, exists, proven bool, err error) {
	if goalNormRulingRef.MatchString(ref) {
		data, readErr := os.ReadFile(filepath.Join(repoRoot, "memory", "rulings.md"))
		if os.IsNotExist(readErr) {
			return 0, 0, false, false, nil
		}
		if readErr != nil {
			return 0, 0, false, false, fmt.Errorf("read rulings register for --approved-ref: %w", readErr)
		}
		needle := "| " + ref + " |"
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), needle) {
				minutes, revision, proven = StrictApprovalTriple(line, goalID, "minutes=")
				return minutes, revision, true, proven, nil
			}
		}
		return 0, 0, false, false, nil
	}
	contains := func(file *GoalFile) (uint64, uint64, bool, bool) {
		for _, line := range file.History {
			if line.Opid == ref && strings.HasPrefix(line.Actor, "human:") {
				minutes, revision, proven := StrictApprovalTriple(line.Reason, goalID, "minutes=")
				return minutes, revision, true, proven
			}
		}
		return 0, 0, false, false
	}
	for _, file := range tree.Live {
		if minutes, revision, exists, proven := contains(file); exists {
			return minutes, revision, true, proven, nil
		}
	}
	for _, file := range tree.Done {
		if minutes, revision, exists, proven := contains(file); exists {
			return minutes, revision, true, proven, nil
		}
	}
	return 0, 0, false, false, nil
}

func refuseGoalNorm(id string, reserved, norm uint64) error {
	return fmt.Errorf("GOAL_NORM_REFUSED: goal %s reservedJobMinutesLimit %dm exceeds the %dm goal norm (%s); split it into an arc of members each within the norm (goal split --id %s --members <draft.md>), or record the human word and pass --approved-ref (strict form: goal=%s minutes=%d goalRevision=<r>)",
		id, reserved, norm, config.GoalNormJobMinutesKey, id, id, reserved)
}

func goalNormApproval(repoRoot string, tree *TreeGoals, file *GoalFile, budget Budget, approvedRef string) (*GoalNormApprovalClaim, error) {
	norm, err := config.GoalNormJobMinutes(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return nil, err
	}
	if approvedRef != strings.TrimSpace(approvedRef) {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: --approved-ref must exactly name a rulings-register row or human goal-history operation")
	}
	if approvedRef == "" {
		if budget.ReservedJobMinutesLimit > norm {
			return nil, refuseGoalNorm(file.Id, budget.ReservedJobMinutesLimit, norm)
		}
		return nil, nil
	}
	minutes, revision, exists, proven, err := RecordedNormApproval(repoRoot, tree, approvedRef, file.Id)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: --approved-ref %s does not name a rulings-register row or human goal-history operation", approvedRef)
	}
	if !proven {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: --approved-ref %s must approve this goal with the exact token form goal=<id> minutes=<n> goalRevision=<r>", approvedRef)
	}
	if revision != file.Revision {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: --approved-ref %s covers goal %s revision %d, not current revision %d; re-approval is required", approvedRef, file.Id, revision, file.Revision)
	}
	if minutes < budget.ReservedJobMinutesLimit {
		return nil, fmt.Errorf("GOAL_NORM_REFUSED: reservedJobMinutesLimit %dm exceeds --approved-ref %s's proven %dm", budget.ReservedJobMinutesLimit, approvedRef, minutes)
	}
	if budget.ReservedJobMinutesLimit <= norm {
		return nil, nil
	}
	return &GoalNormApprovalClaim{ApprovedRef: approvedRef, Minutes: minutes, GoalRevision: revision}, nil
}

func requireWithinGoalNorm(repoRoot string, budget Budget, id, context string) error {
	norm, err := config.GoalNormJobMinutes(filepath.Join(repoRoot, "metasystem.conf"))
	if err != nil {
		return err
	}
	if budget.ReservedJobMinutesLimit <= norm {
		return nil
	}
	if context != "" {
		return fmt.Errorf("GOAL_NORM_REFUSED: goal %s %s with an over-norm tuple; set-budget it within the norm first, or rejoin after release", id, context)
	}
	return refuseGoalNorm(id, budget.ReservedJobMinutesLimit, norm)
}

func sameGoalNormApproval(left, right *GoalNormApprovalClaim) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}
