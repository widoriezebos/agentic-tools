package lease

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The signature universe is every DECLARED adapter runtime in the registry,
// independent of metasystem.runtimes — a root configured for claude only
// must still recognize the fake runtime's argv as an agent, because an
// unconfigured runtime's binary spawning a verb is still an agent invoker.
func TestAgentArgvUsesInstalledNotConfiguredUniverse(t *testing.T) {
	staged := t.TempDir()
	if err := os.WriteFile(filepath.Join(staged, "metasystem.conf"),
		[]byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The UNCONFIGURED fake runtime's signature must still match.
	runtime, agent, err := AgentArgv(staged, "metasystem-fake-agent worker")
	if err != nil {
		t.Fatal(err)
	}
	if !agent || runtime != "fake" {
		t.Fatalf("a declared-but-unconfigured runtime must be recognized: runtime=%q agent=%v", runtime, agent)
	}
	// A plain shell never matches.
	if _, agent, err := AgentArgv(staged, "bash -c ls"); err != nil || agent {
		t.Fatalf("a shell argv must not read as an agent: agent=%v err=%v", agent, err)
	}
	// An unreadable or empty signature set is an ERROR, never a silent
	// pass — the refusal-gate caller fails closed.
	prior := delegateSignatures
	t.Cleanup(func() { delegateSignatures = prior })
	delegateSignatures = func() ([]census.Signature, error) { return nil, errors.New("registry unreadable") }
	if _, _, err := AgentArgv(staged, "anything"); err == nil {
		t.Fatal("an unreadable signature set must error, not answer")
	}
	delegateSignatures = func() ([]census.Signature, error) { return nil, nil }
	if _, _, err := AgentArgv(staged, "anything"); err == nil {
		t.Fatal("an empty signature set must error, not answer")
	}
	delegateSignatures = prior

	// The live probe path: this test's own parent (the go test runner)
	// is a readable, live, non-agent process.
	if _, agent, err := DirectAgentInvoker(staged, int64(os.Getppid())); err != nil || agent {
		t.Fatalf("the test runner must read as a live non-agent: agent=%v err=%v", agent, err)
	}
	// An impossible pid is unprovable and errors (fail closed).
	if _, _, err := DirectAgentInvoker(staged, 999999999); err == nil {
		t.Fatal("an unprovable pid must error")
	}

	// ROUND 4 FINDING 1: Alive with ArgvKnown=false is ABSENCE OF
	// EVIDENCE and must error — joining an unknown argv into "" would
	// silently read as provably-not-an-agent.
	blind := staticProber{exact: identity.Exact{Pid: 4242, ArgvKnown: false}, state: identity.Alive}
	if _, _, err := directAgentInvoker(staged, 4242, blind); err == nil {
		t.Fatal("Alive with unknown argv must refuse, not pass as non-agent")
	}
	// And the same prober WITH a known agent argv still matches.
	seen := staticProber{exact: identity.Exact{Pid: 4242, Argv: []string{"metasystem-fake-agent", "worker"}, ArgvKnown: true}, state: identity.Alive}
	if runtime, agent, err := directAgentInvoker(staged, 4242, seen); err != nil || !agent || runtime != "fake" {
		t.Fatalf("a known agent argv must match: runtime=%q agent=%v err=%v", runtime, agent, err)
	}
}

// staticProber answers Probe with a fixed identity.
type staticProber struct {
	exact identity.Exact
	state identity.Liveness
}

func (s staticProber) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return s.exact, s.state, nil
}
