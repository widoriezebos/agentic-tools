package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
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

// runProcGroupMembers prints the live pids of a process group, optionally
// excluding one — the kill-domain enumeration the job supervisor's
// deadline enforcement sweeps. Indeterminable membership (any probe
// failure but ESRCH) refuses with exit 1: a sweep must never act on an
// undercount.
func runProcGroupMembers(args []string) int {
	flags := flag.NewFlagSet("proc group-members", flag.ContinueOnError)
	pgid := flags.Int64("pgid", 0, "process group id")
	except := flags.Int64("except", 0, "pid to exclude (the caller itself)")
	if flags.Parse(args) != nil {
		return 2
	}
	if *pgid < 1 {
		fmt.Fprintln(os.Stderr, "usage: metasystem proc group-members --pgid P [--except PID]")
		return 2
	}
	// The enumeration must not count its own invocation chain: this
	// process and the shell substitutions that spawned it live inside the
	// caller's group while enumerating, and a domain that contains its
	// own prober can never be proven empty. Walk self's ancestry up to
	// the excluded caller and exclude the whole chain.
	exclusions := []int64{*except}
	for pid := int64(os.Getpid()); pid > 1 && pid != *except; {
		exclusions = append(exclusions, pid)
		parent, ok := identity.ParentPid(pid)
		if !ok {
			break
		}
		pid = parent
	}
	members, err := supervise.GroupMemberPids(*pgid, exclusions...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, pid := range members {
		fmt.Println(pid)
	}
	return 0
}
