package main

// The host's one landing lane (batch-lane design U12): every seat of this
// host resolves the landing checkout through the host's record under
// ~/.metasystem/host, one batch proves at a time on the host, and the
// steward keeps the lane's owner alive. internal/landing/lane owns the
// record, the flocks, the keeper and the view; this file binds them to the
// command's roots, its owner and its clock.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

// landingLaneHome is the home the host lane lives under (the board's). An
// error leaves this host on each seat's own landing.batch-root, as before
// U12.
var landingLaneHome = board.Home

// landingLaneSeams are the reads a lane resolution makes beyond the lane
// itself: who registers, and whether a root is a landing checkout.
type landingLaneSeams struct {
	home func() (string, error)
	// by names the seat that registers the lane.
	by func(installation string) string
	// validate admits a root as a dedicated landing checkout for the seat
	// and returns its canonical form.
	validate func(root, seatRoot string, now time.Time) (string, error)
}

func productionLandingLaneSeams() landingLaneSeams {
	return landingLaneSeams{home: landingLaneHome, by: landingLaneRegistrant, validate: validateLandingCheckout}
}

// landingLaneRegistrant names a seat by its enrolled nickname, else its path.
func landingLaneRegistrant(installation string) string {
	if machine, err := goal.ResolveMachine(installation); err == nil && strings.TrimSpace(machine) != "" {
		return strings.TrimSpace(machine)
	}
	return installation
}

func validateLandingCheckout(root, seatRoot string, now time.Time) (string, error) {
	settings, err := config.ResolveExplicitBatchLanding(root, seatRoot, config.DefaultBatchMaxWait, func() time.Time { return now })
	return settings.Root, err
}

// batchRoot resolves the lane an installation lands through: its own
// landing.batch-root against the host's record (lane.Resolve's table). The
// root is admitted as a landing checkout before this seat registers it, so
// no seat registers a checkout that cannot land.
func (seams landingLaneSeams) batchRoot(installation string, now time.Time) (string, bool, error) {
	confPath := filepath.Join(installation, "metasystem.conf")
	raw, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: confPath, Default: "", DefaultSet: true})
	if err != nil {
		return "", false, err
	}
	raw = strings.TrimSpace(raw)
	home, homeErr := seams.home()
	if homeErr != nil {
		if raw == "" {
			return "", false, nil
		}
		settings, err := config.ResolveBatchLanding(confPath, installation, func() time.Time { return now })
		if err != nil {
			return "", true, err
		}
		return settings.Root, true, nil
	}
	by := seams.by(installation)
	found, err := lane.Resolve(home, raw, by, now, false)
	if err != nil {
		return "", true, err
	}
	if found.Root == "" {
		return "", false, nil
	}
	root, err := seams.validate(found.Root, installation, now)
	if err != nil {
		return "", true, err
	}
	if found.Record.Root == "" {
		if _, err := lane.Resolve(home, root, by, now, true); err != nil {
			return "", true, err
		}
	}
	return root, true, nil
}

// landingOwnerLaneRoot is the lane the owner of repo would serve: its own
// setting against the host's record, "" when there is none. A checkout that
// names itself registers itself when the host has no lane; a setting that
// names another checkout than the host's lane is refused, so the owner of a
// checkout that is not the lane never runs. paused says a person paused the
// lane (landing stop).
func landingOwnerLaneRoot(home func() (string, error), metasystemRoot, repo string, now time.Time) (root string, paused bool, err error) {
	raw, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: filepath.Join(metasystemRoot, "metasystem.conf"), Default: "", DefaultSet: true})
	if errors.Is(err, os.ErrNotExist) {
		// No installation here, so no owner.
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	raw, err = resolvePathFlag(strings.TrimSpace(raw))
	if err != nil {
		return "", false, err
	}
	laneHome, homeErr := home()
	if homeErr != nil {
		return raw, false, nil
	}
	found, err := lane.Resolve(laneHome, raw, "landing owner of "+repo, now, raw != "" && realpath.Resolve(raw) == realpath.Resolve(repo))
	if err != nil {
		return "", false, err
	}
	_, paused = lane.ReadPause(laneHome)
	return found.Root, paused, nil
}

// landingLaneProving is the owner's proving seam: nil when this host keeps
// no lane home, so nothing is gated.
func landingLaneProving(home func() (string, error)) func() (func() error, string, error) {
	laneHome, err := home()
	if err != nil {
		return nil
	}
	return func() (func() error, string, error) { return lane.TryProving(laneHome) }
}

// landingLaneKeeper is the steward's keeper step: nil without a lane home.
func landingLaneKeeper(home func() (string, error)) func() string {
	laneHome, err := home()
	if err != nil {
		return nil
	}
	keeper := lane.Keeper{Home: laneHome, Now: func() time.Time { return time.Now().UTC() },
		Inspect: func(root string) (bool, error) {
			probe, err := landingLaneOwnerProbe(root)
			return probe.Alive, err
		},
		Start: ensureBatchOwner}
	return keeper.Step
}

// landingLaneOwnerProbe reads whether the lane's owner runs, and since when.
func landingLaneOwnerProbe(root string) (lane.OwnerProbe, error) {
	pid, state, err := batchOwnerEnsure.inspect(root)
	switch state {
	case identity.Alive:
		probe := lane.OwnerProbe{Alive: true, PID: pid}
		if exact, liveness, probeErr := (identity.KernelProber{}).Probe(pid); probeErr == nil && liveness == identity.Alive {
			probe.Since = exact.StartedAt
		}
		return probe, nil
	case identity.Dead:
		return lane.OwnerProbe{}, nil
	}
	if err == nil {
		err = errors.New("the owner's liveness is unknown")
	}
	return lane.OwnerProbe{}, err
}

// landingLaneView is the lane as landing status, status and /api/board show
// it; without a lane home, the view says so.
func landingLaneView(home func() (string, error), now time.Time) lane.View {
	laneHome, err := home()
	if err != nil {
		return lane.View{Owner: lane.OwnerView{State: lane.OwnerNotStarted}, Summary: "the landing lane cannot be read: this host has no home for it (" + err.Error() + ")"}
	}
	return lane.BuildView(lane.ViewSources{Home: laneHome, Now: now, Owner: landingLaneOwnerProbe})
}
