package hostturn

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

// The fake host: the deterministic orchestrator stand-in the mission beds
// run instead of a real CLI (formerly scripts/agents/hosts/fake.sh). It
// reads one FAKEHOST:<behavior> marker from the assembled prompt and writes
// the turn's return and result envelope through the fake host ports.

func init() {
	register("fake", hostRuntime{
		cli: "",
		run: runFake,
		usageExtra: "Reads FAKEHOST:<behavior> markers from the assembled prompt. Behaviors:\n" +
			"return-ok (default), return-malformed, dispatch-ghost, dispatch-terminal, solo-build,\n" +
			"close-stream, park-request, exit-nonzero, exit-overloaded, overloaded-result, and\n" +
			"no-return.",
	})
}

var fakeHostBehaviors = map[string]bool{
	"return-ok": true, "return-malformed": true, "dispatch-ghost": true, "dispatch-terminal": true,
	"solo-build": true, "close-stream": true, "park-request": true, "exit-nonzero": true,
	"exit-overloaded": true, "overloaded-result": true, "no-return": true,
}

// fakeHostMarkerRE is the script's `s/.*FAKEHOST:\([a-z-][a-z-]*\).*/\1/p`:
// the greedy prefix takes each line's LAST marker.
var fakeHostMarkerRE = regexp.MustCompile(`.*FAKEHOST:([a-z-]+)`)

// fakeHostBehaviorsIn returns the distinct behaviors the prompt names,
// sorted.
func fakeHostBehaviorsIn(prompt []byte) []string {
	seen := map[string]bool{}
	for _, line := range splitLines(prompt) {
		if match := fakeHostMarkerRE.FindStringSubmatch(line); match != nil {
			seen[match[1]] = true
		}
	}
	var behaviors []string
	for behavior := range seen {
		behaviors = append(behaviors, behavior)
	}
	sort.Strings(behaviors)
	return behaviors
}

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for index, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:index]))
			start = index + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

// fakeHostHold replaces this process with the engine's fixture hold (the
// script's exec): the pid the runner verified stays the hold. It returns
// only when the exec fails.
func (t *Turn) fakeHostHold(ignoreTerm bool) int {
	if ignoreTerm {
		// An ignored disposition survives exec, as the script's
		// `trap '' TERM` did.
		signal.Ignore(syscall.SIGTERM)
	}
	argv := []string{t.d.Engine, "util", "hold", "--tag", t.InstanceTag,
		"--ready-file", t.Path("host-ready"), "--stopped-file", t.Path("host-stopped")}
	if ignoreTerm {
		argv = append(argv, "--ignore-term", "--term-observed-file", t.Path("host-term-observed"))
	}
	err := syscall.Exec(t.d.Engine, argv, t.d.Environ)
	fmt.Fprintf(t.d.Stderr, "fake host hold exec failed: %v\n", err)
	return 126
}

func fakeHostPorts() (delegate.Ports, error) {
	ports, err := delegate.PortsFor("fake")
	if err != nil {
		return delegate.Ports{}, err
	}
	if ports.HostFakeResult == nil || ports.HostFakeReturn == nil {
		return delegate.Ports{}, errors.New("fake host ports are not registered")
	}
	return ports, nil
}

func runFake(t *Turn) int {
	turnRecord := t.Path("turn.json")
	if info, err := os.Stat(turnRecord); err != nil || !info.Mode().IsRegular() {
		fmt.Fprintf(t.d.Stderr, "fake host turn record is missing: %s\n", turnRecord)
		return 3
	}
	if t.d.Getenv("METASYSTEM_FAKE_HOST_START_UNVERIFIED") == "1" {
		return 0
	}
	if !t.requireCLI("") {
		return 3
	}
	hold := t.d.Getenv("METASYSTEM_FAKE_HOST_HOLD") == "1"
	ignoreTerm := t.d.Getenv("METASYSTEM_FAKE_HOST_IGNORE_TERM") == "1"
	if hold || ignoreTerm {
		if config.ConfValue(filepath.Join(t.d.Root, "metasystem.conf"), "metasystem.runtimes", "") != "fake" {
			fmt.Fprintln(t.d.Stderr, "METASYSTEM_FAKE_HOST_HOLD and METASYSTEM_FAKE_HOST_IGNORE_TERM are available only in a fixture-mode root")
			return 3
		}
	}
	if hold {
		// The engine's hold validates the exact fixture owner and watches
		// the inherited leash, so a fake host cannot turn a missing
		// fixture boundary into an unbounded process.
		return t.fakeHostHold(ignoreTerm)
	}

	prompt, err := os.ReadFile(t.Prompt)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	behaviors := fakeHostBehaviorsIn(prompt)
	if len(behaviors) > 1 {
		fmt.Fprintln(t.d.Stderr, "fake host prompt contains multiple behaviors")
		return 3
	}
	behavior := "return-ok"
	if len(behaviors) == 1 {
		behavior = behaviors[0]
	}
	if !fakeHostBehaviors[behavior] {
		fmt.Fprintf(t.d.Stderr, "unknown fake host behavior: %s\n", behavior)
		return 3
	}

	raw, returnPath := t.Path("raw.out"), t.Path("return.json")
	if err := os.WriteFile(raw, []byte(fmt.Sprintf("fake host behavior=%s instance=%s\n", behavior, t.InstanceTag)), 0o644); err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	session := t.ResumeSession
	if session == "" {
		session = "fake-host-session-" + t.Mission
	}
	ports, err := fakeHostPorts()
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	result := func(returnPath, outcome string) int {
		if err := ports.HostFakeResult(t.Result, session, raw, returnPath, outcome); err != nil {
			fmt.Fprintln(t.d.Stderr, err)
			return 1
		}
		return 0
	}

	switch behavior {
	case "exit-nonzero":
		if code := result("", "failed"); code != 0 {
			return code
		}
		return 3
	case "exit-overloaded":
		// The provider's 529 as the real CLI leaves it: on stderr (host.log).
		if err := os.WriteFile(t.Path("host.log"), []byte(`API Error: 529 {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`+"\n"), 0o644); err != nil {
			fmt.Fprintln(t.d.Stderr, err)
			return 1
		}
		if code := result("", "failed"); code != 0 {
			return code
		}
		return 3
	case "overloaded-result":
		// The other real shape: a clean CLI exit whose result document is
		// the provider's own error, left under the runtime-neutral name.
		if err := os.WriteFile(t.Path("provider-result.json"), []byte(`{"is_error":true,"result":"API Error: 529 Overloaded"}`+"\n"), 0o644); err != nil {
			fmt.Fprintln(t.d.Stderr, err)
			return 1
		}
		return result("", "completed")
	case "return-malformed":
		if err := os.WriteFile(returnPath, []byte("{malformed\n"), 0o644); err != nil {
			fmt.Fprintln(t.d.Stderr, err)
			return 1
		}
	case "no-return":
	default:
		state := filepath.Join(t.d.Root, "artifacts", "agents", "missions", t.Mission, "state.json")
		if err := ports.HostFakeReturn(turnRecord, state, returnPath, behavior, t.d.Root); err != nil {
			fmt.Fprintln(t.d.Stderr, err)
			return 1
		}
	}
	return result(returnPath, "completed")
}
