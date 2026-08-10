package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// runIdentityStartedAt prints a pid's start time in epoch seconds on
// stdout, exiting 1 with no output when the pid cannot be read.
func runIdentityStartedAt(args []string) int {
	flags := flag.NewFlagSet("identity started-at", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	if flags.Parse(args) != nil {
		return 2
	}
	exact, state, err := identity.KernelProber{}.Probe(*pid)
	if err != nil || state != identity.Alive {
		return 1
	}
	fmt.Println(exact.StartedAt.Unix())
	return 0
}

// runIdentityProbe prints the full exact identity as JSON, used by
// fixtures and diagnostics.
func runIdentityProbe(args []string) int {
	flags := flag.NewFlagSet("identity probe", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	if flags.Parse(args) != nil {
		return 2
	}
	exact, state, err := identity.KernelProber{}.Probe(*pid)
	result := map[string]any{"pid": *pid, "liveness": state.String()}
	if err != nil {
		result["error"] = err.Error()
	}
	if state == identity.Alive {
		result["startedAt"] = exact.StartedAt.Format("2006-01-02T15:04:05.000000Z07:00")
		result["startedAtUnix"] = exact.StartedAt.Unix()
		result["startedAtUnixMicro"] = exact.StartedAt.UnixMicro()
		result["argv"] = exact.Argv
	}
	encoded, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		fmt.Fprintln(os.Stderr, "identity probe:", marshalErr)
		return 1
	}
	fmt.Println(string(encoded))
	if state == identity.Unknown {
		return 1
	}
	return 0
}
