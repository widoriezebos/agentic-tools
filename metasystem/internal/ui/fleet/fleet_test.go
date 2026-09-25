package fleet

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The composition rules, one test per rule the design states.
//
// Every fixture is stamped relative to `now` rather than written as a date,
// because every rule here is about an age and a fixed date drifts out of one.

var now = time.Date(2026, 9, 25, 14, 37, 0, 0, time.UTC)

// window is the reader's own stale window, the seat package's default.
const window = seat.DefaultStaleMinutes * time.Minute

func ago(d time.Duration) string { return seat.FormatTime(now.Add(-d)) }

// presence is one well-formed record, aged.
func presence(machine string, age time.Duration) seat.Record {
	return seat.Record{
		PresenceSchema: seat.RecordSchema, Machine: machine,
		RepoIdentity: "repo-" + machine, Generation: 4, Engine: "3f9c1e2abcdef",
		ArmedLineage: seat.NoLease, TickSeconds: 600, TickAt: ago(age),
	}
}

func claimed(id, machine, intent string) backlog.Row {
	return backlog.Row{
		Ref: backlog.Ref{Kind: "goal", ID: id, Revision: 3}, Where: backlog.WhereLive,
		Lane: backlog.LaneInProgress, Intent: intent,
		Claim:  &backlog.Claim{Machine: machine, Lineage: "L1", At: ago(2 * time.Hour)},
		Labels: []string{}, BlockedBy: []string{}, OpenBlockers: []string{}, Gaps: []string{},
	}
}

// fleetOf is Compose over a workspace with one reachable holder, one silent
// holder and one machine named only by a claim.
func fleetOf(t *testing.T, change func(*Inputs)) Page {
	t.Helper()
	in := Inputs{
		This: "m1u",
		Presence: seat.Copy{
			Records: map[string]seat.Record{
				"m1u": presence("m1u", 30*time.Second),
				"m1e": presence("m1e", 2*time.Minute),
				"m1c": presence("m1c", 6*time.Hour),
			},
			Malformed: map[string]string{},
		},
		Copy:        Copy{Source: SourceInterface, AttemptedAt: ago(40 * time.Second), SucceededAt: ago(40 * time.Second)},
		Observation: snapshot.Observation{State: snapshot.StateRead, Tip: "5b9d958", ObservedAt: now},
		Board: backlog.Board{Rows: []backlog.Row{
			claimed("finish-test-repairs", "m1e", "Finish the test repairs and integrate them."),
			claimed("tests-parallel-and-deterministic", "m1c", "Run the suite in parallel, deterministically."),
			claimed("fleet-channel-gateway", "m0b", "Open the fleet channel gateway."),
		}},
		Previous: map[string]seat.Observation{
			"m1u": {Standing: seat.Reachable, Since: ago(3 * time.Hour)},
			"m1e": {Standing: seat.Reachable, Since: ago(3 * time.Hour)},
			"m1c": {Standing: seat.Unreachable, Since: ago(5 * time.Hour)},
		},
		Window: window,
	}
	if change != nil {
		change(&in)
	}
	return Compose(in, now)
}

func machineNamed(page Page, machine string) Machine {
	for _, row := range page.Machines {
		if row.Machine == machine {
			return row
		}
	}
	return Machine{}
}

func machineOrder(page Page) []string {
	order := make([]string, 0, len(page.Machines))
	for _, row := range page.Machines {
		order = append(order, row.Machine)
	}
	return order
}

// The whole composition, from one presence copy and one observation.
func TestThePageIsComposedFromTheCopyAndOneObservation(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, nil)

	testutil.Expect(t, "the schema is the one a reader parses", page.SchemaVersion, SchemaVersion)
	testutil.Expect(t, "the page is stamped with the reader's clock", page.ReadAt, seat.FormatTime(now))
	testutil.Expect(t, "the claims name the tip they were read at", page.Claims.Tip, "5b9d958")
	testutil.Expect(t, "and nothing is unavailable", page.Claims.Unavailable, "")
	testutil.Expect(t, "every machine of the refs and the claims has a row", machineOrder(page),
		[]string{"m1u", "m0b", "m1c", "m1e"})
	testutil.Expect(t, "a reachable holder is reachable", machineNamed(page, "m1e").Standing, "reachable")
	testutil.Expect(t, "a silent one is unreachable", machineNamed(page, "m1c").Standing, "unreachable")
	testutil.Expect(t, "a machine named only by a claim is unknown", machineNamed(page, "m0b").Standing, "unknown")
}

// Titles and lanes come out of the same projection the holders do, so a chip
// opens the goal page the board opens.
func TestAHoldCarriesTheTitleAndLaneOfTheSameProjection(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, nil)
	holds := machineNamed(page, "m1e").Holds

	testutil.Require(t, "the reachable holder holds one goal", len(holds), 1)
	testutil.Expect(t, "the goal is named", holds[0].Goal, "finish-test-repairs")
	testutil.Expect(t, "its title is the projection's intent",
		holds[0].Title, "Finish the test repairs and integrate them.")
	testutil.Expect(t, "its lane is the projection's lane", holds[0].Lane, "in-progress")
	testutil.Expect(t, "a reachable holder raises no flag", holds[0].Flag, "")
}

// A ledger this seat could not read leaves every standing standing and raises
// no flag at all: nothing is known about who holds what.
func TestAnUnreadableLedgerReportsStandingsAndNoFlags(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Observation = snapshot.Observation{State: snapshot.StateBroken, Message: "the accepted ref cannot be read"}
		in.Board = backlog.Board{}
	})

	testutil.Expect(t, "the reason travels", page.Claims.Unavailable, "the accepted ref cannot be read")
	testutil.Expect(t, "the standings still stand", len(page.Machines), 3)
	testutil.Expect(t, "nothing needs a human, because nothing is known to be held", page.NeedsYou, []Held{})
	testutil.Expect(t, "and no machine carries a hold", len(machineNamed(page, "m1c").Holds), 0)
}

/* --------------------------------------------------------- the ordering -- */

func TestTheTableIsThisSeatThenFlaggedThenReachableThenUnknown(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, nil)

	testutil.Expect(t, "this seat leads, then the two flagged holders by name, then the reachable one",
		machineOrder(page), []string{"m1u", "m0b", "m1c", "m1e"})
	testutil.Expect(t, "and this seat is marked as such", page.Machines[0].This, true)
}

/* ----------------------------------------------------------- needs you -- */

func TestNeedsYouIsTheSilentAndTheAbsentHolders(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, nil)

	goals := make([]string, 0, len(page.NeedsYou))
	for _, held := range page.NeedsYou {
		goals = append(goals, held.Goal)
	}
	testutil.Expect(t, "the unreachable holder and the one that published nothing are named",
		goals, []string{"tests-parallel-and-deterministic", "fleet-channel-gateway"})
	testutil.Expect(t, "the dated silence says when it began",
		page.NeedsYou[0].Flag, "held by m1c, unreachable since "+clockWords(ago(5*time.Hour), now))
	testutil.Expect(t, "the machine that published nothing has not gone silent",
		page.NeedsYou[1].Flag, "held by m0b, which has published no presence")
}

func TestDatedSilencesComeFirstAndOldestFirst(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Presence.Records["m2a"] = presence("m2a", 9*time.Hour)
		in.Board.Rows = append(in.Board.Rows, claimed("older-silence", "m2a", "An older silence."))
		in.Previous["m2a"] = seat.Observation{Standing: seat.Unreachable, Since: ago(8 * time.Hour)}
	})

	order := make([]string, 0, len(page.NeedsYou))
	for _, held := range page.NeedsYou {
		order = append(order, held.Machine)
	}
	testutil.Expect(t, "the oldest dated silence leads and the undated one comes last",
		order, []string{"m2a", "m1c", "m0b"})
}

/* ------------------------------------------ the since normalisation, D4 -- */

// An unarmed checkout writes no standings file, so seat.Fleet would mint the
// request's own instant as a first observation. The page must not name it.
func TestAnUnarmedCheckoutNamesNoSince(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) { in.Previous = map[string]seat.Observation{} })

	silent := machineNamed(page, "m1c")
	testutil.Expect(t, "the standing is judged all the same", silent.Standing, "unreachable")
	testutil.Expect(t, "but nothing says when the silence began", silent.Since, "")
	testutil.Expect(t, "and the flag says so without a date",
		silent.Holds[0].Flag, "held by m1c, unreachable")
}

// A machine the tick last saw reachable and this page judges unreachable is a
// transition the tick has not written down yet: its `since` is not the old
// one, and this page has none of its own to name.
func TestATransitionTheTickHasNotSeenNamesNoSince(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Previous["m1c"] = seat.Observation{Standing: seat.Reachable, Since: ago(9 * time.Hour)}
	})

	silent := machineNamed(page, "m1c")
	testutil.Expect(t, "the new standing is the judged one", silent.Standing, "unreachable")
	testutil.Expect(t, "the reachable since is not carried over", silent.Since, "")
}

func TestAMatchingObservationKeepsItsFrozenSince(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, nil)

	testutil.Expect(t, "the tick's own first observation is kept",
		machineNamed(page, "m1c").Since, ago(5*time.Hour))
}

/* --------------------------------------------- the words for an unknown -- */

func TestUnknownKeepsItsThreeReasonsApart(t *testing.T) {
	t.Parallel()
	absent := fleetOf(t, nil)
	malformed := fleetOf(t, func(in *Inputs) {
		in.Presence.Malformed["m0b"] = "SEAT_PRESENCE_MALFORMED: the presence record has no tickAt"
	})
	ahead := fleetOf(t, func(in *Inputs) {
		record := presence("m0b", 0)
		record.TickAt = seat.FormatTime(now.Add(4 * time.Hour))
		in.Presence.Records["m0b"] = record
	})

	testutil.Expect(t, "an absent ref has no presence record",
		machineNamed(absent, "m0b").Reason, "no presence record")
	testutil.Expect(t, "a malformed one names its refusal",
		machineNamed(malformed, "m0b").Reason,
		"presence unreadable: SEAT_PRESENCE_MALFORMED: the presence record has no tickAt")
	testutil.Expect(t, "a record from the future names the clock",
		machineNamed(ahead, "m0b").Reason, "clock ahead by 4 h")
	testutil.Expect(t, "the malformed holder's flag says the record is unreadable",
		machineNamed(malformed, "m0b").Holds[0].Flag, "held by m0b, whose presence record is unreadable")
	testutil.Expect(t, "the clock-ahead holder's flag carries the clock",
		machineNamed(ahead, "m0b").Holds[0].Flag, "held by m0b, clock ahead by 4 h")
	testutil.Expect(t, "neither is in needs-you, which is the silent and the absent",
		len(malformed.NeedsYou), 1)
}

// "seen" comes from the record whatever the standing, so an unreachable row
// can still say when its machine was last heard from.
func TestSeenComesFromTheRecordWhateverTheStanding(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, nil)

	testutil.Expect(t, "the unreachable machine was seen six hours ago",
		machineNamed(page, "m1c").Seen, ago(6*time.Hour))
	testutil.Expect(t, "and its engine and generation travel",
		[]any{machineNamed(page, "m1c").Engine, machineNamed(page, "m1c").Generation},
		[]any{"3f9c1e2abcdef", 4})
	testutil.Expect(t, "a machine with no record has nothing to say about being seen",
		machineNamed(page, "m0b").Seen, "")
}

/* -------------------------------------------------------------- running -- */

func TestAPendingChainSaysItHasNotStarted(t *testing.T) {
	t.Parallel()
	started := ago(10 * time.Minute)
	page := fleetOf(t, func(in *Inputs) {
		running := in.Presence.Records["m1e"]
		running.Chain = &seat.Chain{Root: "r1", Job: "j2", Role: "implementer", Round: 2, Goal: "finish-test-repairs", StartedAt: &started}
		in.Presence.Records["m1e"] = running
		pending := in.Presence.Records["m1c"]
		pending.Chain = &seat.Chain{Root: "r2", Job: "j9", Role: "critic", Goal: "tests-parallel-and-deterministic"}
		in.Presence.Records["m1c"] = pending
	})

	testutil.Require(t, "the running machine carries its chain", machineNamed(page, "m1e").Running != nil, true)
	testutil.Expect(t, "with the instant it started", *machineNamed(page, "m1e").Running.StartedAt, started)
	testutil.Require(t, "the reserved one carries its chain too", machineNamed(page, "m1c").Running != nil, true)
	testutil.Expect(t, "with a null start, which is what a reservation has",
		machineNamed(page, "m1c").Running.StartedAt, (*string)(nil))
	testutil.Expect(t, "and the words say so", runningWords(machineNamed(page, "m1c").Running, ""),
		"running critic on tests-parallel-and-deterministic (not started)")
}

/* ------------------------------------------------------------ this seat -- */

func TestAnArmedSeatSaysArmedWithTheVerdictItWasRecordedFrom(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.Health = &Health{
			State: "healthy", ObservedAt: ago(12 * time.Minute),
			Roles: []Role{{Role: "steward-runner", Status: "alive", Reason: "runner alive"}},
		}
		in.Publication = &seat.PublicationState{
			Machine: "m1u", LastSuccessAt: ago(2 * time.Minute), LastOutcome: seat.OutcomePublished,
			Rung: 1, TickSeconds: 600,
		}
	})

	testutil.Expect(t, "the seat is armed", page.This.Armed, ArmedYes)
	testutil.Expect(t, "the verdict says when it was recorded", page.This.Health.ObservedAt, ago(12*time.Minute))
	testutil.Require(t, "the publication travels", page.This.Publication != nil, true)
	testutil.Expect(t, "with the rung it published on", page.This.Publication.Rung, 1)
}

func TestAnUnarmedSeatSaysSoAndAStaleVerdictSaysStale(t *testing.T) {
	t.Parallel()
	none := fleetOf(t, nil)
	dead := fleetOf(t, func(in *Inputs) {
		in.Health = &Health{State: "unhealthy", ObservedAt: ago(time.Minute),
			Roles: []Role{{Role: "steward-runner", Status: "dead", Reason: "no runner"}}}
	})
	stale := fleetOf(t, func(in *Inputs) {
		in.Health = &Health{State: "healthy", ObservedAt: ago(90 * time.Minute),
			Roles: []Role{{Role: "steward-runner", Status: "alive", Reason: "runner alive"}}}
	})
	torn := fleetOf(t, func(in *Inputs) {
		in.Health = &Health{Problem: "the steward's health record is malformed"}
	})

	testutil.Expect(t, "no verdict at all is not armed", none.This.Armed, ArmedNo)
	testutil.Expect(t, "a dead runner is not armed", dead.This.Armed, ArmedNo)
	testutil.Expect(t, "a verdict older than the threshold is stale, not false", stale.This.Armed, ArmedStale)
	testutil.Expect(t, "a file that cannot be read is named as such", torn.This.Armed, ArmedUnreadable)
}

func TestACheckoutWithNoNicknameSaysSo(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.This, in.NoNickname = "", true
	})

	testutil.Expect(t, "the checkout has no nickname", page.This.NoNickname, true)
	testutil.Expect(t, "and the lines say it publishes no presence",
		strings.Contains(strings.Join(page.Lines(now), "\n"),
			"this checkout has no machine nickname and publishes no presence"), true)
}

func TestAnUnreadableJobsDirectoryIsCarriedRatherThanShownAsIdle(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, func(in *Inputs) {
		in.RunningProblem = "chain unread: artifacts/agents/jobs/j1.json: permission denied"
	})

	testutil.Expect(t, "the explanation travels", page.This.RunningProblem,
		"chain unread: artifacts/agents/jobs/j1.json: permission denied")
	testutil.Expect(t, "and the words are the explanation rather than idle",
		runningWords(page.This.Running, page.This.RunningProblem),
		"chain unread: artifacts/agents/jobs/j1.json: permission denied")
}

/* ---------------------------------------------------------- the words -- */

func TestTheSourceStampsBothReadings(t *testing.T) {
	t.Parallel()
	page := fleetOf(t, nil)

	testutil.Expect(t, "the presence copy and the tip are both named", page.Source(),
		"presence from the interface, fetched "+ago(40*time.Second)+"; claims from the accepted tip 5b9d958")
}

func TestASilenceThatBeganTodayIsAClockAndAnOlderOneCarriesItsDate(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "today is a time of day", clockWords(ago(5*time.Hour), now), "09:37")
	testutil.Expect(t, "yesterday carries its date", clockWords(ago(30*time.Hour), now), "2026-09-24 08:37")
}
