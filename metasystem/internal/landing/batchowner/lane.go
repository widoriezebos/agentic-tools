package batchowner

// The host's one landing lane (batch-lane design U12): every seat of this
// host resolves the landing checkout through the host's record under
// ~/.metasystem/host, one batch proves at a time on the host, and the
// steward keeps the lane's owner alive. internal/landing/lane owns the
// record, the flocks, the keeper and the view; this file binds them to the
// command's roots, its owner and its clock.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
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

// LandingOwnerLaneRoot is the lane the owner of repo would serve: the
// host's registered lane checked against its own setting, "" when there is
// none (a host with no home keeps no host state: its own setting decides).
// A checkout never registers itself; a setting that names another
// checkout than the host's lane is refused, so the owner of a checkout that
// is not the lane never runs. paused says a person paused the lane (landing
// stop), or its pause cannot be read.
func LandingOwnerLaneRoot(home func() (string, error), metasystemRoot, repo string, now time.Time) (root string, paused bool, err error) {
	raw, _, err := config.Get(config.GetParams{Key: config.BatchRootKey, ConfPath: filepath.Join(metasystemRoot, "metasystem.conf"), Default: "", DefaultSet: true})
	if errors.Is(err, os.ErrNotExist) {
		// No installation here, so no owner.
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	raw, err = realpath.Absolute(strings.TrimSpace(raw))
	if err != nil {
		return "", false, err
	}
	laneHome, homeErr := home()
	if homeErr != nil {
		// No host state: the checkout's own setting decides (pre-U12).
		return raw, false, nil
	}
	found, err := lane.Resolve(laneHome, raw)
	if err != nil {
		return "", false, err
	}
	_, paused = lane.ReadPause(laneHome)
	return found.Root, paused, nil
}

// LandingCheckoutPresent refuses a landing checkout that is gone: its owner
// is never started there and its directories are never created again.
func LandingCheckoutPresent(root string) error {
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return &lane.Refusal{Code: lane.CodeGone,
			Message: fmt.Sprintf("the landing checkout %s no longer exists; its owner was not started and nothing was created there", root),
			Fix:     "restore that checkout, or a person registers the lane that replaces it: metasystem landing set PATH"}
	}
	return nil
}

// LandingLaneProving is the owner's probe of the proving flock: nil when
// this host keeps no lane home, so nothing is gated.
func LandingLaneProving(home func() (string, error)) func() (string, bool, error) {
	laneHome, err := home()
	if err != nil {
		return nil
	}
	return func() (string, bool, error) { return lane.ProbeProving(laneHome) }
}

// HoldHostProvingFlag asks an internal test run to hold the host's proving
// flock for its whole life (U12).
const HoldHostProvingFlag = "--hold-host-proving"

// BatchProofCommand is the one launcher of a batch's proof children: the tip
// proof, the red diagnosis and the held-trunk-red clearing. The child holds
// the host's proving flock for its life, waiting while another proof holds
// it, and the kernel releases it when the child ends, so one proof runs at a
// time on the host whatever happens to the owner that launched it.
func BatchProofCommand(binary string, args []string, spare bool) *exec.Cmd {
	argv := slices.Clone(args)
	// spare: the early proof alone launches without the lock, because
	// speculative work on spare capacity must never delay a real proof.
	if !spare {
		argv = append(argv, HoldHostProvingFlag)
	}
	return exec.Command(binary, argv...)
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

// LandingLaneKeeper is the steward's keeper step: nil without a lane home.
func LandingLaneKeeper(home func() (string, error)) func() string {
	laneHome, err := home()
	if err != nil {
		return nil
	}
	return landingLaneKeeper(laneHome).Step
}

func landingLaneKeeper(laneHome string) lane.Keeper {
	return lane.Keeper{Home: laneHome, Now: func() time.Time { return time.Now().UTC() },
		Inspect: func(root string) (bool, error) {
			probe, err := LandingLaneOwnerProbe(root)
			return probe.Alive, err
		},
		Start: EnsureBatchOwner, Ready: LandingLaneReady}
}

// LandingLaneReady says whether an owner could run in the landing checkout
// at root: the checkout names its machine (the owner signs what it lands
// with it) and its supervision runs (nothing else starts or keeps the
// owner). Each missing one is a *lane.Refusal naming the one fix.
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

// LandingLaneOwnerProbe reads whether the lane's owner runs, and since when.
func LandingLaneOwnerProbe(root string) (lane.OwnerProbe, error) {
	pid, state, err := BatchOwnerEnsure.Inspect(root)
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
	return lane.BuildView(LandingLaneViewSources(laneHome, now))
}

// LandingLaneViewSources are the production reads of the lane's view.
func LandingLaneViewSources(laneHome string, now time.Time) lane.ViewSources {
	return lane.ViewSources{Home: laneHome, Now: now, Owner: LandingLaneOwnerProbe, Ready: LandingLaneReady, Helm: helm.Active}
}

// EndLaneOwner ends the lane's running owner for a restart: the process the
// checkout lease's holder recorded, proven by its exact identity (pid, start
// time and boot) immediately before a SIGTERM, never found by name. The
// owner releases its lease on TERM and its supervision launches a fresh
// one from the supervision owner's executable path. It returns the ended
// pid, 0 when no owner held the lane, and waits up to 15 seconds for the
// process to be gone.
func EndLaneOwner(root string) (int64, error) {
	holder, err := lease.CurrentHolder(root)
	if errors.Is(err, lease.ErrLeaseAbsent) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if holder.OwnerLineage != LandingOwnerLineage {
		return 0, fmt.Errorf("the landing checkout is held by session %s, not by the landing lane; nothing was ended", holder.OwnerLineage)
	}
	prober := identity.KernelProber{}
	for _, announcement := range lease.AnnouncementsFor(root, holder.Pid) {
		if announcement.MainId != holder.MainId {
			continue
		}
		ref := identity.Ref{Pid: announcement.Pid, StartedAtSec: announcement.PidStartedAt, StartTicks: announcement.PidStartTicks, BootID: announcement.BootID}
		if err := identity.SignalExact(prober, ref, syscall.SIGTERM); errors.Is(err, identity.ErrGone) {
			return 0, nil
		} else if err != nil {
			return holder.Pid, fmt.Errorf("the landing owner pid %d could not be ended: %w", holder.Pid, err)
		}
		// A zombie is ended: it holds no lease or lock, and only its parent
		// (the supervision owner) can reap it.
		for deadline := time.Now().Add(15 * time.Second); identity.LiveRef(prober, ref) == identity.Alive && time.Now().Before(deadline); {
			time.Sleep(100 * time.Millisecond)
		}
		if identity.LiveRef(prober, ref) == identity.Alive {
			return holder.Pid, fmt.Errorf("the landing owner pid %d is still running 15 seconds after it was asked to end", holder.Pid)
		}
		return holder.Pid, nil
	}
	return holder.Pid, fmt.Errorf("the landing owner pid %d has no announcement to prove its identity, so it was not ended", holder.Pid)
}
