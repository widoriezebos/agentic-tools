package hostturn

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// TestRuntimesListsOnlyHostCapableBuiltins pins the host roster: every
// listed runtime declares a host launcher and describes the host
// capability, and the two shipped orchestrator CLIs are among them.
func TestRuntimesListsOnlyHostCapableBuiltins(t *testing.T) {
	t.Parallel()
	names := Runtimes()
	for _, want := range []string{"claude", "codex"} {
		if !slices.Contains(names, want) {
			t.Fatalf("Runtimes() = %v, missing %s", names, want)
		}
	}
	withHost := runtimes.WithHost()
	for _, name := range names {
		if !slices.Contains(withHost, name) {
			t.Fatalf("Runtimes() lists %s, which declares no host launcher", name)
		}
		ops, ok := supervisor.OperationsFor(supervisor.Deps{}, name)
		if !ok {
			t.Fatalf("Runtimes() lists %s, which has no operations", name)
		}
		description, err := ops.Describe(supervisor.Deps{})
		if err != nil || !description.Capabilities.Host {
			t.Fatalf("Runtimes() lists %s, whose description has no host capability (%v)", name, err)
		}
	}
}

// TestHostArgumentGrammar drives Main's argument refusals and help: a
// refusal exits 2 with the usage on stderr, help exits 0 with the usage,
// and no refused turn writes a result envelope.
func TestHostArgumentGrammar(t *testing.T) {
	t.Parallel()
	probe := newHostBed(t, "claude", claudeStub)
	prompt := filepath.Join(probe.turn, "prompt.md")
	result := filepath.Join(probe.turn, "result.json")
	full := func(root string, extra ...string) []string {
		return append([]string{"claude", "start-turn", "--root", root, "--mission", "m1", "--turn-id", "t1",
			"--prompt", prompt, "--result", result, "--instance-tag", "fixture-host-tag"}, extra...)
	}
	cases := []struct {
		name      string
		args      func(root string) []string
		want      int
		wantUsage bool
	}{
		{"no verb", func(string) []string { return []string{"claude"} }, 2, false},
		{"help as verb", func(root string) []string { return []string{"claude", "--help", "--root", root} }, 0, true},
		{"short help as verb", func(root string) []string { return []string{"claude", "-h", "--root", root} }, 0, true},
		{"help among flags", func(root string) []string { return full(root, "-h") }, 0, true},
		{"wrong verb", func(root string) []string { return []string{"claude", "stop-turn", "--root", root} }, 2, true},
		{"root missing", func(string) []string { return []string{"claude", "start-turn", "--mission", "m1"} }, 2, true},
		{"flag without value", func(root string) []string { return full(root, "--resume-session") }, 2, true},
		{"unknown flag", func(root string) []string { return full(root, "--model", "x") }, 2, true},
		{"bad turn id", func(root string) []string { return full(root, "--turn-id", "-t1") }, 2, true},
		{"prompt missing", func(root string) []string { return full(root, "--prompt", filepath.Join(root, "absent.md")) }, 2, true},
		{"prompt is a directory", func(root string) []string { return full(root, "--prompt", filepath.Join(root, "turns")) }, 2, true},
		{"result empty", func(root string) []string { return full(root, "--result", "") }, 2, true},
		{"instance tag empty", func(root string) []string { return full(root, "--instance-tag", "") }, 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newHostBed(t, "claude", claudeStub)
			args := tc.args(b.root)
			// Point the shared prompt and result at this bed's turn.
			for i := range args {
				args[i] = strings.Replace(args[i], probe.turn, b.turn, 1)
			}
			if code := Main(args, b.deps); code != tc.want {
				t.Fatalf("Main(%q) = %d, want %d (stderr %q)", args, code, tc.want, b.stderr.String())
			}
			if usage := strings.Contains(b.stderr.String(), "Usage:"); usage != tc.wantUsage {
				t.Fatalf("usage printed = %v, want %v (stderr %q)", usage, tc.wantUsage, b.stderr.String())
			}
			if present(filepath.Join(b.turn, "result.json")) {
				t.Fatal("a refused or help invocation wrote a result envelope")
			}
		})
	}
}

func TestHostRefusesAnInvalidHandshakePollInterval(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"0", "fast", "010"} {
		b := newHostBed(t, "claude", claudeStub)
		b.env["METASYSTEM_HOST_START_GATE"] = filepath.Join(b.turn, "gate")
		b.env["METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS"] = raw
		if code := b.run("claude"); code != 3 || !strings.Contains(b.stderr.String(), "claude host handshake poll interval is invalid") {
			t.Fatalf("poll interval %q = %d %q", raw, code, b.stderr.String())
		}
		if present(filepath.Join(b.turn, "result.json")) {
			t.Fatalf("poll interval %q: a launch refused at setup wrote a result envelope", raw)
		}
	}
}
