package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/janitor"
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

func runIdentityGroupOwned(args []string) int {
	flags := flag.NewFlagSet("proc group-owned", flag.ContinueOnError)
	pgid := flags.Int64("pgid", 0, "process group id")
	tag := flags.String("tag", "", "instance tag required in a shipped argv position")
	root := pathFlag(flags, "root", "", "checkout root for an authorized fake-runtime fallback")
	record := flags.String("record", "", "job record carrying an exact trusted-launcher proof")
	if flags.Parse(args) != nil {
		return 2
	}
	if *pgid < 2 || *tag == "" || (*root == "") != (*record == "") {
		fmt.Fprintln(os.Stderr, "usage: metasystem internal proc group-owned --pgid P --tag TAG [--root R --record FILE]")
		return 2
	}
	switch janitor.GroupOwnership(*pgid, *tag) {
	case janitor.GroupOwned:
		return 0
	case janitor.GroupNotOwned:
		return 1
	}
	if *root == "" {
		return 3
	}
	if err := unix.Kill(int(-*pgid), 0); err != nil && err != unix.EPERM {
		return 1
	}
	authorization, err := fixtureauth.New(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc group-owned:", err)
		return 3
	}
	matches, err := dispatchcore.RecordedGroupProofMatches(
		*record, *pgid, *tag, authorization.GroupOwnership(),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc group-owned:", err)
		return 3
	}
	if matches {
		return 0
	}
	return 3
}

// runProcSetsid makes this process the leader of a new session and then
// replaces it with the command, so the command runs with no controlling
// terminal and nothing of the engine in its ancestry: the way a fixture
// starts a headless process without an interpreter the dependency ratchet
// bans (perl's setsid-then-exec). Usage: proc setsid -- cmd args...
func runProcSetsid(args []string) int {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "proc setsid: a command is required after --")
		return 2
	}
	path, err := exec.LookPath(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "proc setsid:", err)
		return 127
	}
	if _, err := syscall.Setsid(); err != nil {
		fmt.Fprintln(os.Stderr, "proc setsid: new session:", err)
		return 1
	}
	if err := syscall.Exec(path, args, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "proc setsid: exec:", err)
		return 126
	}
	return 0
}
