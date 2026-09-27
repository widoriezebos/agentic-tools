package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// The arming-side supervision verbs that remain shared outside `up`: the
// reserved-cap ceiling check and generic detached launch.

// runSuperviseLaunchDetached starts a command fully detached — stdin from
// /dev/null, stdout/stderr appended to the log, its own session — and prints
// the child pid.
func runSuperviseLaunchDetached(args []string) int {
	flags := flag.NewFlagSet("supervise launch-detached", flag.ContinueOnError)
	log := flags.String("log", "", "log file the child's output appends to (default /dev/null)")
	cwd := flags.String("cwd", "", "working directory for the child (optional)")
	executionGuardRoot := flags.String("execution-guard-root", "", "checkout whose execution guard registers the detached child")
	executionGuardOwner := flags.String("execution-guard-owner", "", "human-readable execution guard member name")
	var env []string
	flags.Func("env", "KEY=VALUE to add to the child's environment (repeatable)", func(value string) error {
		env = append(env, value)
		return nil
	})
	if flags.Parse(args) != nil {
		return 2
	}
	argv := flags.Args()
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "supervise launch-detached: a command is required")
		return 2
	}
	logPath := *log
	if logPath == "" {
		logPath = os.DevNull
	}
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer logFile.Close()
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer devNull.Close()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = *cwd
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *executionGuardRoot != "" || *executionGuardOwner != "" {
		if *executionGuardRoot == "" || *executionGuardOwner == "" {
			_ = cmd.Process.Kill()
			fmt.Fprintln(os.Stderr, "supervise launch-detached: --execution-guard-root and --execution-guard-owner are required together")
			return 2
		}
		if err := gaterun.RegisterSpawnedExecutionGuardMember(*executionGuardRoot, int64(cmd.Process.Pid), *executionGuardOwner); err != nil {
			_ = cmd.Process.Kill()
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	fmt.Println(cmd.Process.Pid)
	// The child is its own session; it is not waited on here.
	_ = cmd.Process.Release()
	return 0
}

// runSuperviseWatchdogReport relays `supervise watchdog-report`: the health
// judgment lives in supervise.WatchdogReport, and this verb prints its
// lines — nothing when everything is healthy.
func runSuperviseWatchdogReport(args []string) int {
	flags := flag.NewFlagSet("supervise watchdog-report", flag.ContinueOnError)
	repo := pathFlag(flags, "repo", "", "checkout root")
	if flags.Parse(args) != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(os.Stderr, "supervise watchdog-report: --repo is required")
		return 2
	}
	if lines := supervise.WatchdogReport(*repo, time.Now()); len(lines) > 0 {
		fmt.Println(strings.Join(lines, "\n"))
	}
	return 0
}
