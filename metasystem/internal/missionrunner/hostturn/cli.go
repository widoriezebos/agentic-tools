package hostturn

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	// The host ports (claude-result, devin-return, the fake results)
	// register from internal/host.
	_ "github.com/widoriezebos/agentic-tools/metasystem/internal/host"
)

// runCLI runs the runtime CLI in the installation root: stdin from the
// prompt, stdout into stdoutPath, stderr into the turn's host log (appended,
// or truncated first when truncateLog). It returns the shell status.
func (t *Turn) runCLI(argv []string, stdoutPath string, truncateLog bool) int {
	command := exec.Command(argv[0], argv[1:]...)
	if path, err := t.d.LookPath(argv[0]); err == nil {
		command.Path = path
	}
	command.Dir = t.d.Root
	command.Env = t.d.Environ
	stdin, err := os.Open(t.Prompt)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer stdin.Close()
	command.Stdin = stdin
	stdout, err := os.OpenFile(stdoutPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer stdout.Close()
	command.Stdout = stdout
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	if truncateLog {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	log, err := os.OpenFile(t.Path("host.log"), flags, 0o644)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer log.Close()
	command.Stderr = log
	return shellStatus(command.Run())
}

func shellStatus(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
		return 128 + int(signalOf(exitErr))
	}
	return 127
}

func signalOf(exitErr *exec.ExitError) syscall.Signal {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return status.Signal()
	}
	return 0
}
