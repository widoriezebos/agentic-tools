package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func stateSignal(state *os.ProcessState) (bool, int) {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return false, 0
	}
	return true, int(status.Signal())
}

// child is one process this supervisor started and owns: the runtime CLI, an
// ACP server, or a fixture hold. It stays in the supervisor's process group
// (the kill domain), and its exit is reaped by one waiter so a zombie never
// holds group membership after the supervisor stops it.
type child struct {
	command *exec.Cmd
	pid     int
	done    chan struct{}
	status  int
	// stop, when set, ends an in-process child (the Devin ACP client,
	// which runs as a goroutine of this supervisor) in place of signals.
	stop func()
}

// startChild starts command and its reaper.
func startChild(command *exec.Cmd) (*child, error) {
	if err := command.Start(); err != nil {
		return nil, err
	}
	c := &child{command: command, pid: command.Process.Pid, done: make(chan struct{})}
	go func() {
		c.status = exitStatus(command.Wait())
		close(c.done)
	}()
	return c, nil
}

// alive reports whether the child has not been reaped yet.
func (c *child) alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// wait blocks until the child exits and returns its shell status.
func (c *child) wait() int {
	<-c.done
	return c.status
}

// exitedWithin reports whether the child exits within d.
func (c *child) exitedWithin(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-c.done:
		return true
	case <-timer.C:
		return false
	}
}

// terminate stops the exact child: TERM, up to two seconds for it to exit,
// then KILL, and the reap, so its group membership ends with it.
func (c *child) terminate() {
	if c == nil {
		return
	}
	if c.stop != nil {
		c.stop()
		<-c.done
		return
	}
	_ = c.command.Process.Signal(syscall.SIGTERM)
	if !c.exitedWithin(2 * time.Second) {
		_ = c.command.Process.Signal(syscall.SIGKILL)
	}
	<-c.done
}

// pollTick is the handshake loop's cadence while the CLI starts.
const pollTick = 20 * time.Millisecond

// launch starts the runtime CLI in the round's workspace: stdin from the
// prompt (when named), stdout truncated into the stream file, stderr
// appended to the job log. The child stays in this supervisor's process
// group. setupErr is a failure the shell adapters met inside the launch
// subshell (a scratch directory that could not be made); it fails the launch
// the same way.
func (s *Supervision) launch(argv, env []string, stdinPath, stdoutPath string, setupErr error) (*child, error) {
	if setupErr != nil {
		s.logf("%v\n", setupErr)
		fmt.Fprintf(s.d.Stderr, "%s child exited before custody identity was recorded\n", s.runtime)
		return nil, setupErr
	}
	command := exec.Command(argv[0], argv[1:]...)
	if len(argv) > 0 && !strings.Contains(argv[0], "/") {
		if path, err := s.d.LookPath(argv[0]); err == nil {
			command.Path = path
		}
	}
	command.Dir = s.workspace
	command.Env = env
	if stdinPath != "" {
		stdin, err := os.Open(stdinPath)
		if err != nil {
			s.logf("%v\n", err)
			return nil, err
		}
		defer stdin.Close()
		command.Stdin = stdin
	}
	if stdoutPath != "" {
		stdout, err := os.OpenFile(stdoutPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			s.logf("%v\n", err)
			return nil, err
		}
		defer stdout.Close()
		command.Stdout = stdout
	}
	command.Stderr = s.logWriter()
	c, err := startChild(command)
	if err != nil {
		s.logf("%v\n", err)
		fmt.Fprintf(s.d.Stderr, "%s child exited before custody identity was recorded\n", s.runtime)
		return nil, err
	}
	return c, nil
}

// signalPid sends a named signal to one pid, ignoring delivery failures (a
// member that already left the group).
func signalPid(pid int, name string) {
	signal := syscall.SIGTERM
	if name == "KILL" {
		signal = syscall.SIGKILL
	}
	_ = syscall.Kill(pid, signal)
}
