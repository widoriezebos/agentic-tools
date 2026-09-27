package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The fixture's abandon stands in for the engine's, and a fixture that accepts
// what the engine refuses teaches a human the wrong rule on the one card whose
// act cannot be undone. These two are the rules the engine holds before it
// writes anything: the reason is on one line (internal/goal/abandon.go), and the
// tree the act would leave carries no blockedBy cycle (internal/goal/validate.go).

// A reason with a line break in it is the engine's own refusal, in the engine's
// own sentence — the act layer only checks that a reason is there
// (internal/ui/act/act.go Abandon), so the one-line rule is this ledger's to
// hold, as it is the engine's.
func TestTheFixtureAbandonRefusesAReasonThatIsNotOnOneLine(t *testing.T) {
	t.Parallel()

	state := newLedger(false)

	err := state.abandon(proposedAbandon, "overtaken by g1-s44\nand nobody will pick it up", "")

	testutil.Expect(t, "the engine's own words", refusalText(err),
		"abandon needs its reason on one line; a goal that will never be worked owes the reader why")
	testutil.Expect(t, "the goal is still live", state.tree.Live[proposedAbandon] != nil, true)
}

// A successor that is itself one of the goal's live dependents would be
// repointed at itself, and the engine refuses the commit that tree would make
// rather than writing it. So does this ledger, before it repoints anything: the
// dependent must not end up waiting for itself.
func TestTheFixtureAbandonRefusesASuccessorThatWaitsForTheAbandonedGoal(t *testing.T) {
	t.Parallel()

	state := newLedger(false)
	testutil.Require(t, "the dependent waits for the goal before the act",
		strings.Join(state.tree.Live["g1-s26"].Blocked, ","), "g1-s23")

	err := state.abandon("g1-s23", "the census format is decided elsewhere", "g1-s26")

	testutil.Expect(t, "the engine's own words", refusalText(err),
		"the ledger tree this abandon would make does not validate:\nblockedBy cycle: g1-s26 -> g1-s26")
	testutil.Expect(t, "the goal is still live", state.tree.Live["g1-s23"] != nil, true)
	testutil.Expect(t, "the dependent does not block itself",
		strings.Join(state.tree.Live["g1-s26"].Blocked, ","), "g1-s23")
}

// refusalText is what a refusal says, and the empty string where the ledger
// said yes — so one run reports both the missing refusal and what the fixture
// did instead of refusing.
func refusalText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
