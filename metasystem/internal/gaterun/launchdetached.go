package gaterun

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// DetachedLaunch is one fully detached start: stdin from /dev/null, stdout and
// stderr appended to Log (default /dev/null), its own session, and optional
// membership in a checkout execution guard.
type DetachedLaunch struct {
	Argv []string
	Dir  string
	Log  string
	// Env adds KEY=VALUE entries to the inherited environment.
	Env []string
	// GuardRoot and GuardOwner register the started process as a member of
	// the checkout's execution guard while the launcher is still in its
	// native ancestry; both or neither.
	GuardRoot, GuardOwner string
}

// ErrGuardPairIncomplete is a launch naming only one of the guard root and
// owner.
var ErrGuardPairIncomplete = errors.New("--execution-guard-root and --execution-guard-owner are required together")

// LaunchDetached starts the process and returns its pid, which is also its
// process group id. The process is never waited on here: a background wait
// collects its exit status so a resident launcher leaves no zombie group
// leader behind.
func LaunchDetached(launch DetachedLaunch) (int64, error) {
	if len(launch.Argv) == 0 {
		return 0, errors.New("a command is required")
	}
	if (launch.GuardRoot == "") != (launch.GuardOwner == "") {
		return 0, ErrGuardPairIncomplete
	}
	logPath := launch.Log
	if logPath == "" {
		logPath = os.DevNull
	}
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()
	command := exec.Command(launch.Argv[0], launch.Argv[1:]...)
	command.Stdin, command.Stdout, command.Stderr = devNull, logFile, logFile
	command.Dir = launch.Dir
	if len(launch.Env) > 0 {
		command.Env = append(os.Environ(), launch.Env...)
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return 0, err
	}
	pid := int64(command.Process.Pid)
	if launch.GuardRoot != "" {
		if err := RegisterSpawnedExecutionGuardMember(launch.GuardRoot, pid, launch.GuardOwner); err != nil {
			_ = command.Process.Kill()
			_ = command.Wait()
			return 0, err
		}
	}
	go func() { _ = command.Wait() }()
	return pid, nil
}
