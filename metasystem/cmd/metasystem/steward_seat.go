package main

// The steward's seat launcher (g1-s77, D-seat): the steward starts its seat
// main as the launch lane's seat kind, in the checkout, and reads the launch
// back to reap it. The seat kind itself, its environment and its fence claim
// are the launch lane's (internal/launch).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
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
	// settings are the launch settings of an installation.
	settings func(installationRoot string) (launch.Settings, error)
	// laneRoot is the checkout the host's landing lane record names.
	laneRoot func() (string, bool, error)
}

// SeatAllowed answers the steward's seat decision (Amendment 1): the host's
// landing lane never starts a seat, whatever its settings, and elsewhere a
// seat starts only when the installation's layered settings turn
// launch.seat.runtime on. The steward asks with its own root, which holds
// both the checkout's state and the installation's settings.
func (l stewardSeatLauncher) SeatAllowed(root string) (bool, string, error) {
	return l.seatAllowed(root, root)
}

// seatAllowed reads the landing lane against the checkout that holds the
// state root and the settings from the installation's metasystem.conf.
func (l stewardSeatLauncher) seatAllowed(stateRoot, installationRoot string) (bool, string, error) {
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
	settings, err := l.settings(installationRoot)
	if err != nil {
		return false, "", err
	}
	if settings.SeatRuntime == launch.SeatRuntimeOff {
		return false, launch.SeatRuntimeKey + "=" + launch.SeatRuntimeOff + ": a seat is opt-in; set it to claude, codex or auto in metasystem.conf.local", nil
	}
	return true, "", nil
}

// installationSettings are the launch settings of an installation, layered
// as settings read them: metasystem.conf, then metasystem.conf.local, then
// the environment.
func installationSettings(installationRoot string) (launch.Settings, error) {
	return launch.ResolveSettings(filepath.Join(installationRoot, "metasystem.conf"), launchLookupEnv)
}

// hostLandingLaneRoot is the checkout the host's landing lane record names.
func hostLandingLaneRoot() (string, bool, error) {
	home, err := board.Home()
	if err != nil {
		return "", false, err
	}
	return laneRootAt(home)()
}

// laneRootAt reads the checkout the lane record under home names; a record
// an older engine wrote still names it, so the lane starts no seat whoever
// registered it.
func laneRootAt(home string) func() (string, bool, error) {
	return func() (string, bool, error) {
		record, ok, _, err := lane.ReadGuarded(home)
		return record.Root, ok, err
	}
}

func newStewardSeatLauncher() stewardSeatLauncher {
	return stewardSeatLauncher{manager: func() *launch.Manager { return launchManager() }, repositoryTop: stateroot.RepositoryTop,
		settings: installationSettings, laneRoot: hostLandingLaneRoot}
}

func init() { steward.HealthSeatLauncher = newStewardSeatLauncher() }

// StartSeat starts the seat kind with the id, brief and tag the steward
// chose. The seat runs at the top of the checkout that holds the steward's
// state root, where a person starts a session (the hooks find the
// installation from there), and binds to the fence that state root keeps.
// The launch names no goal, for a goal id names no checkout.
func (l stewardSeatLauncher) StartSeat(spec steward.SeatLaunchSpec) error {
	allowed, reason, err := l.seatAllowed(spec.StateRoot, spec.Installation)
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
		settings, err := l.settings(spec.Installation)
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
		ResultPath: filepath.Join(dir, "result.json"), FinishedAt: record.FinishedAt, Runtime: record.Adapter, Model: launchModel(record)}, nil
}

// wireStewardSeat arms the runner's tick with the seat launcher: the
// steward starts its seat main when ready work has no seat.
func wireStewardSeat(config *steward.TickConfig, supplied ...intentOwners) {
	config.Seat = newStewardSeatLauncher()
	owners := defaultIntentOwners()
	if len(supplied) > 0 {
		owners = supplied[0]
	}
	config.DriveWork = func(root string) error {
		// The resident observes every run and starts work only under current agent authority.
		automatic := owners
		automatic.prove = nil
		inv := &intentInvocation{cwd: root, owners: automatic}
		if problem := inv.selectRoot(); problem != nil {
			return errors.New(problem.Summary)
		}
		runner := inv.unitRunner()
		trees, err := inv.registeredWorktrees()
		if err != nil {
			return err
		}
		var work []launch.NamedWork
		var failures error
		for tree := range trees {
			units, err := runner.NamedWork(tree, "")
			if err != nil {
				failures = errors.Join(failures, fmt.Errorf("worktree %s: %w", tree, err))
			}
			work = append(work, units...)
		}
		var ready []launch.NamedWork
		for _, one := range work {
			if one.Record == nil || !one.Running() || one.Record.State == "cancelled" {
				continue
			}
			result, err := runner.Continue(launch.UnitRequest{Resume: one.Run, NonBlocking: true, ObserveOnly: true})
			if err != nil && !errors.Is(err, launch.ErrUnitObserving) {
				failures = errors.Join(failures, fmt.Errorf("run %s: %w", one.Run, err))
				continue
			}
			one.Record = &result.Record
			if len(result.Record.Rounds) == 0 {
				continue
			}
			canAdvance := true
			for _, step := range result.Record.Rounds[len(result.Record.Rounds)-1].Steps {
				if step.State == launch.StepPassed || step.State == launch.StepSkipped {
					continue
				}
				canAdvance = step.State != launch.StepRunning
				break
			}
			if canAdvance {
				ready = append(ready, one)
			}
		}
		if len(ready) == 0 {
			return failures
		}
		allowed, err := runner.ContinuePolicy()
		if err != nil || !allowed {
			return errors.Join(failures, err)
		}
		sort.Slice(ready, func(i, j int) bool {
			left, _ := time.Parse(time.RFC3339Nano, ready[i].Record.Rounds[0].Steps[0].StartedAt)
			right, _ := time.Parse(time.RFC3339Nano, ready[j].Record.Rounds[0].Steps[0].StartedAt)
			if left.Equal(right) {
				return ready[i].Run < ready[j].Run
			}
			return left.Before(right)
		})
		for _, one := range ready {
			before := activeUnitSteps(one.Record)
			_, err = runner.Continue(launch.UnitRequest{Resume: one.Run, NonBlocking: true})
			if err != nil && !errors.Is(err, launch.ErrUnitObserving) {
				failures = errors.Join(failures, fmt.Errorf("run %s: %w", one.Run, err))
				// An error after a launch still started a step: one start per tick.
				after, observeErr := runner.Continue(launch.UnitRequest{Resume: one.Run, NonBlocking: true, ObserveOnly: true})
				if observeErr != nil && !errors.Is(observeErr, launch.ErrUnitObserving) || activeUnitSteps(&after.Record) != before {
					break
				}
				continue
			}
			break
		}
		return failures
	}
	config.ReviewWork = func(root string) error {
		inv := &intentInvocation{cwd: root, owners: owners}
		return inv.driveUnitReviews()
	}
	config.Units = func(root, id string) ([]steward.UnitStage, error) {
		inv := &intentInvocation{cwd: root, owners: owners}
		if problem := inv.selectRoot(); problem != nil {
			return nil, errors.New(problem.Summary)
		}
		_, _, _, _, units, problem := inv.goalUnitStages(id)
		if problem != nil {
			return nil, errors.New(problem.Summary)
		}
		return units, nil
	}
	config.CompletedBoundary = func(root string, now time.Time) error {
		holder, err := lease.CurrentHolder(root)
		if errors.Is(err, lease.ErrLeaseAbsent) {
			return nil
		}
		if err != nil {
			return err
		}
		if holder.MainId == "" {
			return nil
		}
		var started time.Time
		for _, announcement := range lease.AnnouncementsFor(root, holder.Pid) {
			if announcement.MainId == holder.MainId {
				started = time.Unix(announcement.PidStartedAt, 0)
			}
		}
		if holder.SessionId == "" || started.IsZero() {
			return nil
		}
		inv := &intentInvocation{cwd: root, owners: owners}
		if problem := inv.selectRoot(); problem != nil {
			return errors.New(problem.Summary)
		}
		projection, _, problem := inv.projection()
		if problem != nil {
			return errors.New(problem.Summary)
		}
		machine, err := owners.dependencies.machine(inv.stateRoot)
		if err != nil {
			return err
		}
		work, err := goal.ClaimableWorkFromProjection(projection, machine, identity.KernelProber{})
		if err != nil {
			return err
		}
		home := config.WorkStateRoot
		if home == "" {
			home, err = steward.HomeStateRoot()
		}
		if err != nil {
			return err
		}
		if err := steward.ObserveUnitBoundary(root, home, holder.SessionId, started, work, config.Units, now); err != nil {
			return err
		}
		return steward.FinishUnitHandoff(root, holder.SessionId)
	}
}

func stewardProviderProbe(top string, settings func(string) (launch.Settings, error), output func(*exec.Cmd) ([]byte, error)) (bool, error) {
	seat, err := settings(top)
	if err != nil || seat.SeatRuntime != "claude" || seat.SeatModel == "" {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", "-p", "--model", seat.SeatModel, "--output-format", "json")
	cmd.Dir, cmd.Stdin = diskstore.ProcessTempRoot(), strings.NewReader("Reply with the single word ok.")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, launch.KindEnv+"=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, launch.KindEnv+"=probe")
	data, runErr := output(cmd)
	var result struct {
		Type    string `json:"type"`
		IsError *bool  `json:"is_error"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, errors.Join(runErr, err)
	}
	if result.Type == "result" && result.IsError != nil && !*result.IsError {
		return true, nil
	}
	return false, runErr
}

// hostSeatCapped reports whether the steward of any seat of this host has
// stopped starting seats for goal id under the approval approvalOpid. A seat
// whose records can't be read caps nothing: the approve stays the repeat it
// would be without this reading.
func hostSeatCapped(id, approvalOpid string) bool {
	home, err := board.Home()
	if err != nil {
		return false
	}
	entries, err := os.ReadDir(board.Dir(home))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		installation, ok := board.SeatInstallation(home, entry.Name())
		if !ok {
			continue
		}
		if capped, err := steward.SeatCapped(installation, id, approvalOpid); err == nil && capped {
			return true
		}
	}
	return false
}

// activeUnitSteps counts the latest round's steps that are starting or running.
func activeUnitSteps(record *launch.UnitRunRecord) int {
	if record == nil || len(record.Rounds) == 0 {
		return 0
	}
	count := 0
	for _, step := range record.Rounds[len(record.Rounds)-1].Steps {
		if step.State == launch.StepStarting || step.State == launch.StepRunning {
			count++
		}
	}
	return count
}
