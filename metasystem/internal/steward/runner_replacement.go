package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// runnerReplacement carries the decision across watcher passes, including
// after the draining runner has removed its startup announcement.
type runnerReplacement struct {
	Cause      string       `json:"cause"`
	Generation int          `json:"generation"`
	Runner     identity.Ref `json:"runner"`
	Phase      string       `json:"phase"`
	PhaseSince time.Time    `json:"phaseSince"`
	ForcedAt   time.Time    `json:"forcedAt,omitempty"`
	Launched   identity.Ref `json:"launched"`
}

func runnerReplacementPath(root string) string {
	return filepath.Join(runnerDir(root), "runner-replacement.json")
}

func readRunnerReplacement(root string) (*runnerReplacement, error) {
	var pending runnerReplacement
	if err := readJSON(runnerReplacementPath(root), &pending); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read runner replacement: %w", err)
	}
	if (pending.Phase != "stopping" && pending.Phase != "launching") || pending.Generation <= 0 || pending.PhaseSince.IsZero() {
		return nil, fmt.Errorf("invalid runner replacement record")
	}
	return &pending, nil
}

func endRunnerReplacement(root string) error {
	err := os.Remove(runnerReplacementPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (r runnerReplacement) stopping() RunnerRepairOutcome {
	return RunnerRepairOutcome{Status: RunnerRepairStopping, Generation: r.Generation, PreviousPid: r.Runner.Pid}
}

func runnerRecordRef(r RunnerRecord) identity.Ref {
	return identity.Ref{Pid: r.Pid, StartedAtSec: r.PidStartedAt, StartTicks: r.StartTicks, BootID: r.BootID}
}

func beginReplacement(root string, pinned *EnrolledBinary, previous RunnerRecord, alive bool, cause string, now time.Time, deps rearmResolverDeps) (RunnerRepairOutcome, error) {
	pending := runnerReplacement{Cause: cause, Generation: pinned.Install.Generation, Phase: "launching", PhaseSince: now}
	if alive {
		exact, state, err := deps.prober.Probe(previous.Pid)
		if err != nil || state == identity.Unknown {
			return RunnerRepairOutcome{}, fmt.Errorf("runner pid %d cannot be inspected before replacement: %w", previous.Pid, errors.Join(err, errors.New("process identity is unknown")))
		}
		if state == identity.Alive && !exact.Zombie && identity.SameIdentity(exact, runnerRecordRef(previous)) {
			pending.Runner, pending.Phase = exact.Ref(), "stopping"
		}
	}
	if pending.Phase == "launching" {
		return launchReplacement(root, pinned, pending, deps)
	}
	if err := writeJSONAtomic(runnerReplacementPath(root), pending); err != nil {
		return RunnerRepairOutcome{}, err
	}
	if err := runnerStopWriter(runnerStopPath(root), []byte("restart\n"), 0o644); err != nil {
		return RunnerRepairOutcome{}, errors.Join(fmt.Errorf("write restart marker before stopping runner pid %d: %w", previous.Pid, err), endRunnerReplacement(root))
	}
	signal := deps.signal
	if signal == nil {
		signal = syscall.Kill
	}
	// Continue a suspended runner so it can receive the termination signal.
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGCONT} {
		if err := signal(int(pending.Runner.Pid), sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			return RunnerRepairOutcome{}, errors.Join(fmt.Errorf("stop runner pid %d for replacement: %w", pending.Runner.Pid, err), endRunnerReplacement(root))
		}
	}
	return pending.stopping(), nil
}

func continueReplacement(root string, pinned *EnrolledBinary, pending runnerReplacement, now time.Time, deps rearmResolverDeps) (RunnerRepairOutcome, error) {
	if pending.Phase == "stopping" {
		switch identity.LiveRef(deps.prober, pending.Runner) {
		case identity.Unknown:
			return RunnerRepairOutcome{}, fmt.Errorf("runner pid %d cannot be inspected during replacement", pending.Runner.Pid)
		case identity.Alive:
			if pending.ForcedAt.IsZero() && !now.Before(pending.PhaseSince.Add(StopDrainBound)) {
				pending.ForcedAt = now
				if err := writeJSONAtomic(runnerReplacementPath(root), pending); err != nil {
					return RunnerRepairOutcome{}, err
				}
				signal := deps.signal
				if signal == nil {
					signal = syscall.Kill
				}
				if err := signal(int(pending.Runner.Pid), syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
					return RunnerRepairOutcome{}, fmt.Errorf("kill stalled runner pid %d for replacement: %w", pending.Runner.Pid, err)
				}
			}
			return pending.stopping(), nil
		}
		pending.Phase, pending.PhaseSince = "launching", now
	}
	if pending.Launched.Pid == 0 {
		return launchReplacement(root, pinned, pending, deps)
	}
	replacement, alive := liveRunnerWithProber(root, deps.prober)
	if alive && replacement.Pid == pending.Launched.Pid && identity.LiveRef(deps.prober, runnerRecordRef(replacement)) == identity.Alive {
		observed := ReadRunnerExecution(root, deps.prober)
		if observed.Digest != pinned.Install.InstallDigest {
			err := fmt.Errorf("replacement runner execution not verified: got %s (%s), want %s", observed.Digest, observed.Cause, pinned.Install.InstallDigest)
			return RunnerRepairOutcome{}, errors.Join(err, endRunnerReplacement(root))
		}
		if err := endRunnerReplacement(root); err != nil {
			return RunnerRepairOutcome{}, err
		}
		return RunnerRepairOutcome{Status: "RESTORED", Generation: pending.Generation, PreviousPid: pending.Runner.Pid, ReplacementPid: replacement.Pid}, nil
	}
	switch identity.LiveRef(deps.prober, pending.Launched) {
	case identity.Dead:
		return RunnerRepairOutcome{}, errors.Join(fmt.Errorf("the runner died before guarding the repository; see %s", runnerLogPath(root)), endRunnerReplacement(root))
	case identity.Unknown:
		return RunnerRepairOutcome{}, fmt.Errorf("runner pid %d cannot be inspected during launch", pending.Launched.Pid)
	}
	return RunnerRepairOutcome{Status: RunnerRepairLaunching, Generation: pending.Generation, PreviousPid: pending.Runner.Pid}, nil
}

func launchReplacement(root string, pinned *EnrolledBinary, pending runnerReplacement, deps rearmResolverDeps) (RunnerRepairOutcome, error) {
	if err := writeJSONAtomic(runnerReplacementPath(root), pending); err != nil {
		return RunnerRepairOutcome{}, err
	}
	// Repairs have no arming caller; the new runner reports no-lease.
	var err error
	if deps.launch == nil {
		pending.Launched, err = startRunner(root, pinned, "")
	} else {
		var replacement RunnerRecord
		replacement, err = deps.launch(root, pinned, "")
		pending.Launched = runnerRecordRef(replacement)
	}
	if err != nil {
		return RunnerRepairOutcome{}, errors.Join(err, endRunnerReplacement(root))
	}
	if err := writeJSONAtomic(runnerReplacementPath(root), pending); err != nil {
		return RunnerRepairOutcome{}, err
	}
	return RunnerRepairOutcome{Status: RunnerRepairLaunching, Generation: pending.Generation, PreviousPid: pending.Runner.Pid}, nil
}
