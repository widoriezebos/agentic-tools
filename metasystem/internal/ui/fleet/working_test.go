package fleet

// What each row of the page says a machine is doing, and where it came from.
//
// A machine elsewhere shows what its presence record published. This seat's
// row is the one the interface composes for itself, from the job records only
// this host can read, and it shows every job in flight rather than the newest
// chain alone. Both are asserted here, from fakes: nothing opens a repository
// and nothing reads the wall.

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

func workingMinutes(value int) *int { return &value }

// publishedWorking is what a machine elsewhere published about itself.
func publishedWorking() *seat.Working {
	attempts, attemptLimit := int64(3), int64(10)
	reserved, reservedLimit := int64(610), int64(720)
	started := ago(41 * time.Minute)
	endsAt := seat.FormatTime(now.Add(79 * time.Minute))
	return &seat.Working{
		Goal:  "finish-test-repairs",
		Phase: seat.Phase{Role: "implementer", Round: 2},
		Job: seat.WorkingJob{
			ID: "j-17", Role: "implementer", Status: "running",
			StartedAt: &started, CapMinutes: workingMinutes(120), CapEndsAt: &endsAt,
		},
		Box: &seat.Box{
			Attempts: &attempts, AttemptLimit: &attemptLimit,
			ReservedMinutes: &reserved, ReservedMinutesLimit: &reservedLimit,
		},
		Chain: []seat.ChainMember{
			{Job: "j-17", Role: "implementer", Round: 2, Status: "running",
				StartedAt: &started, CapMinutes: workingMinutes(120)},
		},
	}
}

// localJobs is this host's own records: two chains in flight on two goals.
func localJobs() seat.JobSet {
	return seat.JobSet{Records: []seat.JobRecord{
		{Job: "j-41", Role: "reviewer", Goal: "fleet-channel-gateway", Round: 1, Status: "running",
			CreatedAt: ago(8 * time.Minute), StartedAt: ago(8 * time.Minute), CapMinutes: workingMinutes(45)},
		{Job: "j-40", Role: "code-critic", Goal: "tests-parallel-and-deterministic", Round: 2, Status: "running",
			CreatedAt: ago(20 * time.Minute), StartedAt: ago(20 * time.Minute), CapMinutes: workingMinutes(60),
			ReviewRoundLimit: workingMinutes(3)},
	}}
}

func localBox(goal string) *seat.Box {
	if goal != "fleet-channel-gateway" {
		return nil
	}
	attempts, attemptLimit := int64(4), int64(10)
	reserved, reservedLimit := int64(505), int64(720)
	return &seat.Box{
		Attempts: &attempts, AttemptLimit: &attemptLimit,
		ReservedMinutes: &reserved, ReservedMinutesLimit: &reservedLimit,
	}
}

func TestAMachineElsewhereShowsWhatItsPresenceCarried(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		record := in.Presence.Records["m1e"]
		record.Working = publishedWorking()
		in.Presence.Records["m1e"] = record
	})
	row := machineNamed(page, "m1e")
	testutil.Expect(t, "one thing in flight", len(row.Working), 1)
	testutil.Expect(t, "the goal it published", row.Working[0].Goal, "finish-test-repairs")
	testutil.Expect(t, "no local problem", row.WorkingProblem, "")
}

func TestARowWithNoWorkingIsNeverNull(t *testing.T) {
	t.Parallel()
	// The browser reads an empty array rather than a null it has to tell
	// from an absent field.
	row := machineNamed(fleetOf(t, nil), "m1c")
	if row.Working == nil {
		t.Fatalf("working = nil on a row with nothing in flight")
	}
	testutil.Expect(t, "nothing in flight", len(row.Working), 0)
}

func TestThisSeatsRowIsComposedFromItsOwnJobRecords(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Jobs = localJobs()
		in.Box = localBox
		// The record this seat last published carries one chain; the records
		// on disk are newer and carry both, and the records win.
		record := in.Presence.Records["m1u"]
		record.Working = publishedWorking()
		in.Presence.Records["m1u"] = record
	})
	row := machineNamed(page, "m1u")
	testutil.Expect(t, "every job in flight", len(row.Working), 2)
	testutil.Expect(t, "newest first", row.Working[0].Job.ID, "j-41")
	testutil.Expect(t, "then the older one", row.Working[1].Job.ID, "j-40")
	testutil.Expect(t, "a goal with a box", row.Working[0].Box != nil, true)
	testutil.Expect(t, "a goal with none", row.Working[1].Box == nil, true)
	// A critic's round counts against the limit its chain's root froze.
	if row.Working[1].Phase.RoundLimit == nil || *row.Working[1].Phase.RoundLimit != 3 {
		t.Fatalf("the critic's roundLimit = %v", row.Working[1].Phase.RoundLimit)
	}
}

func TestThisSeatSaysWhyItCannotReadItsOwnJobsInsteadOfReadingIdle(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Jobs = seat.JobSet{Unreadable: []string{"artifacts/agents/jobs/torn.json: the job record is not readable JSON"}}
		in.Box = localBox
	})
	row := machineNamed(page, "m1u")
	if !strings.Contains(row.WorkingProblem, "torn.json") {
		t.Fatalf("workingProblem = %q", row.WorkingProblem)
	}
	testutil.Expect(t, "and nothing is claimed to be in flight", len(row.Working), 0)
}

func TestTheFleetLinesSayThePhaseSentencePerMachine(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		record := in.Presence.Records["m1e"]
		record.Working = publishedWorking()
		in.Presence.Records["m1e"] = record
	})
	joined := strings.Join(page.Lines(now), "\n")
	if !strings.Contains(joined, "implementer round 2 on finish-test-repairs · running 41 min, cap 120 min") {
		t.Fatalf("lines = %s", joined)
	}
}

func TestANamedMachineOpensItsBlockAsLines(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		record := in.Presence.Records["m1e"]
		record.Working = publishedWorking()
		in.Presence.Records["m1e"] = record
	})
	joined := strings.Join(page.MachineLines("m1e", now), "\n")
	for _, want := range []string{
		// The goal, with the title the row was composed with.
		"- Goal: finish-test-repairs · Finish the test repairs and integrate them.",
		"its cap ends in 79 min",
		"- Box: attempt 3 of 10 · 610 of 720 min reserved · 7 attempts left",
		seat.ReservedMeaning,
		"- Chain:",
		// It is a reading of a reading, and says so.
		"- As published ",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("machine lines missing %q:\n%s", want, joined)
		}
	}
}

func TestThisSeatsOwnBlockIsNotStampedAsPublished(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Jobs = localJobs()
		in.Box = localBox
	})
	joined := strings.Join(page.MachineLines("m1u", now), "\n")
	if strings.Contains(joined, "As published") {
		t.Fatalf("this seat's own records are not a publication:\n%s", joined)
	}
	if !strings.Contains(joined, "- Box: no box on this goal") {
		t.Fatalf("a goal with no box:\n%s", joined)
	}
}

func TestAMachineTheFleetDoesNotHaveIsSaidRatherThanEmptied(t *testing.T) {
	t.Parallel()
	lines := fleetOf(t, nil).MachineLines("m9z", now)
	testutil.Expect(t, "one line", len(lines), 1)
	if !strings.Contains(lines[0], "no machine called m9z") {
		t.Fatalf("lines = %q", lines)
	}
}
