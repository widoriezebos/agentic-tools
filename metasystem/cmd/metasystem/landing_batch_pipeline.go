package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
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
	_, picture := source.read(now)
	return picture
}

// View is the board as a one-shot command shows it (D14-r2, R23): the same
// direct read and classification the lane decides from, grouped by seat,
// with the bridge's state from its socket's presence. It never connects to
// the bridge.
func (source hostPipeline) View(now time.Time) board.View {
	seats, picture := source.read(now)
	view := board.NewView(seats, board.Picture{Cards: picture.Cards, Unknown: picture.Unknown})
	view.Readable, view.Reason, view.Bridge = picture.Readable, picture.Reason, board.BridgeAbsent
	if home, err := source.home(); err == nil {
		view.Bridge = board.BridgeState(home)
	}
	return view
}

// read is the armed seats of this host and their classified picture.
func (source hostPipeline) read(now time.Time) ([]board.Seat, batch.BoardPicture) {
	seats, err := source.seats()
	if err != nil {
		return nil, batch.BoardPicture{Reason: "registry: " + err.Error()}
	}
	home, err := source.home()
	if err != nil {
		return seats, batch.BoardPicture{Reason: "board: " + err.Error()}
	}
	picture, _ := board.Read(home, seats, source.prober, now, source.stall)
	result := batch.BoardPicture{Cards: picture.Cards, Unknown: picture.Unknown, Readable: true}
	// With no armed seat no claim can be checked, and the ledger is not read.
	if source.claims == nil || len(seats) == 0 {
		return seats, result
	}
	claims, err := source.claims()
	if err != nil {
		return seats, result
	}
	return seats, checkAgainstLedger(result, seats, claims)
}

// seats are the armed checkouts of the host registry, each named by its
// enrolled nickname; an error is an unreadable registry, never an empty one.
func (source hostPipeline) seats() ([]board.Seat, error) {
	path, err := source.registry()
	if err != nil {
		return nil, err
	}
	checkouts, err := registry.ArmedCheckouts(path)
	if err != nil {
		return nil, err
	}
	seats := []board.Seat{}
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
	return seats, nil
}

// hostBoardSource is where the interface reads the host board from: the
// same registry projection, home and prober the lane decides with.
func hostBoardSource(installation string) *httpd.BoardSource {
	source := productionPipeline(pipelineStall(installation), nil)
	home, err := source.home()
	if err != nil {
		return nil
	}
	return &httpd.BoardSource{Home: home, Seats: source.seats, Prober: source.prober, Stall: source.stall,
		Lane: func(now time.Time) lane.View { return landingLaneView(landingLaneHome, now) }}
}

// pipelineStall is the stall bound the installation's configuration sets,
// or the compiled default.
func pipelineStall(installation string) time.Duration {
	if withPipeline, err := (config.BatchLanding{}).WithPipeline(filepath.Join(installation, "metasystem.conf")); err == nil {
		return withPipeline.Pipeline.Stall
	}
	return config.DefaultPipelineSettings().Stall
}

// acceptedClaims maps every live claimed goal of the checkout's accepted
// ledger to its holder: the claims a one-shot view checks the board
// against.
func acceptedClaims(checkout string) func() (map[string]string, error) {
	return func() (map[string]string, error) {
		tip, exists, err := goal.AcceptedLedgerTip(checkout)
		if err != nil || !exists {
			return map[string]string{}, err
		}
		projection, err := goal.ProjectAt(checkout, tip)
		if err != nil {
			return nil, err
		}
		claims := map[string]string{}
		if projection.Tree == nil {
			return claims, nil
		}
		for id, file := range projection.Tree.Live {
			if file != nil && file.Claimed != nil && file.Claimed.Machine != "" {
				claims[id] = file.Claimed.Machine
			}
		}
		return claims, nil
	}
}

// checkAgainstLedger applies the board's ledger checks (claim moved, not
// claimed, no card) to the lane's picture.
func checkAgainstLedger(picture batch.BoardPicture, seats []board.Seat, claims map[string]string) batch.BoardPicture {
	checked := board.CheckClaims(board.Picture{Cards: picture.Cards, Unknown: picture.Unknown}, seats, claims)
	picture.Cards, picture.Unknown = checked.Cards, checked.Unknown
	return picture
}
