package goal

import "testing"

// TestClaimGoalActionNamesThePublicClaim: the turn facts hand an agent a
// ready goal to claim with a command it can run, the public goal claim,
// never an internal entry.
func TestClaimGoalActionNamesThePublicClaim(t *testing.T) {
	t.Parallel()
	selected := GoalFacts{Id: "goal-a", NextStep: "Run it."}
	actions := workActions(t.TempDir(), TurnWorkFacts{ReadSucceeded: true, Selected: &selected, Selection: "claimable"},
		TurnVerdictOptions{SeatActor: Actor{Machine: "mac-a"}})
	if len(actions) != 1 || actions[0].Kind != "claim-goal" || actions[0].Command != "metasystem goal claim goal-a" {
		t.Fatalf("claim action = %+v", actions)
	}
}
