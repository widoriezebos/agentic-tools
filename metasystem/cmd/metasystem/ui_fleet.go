package main

// The interface's own reading of the fleet, and its own presence fetch.
//
// Two things live here. The reader composes GET /api/fleet, and the board's
// and the Overview's holder flags with it, from one observation the caller
// captured and the presence copy this checkout holds; it starts no fetch. The
// fetch owner is the other half: one bounded git call a minute, and only
// while a browser is holding the notifications stream open.
//
// They are apart on purpose. A page must answer from whatever copy is on disk
// even when no fetch has ever succeeded, and a fetch must not be something a
// request can start.

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// fleetReader composes the Fleet page for one checkout.
//
// succeeded says whether this server's own fetch has ever brought a copy in,
// which is what decides whether the page reads the interface's namespace or
// falls back to the tick's; state is what that owner knows about its
// attempts. Both are functions because the owner runs while requests are
// answered, and a value read at construction would be a value from boot.
func fleetReader(
	roots lifecycle.Roots,
	succeeded func() bool,
	state func() fleet.Copy,
) func(snapshot.Observation, backlog.Board, time.Time) (fleet.Page, error) {
	return func(observed snapshot.Observation, board backlog.Board, now time.Time) (fleet.Page, error) {
		transport, err := seat.NewGit(roots.Checkout)
		if err != nil {
			return fleet.Page{}, err
		}
		presence, source, problem := fleet.ReadPresence(transport, succeeded())
		copied := state()
		copied.Source = source
		if problem != "" {
			// A namespace this seat cannot even read locally is the more
			// immediate problem, and it displaces the last fetch's.
			copied.Problem = problem
		}
		machine, enrolled := seat.Machine(roots.Checkout)
		publication, publicationProblem := fleet.ReadPublication(roots.Checkout)
		running, runningProblem := fleet.ReadRunning(roots.Checkout)
		return fleet.Compose(fleet.Inputs{
			This: machine, NoNickname: !enrolled,
			Presence: presence, Copy: copied,
			Observation: observed, Board: board,
			Previous:    fleet.ReadStandings(roots.Checkout),
			Window:      seatPresenceWindow(roots.Installation),
			Publication: publication, PublicationProblem: publicationProblem,
			Health:  fleet.ReadHealth(roots.Checkout),
			Running: running, RunningProblem: runningProblem,
		}, now), nil
	}
}

// fleetFetcher is the interface's presence fetch owner: one attempt in
// flight, a minute between attempt starts, and nothing at all while no
// browser is connected.
//
// Its attempt is the seat package's own bounded transport into the
// interface's own namespace. It never runs through the snapshot loop's ledger
// Fetch: that function returns one error, and a failure there blanks the
// ledger tip and backs the whole loop off toward five minutes, so a presence
// remote that is down would cost this clone its ledger freshness as well.
func fleetFetcher(roots lifecycle.Roots, watch *fleet.Watch) *fleet.Owner {
	return &fleet.Owner{
		Attempt: func() error {
			transport, err := seat.NewGit(roots.Checkout)
			if err != nil {
				return err
			}
			return transport.Fetch(seat.UINamespace)
		},
		Connected: watch.Connected,
		Announce:  watch.Announce,
		// The metadata file is for the Partner's tool, which runs in another
		// process and owns no fetcher. A write that fails costs that tool one
		// provenance line and this server nothing, so it is dropped rather
		// than raised into the fetch's own outcome.
		Record: func(state fleet.Metadata) error { return fleet.SaveMetadata(roots.Checkout, state) },
		Now:    func() time.Time { return time.Now().UTC() },
	}
}
