package supervisor

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

	// The fake host ports (fake-return, fake-result) register from
	// internal/host.
	_ "github.com/widoriezebos/agentic-tools/metasystem/internal/host"
)

// The fake mission host turn (formerly scripts/agents/hosts/fake.sh): one
// FAKEHOST:<behavior> marker from the assembled prompt, acted out by the
// simulated CLI, and the turn's result envelope written through the fake
// host ports. Its exit status is the script's, so finalize ends every turn
// with HostExit instead of the shared finish.

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
	start := 0
	for index := 0; index <= len(prompt); index++ {
		if index < len(prompt) && prompt[index] != '\n' {
			continue
		}
		if match := fakeHostMarkerRE.FindSubmatch(prompt[start:index]); match != nil {
			seen[string(match[1])] = true
		}
		start = index + 1
	}
	var behaviors []string
	for behavior := range seen {
		behaviors = append(behaviors, behavior)
	}
	sort.Strings(behaviors)
	return behaviors
}

// HostPreflight is what the fake host checked before the start gate: its
// turn record (exit 3 when missing), and the unverified-start control that
// ends the turn before the gate (exit 0). done is false to continue.
func (fakeOps) HostPreflight(t *Turn) (code int, done bool) {
	d := t.Deps()
	if info, err := os.Stat(t.Record); err != nil || !info.Mode().IsRegular() {
		fmt.Fprintf(d.Stderr, "fake host turn record is missing: %s\n", t.Record)
		return 3, true
	}
	if d.Getenv("METASYSTEM_FAKE_HOST_START_UNVERIFIED") == "1" {
		return 0, true
	}
	return 0, false
}

// fakeHost is one host turn's simulated CLI and its outcome.
type fakeHost struct {
	behavior, raw, returnPath, session string
	// outcome is the result envelope's outcome; resultReturn is the return
	// path it names ("" for none); exit is the turn's status after a
	// written result.
	outcome, resultReturn string
	exit                  int
	err                   error
}

func prepareFakeHost(t *Turn) (Launch, error) {
	d := t.Deps()
	hold := d.Getenv("METASYSTEM_FAKE_HOST_HOLD") == "1"
	ignoreTerm := d.Getenv("METASYSTEM_FAKE_HOST_IGNORE_TERM") == "1"
	if (hold || ignoreTerm) && config.ConfValue(filepath.Join(d.installation(), "metasystem.conf"), "metasystem.runtimes", "") != "fake" {
		return Launch{Refusal: &Refusal{Error: "METASYSTEM_FAKE_HOST_HOLD and METASYSTEM_FAKE_HOST_IGNORE_TERM are available only in a fixture-mode root"}}, nil
	}
	if hold {
		// The engine's hold validates the exact fixture owner and watches
		// the inherited leash, so a fake host cannot turn a missing fixture
		// boundary into an unbounded process.
		return Launch{Simulated: func(<-chan struct{}) int { return fakeHostHold(t, ignoreTerm) }, Private: &fakeHost{}}, nil
	}
	prompt, err := os.ReadFile(t.Prompt)
	if err != nil {
		return Launch{}, err
	}
	behaviors := fakeHostBehaviorsIn(prompt)
	if len(behaviors) > 1 {
		return Launch{Refusal: &Refusal{Error: "fake host prompt contains multiple behaviors"}}, nil
	}
	behavior := "return-ok"
	if len(behaviors) == 1 {
		behavior = behaviors[0]
	}
	if !fakeHostBehaviors[behavior] {
		return Launch{Refusal: &Refusal{Error: "unknown fake host behavior: " + behavior}}, nil
	}
	h := &fakeHost{behavior: behavior, raw: filepath.Join(t.Dir, "raw.out"), returnPath: filepath.Join(t.Dir, "return.json"),
		session: t.ResumeSession}
	if h.session == "" {
		h.session = "fake-host-session-" + t.Mission
	}
	return Launch{Simulated: func(<-chan struct{}) int { return h.run(t) }, Private: h}, nil
}

// fakeHostHold replaces this process with the engine's fixture hold (the
// script's exec): the pid the runner verified stays the hold. It returns
// only when the exec fails.
func fakeHostHold(t *Turn, ignoreTerm bool) int {
	d := t.Deps()
	if ignoreTerm {
		// An ignored disposition survives exec, as the script's
		// `trap '' TERM` did.
		signal.Ignore(syscall.SIGTERM)
	}
	flags := []string{"--tag", t.Tag,
		"--ready-file", filepath.Join(t.Dir, "host-ready"), "--stopped-file", filepath.Join(t.Dir, "host-stopped")}
	if ignoreTerm {
		flags = append(flags, "--ignore-term", "--term-observed-file", filepath.Join(t.Dir, "host-term-observed"))
	}
	err := syscall.Exec(d.Engine, append([]string{d.Engine, "util", "hold"}, flags...), d.Environ)
	fmt.Fprintf(d.Stderr, "fake host hold exec failed: %v\n", err)
	return 126
}

// run is the simulated host CLI: the raw output and the behavior's files.
func (h *fakeHost) run(t *Turn) int {
	if err := os.WriteFile(h.raw, []byte(fmt.Sprintf("fake host behavior=%s instance=%s\n", h.behavior, t.Tag)), 0o644); err != nil {
		h.err = err
		return 1
	}
	h.outcome, h.resultReturn = "completed", h.returnPath
	switch h.behavior {
	case "exit-nonzero":
		h.outcome, h.resultReturn, h.exit = "failed", "", 3
	case "exit-overloaded":
		// The provider's 529 as the real CLI leaves it: on stderr (host.log).
		h.outcome, h.resultReturn, h.exit = "failed", "", 3
		h.err = os.WriteFile(filepath.Join(t.Dir, "host.log"), []byte(`API Error: 529 {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`+"\n"), 0o644)
	case "overloaded-result":
		// The other real shape: a clean CLI exit whose result document is
		// the provider's own error, left under the runtime-neutral name.
		h.resultReturn = ""
		h.err = os.WriteFile(filepath.Join(t.Dir, "provider-result.json"), []byte(`{"is_error":true,"result":"API Error: 529 Overloaded"}`+"\n"), 0o644)
	case "return-malformed":
		h.err = os.WriteFile(h.returnPath, []byte("{malformed\n"), 0o644)
	case "no-return":
	default:
		ports, err := delegate.PortsFor("fake")
		if err == nil && ports.HostFakeReturn == nil {
			err = errors.New("fake host return port is not registered")
		}
		if err == nil {
			state := filepath.Join(t.Root, "artifacts", "agents", "missions", t.Mission, "state.json")
			err = ports.HostFakeReturn(t.Record, state, h.returnPath, h.behavior, t.Root)
		}
		h.err = err
	}
	if h.err != nil {
		return 1
	}
	return h.exit
}

// finalize writes the result envelope and ends the turn with the script's
// status: a failure before the result is 1, a failing result write is 1,
// otherwise the behavior's own status.
func (h *fakeHost) finalize(t *Turn) (Final, error) {
	code := 1
	if h.err != nil {
		fmt.Fprintln(t.Deps().Stderr, h.err)
		return Final{HostExit: &code}, nil
	}
	ports, err := delegate.PortsFor("fake")
	if err == nil && ports.HostFakeResult == nil {
		err = errors.New("fake host result port is not registered")
	}
	if err == nil {
		err = ports.HostFakeResult(t.Result, h.session, h.raw, h.resultReturn, h.outcome)
	}
	if err != nil {
		fmt.Fprintln(t.Deps().Stderr, err)
		return Final{HostExit: &code}, nil
	}
	code = h.exit
	return Final{HostExit: &code}, nil
}
