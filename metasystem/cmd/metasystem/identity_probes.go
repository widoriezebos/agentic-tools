package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

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
