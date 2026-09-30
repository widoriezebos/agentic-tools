package main

// The steward's seat launcher (g1-s77, D-seat): the steward starts its seat
// main as the launch lane's seat kind, in the checkout, and reads the launch
// back to reap it. The seat kind itself, its environment and its fence claim
// are the launch lane's (internal/launch).

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// stewardSeatKind is the launch kind of a seat main.
const stewardSeatKind = "seat"

// stewardSeatLauncher reaches the launch manager the work verbs use.
type stewardSeatLauncher struct {
	manager func() *launch.Manager
	// start is the manager's Start; a test records the spec instead.
	start func(launch.StartSpec) (launch.Record, error)
	// repositoryTop is the Git top of the checkout that holds a path.
	repositoryTop func(string) (string, error)
	// settings are the launch settings of the installation at a state root.
	settings func(stateRoot string) (launch.Settings, error)
	// laneRoot is the checkout the host's landing lane record names.
	laneRoot func() (string, bool, error)
}

// SeatAllowed answers the steward's seat decision (Amendment 1): the host's
// landing lane never starts a seat, whatever its settings, and elsewhere a
// seat starts only when the installation's layered settings turn
// launch.seat.runtime on.
func (l stewardSeatLauncher) SeatAllowed(stateRoot string) (bool, string, error) {
	top, err := l.repositoryTop(stateRoot)
	if err != nil {
		return false, "", fmt.Errorf("the checkout that holds %s cannot be read: %w", stateRoot, err)
	}
	laneRoot, registered, err := l.laneRoot()
	if err != nil {
		return false, "", fmt.Errorf("the host's landing lane cannot be read: %w", err)
	}
	if registered {
		named := realpath.Resolve(filepath.Clean(laneRoot))
		if named == realpath.Resolve(filepath.Clean(top)) || named == realpath.Resolve(filepath.Clean(stateRoot)) {
			return false, "this checkout is the host's landing lane, and the landing lane never starts a seat", nil
		}
	}
	settings, err := l.settings(stateRoot)
	if err != nil {
		return false, "", err
	}
	if settings.SeatRuntime == launch.SeatRuntimeOff {
		return false, launch.SeatRuntimeKey + "=" + launch.SeatRuntimeOff + ": a seat is opt-in; set it to claude, codex or auto in metasystem.conf.local", nil
	}
	return true, "", nil
}

// installationSettings are the launch settings of the installation at a
// state root, layered as settings read them: metasystem.conf, then
// metasystem.conf.local, then the environment.
func installationSettings(stateRoot string) (launch.Settings, error) {
	return launch.ResolveSettings(filepath.Join(stateRoot, "metasystem.conf"), launchLookupEnv)
}

// hostLandingLaneRoot is the checkout the host's landing lane record names.
func hostLandingLaneRoot() (string, bool, error) {
	home, err := board.Home()
	if err != nil {
		return "", false, err
	}
	record, ok, err := lane.Read(home)
	return record.Root, ok, err
}

func newStewardSeatLauncher() stewardSeatLauncher {
	return stewardSeatLauncher{manager: func() *launch.Manager { return launchManager() }, repositoryTop: stateroot.RepositoryTop,
		settings: installationSettings, laneRoot: hostLandingLaneRoot}
}

// StartSeat starts the seat kind with the id, brief and tag the steward
// chose. The seat runs at the top of the checkout that holds the steward's
// state root, where a person starts a session (the hooks find the
// installation from there), and binds to the fence that state root keeps.
// The launch names no goal, for a goal id names no checkout.
func (l stewardSeatLauncher) StartSeat(spec steward.SeatLaunchSpec) error {
	allowed, reason, err := l.SeatAllowed(spec.StateRoot)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("no seat starts in %s: %s", spec.StateRoot, reason)
	}
	top, err := l.repositoryTop(spec.StateRoot)
	if err != nil {
		return fmt.Errorf("the checkout that holds %s cannot be read: %w", spec.StateRoot, err)
	}
	start := l.start
	if start == nil {
		// The seat runs on the installation's own settings, which turned
		// it on, not on those the engine binary's folder would resolve.
		settings, err := l.settings(spec.StateRoot)
		if err != nil {
			return err
		}
		manager := *l.manager()
		manager.Settings, manager.SettingsError = settings, nil
		start = manager.Start
	}
	_, err = start(launch.StartSpec{ID: spec.ID, Kind: stewardSeatKind,
		WorkingDirectory: top, FenceRoot: spec.StateRoot, Brief: spec.Brief, Tag: spec.Tag})
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
