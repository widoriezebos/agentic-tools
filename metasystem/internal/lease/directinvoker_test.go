package lease

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The round-3 regression: the signature universe is every INSTALLED
// adapter, independent of metasystem.runtimes — a root configured for
// claude only must still recognize the fake adapter's argv as an
// agent, because an unconfigured runtime's binary spawning a verb is
// still an agent invoker.
func TestAgentArgvUsesInstalledNotConfiguredUniverse(t *testing.T) {
	repo := func() string {
		root, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Dir(filepath.Dir(root)) // internal/lease -> repo root
	}()
	staged := t.TempDir()
	adapters := filepath.Join(staged, "scripts", "agents", "adapters")
	if err := os.MkdirAll(adapters, 0o755); err != nil {
		t.Fatal(err)
	}
	// Copy every real adapter, then configure ONLY claude.
	if err := exec.Command("cp", "-R",
		filepath.Join(repo, "scripts", "agents", "adapters")+"/.",
		adapters).Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staged, "metasystem.conf"),
		[]byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The UNCONFIGURED fake adapter's signature must still match.
	runtime, agent, err := AgentArgv(staged, "metasystem-fake-agent worker")
	if err != nil {
		t.Fatal(err)
	}
	if !agent || runtime != "fake" {
		t.Fatalf("an installed-but-unconfigured adapter must be recognized: runtime=%q agent=%v", runtime, agent)
	}
	// A plain shell never matches.
	if _, agent, err := AgentArgv(staged, "bash -c ls"); err != nil || agent {
		t.Fatalf("a shell argv must not read as an agent: agent=%v err=%v", agent, err)
	}
	// An unreadable signature set is an ERROR, never a silent pass —
	// the refusal-gate caller fails closed.
	if _, _, err := AgentArgv(filepath.Join(staged, "absent"), "anything"); err == nil {
		t.Fatal("a root without adapters must error, not answer")
	}

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
