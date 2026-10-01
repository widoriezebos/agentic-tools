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
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// LandingLaneHome is the home the host lane lives under (the board's). An
// error leaves this host on each seat's own landing.batch-root, as before
// U12.
var LandingLaneHome = board.Home

// LandingLaneSeams are the reads a lane resolution makes beyond the lane
// itself: the host's home, and whether a root is a landing checkout.
type LandingLaneSeams struct {
	Home func() (string, error)
	// Validate admits a root as a dedicated landing checkout for the seat
	// and returns its canonical form.
	Validate func(root, seatRoot string, now time.Time) (string, error)
}

func ProductionLandingLaneSeams() LandingLaneSeams {
	return LandingLaneSeams{Home: LandingLaneHome, Validate: ValidateLandingCheckout}
}

// LandingLaneRegistrant names a seat by its enrolled nickname, else its path.
func LandingLaneRegistrant(installation string) string {
	if machine, err := goal.ResolveMachine(installation); err == nil && strings.TrimSpace(machine) != "" {
		return strings.TrimSpace(machine)
	}
	return installation
}

func ValidateLandingCheckout(root, seatRoot string, now time.Time) (string, error) {
	settings, err := config.ResolveExplicitBatchLanding(root, seatRoot, config.DefaultBatchMaxWait, func() time.Time { return now })
	return settings.Root, err
}

// BatchRoot resolves the lane an installation's work joins: the host's
// registered lane (lane.Resolve's table), admitted for a join by the lane's
// gate, which refuses while a person unsets the lane. No registered lane is
// not configured: the seat lands its own work.
func (seams LandingLaneSeams) BatchRoot(installation string, now time.Time) (string, bool, error) {
	root, configured, err := seams.Resolve(installation, now)
	if err != nil || !configured {
		return root, configured, err
	}
	home, err := seams.Home()
	if err != nil {
		// No host state, no host lane: the seat's own setting decided.
		return root, true, nil
	}
	if err := lane.Gate(home, lane.OpJoin, lane.AuthorityAgent, nil); err != nil {
		return "", true, err
	}
	return root, true, nil
}

// LandingLaneRoot is the same resolution for a reader: it never gates.
// helm's report and the steward's trunk-red check read the lane through it.
func LandingLaneRoot(installation string, now time.Time) (string, bool, error) {
	return ProductionLandingLaneSeams().Resolve(installation, now)
}

func init() { steward.LandingLaneRoot = LandingLaneRoot }

// Resolve is the host's registered lane as installation sees it, checked
// against its own landing.batch-root. Only a person's landing set registers
// a lane (design r10 §1): with no record there is no lane, whatever the
// seat names. A host with no home at all keeps no host state, so there the
// seat's own setting decides, as before the host lane (U12); it registers
// nothing.
func (seams LandingLaneSeams) Resolve(installation string, now time.Time) (string, bool, error) {
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
	if found.Root == "" {
		return "", false, nil
	}
	root, err := seams.Validate(found.Root, installation, now)
	if err != nil {
		return "", true, err
	}
	return root, true, nil
}

// HoldHostProvingFlag asks an internal test run to hold the host's proving
// flock for its whole life (U12).
const HoldHostProvingFlag = "--hold-host-proving"

// BatchProofCommand is the one launcher of a batch's proof children
// (landing prove, K6). The child holds the host's proving flock for its
// life, waiting while another proof holds it, and the kernel releases it
// when the child ends, so one proof runs at a time on the host whatever
// happens to the process that launched it.
func BatchProofCommand(binary string, args []string) *exec.Cmd {
	return exec.Command(binary, append(slices.Clone(args), HoldHostProvingFlag)...)
}

// HoldHostProvingFor is internal test run's side of the launcher: with the
// flag it takes the proving flock (waiting while another proof holds it) and
// returns the arguments without the flag; without a lane home it holds
// nothing.
func HoldHostProvingFor(home func() (string, error), args []string) ([]string, func() error, error) {
	rest := slices.DeleteFunc(slices.Clone(args), func(arg string) bool { return arg == HoldHostProvingFlag })
	nothing := func() error { return nil }
	if len(rest) == len(args) {
		return rest, nothing, nil
	}
	laneHome, err := home()
	if err != nil {
		return rest, nothing, nil
	}
	release, err := lane.HoldProving(laneHome)
	if err != nil {
		return rest, nothing, fmt.Errorf("the host's proving lock could not be taken: %w", err)
	}
	return rest, release, nil
}

// LandingAgentLive names a landing agent launch on this computer that has
// not ended; the engine supplies it (the launch store is the command
// layer's). nil is none.
var LandingAgentLive func() (id string, live bool, err error)

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
		return lane.View{Owner: lane.OwnerView{State: lane.OwnerNotStarted}, Summary: "the landing lane cannot be read: this host has no home for it (" + err.Error() + ")"}
	}
	return lane.BuildView(LandingLaneViewSources(laneHome, now))
}

// LandingLaneViewSources are the production reads of the lane's view as the
// board shows it. Its wake omits validation due on purpose: that read
// projects the goal ledger, which a page polled every few seconds does not
// pay for; landing status --json carries the whole wake.
func LandingLaneViewSources(laneHome string, now time.Time) lane.ViewSources {
	return lane.ViewSources{Home: laneHome, Now: now, Owner: landingAgentProbe, Ready: LandingLaneReady, Helm: helm.Active}
}
