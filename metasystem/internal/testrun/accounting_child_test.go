package testrun

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The pinned policy child of a cadence preflight is asked with no goal (the
// preflight accounts to none), so it plans for no goal instead of refusing
// for want of a claimed one; a cadence child named a goal still accounts to it.
func TestCadencePolicyChildWithoutAGoalAccountsToNoGoal(t *testing.T) {
	t.Parallel()
	child := SelectionRequest{Root: "landing", Tree: "tree", Mode: testpolicy.ModeDeep, Purpose: testpolicy.PurposeCadence, PolicyChild: true}
	if accounts, err := accountsToGoal(child); err != nil || accounts {
		t.Fatalf("goal-less cadence policy child accounts-to-goal=%t err=%v", accounts, err)
	}
	child.GoalID = "standing-validation"
	if accounts, err := accountsToGoal(child); err != nil || !accounts {
		t.Fatalf("a cadence child named a goal accounts-to-goal=%t err=%v", accounts, err)
	}
	child.GoalID, child.PolicyChild = "", false
	if accounts, _ := accountsToGoal(child); !accounts {
		t.Fatal("an outer cadence run with no goal stopped accounting to one")
	}
}
