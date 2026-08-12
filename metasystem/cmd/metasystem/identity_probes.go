package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// runIdentityExists exits 0 when the pid exists — a zero-signal probe where a
// permission denial still proves existence, which a shell kill -0 cannot
// distinguish from no-such-process.
func runIdentityExists(args []string) int {
	flags := flag.NewFlagSet("proc exists", flag.ContinueOnError)
	pid := flags.Int64("pid", 0, "process id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *pid < 1 {
		fmt.Fprintln(os.Stderr, "proc exists: --pid must be positive")
		return 2
	}
	switch unix.Kill(int(*pid), 0) {
	case nil, unix.EPERM:
		return 0
	default:
		return 1
	}
}

// runIdentityGroupExists exits 0 when the process group exists, with the same
// permission-denial-proves-existence rule.
func runIdentityGroupExists(args []string) int {
	flags := flag.NewFlagSet("proc group-exists", flag.ContinueOnError)
	pgid := flags.Int64("pgid", 0, "process group id")
	if flags.Parse(args) != nil {
		return 2
	}
	if *pgid < 1 {
		fmt.Fprintln(os.Stderr, "proc group-exists: --pgid must be positive")
		return 2
	}
	switch unix.Kill(int(-*pgid), 0) {
	case nil, unix.EPERM:
		return 0
	default:
		return 1
	}
}
