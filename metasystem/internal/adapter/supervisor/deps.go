// Package supervisor is the delegate-supervisor process entrypoint: the
// persistent owner of one delegate round (dispatch or follow-up) from the
// launch capability through the runtime CLI's exit to the terminal record
// write, and the runtime adapter's small verbs (identity, probe, contract,
// output stream, cancel, selftest). It replaces the shell adapters that
// lived under scripts/agents/adapters (runtime-common.sh and one script per
// runtime).
//
// It is a composition package above the owners (design verbs-object-action
// 6.3): the runtime transformations stay in internal/adapter, the job-record
// owners in internal/dispatch, the ACP wire in internal/acp. The record
// writes the delegate lifecycle's internal callbacks own (__record-cas,
// __handshake, __register-custody, __protocol-error, __repair-claim,
// __cancel-owned) are calls into the engine's delegate entry
// (`ENGINE internal delegate CALLBACK`, internal/delegation), so authority is
// checked exactly where it was: against this supervisor process, the pid the
// job record names, as the callback process's parent.
package supervisor

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// Deps are the process facts and seams one supervisor runs against. The
// production constructor is ProcessDeps; tests replace the seams.
type Deps struct {
	// Root is the installation root: the directory holding scripts/ and
	// artifacts/ (the adapter scripts' `root`).
	Root string
	// Engine is the engine binary children run (the claude SessionStart
	// hook, the fixture holds): METASYSTEM_BIN, else ROOT/bin/metasystem.
	Engine string
	// Self is the running executable, the binary a self-test's delegate
	// children run (the front door admits --adapter-selftest only from a
	// parent of the same executable); empty falls back to Engine.
	Self string
	// Environ is the environment children inherit before per-launch
	// additions.
	Environ []string
	Getenv  func(string) string
	// Pid is this supervisor's process id; the supervisor leads its own
	// process group, so it is also the kill domain's group id.
	Pid            int
	Stdout, Stderr io.Writer
	Clock          Clock
	// Dispatch runs one delegate lifecycle callback or command and returns
	// its exit status.
	Dispatch Dispatcher
	// GroupMembers lists the live members of a process group, except the
	// given pids; an error is an indeterminable enumeration.
	GroupMembers func(pgid int, except ...int) ([]int, error)
	// LookPath resolves a runtime CLI on PATH.
	LookPath func(string) (string, error)
	// Git runs read-only git queries.
	Git GitQuery
}

// Clock is the supervisor's time: wall readings and waits.
type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

// Dispatcher runs one delegate lifecycle command (`ENGINE internal delegate
// ARGS`) with the given output streams.
type Dispatcher interface {
	Run(stdout, stderr io.Writer, args ...string) int
}

type systemClock struct{}

func (systemClock) Now() time.Time        { return time.Now() }
func (systemClock) Sleep(d time.Duration) { time.Sleep(d) }

// SystemClock is the wall clock.
func SystemClock() Clock { return systemClock{} }

// EngineDispatcher execs the installation's delegate entry, `ENGINE internal
// delegate ARGS`, with METASYSTEM_DELEGATE_ROOT naming the installation (the
// retired runtime-common.sh delegate_callback). The callback process is a
// child of this supervisor, so the lifecycle's authority checks see the
// supervisor the job record names as its caller's parent.
type EngineDispatcher struct {
	Root    string
	Engine  string
	Environ []string
}

func (d EngineDispatcher) Run(stdout, stderr io.Writer, args ...string) int {
	command := exec.Command(d.Engine, append([]string{"internal", "delegate"}, args...)...)
	command.Env = append(append([]string(nil), d.Environ...), "METASYSTEM_DELEGATE_ROOT="+d.Root)
	command.Stdout = stdout
	command.Stderr = stderr
	return exitStatus(command.Run())
}

// ProcessDeps builds the production seams for an installation root.
func ProcessDeps(root string) Deps {
	engine := os.Getenv("METASYSTEM_BIN")
	if engine == "" {
		engine = filepath.Join(root, "bin", "metasystem")
	}
	self, err := os.Executable()
	if err != nil {
		self = ""
	}
	environ := os.Environ()
	return Deps{
		Root: root, Engine: engine, Self: self, Environ: environ, Getenv: os.Getenv,
		Pid: os.Getpid(), Stdout: os.Stdout, Stderr: os.Stderr,
		Clock:        SystemClock(),
		Dispatch:     EngineDispatcher{Root: root, Engine: engine, Environ: environ},
		GroupMembers: groupMembers,
		LookPath:     exec.LookPath,
		Git:          runGit,
	}
}

func groupMembers(pgid int, except ...int) ([]int, error) {
	excluded := make([]int64, 0, len(except))
	for _, pid := range except {
		excluded = append(excluded, int64(pid))
	}
	members, err := supervise.GroupMemberPids(int64(pgid), excluded...)
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, len(members))
	for _, pid := range members {
		out = append(out, int(pid))
	}
	return out, nil
}

// lifecycleStatus is the delegate lifecycle's status of one job (its record
// status line), empty when the lifecycle refuses or fails.
func (d Deps) lifecycleStatus(job string) string {
	var out bytes.Buffer
	if d.Dispatch.Run(&out, io.Discard, "status", "--job", job) != 0 {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// lifecycleReap is the delegate lifecycle's single-job reap; its outcome is
// tolerated, as the self-test's reap between polls always was.
func (d Deps) lifecycleReap(job string) {
	_ = d.Dispatch.Run(io.Discard, io.Discard, "reap", "--job", job)
}

func (d Deps) agents() string { return filepath.Join(d.Root, "artifacts", "agents") }
func (d Deps) jobs() string   { return filepath.Join(d.agents(), "jobs") }

// exitStatus maps a finished command to the shell's status: its exit code,
// 128 plus the signal for a signalled child, 127 when it never started.
func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return waitStatusCode(exitErr.ProcessState)
	}
	return 127
}

func waitStatusCode(state *os.ProcessState) int {
	if state == nil {
		return 127
	}
	if code := state.ExitCode(); code >= 0 {
		return code
	}
	if signaled, signal := stateSignal(state); signaled {
		return 128 + signal
	}
	return 1
}

// self is the binary a self-test's delegate children run.
func (d Deps) self() string {
	if d.Self != "" {
		return d.Self
	}
	return d.Engine
}
