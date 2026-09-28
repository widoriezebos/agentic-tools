package hostturn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"golang.org/x/sys/unix"
)

// The Devin host turn driven end to end against a stub `devin` the test
// writes (the ported ACP-H fixtures and the legacy host turn).

const hostDevinStub = `#!/bin/bash
S="$STUB_DIR"
printf '%s\n' "$*" >> "$S/argv.log"
case "$1" in
  acp) exec /bin/bash "$S/acp-server.sh" ;;
  -p)
    shift
    export_path=
    while [ $# -gt 0 ]; do
      case "$1" in --export) export_path=$2; shift 2 ;; *) shift ;; esac
    done
    [ -f "$S/transcript" ] && cp "$S/transcript" "$export_path"
    [ -f "$S/stdout" ] && cat "$S/stdout"
    echo "stub stderr" >&2
    exit "$(cat "$S/exit" 2>/dev/null || echo 0)" ;;
esac
echo "fake devin: unscripted mode $1" >&2
exit 9
`

type hostFixture struct {
	t              *testing.T
	root, stubDir  string
	stdout, stderr *bytes.Buffer
	env            map[string]string
}

func newHostFixture(t *testing.T, conf string) *hostFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &hostFixture{t: t, root: root, stubDir: filepath.Join(root, "stub")}
	f.write(filepath.Join(root, "metasystem.conf"), conf)
	stub := filepath.Join(f.stubDir, "devin")
	f.write(stub, hostDevinStub)
	if err := os.Chmod(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	f.stdout, f.stderr = &bytes.Buffer{}, &bytes.Buffer{}
	f.env = map[string]string{"PATH": "/usr/bin:/bin", "HOME": root, "STUB_DIR": f.stubDir}
	return f
}

func (f *hostFixture) write(path, content string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *hostFixture) deps() supervisor.Deps {
	var environ []string
	for name, value := range f.env {
		environ = append(environ, name+"="+value)
	}
	stub := filepath.Join(f.stubDir, "devin")
	return supervisor.Deps{
		Root: f.root, Environ: environ,
		Getenv: func(name string) string { return f.env[name] },
		Pid:    os.Getpid(), Stdout: f.stdout, Stderr: f.stderr,
		Clock: supervisor.SystemClock(),
		LookPath: func(name string) (string, error) {
			if name == "devin" {
				return stub, nil
			}
			return "", fmt.Errorf("%s: not found", name)
		},
	}
}

// turn stages turns/ID and runs one host turn.
func (f *hostFixture) turn(id, resume string) (int, string) {
	dir := filepath.Join(f.root, "turns", id)
	f.write(filepath.Join(dir, "turn.json"), `{"model":"devin-fixture-model"}`+"\n")
	f.write(filepath.Join(dir, "prompt.md"), "reply with the return\n")
	args := []string{"devin", "start-turn", "--root", f.root, "--mission", "fx", "--turn-id", id,
		"--prompt", filepath.Join(dir, "prompt.md"), "--result", filepath.Join(dir, "result.json"),
		"--instance-tag", "metasystem-host-fx-" + id}
	if resume != "" {
		args = append(args, "--resume-session", resume)
	}
	return Main(args, func(string) supervisor.Deps { return f.deps() }), dir
}

func readField(t *testing.T, path, name string) any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return object[name]
}

const hostACPServer = `read -r _
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}'
read -r _
echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"host-fx-1"}}'
read -r request
case "$request" in
  *session/set_mode*) echo '{"jsonrpc":"2.0","id":3,"result":{}}' ;;
  *) echo "fake devin: expected set_mode, got: $request" >&2; exit 9 ;;
esac
read -r _
echo '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"host-fx-1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"{\"status\":\"done\"}"}}}}'
echo '{"jsonrpc":"2.0","id":4,"result":{"stopReason":"end_turn","usage":{"inputTokens":9,"outputTokens":2,"totalTokens":11}}}'
exec cat >/dev/null
`

// ACP-H-001: the devin HOST rides the same wire; the result carries the
// acp transport pin with the wire session.
func TestDevinHostACPTurnCompletes(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "dispatch.transport.devin=acp\n")
	f.write(filepath.Join(f.stubDir, "acp-server.sh"), hostACPServer)
	code, dir := f.turn("t1", "")
	if code != 0 {
		log, _ := os.ReadFile(filepath.Join(dir, "host.log"))
		t.Fatalf("host turn exit %d\nstderr %s\nlog %s", code, f.stderr, log)
	}
	result := filepath.Join(dir, "result.json")
	if readField(t, result, "transport") != "acp" || readField(t, result, "sessionId") != "host-fx-1" || readField(t, result, "outcome") != "completed" {
		data, _ := os.ReadFile(result)
		t.Fatalf("result %s", data)
	}
	journal, _ := os.ReadFile(filepath.Join(dir, "acp-journal.log"))
	if !strings.Contains(string(journal), "session/set_mode") {
		t.Fatal("the graded set_mode never reached the wire")
	}
	returned, _ := os.ReadFile(filepath.Join(dir, "return.json"))
	if !strings.Contains(strings.ReplaceAll(string(returned), " ", ""), `"status":"done"`) {
		t.Fatalf("the wire reply did not become the return: %s", returned)
	}
	argv, _ := os.ReadFile(filepath.Join(f.stubDir, "argv.log"))
	if strings.TrimSpace(string(argv)) != "acp" {
		t.Fatalf("server argv %q", argv)
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if entry.Type()&os.ModeNamedPipe != 0 {
			t.Fatalf("fifo left in the turn directory: %s", entry.Name())
		}
	}
	if usage, _ := os.ReadFile(filepath.Join(dir, "usage.json")); len(usage) == 0 {
		t.Fatal("wire usage must be written")
	}
}

// ACP-H-002: an invalid transport value refuses before any launch and
// writes no result; an unreadable configuration refuses the same way.
func TestDevinHostTransportRefusals(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "dispatch.transport.devin=carrier-pigeon\n")
	code, dir := f.turn("t2", "")
	if code != 3 || !strings.Contains(f.stderr.String(), "devin host: transport configuration invalid: carrier-pigeon") {
		t.Fatalf("exit %d stderr %s", code, f.stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "result.json")); err == nil {
		t.Fatal("a refused transport still wrote a result")
	}
	if _, err := os.Stat(filepath.Join(f.stubDir, "argv.log")); err == nil {
		t.Fatal("a refused transport launched the CLI")
	}
	f = newHostFixture(t, "dispatch.transport.devin=acp\ndispatch.transport.devin=legacy\n")
	code, _ = f.turn("t3", "")
	if code != 3 || !strings.Contains(f.stderr.String(), "devin host: transport configuration unreadable") {
		t.Fatalf("exit %d stderr %s", code, f.stderr)
	}
}

func TestDevinHostStaleNamedReturnRefused(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "# legacy by absence\n")
	dir := filepath.Join(f.root, "turns", "t1")
	f.write(filepath.Join(dir, "devin-return.json"), "{}")
	code, _ := f.turn("t1", "")
	if code != 3 || !strings.Contains(f.stderr.String(), "stale named return file from a crashed earlier attempt: "+filepath.Join(dir, "devin-return.json")) {
		t.Fatalf("exit %d stderr %s", code, f.stderr)
	}
}

func (f *hostFixture) legacyTurn(session string, prompt, steps int) {
	f.write(filepath.Join(f.stubDir, "stdout"), `{"status":"done"}`)
	f.write(filepath.Join(f.stubDir, "transcript"), fmt.Sprintf(
		`{"session_id":%q,"final_metrics":{"total_prompt_tokens":%d,"total_completion_tokens":2,"total_cached_tokens":0,"total_steps":%d}}`,
		session, prompt, steps))
}

// The legacy host turn, and the per-session usage store: the second turn of
// a session publishes the delta against the first turn's stored totals; a
// resumed turn with no stored predecessor publishes unavailable.
func TestDevinHostLegacyTurnsAndSessionUsageStore(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "# legacy by absence\n")
	f.legacyTurn("sess-h", 10, 1)
	code, dir := f.turn("t1", "")
	if code != 0 {
		log, _ := os.ReadFile(filepath.Join(dir, "host.log"))
		t.Fatalf("exit %d stderr %s log %s", code, f.stderr, log)
	}
	result := filepath.Join(dir, "result.json")
	if readField(t, result, "sessionId") != "sess-h" || readField(t, result, "outcome") != "completed" {
		data, _ := os.ReadFile(result)
		t.Fatalf("result %s", data)
	}
	if _, present := readFieldMap(t, result)["transport"]; present {
		t.Fatal("the legacy path records no transport pin")
	}
	argv, _ := os.ReadFile(filepath.Join(f.stubDir, "argv.log"))
	for _, want := range []string{"--permission-mode dangerous", "--model devin-fixture-model",
		"--config " + filepath.Join(dir, "devin-config.json"), "--prompt-file " + filepath.Join(dir, "prompt.devin.md")} {
		if !strings.Contains(string(argv), want) {
			t.Fatalf("argv %q lacks %q", argv, want)
		}
	}
	if log, _ := os.ReadFile(filepath.Join(dir, "host.log")); string(log) != "stub stderr\n" {
		t.Fatalf("host log is the CLI's stderr: %q", log)
	}
	store := filepath.Join(f.root, "turns", ".session-usage", lease.Slug("sess-h")+".json")
	if _, err := os.Stat(store); err != nil {
		t.Fatalf("the completed turn must publish its cumulative totals: %v", err)
	}

	f.legacyTurn("sess-h", 25, 2)
	code, dir2 := f.turn("t2", "sess-h")
	if code != 0 {
		t.Fatalf("exit %d stderr %s", code, f.stderr)
	}
	argv, _ = os.ReadFile(filepath.Join(f.stubDir, "argv.log"))
	if !strings.Contains(string(argv), "-r sess-h") {
		t.Fatalf("the resumed turn must pass -r: %s", argv)
	}
	usage := readFieldMap(t, filepath.Join(dir2, "usage.json"))
	if usage["availability"] == "unavailable" || fmt.Sprint(usage["inputTokens"]) != "15" {
		t.Fatalf("the second turn publishes the delta: %v", usage)
	}

	f.legacyTurn("sess-x", 40, 3)
	code, dir3 := f.turn("t3", "sess-x")
	if code != 0 {
		t.Fatalf("exit %d stderr %s", code, f.stderr)
	}
	if usage := readFieldMap(t, filepath.Join(dir3, "usage.json")); usage["availability"] != "unavailable" {
		t.Fatalf("a resumed turn without a stored predecessor is unavailable: %v", usage)
	}
}

func TestDevinHostLegacyEmptyReplyFails(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "# legacy by absence\n")
	f.write(filepath.Join(f.stubDir, "transcript"), `{"session_id":"sess-e"}`)
	code, dir := f.turn("t1", "")
	if code != 3 {
		t.Fatalf("an empty reply under --require-reply fails the turn: exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(f.root, "turns", ".session-usage", lease.Slug("sess-e")+".json")); err == nil {
		t.Fatal("a failed turn must not publish its totals")
	}
	if readField(t, filepath.Join(dir, "result.json"), "outcome") != "failed" {
		t.Fatal("the result names the failure")
	}
}

func TestDevinHostACPSignalCancels(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "dispatch.transport.devin=acp\n")
	observed := filepath.Join(f.stubDir, "cancel-observed")
	f.write(filepath.Join(f.stubDir, "acp-server.sh"), fmt.Sprintf(`read -r _
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}'
read -r _
echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"host-fx-c"}}'
read -r _
echo '{"jsonrpc":"2.0","id":3,"result":{}}'
read -r _
read -r cancel_line || cancel_line=""
case "$cancel_line" in *session/cancel*) echo seen > %q ;; esac
exec cat >/dev/null
`, observed))
	channels := make(chan chan<- os.Signal, 1)
	t.Cleanup(supervisor.HookTermination(f.root, func(c chan<- os.Signal) { channels <- c }))
	dir := filepath.Join(f.root, "turns", "t1")
	go func() {
		c := <-channels
		// Unbounded here; the test binary's timeout bounds it.
		for {
			if data, _ := os.ReadFile(filepath.Join(dir, "acp-journal.log")); strings.Contains(string(data), "session/prompt") {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		c <- syscall.SIGTERM
	}()
	code, _ := f.turn("t1", "")
	if code != 143 {
		t.Fatalf("exit %d, want 143", code)
	}
	// The wire decides: the journal holds the outbound courtesy cancel (the
	// stub's own observation races its termination).
	if data, _ := os.ReadFile(filepath.Join(dir, "acp-journal.log")); !strings.Contains(string(data), "session/cancel") {
		t.Fatalf("the courtesy session/cancel never reached the wire: %s", data)
	}
	_ = observed
	if _, err := os.Stat(filepath.Join(dir, "result.json")); err == nil {
		t.Fatal("a signalled host turn writes no result")
	}
}

// A server that dies before the client opens its ends must not strand the
// client in a blocked fifo open: the turn fails with a result.
func TestDevinHostACPServerDeadFailsTheTurn(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "dispatch.transport.devin=acp\n")
	f.write(filepath.Join(f.stubDir, "acp-server.sh"), "exit 0\n")
	code, dir := f.turn("t1", "")
	if code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
	if readField(t, filepath.Join(dir, "result.json"), "outcome") != "failed" {
		t.Fatal("the result names the failure")
	}
}

func readFieldMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return object
}

// A tool child of the ACP server that outlives the server and holds its
// stderr must not strand the host turn after delivery: the server writes
// host.log through its own descriptor, never a pipe exec.Cmd.Wait drains.
// Proven by ordering, not by seconds: the orphan holds the server's stderr
// until the test releases it, and the turn must return while it still holds
// it. A turn that waited on the orphan's stderr would never return; the test
// binary's own timeout reports that hang.
func TestDevinHostACPOrphanHoldingStderrDoesNotStrandTheTurn(t *testing.T) {
	t.Parallel()
	f := newHostFixture(t, "dispatch.transport.devin=acp\n")
	release := filepath.Join(f.stubDir, "orphan-release")
	if err := unix.Mkfifo(release, 0o600); err != nil {
		t.Fatal(err)
	}
	released := filepath.Join(f.stubDir, "orphan-released")
	orphan := "( IFS= read -r _ <'" + release + "'; : >'" + released + "' ) </dev/null >/dev/null &\nexit 0\n"
	f.write(filepath.Join(f.stubDir, "acp-server.sh"), strings.Replace(hostACPServer, "exec cat >/dev/null\n", orphan, 1))
	releaseOrphan := func() {
		// Read-write open never blocks on a FIFO; the newline ends the
		// orphan's read if it is still alive.
		if fifo, err := os.OpenFile(release, os.O_RDWR, 0); err == nil {
			_, _ = fifo.WriteString("\n")
			_ = fifo.Close()
		}
	}
	t.Cleanup(releaseOrphan)
	code, dir := f.turn("t-orphan", "")
	if code != 0 {
		log, _ := os.ReadFile(filepath.Join(dir, "host.log"))
		t.Fatalf("host turn exit %d\nstderr %s\nlog %s", code, f.stderr, log)
	}
	if _, err := os.Stat(released); err == nil {
		t.Fatal("the orphan was released before the turn returned: the ordering proves nothing")
	}
	if readField(t, filepath.Join(dir, "result.json"), "outcome") != "completed" {
		t.Fatal("the turn did not complete")
	}
}

// TestDevinHostRunsInTheCheckout: the Devin host CLI runs in the
// installation's checkout on both transports, as hosts/devin.sh did
// (`cd "$root"` for `devin -p`, and the ACP server started from the root).
func TestDevinHostRunsInTheCheckout(t *testing.T) {
	t.Parallel()
	want := func(f *hostFixture) string {
		root, err := filepath.EvalSymlinks(f.root)
		if err != nil {
			t.Fatal(err)
		}
		return root
	}
	recordCwd := func(f *hostFixture) {
		stub := filepath.Join(f.stubDir, "devin")
		f.write(stub, strings.Replace(hostDevinStub, "#!/bin/bash\n", "#!/bin/bash\npwd -P >\"$STUB_DIR/cwd\"\n", 1))
		if err := os.Chmod(stub, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	legacy := newHostFixture(t, "# legacy by absence\n")
	recordCwd(legacy)
	legacy.legacyTurn("sess-cwd", 10, 1)
	if code, dir := legacy.turn("t1", ""); code != 0 {
		log, _ := os.ReadFile(filepath.Join(dir, "host.log"))
		t.Fatalf("legacy exit %d stderr %s log %s", code, legacy.stderr, log)
	}
	if got, _ := os.ReadFile(filepath.Join(legacy.stubDir, "cwd")); strings.TrimSpace(string(got)) != want(legacy) {
		t.Fatalf("the legacy devin host ran in %q, want the checkout %q", got, want(legacy))
	}

	acp := newHostFixture(t, "dispatch.transport.devin=acp\n")
	recordCwd(acp)
	acp.write(filepath.Join(acp.stubDir, "acp-server.sh"), hostACPServer)
	if code, dir := acp.turn("t1", ""); code != 0 {
		log, _ := os.ReadFile(filepath.Join(dir, "host.log"))
		t.Fatalf("acp exit %d stderr %s log %s", code, acp.stderr, log)
	}
	if got, _ := os.ReadFile(filepath.Join(acp.stubDir, "cwd")); strings.TrimSpace(string(got)) != want(acp) {
		t.Fatalf("the devin ACP host server ran in %q, want the checkout %q", got, want(acp))
	}
}
