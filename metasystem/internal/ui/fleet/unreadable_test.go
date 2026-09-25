package fleet

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

/* ------------------------------------------- a copy nothing could read -- */

// A namespace this seat could not read is the absence of evidence, and the
// absence of evidence is not evidence of absence: every machine the ledger
// names would otherwise be judged as having published nothing, and every
// claim on the board would be flagged as held by such a machine.
func TestACopyThatCouldNotBeReadJudgesNoMachineAbsent(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Presence = seat.Copy{Records: map[string]seat.Record{}, Malformed: map[string]string{}}
		in.PresenceProblem = "presence ref list: exit status 128 (not a repository)"
		in.Copy.Problem = in.PresenceProblem
	})

	testutil.Expect(t, "no machine is reported at all", page.Machines, []Machine{})
	testutil.Expect(t, "nothing is said to need a human", page.NeedsYou, []Held{})
	testutil.Expect(t, "and the copy block carries the reason",
		page.Copy.Problem, "presence ref list: exit status 128 (not a repository)")
	testutil.Expect(t, "the claims were still read, so the page is not blind about the ledger",
		page.Claims.Tip, "5b9d958")
}

// The same copy read successfully and found empty IS evidence: the machines
// the ledger names have published nothing, and their goals need a human.
func TestAnEmptyCopyThatWasReadStillJudgesTheMachinesItNames(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Presence = seat.Copy{Records: map[string]seat.Record{}, Malformed: map[string]string{}}
	})

	testutil.Expect(t, "every claiming machine has a row", len(page.Machines), 3)
	testutil.Expect(t, "each of them unknown by absence",
		machineNamed(page, "m1c").Reason, "no presence record")
	testutil.Expect(t, "and each of their goals needs a human", len(page.NeedsYou), 3)
}

/* ------------------------------------------ who a remedy is printed for -- */

// The two cases a human has an act for, and the two that are reading problems
// rather than silences.
func TestOnlyTheSilentAndTheAbsentNeedAHuman(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Presence.Malformed["m0b"] = "SEAT_PRESENCE_MALFORMED: the presence record has no tickAt"
		ahead := presence("m2a", 0)
		ahead.TickAt = seat.FormatTime(now.Add(4 * time.Hour))
		in.Presence.Records["m2a"] = ahead
		in.Board.Rows = append(in.Board.Rows, claimed("clock-ahead", "m2a", "A machine whose clock is ahead."))
	})

	goals := make([]string, 0, len(page.NeedsYou))
	for _, held := range page.NeedsYou {
		goals = append(goals, held.Goal)
	}
	testutil.Expect(t, "the unreachable holder alone needs a human here",
		goals, []string{"tests-parallel-and-deterministic"})
	testutil.Expect(t, "a malformed record is still flagged where it is seen",
		machineNamed(page, "m0b").Holds[0].Flag, "held by m0b, whose presence record is unreadable")
	testutil.Expect(t, "and so is a clock too far ahead",
		machineNamed(page, "m2a").Holds[0].Flag, "held by m2a, clock ahead by 4 h")
}
