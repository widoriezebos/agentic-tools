package main

import (
	"errors"
	"io/fs"
	"os"
	"sort"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
)

// hostPipeline is the production pipeline source (batch-lane design D14,
// R24): the host registry's armed checkouts, each named by its enrolled
// nickname, read from the host board by board.Read, then checked against
// the goal ledger at the tree the owner fetched. Every input is a seam, so
// a witness drives the real source over a fixture registry and board.
type hostPipeline struct {
	registry func() (string, error)
	machine  func(checkout string) (string, error)
	home     func() (string, error)
	claims   func() (map[string]string, error)
	prober   identity.Prober
	stall    time.Duration
}

// productionPipeline reads the host this owner runs on; claims reads the
// live claims of the goal ledger at the owner's latest fetched tree.
func productionPipeline(stall time.Duration, claims func() (map[string]string, error)) hostPipeline {
	return hostPipeline{registry: registry.DefaultPath, machine: goal.ResolveMachine, home: board.Home, claims: claims,
		prober: identity.KernelProber{}, stall: stall}
}

// Board reads the picture afresh.
func (source hostPipeline) Board(now time.Time) batch.BoardPicture {
	path, err := source.registry()
	if err != nil {
		return batch.BoardPicture{Reason: "registry: " + err.Error()}
	}
	checkouts, err := registry.ArmedCheckouts(path)
	if err != nil {
		return batch.BoardPicture{Reason: "registry: " + err.Error()}
	}
	var seats []board.Seat
	for _, checkout := range checkouts {
		// A registration whose checkout is gone is stale, as the disk pass
		// reads it; a checkout without a nickname is no seat.
		if _, statErr := os.Stat(checkout); errors.Is(statErr, fs.ErrNotExist) {
			continue
		}
		machine, resolveErr := source.machine(checkout)
		if resolveErr != nil || machine == "" {
			continue
		}
		seats = append(seats, board.Seat{Machine: machine, Installation: checkout})
	}
	home, err := source.home()
	if err != nil {
		return batch.BoardPicture{Reason: "board: " + err.Error()}
	}
	picture, _ := board.Read(home, seats, source.prober, now, source.stall)
	result := batch.BoardPicture{Cards: picture.Cards, Unknown: picture.Unknown, Readable: true}
	if source.claims == nil {
		return result
	}
	claims, err := source.claims()
	if err != nil {
		return result
	}
	return checkAgainstLedger(result, seats, claims)
}

// checkAgainstLedger applies the checks only a reader of the ledger can
// make: a card whose goal the live claim gives to another machine (claim
// moved: a delayed writer of a seat that handed over cannot resurrect it), a
// live card for a goal nobody claims (not claimed), and a goal claimed by a
// seat of this host with no card at all (no card: an older engine, or a
// writer that failed). Each is Unknown, never near.
func checkAgainstLedger(picture batch.BoardPicture, seats []board.Seat, claims map[string]string) batch.BoardPicture {
	carded := map[string]bool{}
	var believed []board.Card
	for _, card := range picture.Cards {
		carded[card.Goal] = true
		if card.Stage.Terminal() {
			believed = append(believed, card)
			continue
		}
		holder, claimed := claims[card.Goal]
		kept := card
		switch {
		case !claimed:
			picture.Unknown = append(picture.Unknown, board.Unknown{Seat: card.Seat, Goal: card.Goal, Reason: "not claimed", Card: &kept})
		case holder != card.Seat.Machine:
			picture.Unknown = append(picture.Unknown, board.Unknown{Seat: card.Seat, Goal: card.Goal, Reason: "claim moved to " + holder, Card: &kept})
		default:
			believed = append(believed, card)
		}
	}
	for _, unknown := range picture.Unknown {
		carded[unknown.Goal] = true
	}
	goals := make([]string, 0, len(claims))
	for goalID := range claims {
		goals = append(goals, goalID)
	}
	sort.Strings(goals)
	for _, goalID := range goals {
		holder := claims[goalID]
		if carded[goalID] {
			continue
		}
		for _, seat := range seats {
			if seat.Machine == holder {
				picture.Unknown = append(picture.Unknown, board.Unknown{Seat: seat, Goal: goalID, Reason: "no card"})
			}
		}
	}
	picture.Cards = believed
	return picture
}
