package batchowner

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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/httpd"
)

// HostPipeline is the production pipeline source (batch-lane design D14,
// R24): the host registry's armed checkouts, each named by its enrolled
// nickname, read from the host board by board.Read, then checked against
// the goal ledger at the tree the owner fetched. Every input is a seam, so
// a witness drives the real source over a fixture registry and board.
type HostPipeline struct {
	Registry func() (string, error)
	Machine  func(checkout string) (string, error)
	Home     func() (string, error)
	claims   func() (map[string]string, error)
	Prober   identity.Prober
	stall    time.Duration
}

// ProductionPipeline reads the host this owner runs on; claims reads the
// live claims of the goal ledger at the owner's latest fetched tree.
func ProductionPipeline(stall time.Duration, claims func() (map[string]string, error)) HostPipeline {
	return HostPipeline{Registry: registry.DefaultPath, Machine: goal.ResolveMachine, Home: board.Home, claims: claims,
		Prober: identity.KernelProber{}, stall: stall}
}

// View is the board as a one-shot command shows it (D14-r2, R23): the same
// direct read and classification the lane decides from, grouped by seat,
// with the bridge's state from its socket's presence. It never connects to
// the bridge.
func (source HostPipeline) View(now time.Time) board.View {
	seats, picture := source.read(now)
	view := board.NewView(seats, board.Picture{Cards: picture.Cards, Unknown: picture.Unknown})
	view.Readable, view.Reason, view.Bridge = picture.Readable, picture.Reason, board.BridgeAbsent
	if home, err := source.Home(); err == nil {
		view.Bridge = board.BridgeState(home)
	}
	return view
}

// read is the armed seats of this host and their classified picture.
func (source HostPipeline) read(now time.Time) ([]board.Seat, boardPicture) {
	seats, err := source.Seats()
	if err != nil {
		return nil, boardPicture{Reason: "registry: " + err.Error()}
	}
	home, err := source.Home()
	if err != nil {
		return seats, boardPicture{Reason: "board: " + err.Error()}
	}
	picture, _ := board.Read(home, seats, source.Prober, now, source.stall)
	result := boardPicture{Cards: picture.Cards, Unknown: picture.Unknown, Readable: true}
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

// Seats are the armed checkouts of the host registry, each named by its
// enrolled nickname; an error is an unreadable registry, never an empty one.
func (source HostPipeline) Seats() ([]board.Seat, error) {
	path, err := source.Registry()
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
		machine, resolveErr := source.Machine(checkout)
		if resolveErr != nil || machine == "" {
			continue
		}
		seats = append(seats, board.Seat{Machine: machine, Installation: checkout})
	}
	return seats, nil
}

// HostBoardSource is where the interface reads the host board from: the
// same registry projection, home and prober the lane decides with.
func HostBoardSource(installation string) *httpd.BoardSource {
	source := ProductionPipeline(PipelineStall(installation), nil)
	home, err := source.Home()
	if err != nil {
		return nil
	}
	return &httpd.BoardSource{Home: home, Seats: source.Seats, Prober: source.Prober, Stall: source.stall,
		Stuck: func(now time.Time) ([]launch.UnitStanding, error) {
			limits, err := steward.StuckUnitLimits(installation)
			if err != nil {
				return nil, err
			}
			return launch.UnitStandings("", launch.Store{}, limits, now)
		},
		Lane: func(now time.Time) plain.Status { return landingLaneStatus(LandingLaneHome, now) }}
}

// PipelineStall is the stall bound the installation's configuration sets,
// or the compiled default.
func PipelineStall(installation string) time.Duration {
	if withPipeline, err := (config.BatchLanding{}).WithPipeline(filepath.Join(installation, "metasystem.conf")); err == nil {
		return withPipeline.Pipeline.Stall
	}
	return config.DefaultPipelineSettings().Stall
}

// AcceptedClaims maps every live claimed goal of the accepted ledger to its
// holder: the claims a one-shot view checks the board against. ledgerRoot
// is the installation's state root: the ledger's files are read relative to
// it, and a template checkout's repository top holds none.
func AcceptedClaims(ledgerRoot string) func() (map[string]string, error) {
	return func() (map[string]string, error) {
		tip, exists, err := goal.AcceptedLedgerTip(ledgerRoot)
		if err != nil || !exists {
			return map[string]string{}, err
		}
		projection, err := goal.ProjectAt(ledgerRoot, tip)
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
func checkAgainstLedger(picture boardPicture, seats []board.Seat, claims map[string]string) boardPicture {
	checked := board.CheckClaims(board.Picture{Cards: picture.Cards, Unknown: picture.Unknown}, seats, claims)
	picture.Cards, picture.Unknown = checked.Cards, checked.Unknown
	return picture
}

// boardPicture is what a pipeline source reads from the host: the cards it
// believes and the ones it cannot, classified by board.Classify and by the
// ledger checks only the source can make. Readable is false, with Reason,
// when the host registry cannot be read.
type boardPicture struct {
	Cards    []board.Card
	Unknown  []board.Unknown
	Readable bool
	Reason   string
}
