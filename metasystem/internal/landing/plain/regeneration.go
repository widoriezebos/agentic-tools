package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type RunningRegeneration struct {
	Goal     string   `json:"goal"`
	SHA      string   `json:"sha"`
	Command  []string `json:"command"`
	Log      string   `json:"log"`
	Since    string   `json:"since"`
	Pid      int64    `json:"pid"`
	Process  string   `json:"process"`
	State    string   `json:"state"`
	LogBytes int64    `json:"log_bytes"`
}

func regenerationRunningPath(install string) string {
	return filepath.Join(Dir(install), "regenerating.json")
}

func ReadRunningRegeneration(install string, seams ProveSeams) (*RunningRegeneration, error) {
	data, err := os.ReadFile(regenerationRunningPath(install))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var running RunningRegeneration
	if err := json.Unmarshal(data, &running); err != nil {
		return nil, err
	}
	running.State = "running"
	if !seams.alive(Running{Pid: running.Pid, Process: running.Process}) {
		running.State = "died"
	}
	info, err := os.Stat(running.Log)
	if err != nil {
		return &running, err
	}
	running.LogBytes = info.Size()
	return &running, nil
}

func runRegeneration(home, install string, running RunningRegeneration, argv []string, dir string, log *os.File, run func([]string, string, *os.File, func(int64) error) error) error {
	// Publication and pause serialize: stop sees the command before it
	// can pause the lane, or this run sees the pause before starting it.
	held, err := lock.File(lane.LockPath(home), 0o600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer func() {
		if held != nil {
			_ = held.Release()
		}
	}()
	if _, paused := lane.ReadPause(home); paused {
		return errors.New("the landing lane is stopped")
	}
	started := func(pid int64) error {
		running.Pid, running.Process = pid, processRef(pid)
		err := writeRegeneration(install, running)
		_ = held.Release()
		held = nil
		return err
	}
	if run == nil {
		run = runRegenerationArgv
	}
	err = run(argv, dir, log, started)
	removeErr := os.Remove(regenerationRunningPath(install))
	if errors.Is(removeErr, os.ErrNotExist) {
		removeErr = nil
	}
	return errors.Join(err, removeErr)
}

func runRegenerationArgv(argv []string, dir string, log *os.File, started func(int64) error) error {
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir, command.Stdout, command.Stderr = dir, log, log
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return err
	}
	if err := started(int64(command.Process.Pid)); err != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Wait()
		return err
	}
	return command.Wait()
}

func commandExit(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if status, ok := exit.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exit.ExitCode()
	}
	return -1
}

// StopRegeneration ends the recorded command and its children after a pause.
// The resolver then restores the merge and returns the goal with its log.
func StopRegeneration(install string) error {
	running, err := ReadRunningRegeneration(install, ProveSeams{})
	if err != nil || running == nil || running.State == "died" {
		return err
	}
	ref, err := identity.ParseRef(running.Process)
	if err != nil {
		return fmt.Errorf("the regeneration's process identity cannot be read: %w", err)
	}
	return identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL, func(pid int, sig syscall.Signal) error {
		return syscall.Kill(-pid, sig)
	})
}
