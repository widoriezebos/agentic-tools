package goal

import (
	"strings"
	"testing"
)

func TestForeignDoneNamesResolvedAgentAndMissingAuthority(t *testing.T) {
	t.Parallel()
	owner, caller := fakeGoalEndpointPair(t)
	if result, err := Open(verbReqFor(owner, "01J5X00000000000000000FD00", "mac-a"), "foreign-done", "Finish the work.", "main", "Build it."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", result, err)
	}
	if result, err := claimApprovedForTest(t, verbReqFor(owner, "01J5X00000000000000000FD10", "mac-a"), "foreign-done", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", result, err)
	}
	request := verbReqFor(caller, "01J5X00000000000000000FD20", "mac-b")
	request.Actor.Lineage = "agent-session"
	result, err := Done(request, "foreign-done", "The work exists.")
	if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "claimed by mac-a+lin-1") || !strings.Contains(result.Detail, "caller resolved to agent mac-b+agent-session because no person's authority was proven") {
		t.Fatalf("foreign caller's authority refusal: %+v %v", result, err)
	}
}
