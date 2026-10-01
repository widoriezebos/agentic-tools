package batchowner

// The host's one landing lane (batch-lane design U12): every seat of this
// host resolves the landing checkout through the host's record under
// ~/.metasystem/host, and one batch proves at a time on the host.
// internal/landing/lane owns the record, the flocks and the view; this file
// binds them to the command's roots, the landing agent and the clock.

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// LandingLaneHome is the home the host lane lives under (the board's). An
// error leaves this host on each seat's own landing.batch-root, as before
// U12.
var LandingLaneHome = board.Home

// LandingLaneSeams are the reads a lane resolution makes beyond the lane
// itself: the host's home.
type LandingLaneSeams struct {
	Home func() (string, error)
}

func ProductionLandingLaneSeams() LandingLaneSeams {
	return LandingLaneSeams{Home: LandingLaneHome}
}

// LandingLaneRegistrant names a seat by its enrolled nickname, else its path.
func LandingLaneRegistrant(installation string) string {
	if machine, err := goal.ResolveMachine(installation); err == nil && strings.TrimSpace(machine) != "" {
		return strings.TrimSpace(machine)
	}
	return installation
}

// ValidateLandingCheckout admits root as a dedicated landing checkout for
// the seat at seatRoot (landing set) and returns its canonical form.
func ValidateLandingCheckout(root, seatRoot string, now time.Time) (string, error) {
	settings, err := config.ResolveExplicitBatchLanding(root, seatRoot, config.DefaultBatchMaxWait, func() time.Time { return now })
	return settings.Root, err
}

// LandingLaneRoot is the lane installation lands through, for a seat's work
// land and for a reader alike (helm's report, the steward's trunk-red check).
func LandingLaneRoot(installation string, now time.Time) (string, bool, error) {
	return ProductionLandingLaneSeams().BatchRoot(installation, now)
}

func init() { steward.LandingLaneRoot = LandingLaneRoot }

// BatchRoot is the lane installation's work joins: the host's registered
// lane, as lane.Resolve reads it against the seat's own landing.batch-root,
// and nothing more. Only a person's landing set registers a lane (design r10
// §1): with no record there is no lane, whatever the seat names, and the
// seat lands its own work. A lane a person stopped or is unsetting is still
// the lane: joins wait in a stopped lane, and an unset confirms every member
// under the host flock before it unregisters. A host with no home at all
// keeps no host state, so there the seat's own setting decides, as before
// the host lane (U12); it registers nothing.
func (seams LandingLaneSeams) BatchRoot(installation string, now time.Time) (string, bool, error) {
	confPath := filepath.Join(installation, "metasystem.conf")
	raw, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: confPath, Default: "", DefaultSet: true})
	if err != nil {
		return "", false, err
	}
	raw = strings.TrimSpace(raw)
	home, homeErr := seams.Home()
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
	found, err := lane.Resolve(home, raw)
	if err != nil {
		return "", true, err
	}
	return found.Root, found.Root != "", nil
}

// LandingAgentProbe reads whether the host lane's landing agent runs, and
// since when: the lane's view shows it as the lane's runner. The
// engine supplies it (the launch store is the command layer's); nil reads
// no agent running.
var LandingAgentProbe func(root string) (lane.OwnerProbe, error)

func landingAgentProbe(root string) (lane.OwnerProbe, error) {
	if LandingAgentProbe == nil {
		return lane.OwnerProbe{}, nil
	}
	return LandingAgentProbe(root)
}

// LandingLaneReady says whether the lane can run in the landing checkout at
// root: the checkout names its machine (the lane's claims carry it) and its
// supervision runs (its steward wakes the landing agent). Each missing one
// is a *lane.Refusal naming the one fix.
func LandingLaneReady(root string) error {
	return landingLaneReady(root, goal.ResolveMachine, LandingLaneArmed)
}

func landingLaneReady(root string, machine func(string) (string, error), armed func(string) (bool, error)) error {
	if _, err := machine(root); err != nil {
		return lane.NoMachineRefusal(root)
	}
	running, err := armed(root)
	if err != nil {
		return err
	}
	if !running {
		return lane.UnarmedRefusal(root)
	}
	return nil
}

// LandingLaneArmed reports whether the supervision of the landing checkout
// at root runs: its installation's supervision owner record names a live
// process. No record, or a record of a process that is gone, is not armed.
func LandingLaneArmed(root string) (bool, error) {
	owner, err := supervise.ReadArmingOwner(batch.ModuleRoot(root))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("the supervision owner record of %s is unreadable: %w", root, err)
	}
	ref := identity.Ref{Pid: owner.Pid, StartedAtSec: owner.PidStartedAt, StartTicks: owner.PidStartTicks, BootID: owner.BootID}
	switch identity.AliveTaggedRef(identity.KernelProber{}, ref, owner.InstanceTag) {
	case identity.Alive:
		return true, nil
	case identity.Dead:
		return false, nil
	}
	return false, fmt.Errorf("whether the supervision owner pid %d of %s runs is unknown", owner.Pid, root)
}

// landingLaneView is the lane as landing status, status and /api/board show
// it; without a lane home, the view says so.
func landingLaneView(home func() (string, error), now time.Time) lane.View {
	laneHome, err := home()
	if err != nil {
		return lane.View{Owner: lane.OwnerView{State: lane.OwnerUnready}, Summary: "the landing lane cannot be read: this host has no home for it (" + err.Error() + ")"}
	}
	return lane.BuildView(LandingLaneViewSources(laneHome, now))
}

// LandingLaneViewSources are the production reads of the lane's view as the
// board shows it.
func LandingLaneViewSources(laneHome string, now time.Time) lane.ViewSources {
	return lane.ViewSources{Home: laneHome, Now: now, Owner: landingAgentProbe, Ready: LandingLaneReady, Wake: plain.KeeperWake(laneHome)}
}
