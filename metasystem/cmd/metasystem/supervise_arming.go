package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
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
	pid, err := gaterun.LaunchDetached(gaterun.DetachedLaunch{
		Argv: argv, Dir: *cwd, Log: *log, Env: env,
		GuardRoot: *executionGuardRoot, GuardOwner: *executionGuardOwner,
	})
	if errors.Is(err, gaterun.ErrGuardPairIncomplete) {
		fmt.Fprintln(os.Stderr, "supervise launch-detached:", err)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(pid)
	return 0
}
