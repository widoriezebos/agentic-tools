package act

// How the registry spells a goal.
//
// The registration is keyed by the goal id as the route spelled it, and the
// engine trims the id before it publishes (internal/goal/verbs.go Block and
// Unblock). So " g " and "g" are one goal to the ledger and two registrations
// here, and the second press the registry exists to refuse got through: two
// tabs on one card, one of them with space around the id, both publishing
// (Astra E-05).
//
// The id is therefore normalised in the one place the registration is made.

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The registration is the goal the LEDGER means, not the goal as a route
// spelled it: the engine trims an id before it publishes, so a second press
// carrying the same id with space around it is the same act on the same goal
// and is refused as one.
func TestTheHoldIsKeyedByTheGoalTheLedgerMeans(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-spelled")
	held := ownerOf(bed.root)

	clear, err := held.begin("ui-spelled", "goal block")
	if err != nil {
		t.Fatalf("the first press: %v", err)
	}
	defer clear()

	for _, spelling := range []string{" ui-spelled", "ui-spelled ", "  ui-spelled  ", "\tui-spelled\n"} {
		second, again := held.begin(spelling, "goal block")
		if again == nil {
			second()
			t.Fatalf("a second press spelled %q was admitted beside the act in flight", spelling)
		}
		refusal := refusalOf(t, again)
		testutil.Expect(t, "the refusal "+spelling+" meets", refusal.Code, "in-flight")
	}
}

// And the whole way through: two block presses on one goal, one of them spelled
// with space around the id, exclude each other exactly as two identical ones do.
func TestASecondBlockSpelledWithSpaceIsTheActAlreadyRunning(t *testing.T) {
	t.Parallel()
	bed := ledger(t)
	openGoal(t, bed, "ui-edge-waits")
	openGoal(t, bed, "ui-edge-blocker")
	authority := sessionFor(t, bed)
	held := ownerOf(bed.root)

	clear, err := held.begin("ui-edge-waits", "goal block")
	if err != nil {
		t.Fatalf("the act this test holds: %v", err)
	}
	defer clear()

	err = authority.Block(" ui-edge-waits ", "ui-edge-blocker")
	refusal := refusalOf(t, err)
	testutil.Expect(t, "the press is refused as the act already running", refusal.Code, "in-flight")
	if !strings.Contains(refusal.Message, "another press") {
		t.Fatalf("the refusal reads %q", refusal.Message)
	}
}
