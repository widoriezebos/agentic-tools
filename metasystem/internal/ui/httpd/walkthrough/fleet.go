package main

// The fleet, as this fixture invents it.
//
// There is no remote here and no steward tick, so the presence copy is made
// rather than fetched: three machines, one of each standing the design names,
// with the claims the canned ledger already carries joined to them by the
// same composer the server uses. What a standing means is the seat package's;
// this only supplies records with ages.
//
// `-proven` is also the armed seat. A server that proved its human at boot is
// the one a human started from their own terminal, which is the seat that is
// arming a steward in the first place, so that flag carries the health
// verdict, the publication state and this machine's own presence record.

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// The machines this fixture publishes presence for. The third machine the
// ledger names in a claim, m0b, is deliberately absent from this list: its
// whole shape is that it has published nothing, so the fleet knows it only
// because the canned ledger carries its claim.
const (
	// fixtureReachable holds two goals and ticked a moment ago.
	fixtureReachable = "m1e"
	// fixtureSilent holds one goal and has not been heard from for hours.
	fixtureSilent = "m2a"
	// fixtureThis is the seat this fixture is serving.
	fixtureThis = "m1u"
)

// announceFixturePresence stands in for the fetch owner this fixture has
// none of: it tells every open stream that a presence attempt finished, on a
// cadence, so a Fleet page that is already open can be watched re-reading.
func announceFixturePresence(watch *fleet.Watch, every time.Duration) {
	for range time.Tick(every) {
		watch.Announce()
	}
}

// fixturePresence is the presence copy, invented.
func fixturePresence(now time.Time, proven bool) seat.Copy {
	copied := seat.Copy{Records: map[string]seat.Record{}, Malformed: map[string]string{}}
	copied.Records[fixtureReachable] = fixtureRecord(fixtureReachable, now.Add(-2*time.Minute), "3f9c1e2a4b5c6d7e",
		&seat.Chain{Root: "r-1", Job: "j-17", Role: "implementer", Round: 2, Goal: "g1-s15",
			StartedAt: stamped(now.Add(-40 * time.Minute))})
	// A reservation that has not begun: its startedAt is null, and the row
	// says "not started" rather than reading as work in flight.
	copied.Records[fixtureSilent] = fixtureRecord(fixtureSilent, now.Add(-6*time.Hour), "8a01d77e1f2a3b4c",
		&seat.Chain{Root: "r-2", Job: "j-22", Role: "critic", Goal: "g1-s19"})
	if proven {
		copied.Records[fixtureThis] = fixtureRecord(fixtureThis, now.Add(-30*time.Second), "5b9d958c0d1e2f30", nil)
	}
	return copied
}

func fixtureRecord(machine string, at time.Time, engine string, chain *seat.Chain) seat.Record {
	return seat.Record{
		PresenceSchema: seat.RecordSchema, Machine: machine,
		RepoIdentity: "walkthrough-" + machine, Generation: 4, Engine: engine,
		ArmedLineage: seat.NoLease, TickSeconds: 600, TickAt: seat.FormatTime(at), Chain: chain,
	}
}

func stamped(at time.Time) *string {
	held := seat.FormatTime(at)
	return &held
}

// fixtureStandings is the standings file a tick would have left behind: the
// frozen first observation of each standing, which is the only `since` a page
// may name.
func fixtureStandings(now time.Time) map[string]seat.Observation {
	return map[string]seat.Observation{
		fixtureReachable: {Standing: seat.Reachable, Since: seat.FormatTime(now.Add(-9 * time.Hour))},
		fixtureSilent:    {Standing: seat.Unreachable, Since: seat.FormatTime(now.Add(-5 * time.Hour))},
		fixtureThis:      {Standing: seat.Reachable, Since: seat.FormatTime(now.Add(-11 * time.Hour))},
	}
}

// fixtureHealth is the steward's last recorded verdict on an armed seat: one
// role that is not alive, so the page has both halves of its role list.
func fixtureHealth(now time.Time) *fleet.Health {
	return &fleet.Health{
		State: "unhealthy", ObservedAt: seat.FormatTime(now.Add(-12 * time.Minute)),
		Roles: []fleet.Role{
			{Role: "steward-runner", Status: "alive", Reason: "runner alive, pid 40881"},
			{Role: "supervision-owner", Status: "alive", Reason: "this session owns supervision"},
			{Role: "repo-watcher", Status: "alive", Reason: "watching the checkout"},
			{Role: "ledger-attention", Status: "alive", Reason: "the canonical branch was reached 2 min ago"},
			{Role: "seat-presence", Status: "alive", Reason: "presence published 2 min ago"},
			{Role: "census-freshness", Status: "unknown", Reason: "no census has been taken on this checkout"},
			{Role: "retro-debt", Status: "dead", Reason: "a retro is due: 41 receipts since the last one"},
		},
	}
}

// fixtureFleet is the Fleet page this fixture serves, composed from the canned
// presence above and the observation the board was drawn from.
func fixtureFleet(proven bool) func(snapshot.Observation, backlog.Board, time.Time) (fleet.Page, error) {
	return func(observed snapshot.Observation, board backlog.Board, now time.Time) (fleet.Page, error) {
		in := fleet.Inputs{
			This: fixtureThis, Presence: fixturePresence(now, proven),
			Copy: fleet.Copy{
				Source:      fleet.SourceInterface,
				AttemptedAt: seat.FormatTime(now.Add(-40 * time.Second)),
				SucceededAt: seat.FormatTime(now.Add(-40 * time.Second)),
			},
			Observation: observed, Board: board,
			Previous: fixtureStandings(now),
			Window:   seat.DefaultStaleMinutes * time.Minute,
		}
		if proven {
			in.Health = fixtureHealth(now)
			in.Publication = &seat.PublicationState{
				Machine: fixtureThis, LastAttemptAt: seat.FormatTime(now.Add(-2 * time.Minute)),
				LastSuccessAt: seat.FormatTime(now.Add(-2 * time.Minute)),
				LastOutcome:   seat.OutcomePublished, Rung: 1, TickSeconds: 600,
			}
			in.Running = &seat.Chain{
				Root: "r-3", Job: "j-31", Role: "reviewer", Round: 1, Goal: "g1-s21",
				StartedAt: stamped(now.Add(-8 * time.Minute)),
			}
		}
		return fleet.Compose(in, now), nil
	}
}
