package supervisor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// The Devin supervisor driven end to end: a real job record with an admitted
// launch capability, a stub `devin` executable the test writes, a fake
// dispatch.sh that records every callback and keeps the record the way the
// lease-held callbacks would, and Git stubbed per test.

const devinStub = `#!/bin/bash
S="$STUB_DIR"
printf '%s\n' "$*" >> "$S/argv.log"
case "$1" in
  list)
    if [ -f "$S/list-exit" ]; then exit "$(cat "$S/list-exit")"; fi
    n=0; [ -f "$S/list-count" ] && n=$(cat "$S/list-count"); n=$((n+1)); echo "$n" > "$S/list-count"
    if [ "$n" -eq 1 ]; then cat "$S/list-before"; else cat "$S/list-current"; fi
    exit 0 ;;
  acp)
    exec /bin/bash "$S/acp-server.sh" ;;
  -p)
    shift
    export_path=
    while [ $# -gt 0 ]; do
      case "$1" in --export) export_path=$2; shift 2 ;; *) shift ;; esac
    done
    n=0; [ -f "$S/p-count" ] && n=$(cat "$S/p-count"); n=$((n+1)); echo "$n" > "$S/p-count"
    pre=""; [ "$n" -gt 1 ] && pre="repair-"
    if [ -f "$S/${pre}wait-handshake" ]; then
      i=0; while [ ! -f "$S/handshaken" ] && [ $i -lt 1000 ]; do sleep 0.01; i=$((i+1)); done
    fi
    [ -f "$S/${pre}transcript" ] && cp "$S/${pre}transcript" "$export_path"
    [ -f "$S/${pre}named" ] && cp "$S/${pre}named" "$(cat "$S/${pre}named-path")"
    [ -f "$S/${pre}stdout" ] && cat "$S/${pre}stdout"
    exit "$(cat "$S/${pre}exit" 2>/dev/null || echo 0)" ;;
esac
exit 9
`

// callbackRecorder stands in for dispatch.sh's lease-held internal callbacks.
type callbackRecorder struct {
	t        *testing.T
	record   string
	stubDir  string
	claimRC  int
	mu       sync.Mutex
	calls    [][]string
	patches  []map[string]any
	casCodes []int
}

func (f *callbackRecorder) readRecord() map[string]any {
	data, err := os.ReadFile(f.record)
	if err != nil {
		f.t.Errorf("fake dispatch: %v", err)
		return map[string]any{}
	}
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		f.t.Errorf("fake dispatch: %v", err)
	}
	return record
}

func (f *callbackRecorder) writeRecord(record map[string]any) {
	data, _ := json.MarshalIndent(record, "", "  ")
	if err := os.WriteFile(f.record, data, 0o644); err != nil {
		f.t.Errorf("fake dispatch: %v", err)
	}
}

func flagValue(args []string, name string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

func (f *callbackRecorder) Run(stdout, stderr io.Writer, args ...string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string(nil), args...))
	record := f.readRecord()
	switch args[0] {
	case "__register-custody":
		custody, _ := record["custodyProcesses"].([]any)
		record["custodyProcesses"] = append(custody, map[string]any{"pid": flagValue(args, "--pid")})
		f.writeRecord(record)
		return 0
	case "__handshake":
		if record["status"] != "pending" {
			return 3
		}
		record["status"] = "running"
		record["sessionId"] = flagValue(args, "--session")
		f.writeRecord(record)
		_ = os.WriteFile(filepath.Join(f.stubDir, "handshaken"), nil, 0o644)
		return 0
	case "__record-cas":
		var patch map[string]any
		data, _ := os.ReadFile(flagValue(args, "--patch"))
		_ = json.Unmarshal(data, &patch)
		f.patches = append(f.patches, patch)
		if record["status"] != flagValue(args, "--expect") {
			f.casCodes = append(f.casCodes, 3)
			return 3
		}
		for key, value := range patch {
			record[key] = value
		}
		record["status"] = flagValue(args, "--status")
		f.writeRecord(record)
		f.casCodes = append(f.casCodes, 0)
		return 0
	case "__protocol-error":
		if record["status"] != "running" {
			return 3
		}
		record["status"] = "failed"
		record["error"] = "protocol_error"
		f.writeRecord(record)
		return 0
	case "__repair-claim":
		return f.claimRC
	}
	return 2
}

func (f *callbackRecorder) verbs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, call := range f.calls {
		word := call[0]
		if word == "__record-cas" {
			word += ":" + flagValue(call, "--expect") + ">" + flagValue(call, "--status")
		}
		out = append(out, word)
	}
	return out
}

// terminal is the last record status and error the fake holds.
func (f *callbackRecorder) terminal() (string, string, string) {
	record := f.readRecord()
	status, _ := record["status"].(string)
	failure, _ := record["error"].(string)
	phase, _ := record["phase"].(string)
	return status, failure, phase
}

type devinFixture struct {
	t                                *testing.T
	root, workspace, stubDir, record string
	roundDir, job, tag, gate, capRaw string
	dispatch                         *callbackRecorder
	stdout, stderr                   *bytes.Buffer
	env                              map[string]string
	validReturn                      string
}

func resolvedTempDir(t *testing.T) string {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newDevinFixture(t *testing.T, transport, verb string) *devinFixture {
	t.Helper()
	f := &devinFixture{t: t, root: resolvedTempDir(t), job: "job-1", tag: "metasystem-job-job-1-abc", capRaw: "launch-word"}
	f.workspace = filepath.Join(f.root, "workspace")
	f.stubDir = filepath.Join(f.root, "stub")
	f.record = filepath.Join(f.root, "artifacts", "agents", "jobs", f.job+".json")
	f.roundDir = filepath.Join(f.root, "artifacts", "agents", f.job, "rounds", "1")
	f.gate = filepath.Join(f.root, "gate")
	for _, dir := range []string{f.workspace, f.stubDir, f.roundDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "protocol", "schemas", "implementer.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.root, "internal", "protocol", "schemas", "implementer.schema.json"), string(schema))
	// The fixture's empty transport is legacy, named: an absent key resolves
	// the compiled default (acp); an absent file is unreadable.
	conf := "dispatch.transport.devin=legacy\n"
	if transport == "absent" {
		conf = "# no transport key\n"
	} else if transport != "" {
		conf = "dispatch.transport.devin=" + transport + "\n"
	}
	writeFile(t, filepath.Join(f.root, "metasystem.conf"), conf)
	writeFile(t, f.gate, "open\n")
	writeFile(t, filepath.Join(f.roundDir, "prompt.md"), "do the work\n")
	writeFile(t, filepath.Join(f.roundDir, "composition.json"), `{"references":[]}`)
	digest := sha256.Sum256([]byte(f.capRaw))
	record := map[string]any{
		"jobId": f.job, "status": "pending", "instanceTag": f.tag, "round": 1,
		"role": "implementer", "runtime": "devin", "workspaceRoot": f.workspace,
		"requestedModel": "devin-model", "sessionId": nil, "effectiveModel": nil,
		"permissions": map[string]any{"requested": map[string]any{
			"readRoots": []any{f.workspace}, "writeRoots": []any{f.workspace},
			"network": "deny", "approvals": "deny", "tools": "runtime-default",
		}},
		"launchCapability": map[string]any{
			"digest": hex.EncodeToString(digest[:]), "jobId": f.job, "operationId": f.job,
			"instanceTag": f.tag, "adapterVerb": verb, "status": "minted",
			"mintedAt": "2026-09-27T10:00:00Z",
		},
	}
	if verb == "follow-up" {
		record["sessionId"] = "sess-1"
	}
	data, _ := json.MarshalIndent(record, "", "  ")
	writeFile(t, f.record, string(data))
	stub := filepath.Join(f.stubDir, "devin")
	writeFile(t, stub, devinStub)
	if err := os.Chmod(stub, 0o755); err != nil {
		t.Fatal(err)
	}
	f.dispatch = &callbackRecorder{t: t, record: f.record, stubDir: f.stubDir}
	f.stdout, f.stderr = &bytes.Buffer{}, &bytes.Buffer{}
	f.env = map[string]string{
		"PATH":                                  "/usr/bin:/bin",
		"HOME":                                  f.root,
		"STUB_DIR":                              f.stubDir,
		"METASYSTEM_HEARTBEAT_INTERVAL_MS":      "5",
		"METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS": "5",
	}
	f.validReturn = `{
	  "schemaVersion": 2, "jobId": "job-1", "round": 1, "runtime": "devin",
	  "sessionId": "sess-1",
	  "model": {"requested": "devin-model", "effective": "unobserved"},
	  "evidence": [{"command": "local fixture", "observed": "canned role return", "level": "ran"}],
	  "gaps": [], "mode": "implement",
	  "riskiestPart": "fixture boundary", "diffBoundary": [],
	  "whatWasDone": "fixture implementation",
	  "claimed": {"sessionId": null, "model": null}
	}`
	return f
}

func (f *devinFixture) deps() Deps {
	var environ []string
	for name, value := range f.env {
		environ = append(environ, name+"="+value)
	}
	stub := filepath.Join(f.stubDir, "devin")
	return Deps{
		Root: f.root, Engine: "/nonexistent/metasystem", Environ: environ,
		Getenv: func(name string) string { return f.env[name] },
		Pid:    os.Getpid(), Stdout: f.stdout, Stderr: f.stderr,
		Clock:    SystemClock(),
		Dispatch: f.dispatch,
		GroupMembers: func(int, ...int) ([]int, error) {
			return nil, nil
		},
		LookPath: func(name string) (string, error) {
			if name == "devin" {
				return stub, nil
			}
			return "", fmt.Errorf("%s: not found", name)
		},
	}
}

func (f *devinFixture) stub(name, content string) {
	writeFile(f.t, filepath.Join(f.stubDir, name), content)
}

func (f *devinFixture) run(verb string) int {
	return Main([]string{"devin", verb, "--root", f.root,
		"--job", f.job, "--start-gate", f.gate, "--instance-tag", f.tag,
		"--launch-capability", f.capRaw}, func(string) Deps { return f.deps() })
}

func (f *devinFixture) transcript(session string) string {
	return fmt.Sprintf(`{"session_id":%q,"agent":{"model_name":"devin-model"},"final_metrics":{"total_prompt_tokens":10,"total_completion_tokens":2,"total_cached_tokens":0,"total_steps":3}}`, session)
}

// legacyHappyStubs stages a turn that correlates by listing and delivers the
// return on stdout.
func (f *devinFixture) legacyHappyStubs() {
	f.stub("list-before", `[]`)
	f.stub("list-current", fmt.Sprintf(`[{"id":"sess-1","working_directory":%q}]`, f.workspace))
	f.stub("wait-handshake", "")
	f.stub("transcript", f.transcript("sess-1"))
	f.stub("stdout", f.validReturn)
}

func (f *devinFixture) expectTerminal(status, failure string) {
	f.t.Helper()
	gotStatus, gotFailure, _ := f.dispatch.terminal()
	if gotStatus != status || gotFailure != failure {
		f.t.Fatalf("record ended %s/%s, want %s/%s\ncalls: %v\nstderr: %s\nlog: %s",
			gotStatus, gotFailure, status, failure, f.dispatch.verbs(), f.stderr, f.jobLog())
	}
}

func (f *devinFixture) jobLog() string {
	data, _ := os.ReadFile(filepath.Join(f.root, "artifacts", "agents", "jobs", f.job+".log"))
	return string(data)
}

func TestDevinTransportRefusalInvalid(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "carrier-pigeon", "dispatch")
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(f.stderr.String(), "devin transport refused: transport-config-invalid:carrier-pigeon") {
		t.Fatalf("stderr: %s", f.stderr)
	}
	if len(f.dispatch.verbs()) != 0 {
		t.Fatalf("a refused transport must not reach any callback: %v", f.dispatch.verbs())
	}
	record := f.dispatch.readRecord()
	if record["launchCapability"].(map[string]any)["status"] != "minted" {
		t.Fatal("a refused transport must not spend the launch capability")
	}
}

func TestDevinTransportRefusalUnreadable(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	// A duplicate key is a malformed source: the configuration cannot be
	// read, which refuses rather than failing open to legacy.
	writeFile(t, filepath.Join(f.root, "metasystem.conf"), "dispatch.transport.devin=acp\ndispatch.transport.devin=legacy\n")
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(f.stderr.String(), "devin transport refused: transport-config-unreadable") {
		t.Fatalf("stderr: %s", f.stderr)
	}
}

func TestDevinOutputStreamFollowsTransport(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ transport, want string }{
		{"", "/r/raw.out"}, {"legacy", "/r/raw.out"}, {"acp", "/r/acp-outcome.json"}, {"absent", "/r/acp-outcome.json"},
	} {
		f := newDevinFixture(t, tc.transport, "dispatch")
		code := Main([]string{"devin", "output-stream", "--root", f.root, "--round-dir", "/r/"}, func(string) Deps { return f.deps() })
		if code != 0 || strings.TrimSpace(f.stdout.String()) != tc.want {
			t.Fatalf("transport %q: exit %d stream %q", tc.transport, code, f.stdout)
		}
	}
	f := newDevinFixture(t, "bogus", "dispatch")
	code := Main([]string{"devin", "output-stream", "--root", f.root, "--round-dir", "/r"}, func(string) Deps { return f.deps() })
	if code != 1 || !strings.Contains(f.stderr.String(), "devin transport refused: transport-config-invalid:bogus") {
		t.Fatalf("exit %d stderr %s", code, f.stderr)
	}
}

func TestDevinSmallVerbs(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	run := func(args ...string) (int, string) {
		f.stdout.Reset()
		code := Main(append([]string{"devin", args[0], "--root", f.root}, args[1:]...), func(string) Deps { return f.deps() })
		return code, f.stdout.String()
	}
	if code, out := run("local-config-paths"); code != 0 || out != ".devin/config.json\n.devin/config.local.json\n.devin/hooks.v1.json\n" {
		t.Fatalf("local-config-paths: %d %q", code, out)
	}
	if code, out := run("enforcement-map"); code != 0 || out != `{"writeRoots":"notEnforced","readRoots":"notEnforced","network":"notEnforced"}`+"\n" {
		t.Fatalf("enforcement-map: %d %q", code, out)
	}
	if code, out := run("signature"); code != 0 || !strings.Contains(out, "devin-delegate-acp") || !strings.Contains(out, `exclude ^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)`) {
		t.Fatalf("signature: %d %q", code, out)
	}
	if code, out := run("contract"); code != 0 || !strings.Contains(out, `"runtime"`) {
		t.Fatalf("contract: %d %q", code, out)
	}
}

func TestDevinTransportSwitchRefusedLegacyToACP(t *testing.T) {
	t.Parallel()
	// The record was born on legacy; the configuration now says acp.
	f := newDevinFixture(t, "acp", "follow-up")
	record := f.dispatch.readRecord()
	record["transport"] = "legacy"
	f.dispatch.writeRecord(record)
	if code := f.run("follow-up"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "transport_switch_refused")
}

func TestDevinTransportSwitchRefusedACPToLegacy(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "legacy", "follow-up")
	record := f.dispatch.readRecord()
	record["transport"] = "acp"
	f.dispatch.writeRecord(record)
	if code := f.run("follow-up"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "transport_switch_refused")
}

func TestDevinStaleNamedReturnRefusedBothTransports(t *testing.T) {
	t.Parallel()
	for _, transport := range []string{"legacy", "acp"} {
		f := newDevinFixture(t, transport, "dispatch")
		writeFile(t, filepath.Join(f.roundDir, "devin-return.json"), "{}")
		if code := f.run("dispatch"); code != 1 {
			t.Fatalf("%s: exit %d", transport, code)
		}
		f.expectTerminal("failed", "stale_named_return")
		if data, _ := os.ReadFile(filepath.Join(f.roundDir, "devin-return.json")); string(data) != "{}" {
			t.Fatalf("%s: the stale file must never be clobbered", transport)
		}
	}
}

func TestDevinLegacyHappyPathCompletes(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	if code := f.run("dispatch"); code != 0 {
		t.Fatalf("exit %d\ncalls %v\nstderr %s\nlog %s", code, f.dispatch.verbs(), f.stderr, f.jobLog())
	}
	f.expectTerminal("completed", "")
	verbs := strings.Join(f.dispatch.verbs(), " ")
	want := "__register-custody __handshake __record-cas:running>running __record-cas:running>completed"
	if verbs != want {
		t.Fatalf("callback sequence %q, want %q", verbs, want)
	}
	argv, _ := os.ReadFile(filepath.Join(f.stubDir, "argv.log"))
	lines := strings.Split(strings.TrimSpace(string(argv)), "\n")
	if lines[0] != "list --format json" {
		t.Fatalf("the baseline must list first: %q", lines)
	}
	var turn string
	for _, line := range lines {
		if strings.HasPrefix(line, "-p ") {
			turn = line
		}
	}
	for _, want := range []string{
		"--prompt-file " + filepath.Join(f.roundDir, "prompt.devin.md"),
		"--respect-workspace-trust false", "--model devin-model", "--permission-mode dangerous",
		"--config " + filepath.Join(f.roundDir, f.tag),
		"--export " + filepath.Join(f.roundDir, "transcript.atif.json"),
	} {
		if !strings.Contains(turn, want) {
			t.Fatalf("turn argv %q lacks %q", turn, want)
		}
	}
	if strings.Contains(turn, " -r ") {
		t.Fatalf("a fresh dispatch must not resume: %q", turn)
	}
	events, _ := os.ReadFile(filepath.Join(f.roundDir, "events.jsonl"))
	if !strings.Contains(string(events), `{"type":"session-correlated","session_id":"sess-1","predicate":"listed-for-this-workspace-plus-live-process"}`) {
		t.Fatalf("events: %s", events)
	}
	if !strings.Contains(f.jobLog(), "devin cli exit status=0") {
		t.Fatalf("log: %s", f.jobLog())
	}
	if _, err := os.Stat(filepath.Join(f.roundDir, "session-usage.json")); err != nil {
		t.Fatalf("cumulative usage for the successor: %v", err)
	}
}

func TestDevinLegacyFollowUpResumesWithoutListingCorrelation(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "follow-up")
	f.legacyHappyStubs()
	if code := f.run("follow-up"); code != 0 {
		t.Fatalf("exit %d\ncalls %v\nstderr %s\nlog %s", code, f.dispatch.verbs(), f.stderr, f.jobLog())
	}
	argv, _ := os.ReadFile(filepath.Join(f.stubDir, "argv.log"))
	if !strings.Contains(string(argv), "-r sess-1") {
		t.Fatalf("follow-up must resume the requested session: %s", argv)
	}
	if strings.Count(string(argv), "list --format json") != 1 {
		t.Fatalf("a follow-up lists only the baseline: %s", argv)
	}
	// Round 1 has no predecessor artifact, so the resumed round's usage is
	// published unavailable rather than as session totals.
	usage, _ := os.ReadFile(filepath.Join(f.roundDir, "usage.json"))
	if !strings.Contains(string(usage), `"unavailable"`) {
		t.Fatalf("usage without a predecessor must be unavailable: %s", usage)
	}
}

func TestDevinSessionBaselineUnavailable(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.stub("list-exit", "4")
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "session_baseline_unavailable")
	if strings.Contains(strings.Join(f.dispatch.verbs(), " "), "__register-custody") {
		t.Fatal("a refused baseline must not launch the CLI")
	}
}

func TestDevinSessionBaselineUnreadable(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.stub("list-before", `[{"id": "half`)
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "session_baseline_unreadable")
}

func TestDevinAmbiguousSessionCorrelation(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.stub("list-before", `[]`)
	f.stub("list-current", fmt.Sprintf(`[{"id":"sess-1","working_directory":%q},{"id":"sess-2","working_directory":%q}]`, f.workspace, f.workspace))
	f.stub("wait-handshake", "")
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "ambiguous_session_correlation")
	if !strings.Contains(f.jobLog(), "ambiguous-session-correlation:sess-1,sess-2") {
		t.Fatalf("log: %s", f.jobLog())
	}
}

func TestDevinTranscriptOversize(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	f.stub("transcript", `{"session_id":"sess-1","pad":"`+strings.Repeat("x", devinTranscriptCeiling)+`"}`)
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "transcript_oversize")
}

func TestDevinEmptyReplyWithoutSession(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.stub("list-before", `[]`)
	f.stub("list-current", `[]`)
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "empty_reply")
}

func TestDevinHandshakeMissingSessionWhenCandidatesPresent(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.stub("list-before", `[]`)
	f.stub("list-current", `[]`)
	// No session correlated, empty stdout, but the named file exists: the
	// presence scan names the missing handshake, not an empty reply.
	f.stub("named", f.validReturn)
	f.stub("named-path", filepath.Join(f.roundDir, "devin-return.json"))
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "handshake_missing_session_id")
}

func TestDevinDeliveryRepairClaimSpentIsEmptyReply(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	os.Remove(filepath.Join(f.stubDir, "stdout"))
	f.dispatch.claimRC = 3
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "empty_reply")
	if !strings.Contains(strings.Join(f.dispatch.verbs(), " "), "__repair-claim") {
		t.Fatalf("the repair must be claimed before the paid call: %v", f.dispatch.verbs())
	}
	argv, _ := os.ReadFile(filepath.Join(f.stubDir, "argv.log"))
	if strings.Count(string(argv), "-p ") != 1 {
		t.Fatalf("a spent claim must not run the repair CLI: %s", argv)
	}
}

func TestDevinDeliveryRepairClaimMechanical(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	os.Remove(filepath.Join(f.stubDir, "stdout"))
	f.dispatch.claimRC = 1
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "collect_mechanical")
}

func TestDevinDeliveryRepairDelivers(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	os.Remove(filepath.Join(f.stubDir, "stdout"))
	// The repair turn resumes the session and writes the named repair file.
	f.stub("repair-transcript", f.transcript("sess-1"))
	f.stub("repair-named", f.validReturn)
	f.stub("repair-named-path", filepath.Join(f.roundDir, "devin-return.repair-1.json"))
	if code := f.run("dispatch"); code != 0 {
		t.Fatalf("exit %d\ncalls %v\nstderr %s\nlog %s", code, f.dispatch.verbs(), f.stderr, f.jobLog())
	}
	f.expectTerminal("completed", "")
	argv, _ := os.ReadFile(filepath.Join(f.stubDir, "argv.log"))
	if !strings.Contains(string(argv), "-r sess-1 --export "+filepath.Join(f.roundDir, "transcript.repair-1.atif.json")) {
		t.Fatalf("the repair must resume the same session: %s", argv)
	}
	if !strings.Contains(f.jobLog(), "delivery repair attempt 1: no return was delivered, asking session sess-1 to write") {
		t.Fatalf("log: %s", f.jobLog())
	}
}

func TestDevinDeliveryRepairProviderFailureIsProtocolError(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	os.Remove(filepath.Join(f.stubDir, "stdout"))
	f.stub("repair-exit", "7")
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "protocol_error")
	violation, _ := os.ReadFile(filepath.Join(f.roundDir, "protocol-violation.txt"))
	if string(violation) != "delivery repair provider call failed rc=7\n" {
		t.Fatalf("violation %q", violation)
	}
	// No repair transcript: the repair's spend cannot be read.
	usage, _ := os.ReadFile(filepath.Join(f.roundDir, "usage.json"))
	if !strings.Contains(string(usage), `"unavailable"`) {
		t.Fatalf("usage %s", usage)
	}
}

func TestDevinSessionIdentityDisagreement(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	f.stub("transcript", f.transcript("sess-other"))
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "session_identity_disagreement")
}

// acpServerScript is a stub `devin acp` server: initialize, session/new
// (or session/load), set_mode, prompt — answered with the candidate as one
// agent message chunk and the usage verbatim.
func acpServerScript(t *testing.T, session, candidate string) string {
	text, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(`read -r _
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}'
read -r _
echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"%[1]s"}}'
read -r request
case "$request" in
  *session/set_mode*) echo '{"jsonrpc":"2.0","id":3,"result":{}}' ;;
  *) echo "expected set_mode, got: $request" >&2; exit 9 ;;
esac
read -r _
cat <<'FRAME'
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"%[1]s","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":%[2]s}}}}
{"jsonrpc":"2.0","id":4,"result":{"stopReason":"end_turn","usage":{"inputTokens":9,"outputTokens":2,"totalTokens":11}}}
FRAME
exec cat >/dev/null
`, session, text)
}

func fifosLeft(t *testing.T, dir string) []string {
	var left []string
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if entry.Type()&os.ModeNamedPipe != 0 {
			left = append(left, entry.Name())
		}
	}
	return left
}

func TestDevinACPHappyPathCompletes(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "acp", "dispatch")
	compact := strings.Join(strings.Fields(f.validReturn), " ")
	f.stub("acp-server.sh", acpServerScript(t, "sess-1", compact))
	if code := f.run("dispatch"); code != 0 {
		t.Fatalf("exit %d\ncalls %v\nstderr %s\nlog %s\nrecord %v", code, f.dispatch.verbs(), f.stderr, f.jobLog(), f.dispatch.readRecord())
	}
	f.expectTerminal("completed", "")
	verbs := strings.Join(f.dispatch.verbs(), " ")
	// The server child is custody-registered; the in-process client is not.
	want := "__register-custody __handshake __record-cas:running>running __record-cas:running>completed"
	if verbs != want {
		t.Fatalf("callback sequence %q, want %q", verbs, want)
	}
	record := f.dispatch.readRecord()
	if record["transport"] != "acp" {
		t.Fatalf("the transport pin must be recorded: %v", record["transport"])
	}
	events, _ := os.ReadFile(filepath.Join(f.roundDir, "events.jsonl"))
	if !strings.Contains(string(events), fmt.Sprintf(`"client_pid":%d,"client":"in-process","mode":"accept-edits"`, os.Getpid())) {
		t.Fatalf("events: %s", events)
	}
	// The mid-turn handshake writes the correlation event; a turn that
	// settles before the loop reads the session file handshakes from the
	// outcome instead, which writes none (as the script did).
	if strings.Contains(string(events), "session-correlated") &&
		!strings.Contains(string(events), `{"type":"session-correlated","session_id":"sess-1","predicate":"acp-wire-typed"}`) {
		t.Fatalf("events: %s", events)
	}
	argv, _ := os.ReadFile(filepath.Join(f.stubDir, "argv.log"))
	if strings.TrimSpace(string(argv)) != "acp" {
		t.Fatalf("server argv: %q", argv)
	}
	journal, _ := os.ReadFile(filepath.Join(f.roundDir, "acp-journal.log"))
	if !strings.Contains(string(journal), "session/set_mode") {
		t.Fatalf("the graded set_mode must reach the wire: %s", journal)
	}
	if left := fifosLeft(t, f.roundDir); len(left) != 0 {
		t.Fatalf("fifo pair left as evidence: %v", left)
	}
	if !strings.Contains(f.jobLog(), "acp client exit status=0") {
		t.Fatalf("log: %s", f.jobLog())
	}
	usage, _ := os.ReadFile(filepath.Join(f.roundDir, "usage.json"))
	if !strings.Contains(string(usage), "11") && !strings.Contains(string(usage), "9") {
		t.Fatalf("usage from the wire: %s", usage)
	}
}

func TestDevinACPPreflightRefused(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "acp", "dispatch")
	record := f.dispatch.readRecord()
	record["permissions"].(map[string]any)["requested"].(map[string]any)["network"] = "ask"
	f.dispatch.writeRecord(record)
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	f.expectTerminal("failed", "acp_preflight_refused")
	if !strings.Contains(f.jobLog(), "network=ask is unsupported on ACP v1") {
		t.Fatalf("log: %s", f.jobLog())
	}
	if left := fifosLeft(t, f.roundDir); len(left) != 0 {
		t.Fatalf("a refused preflight must not create fifos: %v", left)
	}
}

func TestDevinACPModeUnmapped(t *testing.T) {
	t.Parallel()
	// Preflight admits only mapped grades, so the unmapped refusal is the
	// dialect's defense in depth; the resolver itself refuses by name.
	if _, err := devinACPMode(""); err == nil {
		t.Fatal("an empty grade must refuse")
	}
	if _, err := devinACPMode("bypass"); err == nil || !strings.Contains(err.Error(), "no mode mapped for tools=bypass") {
		t.Fatalf("err %v", err)
	}
	if mode, err := devinACPMode("read-only"); err != nil || mode != "ask" {
		t.Fatalf("read-only maps to ask: %q %v", mode, err)
	}
}

func TestDevinACPServerDiedBeforeHandshake(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "acp", "dispatch")
	f.stub("acp-server.sh", "exit 0\n")
	if code := f.run("dispatch"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	status, failure, _ := f.dispatch.terminal()
	// The server's death either wins the handshake loop (acp_server_died)
	// or reaches the client first, whose typed outcome names the dead peer;
	// both land pending-failed before any session.
	if status != "failed" || !strings.HasPrefix(failure, "acp_") {
		t.Fatalf("record ended %s/%s\nlog %s", status, failure, f.jobLog())
	}
	if left := fifosLeft(t, f.roundDir); len(left) != 0 {
		t.Fatalf("fifo pair left: %v", left)
	}
}

func TestDevinACPSignalCancelsTheClient(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "acp", "dispatch")
	observed := filepath.Join(f.stubDir, "cancel-observed")
	f.stub("acp-server.sh", fmt.Sprintf(`read -r _
echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":[]}}'
read -r _
echo '{"jsonrpc":"2.0","id":2,"result":{"sessionId":"sess-1"}}'
read -r _
echo '{"jsonrpc":"2.0","id":3,"result":{}}'
read -r _
read -r cancel_line || cancel_line=""
case "$cancel_line" in *session/cancel*) echo seen > %q ;; esac
exec cat >/dev/null
`, observed))
	signals := make(chan chan<- os.Signal, 1)
	signalHooks.Store(f.root, func(c chan<- os.Signal) { signals <- c })
	t.Cleanup(func() { signalHooks.Delete(f.root) })
	go func() {
		c := <-signals
		journal := filepath.Join(f.roundDir, "acp-journal.log")
		// Unbounded here; the test binary's timeout bounds it.
		for {
			if data, _ := os.ReadFile(journal); strings.Contains(string(data), "session/prompt") {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		c <- syscall.SIGTERM
	}()
	if code := f.run("dispatch"); code != 143 {
		t.Fatalf("exit %d, want 143\nlog %s", code, f.jobLog())
	}
	// The wire decides: the journal holds the outbound courtesy cancel (the
	// stub's own observation races its termination).
	if data, _ := os.ReadFile(filepath.Join(f.roundDir, "acp-journal.log")); !strings.Contains(string(data), "session/cancel") {
		t.Fatalf("the courtesy session/cancel never reached the wire: %s", data)
	}
	_ = observed
	outcome, _ := os.ReadFile(filepath.Join(f.roundDir, "acp-outcome.json"))
	if !strings.Contains(string(outcome), `"row":"cancelled"`) {
		t.Fatalf("the typed cancelled outcome: %s", outcome)
	}
	if left := fifosLeft(t, f.roundDir); len(left) != 0 {
		t.Fatalf("fifo pair left: %v", left)
	}
}
