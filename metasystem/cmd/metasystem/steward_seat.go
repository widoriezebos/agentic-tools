package main

// The steward's seat launcher (g1-s77, D-seat): the steward starts its seat
// main as the launch lane's seat kind, in the checkout, and reads the launch
// back to reap it. The seat kind itself, its environment and its fence claim
// are the launch lane's (internal/launch).

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// stewardSeatKind is the launch kind of a seat main.
const stewardSeatKind = "seat"

// stewardSeatLauncher reaches the launch manager the work verbs use.
type stewardSeatLauncher struct {
	manager func() *launch.Manager
	// start is the manager's Start; a test records the spec instead.
	start func(launch.StartSpec) (launch.Record, error)
}

func newStewardSeatLauncher() stewardSeatLauncher {
	return stewardSeatLauncher{manager: func() *launch.Manager { return launchManager() }}
}

// StartSeat starts the seat kind with the id, brief and tag the steward
// chose, in the checkout; the launch names no goal, for a goal id names no
// checkout.
func (l stewardSeatLauncher) StartSeat(spec steward.SeatLaunchSpec) error {
	start := l.start
	if start == nil {
		start = l.manager().Start
	}
	_, err := start(launch.StartSpec{ID: spec.ID, Kind: stewardSeatKind,
		WorkingDirectory: spec.WorkingDirectory, Brief: spec.Brief, Tag: spec.Tag})
	return err
}

// SeatLaunch reads one seat launch's state, reconciled as work status reads
// it; a launch with no record is not found, which the steward reaps as a
// seat that never ran.
func (l stewardSeatLauncher) SeatLaunch(id string) (steward.SeatLaunchState, error) {
	manager := l.manager()
	record, err := manager.Status(id)
	if errors.Is(err, fs.ErrNotExist) {
		return steward.SeatLaunchState{}, nil
	}
	if err != nil {
		return steward.SeatLaunchState{}, err
	}
	dir, err := manager.Store.StateDir(id)
	if err != nil {
		return steward.SeatLaunchState{}, err
	}
	return steward.SeatLaunchState{Found: true, Terminal: record.State.Terminal(), State: string(record.State),
		ResultPath: filepath.Join(dir, "result.json")}, nil
}

// wireStewardSeat arms the runner's tick with the seat launcher: the
// steward starts its seat main when ready work has no seat.
func wireStewardSeat(config *steward.TickConfig) {
	config.Seat = newStewardSeatLauncher()
}
