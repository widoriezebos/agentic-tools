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
// writes that dispatch.sh's lease-held internal callbacks own (__record-cas,
// __handshake, __register-custody, __protocol-error, __repair-claim,
// __cancel-owned) stay callbacks into dispatch.sh until its port (U6b), so
// authority is checked exactly where it was: against this supervisor
// process, the pid the job record names.
package supervisor

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	// Environ is the environment children inherit before per-launch
	// additions.
	Environ []string
	Getenv  func(string) string
	// Pid is this supervisor's process id; the supervisor leads its own
	// process group, so it is also the kill domain's group id.
	Pid            int
	Stdout, Stderr io.Writer
	Clock          Clock
	// Dispatch runs one dispatch.sh internal callback and returns its exit
	// status.
	Dispatch Dispatcher
	// GroupMembers lists the live members of a process group, except the
	// given pids; an error is an indeterminable enumeration.
	GroupMembers func(pgid int, except ...int) ([]int, error)
	// LookPath resolves a runtime CLI on PATH.
	LookPath func(string) (string, error)
}

// Clock is the supervisor's time: wall readings and waits.
type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

// Dispatcher runs `scripts/agents/dispatch.sh ARGS` with the given output
// streams.
type Dispatcher interface {
	Run(stdout, stderr io.Writer, args ...string) int
}

type systemClock struct{}

func (systemClock) Now() time.Time        { return time.Now() }
func (systemClock) Sleep(d time.Duration) { time.Sleep(d) }

// SystemClock is the wall clock.
func SystemClock() Clock { return systemClock{} }

// ScriptDispatcher execs the installation's dispatch.sh.
type ScriptDispatcher struct {
	Root    string
	Environ []string
}

func (d ScriptDispatcher) Run(stdout, stderr io.Writer, args ...string) int {
	command := exec.Command(filepath.Join(d.Root, "scripts", "agents", "dispatch.sh"), args...)
	command.Env = d.Environ
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
	environ := os.Environ()
	return Deps{
		Root: root, Engine: engine, Environ: environ, Getenv: os.Getenv,
		Pid: os.Getpid(), Stdout: os.Stdout, Stderr: os.Stderr,
		Clock:        SystemClock(),
		Dispatch:     ScriptDispatcher{Root: root, Environ: environ},
		GroupMembers: groupMembers,
		LookPath:     exec.LookPath,
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
