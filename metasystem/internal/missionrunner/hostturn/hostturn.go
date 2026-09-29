// Package hostturn is one mission host turn: the orchestrator runtime's CLI
// launched in the checkout, supervised to its exit, and its outcome written
// as the turn's result envelope with the exit taxonomy the mission runner
// interprets (0 completed, 3 failed, 6 missing session). The runner launches
// it as the engine's delegate-supervisor entry,
// `ENGINE delegate-supervisor RUNTIME start-turn ...`, a process of its own
// that leads its own group and carries the turn's instance tag. It replaces
// the shell hosts that lived under scripts/agents/hosts.
//
// It is a composition package above the owners: the result adjudication
// lives in internal/host, the runtime transformations in internal/adapter.
package hostturn

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// Turn is one host turn's arguments and paths.
type Turn struct {
	d       supervisor.Deps
	runtime string

	Mission, TurnID, Prompt, Result, ResumeSession, InstanceTag string

	// TurnDir is the prompt's directory, physically resolved.
	TurnDir string
}

// Runtimes lists the runtimes that serve host turns: every built-in whose
// operations declare the host role.
func Runtimes() []string {
	var names []string
	for _, name := range runtimes.WithHost() {
		if ops, ok := supervisor.OperationsFor(supervisor.Deps{}, name); ok {
			if description, err := ops.Describe(supervisor.Deps{}); err == nil && description.Capabilities.Host {
				names = append(names, name)
			}
		}
	}
	return names
}

func usage(d supervisor.Deps, runtime string) {
	fmt.Fprintf(d.Stderr, "Usage:\n  metasystem %s %s %s --root ROOT --mission <id> --turn-id <id>\n      --prompt <file> --result <file> [--resume-session <sid>] --instance-tag <tag>\n",
		runtimes.SupervisorEntry, runtime, runtimes.SupervisorHostTurn)
}

var idRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Main runs `RUNTIME start-turn --root ROOT [flags]` and returns the host
// exit status.
func Main(args []string, newDeps func(root string) supervisor.Deps) int {
	if len(args) < 2 {
		return 2
	}
	name, verb, rest := args[0], args[1], args[2:]
	root := ""
	if len(rest) >= 2 && rest[0] == "--root" {
		root, rest = rest[1], rest[2:]
	}
	d := newDeps(root)
	ops, err := supervisor.OperationsAt(d, name)
	if err != nil {
		fmt.Fprintf(d.Stderr, "host adapter is not installed: %s: %v\n", name, err)
		return 2
	}
	if verb == "-h" || verb == "--help" {
		usage(d, name)
		return 0
	}
	if verb != runtimes.SupervisorHostTurn || root == "" || !filepath.IsAbs(root) {
		usage(d, name)
		return 2
	}
	t := &Turn{d: d, runtime: name}
	for len(rest) > 0 {
		if rest[0] == "-h" || rest[0] == "--help" {
			usage(d, name)
			return 0
		}
		if len(rest) < 2 {
			usage(d, name)
			return 2
		}
		value := rest[1]
		switch rest[0] {
		case "--mission":
			t.Mission = value
		case "--turn-id":
			t.TurnID = value
		case "--prompt":
			t.Prompt = value
		case "--result":
			t.Result = value
		case "--resume-session":
			t.ResumeSession = value
		case "--instance-tag":
			t.InstanceTag = value
		default:
			usage(d, name)
			return 2
		}
		rest = rest[2:]
	}
	if !idRE.MatchString(t.Mission) || !idRE.MatchString(t.TurnID) {
		usage(d, name)
		return 2
	}
	if info, err := os.Stat(t.Prompt); err != nil || info.IsDir() || t.Result == "" || t.InstanceTag == "" {
		usage(d, name)
		return 2
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(t.Prompt))
	if err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	if t.TurnDir, err = filepath.Abs(dir); err != nil {
		fmt.Fprintln(d.Stderr, err)
		return 1
	}
	return runHostOps(t, ops)
}

// requireCLI refuses a turn whose runtime CLI is absent, then waits for the
// runner's start gate; either failure is the host's setup failure (3).
func (t *Turn) requireCLI(cli string) bool {
	if cli != "" {
		if _, err := t.d.LookPath(cli); err != nil {
			fmt.Fprintf(t.d.Stderr, "%s CLI is not installed\n", cli)
			return false
		}
	}
	return t.waitForStartGate()
}

func (t *Turn) waitForStartGate() bool {
	gate := t.d.Getenv("METASYSTEM_HOST_START_GATE")
	if gate == "" {
		return true
	}
	capSeconds := 10
	if raw := t.d.Getenv("METASYSTEM_HOST_START_GATE_TIMEOUT_SEC"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || !positiveRE.MatchString(raw) {
			fmt.Fprintf(t.d.Stderr, "%s host start-gate timeout is invalid\n", t.runtime)
			return false
		}
		capSeconds = value
	}
	poll := 20
	if raw := t.d.Getenv("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || !positiveRE.MatchString(raw) {
			fmt.Fprintf(t.d.Stderr, "%s host handshake poll interval is invalid\n", t.runtime)
			return false
		}
		poll = value
	}
	err := adapter.WaitStartGateFile(t.d.Root, gate, time.Duration(capSeconds)*time.Second, time.Duration(poll)*time.Millisecond, t.d.Getenv)
	if err == nil {
		return true
	}
	if errors.Is(err, adapter.ErrStartGateExpired) {
		fmt.Fprintf(t.d.Stderr, "%s host start gate was not released within %ds\n", t.runtime, capSeconds)
	}
	return false
}

var positiveRE = regexp.MustCompile(`^[1-9][0-9]*$`)

// Path joins a file name onto the turn directory.
func (t *Turn) Path(name string) string { return filepath.Join(t.TurnDir, name) }

// protocolFile writes compiled-in protocol bytes into the turn directory:
// the runtime CLIs take the orchestrator schema and the requested
// permission envelope as paths.
func (t *Turn) protocolFile(name string, read func(string) ([]byte, error), item string) (string, error) {
	data, err := read(item)
	if err != nil {
		return "", err
	}
	path := t.Path(name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write host turn %s: %w", name, err)
	}
	return path, nil
}
