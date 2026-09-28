package hostturn

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
)

// hostBed is one installation with a turn directory and a stub runtime CLI
// on a private lookup path (the host rows of mission-fixtures.sh: the
// rotated session, the missing session's exit 6, and the unreleased gate).
type hostBed struct {
	t                    *testing.T
	root, turn, bin, cli string
	stdout, stderr       bytes.Buffer
	env                  map[string]string
}

func newHostBed(t *testing.T, cli, script string) *hostBed {
	t.Helper()
	root := t.TempDir()
	b := &hostBed{t: t, root: root, turn: filepath.Join(root, "turns", "t1"), bin: filepath.Join(root, "bin-stub"), cli: cli, env: map[string]string{}}
	for _, dir := range []string{b.turn, b.bin} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := testexec.WriteFile(filepath.Join(b.bin, cli), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.turn, "turn.json"), []byte(`{"missionId":"m1","turnId":"t1","cycle":1,"model":"fixture-model"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b.turn, "prompt.md"), []byte("host prompt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return b
}

func (b *hostBed) deps(string) supervisor.Deps {
	environ := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + b.root}
	for name, value := range b.env {
		environ = append(environ, name+"="+value)
	}
	return supervisor.Deps{
		Root: b.root, Environ: environ, Stdout: &b.stdout, Stderr: &b.stderr,
		Getenv: func(name string) string { return b.env[name] },
		LookPath: func(name string) (string, error) {
			if name == b.cli {
				return filepath.Join(b.bin, name), nil
			}
			return "", exec.ErrNotFound
		},
	}
}

func (b *hostBed) run(runtime string, extra ...string) int {
	args := append([]string{runtime, "start-turn", "--root", b.root, "--mission", "m1", "--turn-id", "t1",
		"--prompt", filepath.Join(b.turn, "prompt.md"), "--result", filepath.Join(b.turn, "result.json"),
		"--instance-tag", "fixture-host-tag"}, extra...)
	return Main(args, b.deps)
}

func (b *hostBed) result() map[string]any {
	b.t.Helper()
	data, err := os.ReadFile(filepath.Join(b.turn, "result.json"))
	if err != nil {
		b.t.Fatalf("no result envelope: %v (stderr %q)", err, b.stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		b.t.Fatal(err)
	}
	return result
}

// claudeStub answers one blocking json turn; FAKE_CLAUDE_SESSION=none omits
// the session.
const claudeStub = `session=${FAKE_CLAUDE_SESSION:-rotated-session}
cat >/dev/null
if [ "$session" = none ]; then
  printf '{"type":"result","result":"{}","usage":{"input_tokens":1,"output_tokens":1}}\n'
else
  printf '{"type":"result","session_id":"%s","result":"{}","usage":{"input_tokens":1,"output_tokens":1}}\n' "$session"
fi
`

func TestClaudeHostReportsTheRotatedSession(t *testing.T) {
	t.Parallel()
	b := newHostBed(t, "claude", claudeStub)
	if code := b.run("claude", "--resume-session", "announced-session"); code != 0 {
		t.Fatalf("claude host = %d: %s", code, b.stderr.String())
	}
	result := b.result()
	if result["outcome"] != "completed" || result["sessionId"] != "rotated-session" {
		t.Fatalf("rotated session result = %v", result)
	}
	if data, _ := os.ReadFile(filepath.Join(b.turn, "raw.out")); !strings.Contains(string(data), "rotated-session") {
		t.Fatalf("raw.out does not carry the provider result: %q", data)
	}
}

func TestClaudeHostMissingSessionIsUnresumable(t *testing.T) {
	t.Parallel()
	b := newHostBed(t, "claude", claudeStub)
	b.env["FAKE_CLAUDE_SESSION"] = "none"
	if code := b.run("claude", "--resume-session", "announced-session"); code != 6 {
		t.Fatalf("missing session = %d, want 6", code)
	}
	if result := b.result(); result["outcome"] != "unresumable" || result["sessionId"] != nil {
		t.Fatalf("missing session result = %v", result)
	}
}

func TestClaudeHostFailedCLIIsFailed(t *testing.T) {
	t.Parallel()
	b := newHostBed(t, "claude", "cat >/dev/null\necho boom >&2\nexit 9\n")
	if code := b.run("claude"); code != 3 {
		t.Fatalf("failed CLI = %d, want 3", code)
	}
	if result := b.result(); result["outcome"] != "failed" {
		t.Fatalf("failed CLI result = %v", result)
	}
	if log, _ := os.ReadFile(filepath.Join(b.turn, "host.log")); !strings.Contains(string(log), "boom") {
		t.Fatalf("host.log lacks the CLI stderr: %q", log)
	}
}

func TestHostRefusesAbsentCLIAndBadArguments(t *testing.T) {
	t.Parallel()
	b := newHostBed(t, "claude", claudeStub)
	b.cli = "not-claude"
	if code := b.run("codex"); code != 3 || !strings.Contains(b.stderr.String(), "codex CLI is not installed") {
		t.Fatalf("absent CLI = %d %q", code, b.stderr.String())
	}
	if code := Main([]string{"claude", "start-turn", "--root", b.root, "--mission", "Bad_Id"}, b.deps); code != 2 {
		t.Fatalf("bad mission id = %d, want 2", code)
	}
	if code := Main([]string{"claude", "start-turn", "--root", "relative"}, b.deps); code != 2 {
		t.Fatalf("relative root = %d, want 2", code)
	}
	if code := Main([]string{"nope", "start-turn", "--root", b.root}, b.deps); code != 2 {
		t.Fatalf("unknown runtime = %d, want 2", code)
	}
}

func TestHostUnreleasedStartGateFailsTheLaunch(t *testing.T) {
	t.Parallel()
	b := newHostBed(t, "claude", claudeStub)
	b.env["METASYSTEM_HOST_START_GATE"] = filepath.Join(b.turn, "never-released")
	b.env["METASYSTEM_HOST_START_GATE_TIMEOUT_SEC"] = "1"
	if code := b.run("claude"); code != 3 || !strings.Contains(b.stderr.String(), "claude host start gate was not released within 1s") {
		t.Fatalf("unreleased gate = %d %q", code, b.stderr.String())
	}
	if _, err := os.Stat(filepath.Join(b.turn, "result.json")); err == nil {
		t.Fatal("a launch that never passed the gate wrote a result envelope")
	}
	b.env["METASYSTEM_HOST_START_GATE_TIMEOUT_SEC"] = "0"
	b.stderr.Reset()
	if code := b.run("claude"); code != 3 || !strings.Contains(b.stderr.String(), "start-gate timeout is invalid") {
		t.Fatalf("invalid gate timeout = %d %q", code, b.stderr.String())
	}
}

// codexStub emits the thread and turn events and writes the reply to the
// -o output file.
const codexStub = `out=
while [ $# -gt 0 ]; do
  case "$1" in -o|--output-last-message) out=$2; shift 2 ;; *) shift ;; esac
done
cat >/dev/null
printf '{"type":"thread.started","thread_id":"codex-thread"}\n'
printf '{"type":"turn.completed","usage":{"input_tokens":3,"output_tokens":2}}\n'
[ -z "$out" ] || printf '{"fixture":"return"}' >"$out"
`

func TestCodexHostCompletesWithItsThread(t *testing.T) {
	t.Parallel()
	b := newHostBed(t, "codex", codexStub)
	if code := b.run("codex"); code != 0 {
		t.Fatalf("codex host = %d: %s", code, b.stderr.String())
	}
	result := b.result()
	if result["outcome"] != "completed" || result["sessionId"] != "codex-thread" {
		t.Fatalf("codex result = %v", result)
	}
	if data, _ := os.ReadFile(filepath.Join(b.turn, "return.json")); string(data) != `{"fixture":"return"}` {
		t.Fatalf("codex return = %q", data)
	}
	if _, err := os.Stat(filepath.Join(b.turn, "usage.json")); err != nil {
		t.Fatalf("codex host wrote no usage: %v", err)
	}
}

// TestCodexHostRunsInTheCheckout: the Codex host CLI runs in the
// installation's checkout (hosts/codex.sh's `cd "$root"`), for a fresh turn
// and for a resumed thread, whose `codex exec resume` takes no -C.
func TestCodexHostRunsInTheCheckout(t *testing.T) {
	t.Parallel()
	for _, resume := range []string{"", "codex-thread"} {
		b := newHostBed(t, "codex", "pwd -P >\"$HOST_CWD_FILE\"\n"+codexStub)
		cwdFile := filepath.Join(b.root, "host-cwd")
		b.env["HOST_CWD_FILE"] = cwdFile
		var extra []string
		if resume != "" {
			extra = []string{"--resume-session", resume}
		}
		if code := b.run("codex", extra...); code != 0 {
			t.Fatalf("codex host (resume %q) = %d: %s", resume, code, b.stderr.String())
		}
		want, err := filepath.EvalSymlinks(b.root)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(cwdFile); strings.TrimSpace(string(got)) != want {
			t.Fatalf("codex host (resume %q) ran in %q, want the checkout %q", resume, got, want)
		}
	}
}
