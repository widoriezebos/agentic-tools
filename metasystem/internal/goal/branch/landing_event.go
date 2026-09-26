package branch

import (
	"context"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func init() {
	goal.RegisterGoalBranchLandingVerifier(verifiedLastLanding)
}

// verifiedLastLanding recognizes the commit that lands a goal's last unit
// from its goal branch: Goal-Last names the goal, the manifest names the same
// goal, and the landed entries digest to the recorded Goal-Digest. A commit
// without a landing manifest is not a landing; a manifest whose digest does
// not verify is an error rather than a match.
func verifiedLastLanding(ctx context.Context, root, commit, goalID string) (bool, string, error) {
	if err := ctx.Err(); err != nil {
		return false, "", err
	}
	if !IsLastLanding(root, commit, goalID) {
		return false, "", nil
	}
	if manifestGoal, _, _, _, err := landedManifestWithGit(root, commit, gitOutput); err != nil || manifestGoal != goalID {
		return false, "", nil
	}
	verified, err := VerifyLanded(root, commit)
	if err != nil {
		return false, "", fmt.Errorf("goal %s landing %s does not verify: %w", goalID, commit, err)
	}
	return true, fmt.Sprintf("goal-branch goal=%s units=%s digest=%s", verified.Goal, verified.Units, verified.Actual), nil
}
