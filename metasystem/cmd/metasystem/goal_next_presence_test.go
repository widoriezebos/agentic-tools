package main

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The silent-holder lines `goal next` prints to standard error: one per claim
// of another machine that has gone quiet, with the remedy that exists for
// that goal and no other. What a standing means is proved in internal/seat
// and internal/ui/fleet; this is the join and the words.

var presenceNow = time.Date(2026, 9, 25, 14, 37, 0, 0, time.UTC)

func presenceWindow() time.Duration { return seat.DefaultStaleMinutes * time.Minute }

func heldGoal(id, machine string) *goal.GoalFile {
	return &goal.GoalFile{
		Id: id, State: goal.StateClaimed, Intent: "Do " + id, Origin: "main",
		NextStep: "Start " + id + ".", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Claimed: &goal.ClaimRecord{Machine: machine, Lineage: "coordinator", At: "2026-09-25T08:00:00Z"},
	}
}

func presenceTree(files ...*goal.GoalFile) *goal.TreeGoals {
	tree := &goal.TreeGoals{
		Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	for _, file := range files {
		tree.Live[file.Id] = file
	}
	return tree
}

func presenceRecord(machine string, age time.Duration) seat.Record {
	return seat.Record{
		PresenceSchema: seat.RecordSchema, Machine: machine, RepoIdentity: "repo-" + machine,
		Generation: 4, Engine: "3f9c1e2", ArmedLineage: seat.NoLease, TickSeconds: 600,
		TickAt: seat.FormatTime(presenceNow.Add(-age)),
	}
}

func TestGoalNextFlagsASilentHolderAndNamesTheStealRemedy(t *testing.T) {
	t.Parallel()
	tree := presenceTree(heldGoal("tests-parallel-and-deterministic", "m1c"))
	copied := seat.Copy{
		Records:   map[string]seat.Record{"m1c": presenceRecord("m1c", 6*time.Hour)},
		Malformed: map[string]string{},
	}
	previous := map[string]seat.Observation{
		"m1c": {Standing: seat.Unreachable, Since: seat.FormatTime(presenceNow.Add(-5 * time.Hour))},
	}

	lines := silentHolders(tree, "m1u", copied, previous, presenceNow, presenceWindow())

	testutil.Expect(t, "one line per held goal", lines, []string{
		"goal tests-parallel-and-deterministic is held by m1c, unreachable since 09:37; " +
			"a human reassigns it with goal steal",
	})
}

// A machine named only by a claim has not gone silent: it has published
// nothing, which is said differently and has the same remedy.
func TestGoalNextSaysAMachineThatHasPublishedNothingDifferently(t *testing.T) {
	t.Parallel()
	tree := presenceTree(heldGoal("fleet-channel-gateway", "m0b"))
	copied := seat.Copy{Records: map[string]seat.Record{}, Malformed: map[string]string{}}

	lines := silentHolders(tree, "m1u", copied, map[string]seat.Observation{}, presenceNow, presenceWindow())

	testutil.Expect(t, "the words say what is actually known", lines, []string{
		"goal fleet-channel-gateway is held by m0b, which has published no presence; " +
			"a human reassigns it with goal steal",
	})
}

// The remedies are not interchangeable: steal refuses a fenced claim, and
// resume is what lifts a breach fence while keeping the owner.
func TestGoalNextNamesResumeForAFencedClaim(t *testing.T) {
	t.Parallel()
	fenced := heldGoal("fenced-work", "m1c")
	fenced.StopCapability = &goal.StopCapability{Generation: 1, Revision: 9, Machine: "m1c"}
	fenced.StopFence = &goal.StopFence{
		StopID: "S1", Revision: 9, Epoch: 1, CapabilityGeneration: 1,
		ClosedAt: "2026-09-25T09:10:00Z", Reason: goal.StopReasonElapsedLimit,
	}
	copied := seat.Copy{
		Records:   map[string]seat.Record{"m1c": presenceRecord("m1c", 6*time.Hour)},
		Malformed: map[string]string{},
	}
	previous := map[string]seat.Observation{
		"m1c": {Standing: seat.Unreachable, Since: seat.FormatTime(presenceNow.Add(-5 * time.Hour))},
	}

	lines := silentHolders(presenceTree(fenced), "m1u", copied, previous, presenceNow, presenceWindow())

	testutil.Expect(t, "a fenced claim is resumed and never stolen", lines, []string{
		"goal fenced-work is held by m1c, unreachable since 09:37; a human lifts the fence with goal resume",
	})
}

func TestGoalNextSaysNothingAboutThisMachineOrAReachableOne(t *testing.T) {
	t.Parallel()
	tree := presenceTree(heldGoal("mine", "m1u"), heldGoal("theirs", "m1e"))
	copied := seat.Copy{
		Records: map[string]seat.Record{
			"m1u": presenceRecord("m1u", 30*time.Second),
			"m1e": presenceRecord("m1e", 2*time.Minute),
		},
		Malformed: map[string]string{},
	}

	lines := silentHolders(tree, "m1u", copied, map[string]seat.Observation{}, presenceNow, presenceWindow())

	testutil.Expect(t, "a reachable holder and this machine's own claim say nothing", lines, []string{})
}

// An unarmed checkout writes no standings file, so there is no frozen first
// observation to name and the line says the silence without a date rather
// than dating it from this instant.
func TestGoalNextNamesNoSinceOnACheckoutWithNoStandings(t *testing.T) {
	t.Parallel()
	tree := presenceTree(heldGoal("tests-parallel-and-deterministic", "m1c"))
	copied := seat.Copy{
		Records:   map[string]seat.Record{"m1c": presenceRecord("m1c", 6*time.Hour)},
		Malformed: map[string]string{},
	}

	lines := silentHolders(tree, "m1u", copied, map[string]seat.Observation{}, presenceNow, presenceWindow())

	testutil.Expect(t, "the silence is named without a date it cannot vouch for", lines, []string{
		"goal tests-parallel-and-deterministic is held by m1c, unreachable; a human reassigns it with goal steal",
	})
}
