package supervisor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
)

// The Claude and Codex delegate rounds through the shared round, against
// stub CLIs: the handshake from each runtime's own channel (Claude's
// SessionStart signal file, Codex's event stream), the terminal record, and
// the return normalized from the runtime's result.

// stubRound is a fake install whose runtime is a stubbed real CLI.
func stubRound(t *testing.T, runtime, cli, script string) (*fakeInstall, Deps) {
	t.Helper()
	f := newFakeInstall(t, installOptions{role: "verifier"})
	editRecord(filepath.Join(f.agents(), "jobs", f.job+".json"), func(record map[string]any) { record["runtime"] = runtime })
	stub := filepath.Join(f.root, "stub-bin", cli)
	mustWrite(t, stub, "#!/bin/sh\n"+script)
	if err := os.Chmod(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	d := f.deps()
	d.LookPath = func(name string) (string, error) {
		if name == cli {
			return stub, nil
		}
		return "", exec.ErrNotFound
	}
	d.Git = func(string, ...string) (string, bool) { return "", false }
	return f, d
}

func stubReturn(t *testing.T, f *fakeInstall) string {
	t.Helper()
	path := filepath.Join(f.root, "stub-return.json")
	if err := adapter.WriteFakeReturn(filepath.Join(f.agents(), "jobs", f.job+".json"), f.roundFile("prompt.md"), path); err != nil {
		t.Fatal(err)
	}
	data := readText(t, path)
	return strings.Join(strings.Fields(data), " ")
}

func TestClaudeDelegateRoundCompletes(t *testing.T) {
	f, d := stubRound(t, "claude", "claude", "")
	ret := stubReturn(t, f)
	// The stub signals the session through the hook's channel, then streams
	// the result line whose result is the return.
	script := `cat >/dev/null
printf '{"session_id":"claude-session","model":"claude-model"}' >"$METASYSTEM_CLAUDE_SESSION_SIGNAL"
printf '%s\n' "$STUB_RESULT"
`
	mustWrite(t, filepath.Join(f.root, "stub-bin", "claude"), "#!/bin/sh\n"+script)
	resultLine := `{"type":"result","session_id":"claude-session","model":"claude-model","result":` + quoteJSON(ret) + `,"usage":{"input_tokens":3,"output_tokens":2}}`
	d.Environ = append(d.Environ, "STUB_RESULT="+resultLine)
	args := append([]string{"claude"}, f.superviseArgs()[1:]...)
	if code := Main(args, func(string) Deps { return d }); code != 0 {
		t.Fatalf("claude round = %d: %s\nlog: %s", code, f.stderr.String(), f.logText())
	}
	calls := f.dispatcher.calls(t)
	names := strings.Join(callNames(calls), " ")
	if !strings.HasPrefix(names, "__register-custody __handshake") || !strings.HasSuffix(names, "__record-cas") {
		t.Fatalf("callbacks = %s", names)
	}
	if hs := calls[1]; flagValue(hs, "--session") != "claude-session" || flagValue(hs, "--model") != "claude-model" {
		t.Fatalf("handshake = %v", hs)
	}
	if last := calls[len(calls)-1]; flagValue(last, "--status") != "completed" {
		t.Fatalf("terminal = %v", last)
	}
	for _, name := range []string{"claude-settings.json", "claude-result.json", "raw.out", "return.json", "usage.json"} {
		if !exists(f.roundFile(name)) {
			t.Fatalf("round lacks %s", name)
		}
	}
}

func TestCodexDelegateRoundCompletes(t *testing.T) {
	f, d := stubRound(t, "codex", "codex", "")
	ret := stubReturn(t, f)
	script := `out=
while [ $# -gt 0 ]; do case "$1" in -o|--output-last-message) out=$2; shift 2 ;; *) shift ;; esac; done
cat >/dev/null
printf '{"type":"thread.started","thread_id":"codex-thread"}\n'
printf '{"type":"turn.started","turn_id":"codex-turn"}\n'
printf '{"type":"turn.completed","usage":{"input_tokens":3,"output_tokens":2}}\n'
printf '%s' "$STUB_RETURN" >"$out"
`
	mustWrite(t, filepath.Join(f.root, "stub-bin", "codex"), "#!/bin/sh\n"+script)
	d.Environ = append(d.Environ, "STUB_RETURN="+ret)
	args := append([]string{"codex"}, f.superviseArgs()[1:]...)
	if code := Main(args, func(string) Deps { return d }); code != 0 {
		t.Fatalf("codex round = %d: %s\nlog: %s", code, f.stderr.String(), f.logText())
	}
	calls := f.dispatcher.calls(t)
	if hs := calls[1]; calls[1][0] != "__handshake" || flagValue(hs, "--session") != "codex-thread" {
		t.Fatalf("callbacks = %v", calls)
	}
	if last := calls[len(calls)-1]; flagValue(last, "--status") != "completed" {
		t.Fatalf("terminal = %v", last)
	}
}

func TestClaudeRoundRefusesAnInvalidBudgetAfterTheEnvelope(t *testing.T) {
	f, d := stubRound(t, "claude", "claude", "exit 0\n")
	d.Environ = append(d.Environ, "METASYSTEM_CLAUDE_MAX_BUDGET_USD=not-a-number")
	args := append([]string{"claude"}, f.superviseArgs()[1:]...)
	if code := Main(args, func(string) Deps { return d }); code != 1 {
		t.Fatalf("invalid budget = %d", code)
	}
	calls := f.dispatcher.calls(t)
	if len(calls) != 1 || flagValue(calls[0], "--expect") != "pending" {
		t.Fatalf("calls = %v", calls)
	}
	if patch := readJSON(t, flagValue(calls[0], "--patch")); patch["error"] != "invalid_native_budget" {
		t.Fatalf("patch = %v", patch)
	}
}

func quoteJSON(text string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
