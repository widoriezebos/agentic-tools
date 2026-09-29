package supervisor

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// cliProbeStub answers `CLI --version` with a dotted version and the
// runtime's authentication status check with the exit code in auth-exit
// (0 when absent).
const cliProbeStub = `#!/bin/bash
case "$1" in
  --version) echo "$(basename "$0") 3.4.5" ;;
  auth|login) exit "$(cat "$(dirname "$0")/auth-exit" 2>/dev/null || echo 0)" ;;
  *) exit 9 ;;
esac
`

type probeFixture struct {
	root, home, stubs string
	stdout, stderr    *strings.Builder
	dispatch          *recordingDispatcher
	env               map[string]string
}

// newProbeFixture stages an installation root inside a stubbed git work
// tree, a HOME, and a stub CLI for runtime on the fixture's own PATH.
func newProbeFixture(t *testing.T, runtime string) *probeFixture {
	t.Helper()
	f := &probeFixture{
		root: resolvedTempDir(t), home: t.TempDir(), stubs: t.TempDir(),
		stdout: &strings.Builder{}, stderr: &strings.Builder{}, dispatch: &recordingDispatcher{},
	}
	f.env = map[string]string{"HOME": f.home}
	if err := testexec.WriteFile(filepath.Join(f.stubs, runtime), []byte(cliProbeStub), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *probeFixture) deps() Deps {
	return Deps{
		Root: f.root, Engine: "/no/engine", Environ: []string{"PATH=/usr/bin:/bin"},
		Getenv: func(name string) string { return f.env[name] },
		Stdout: f.stdout, Stderr: f.stderr, Dispatch: f.dispatch,
		LookPath: func(name string) (string, error) {
			path := filepath.Join(f.stubs, name)
			if _, err := os.Stat(path); err != nil {
				return "", exec.ErrNotFound
			}
			return path, nil
		},
		Git: func(_ string, args ...string) (string, bool) {
			if strings.Join(args, " ") == "rev-parse --show-toplevel" {
				return f.root, true
			}
			return "", false
		},
	}
}

func (f *probeFixture) snapshots(t *testing.T, runtime string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(f.root, "artifacts", "agents", "capabilities", runtime+"-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

// The Claude and Codex probes: the CLI version and the settings sources the
// session merges make the configuration identity, the CLI's authentication
// check gates it, and the snapshot carries the runtime's declared
// transports, capabilities and registry enforcement. The printed line is
// the snapshot's path.
func TestBuiltinProbesWriteTheirSnapshot(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		runtime     string
		probe       func(Deps, []string) int
		transports  string
		nativeBudge bool
		configEnv   string
	}{
		{"claude", claudeProbe, `["stdin","file","json","stream-json"]`, true, "CLAUDE_CONFIG_DIR"},
		{"codex", codexProbe, `["stdin","jsonl","file"]`, false, "CODEX_HOME"},
	} {
		for _, explicitConfig := range []bool{false, true} {
			f := newProbeFixture(t, row.runtime)
			if explicitConfig {
				f.env[row.configEnv] = t.TempDir()
			}
			if code := row.probe(f.deps(), nil); code != 0 {
				t.Fatalf("%s probe exit %d: %s", row.runtime, code, f.stderr)
			}
			matches := f.snapshots(t, row.runtime)
			if len(matches) != 1 || strings.TrimSpace(f.stdout.String()) != matches[0] {
				t.Fatalf("%s snapshots %v, printed %q", row.runtime, matches, f.stdout)
			}
			data, err := os.ReadFile(matches[0])
			if err != nil {
				t.Fatal(err)
			}
			var snapshot map[string]any
			if err := json.Unmarshal(data, &snapshot); err != nil {
				t.Fatal(err)
			}
			if snapshot["runtime"] != row.runtime || snapshot["cliVersion"] != "3.4.5" {
				t.Fatalf("%s identity %v %v", row.runtime, snapshot["runtime"], snapshot["cliVersion"])
			}
			if transports, _ := json.Marshal(snapshot["transports"]); string(transports) != row.transports {
				t.Fatalf("%s transports %s", row.runtime, transports)
			}
			caps, _ := snapshot["capabilities"].(map[string]any)
			if caps["nativeBudget"] != row.nativeBudge || caps["protocolServer"] != true {
				t.Fatalf("%s capabilities %v", row.runtime, caps)
			}
			declared, _ := runtimes.EnforcementMapJSON(row.runtime)
			var want, got any
			_ = json.Unmarshal([]byte(declared), &want)
			got = snapshot["envelopeEnforcement"]
			if wantJSON, _ := json.Marshal(want); string(wantJSON) != probeSnapshotJSON(t, got) {
				t.Fatalf("%s enforcement %v, registry %s", row.runtime, got, declared)
			}
		}
	}
}

func probeSnapshotJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Every refusal the probe can meet names its cause and writes no snapshot.
func TestBuiltinProbesRefuseWithoutTheirFacts(t *testing.T) {
	t.Parallel()
	for _, runtime := range []string{"claude", "codex"} {
		probe := registry[runtime].probe
		for _, row := range []struct {
			name   string
			edit   func(*probeFixture, *Deps)
			args   []string
			code   int
			stderr string
		}{
			{"unauthenticated", func(f *probeFixture, _ *Deps) {
				if err := os.WriteFile(filepath.Join(f.stubs, "auth-exit"), []byte("1\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}, nil, 1, runtime + " authentication is unavailable"},
			{"no CLI", func(f *probeFixture, _ *Deps) {
				if err := os.Remove(filepath.Join(f.stubs, runtime)); err != nil {
					t.Fatal(err)
				}
			}, nil, 1, runtime + " CLI is not installed"},
			{"no HOME", func(f *probeFixture, _ *Deps) { delete(f.env, "HOME") }, nil, 1, "HOME is not set"},
			{"outside a work tree", func(_ *probeFixture, d *Deps) {
				d.Git = func(string, ...string) (string, bool) { return "", false }
			}, nil, 1, "not inside a git work tree"},
			{"unexpected arguments", func(*probeFixture, *Deps) {}, []string{"--extra"}, 2, "Usage:"},
		} {
			f := newProbeFixture(t, runtime)
			d := f.deps()
			row.edit(f, &d)
			if code := probe(d, row.args); code != row.code || !strings.Contains(f.stderr.String()+f.stdout.String(), row.stderr) {
				t.Errorf("%s %s: exit %d output %q %q, want %d with %q", runtime, row.name, code, f.stdout, f.stderr, row.code, row.stderr)
			}
			if matches := f.snapshots(t, runtime); len(matches) != 0 {
				t.Errorf("%s %s: a refused probe wrote %v", runtime, row.name, matches)
			}
		}
	}
}

// The self-test refuses an unfilled model before any turn, and a probe
// that fails after the identity read aborts it with the runtime named.
func TestBuiltinSelftestRefusals(t *testing.T) {
	t.Parallel()
	for _, runtime := range []string{"claude", "codex"} {
		f := newProbeFixture(t, runtime)
		if err := os.WriteFile(filepath.Join(f.root, "metasystem.conf"), []byte("metasystem.runtimes="+runtime+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if code := registry[runtime].selftest(f.deps()); code != 1 || !strings.Contains(f.stderr.String(), "filled role.default.model."+runtime) {
			t.Errorf("%s selftest without a model: exit %d stderr %q", runtime, code, f.stderr)
		}

		f = newProbeFixture(t, runtime)
		if err := os.WriteFile(filepath.Join(f.root, "metasystem.conf"),
			[]byte("role.default.model."+runtime+"=stub-model\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.stubs, "auth-exit"), []byte("1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		code := registry[runtime].selftest(f.deps())
		if code != 1 || !strings.Contains(f.stderr.String(), runtime+" probe failed") {
			t.Errorf("%s selftest with a failing probe: exit %d stderr %q", runtime, code, f.stderr)
		}
		if f.stdout.Len() != 0 {
			t.Errorf("%s selftest's quiet probe printed %q", runtime, f.stdout)
		}
		if matches := f.snapshots(t, runtime); len(matches) != 0 {
			t.Errorf("%s selftest's failed probe wrote %v", runtime, matches)
		}
	}
}

// The operation interface of a built-in resolves through the registry, and
// its shared operations answer as the built-in's own functions do.
func TestBuiltinOperationsThroughTheRegistry(t *testing.T) {
	t.Parallel()
	f := newProbeFixture(t, "claude")
	d := f.deps()
	ops, err := OperationsAt(d, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if other, ok := OperationsFor(d, "claude"); !ok || other == nil {
		t.Fatal("OperationsFor did not find the claude built-in")
	}
	if _, ok := OperationsFor(d, "no-such-runtime"); ok {
		t.Fatal("OperationsFor found a runtime the installation lacks")
	}
	if _, err := OperationsAt(d, "no-such-runtime"); err == nil {
		t.Fatal("OperationsAt answered for a runtime the installation lacks")
	}

	identity, err := ops.ConfigIdentity(d)
	if err != nil || !strings.Contains(identity, `"cliVersion":"3.4.5"`) {
		t.Fatalf("config identity %q, %v", identity, err)
	}
	contract, err := ops.Contract(d)
	var snapshot map[string]any
	if err == nil {
		err = json.Unmarshal(contract, &snapshot)
	}
	if err != nil || snapshot["runtime"] != "claude" || snapshot["cliVersion"] != "0.0.0-contract" {
		t.Fatalf("contract %s, %v", contract, err)
	}
	if code := ops.Probe(d, nil); code != 0 || len(f.snapshots(t, "claude")) != 1 {
		t.Fatalf("probe exit %d: %s", code, f.stderr)
	}
	if code := ops.Selftest(d); code != 1 || !strings.Contains(f.stderr.String(), "cannot read metasystem configuration") {
		t.Fatalf("self-test without a configuration exit %d stderr %q, want 1 naming the configuration", code, f.stderr)
	}
	if result := ops.Repair(&Turn{d: d}, RepairInput{}); result.Status != 1 {
		t.Fatalf("an undeclared repair answered %+v", result)
	}
	if code := ops.Cancel(d, "job-7"); code != 0 {
		t.Fatalf("cancel exit %d", code)
	}
	calls := f.dispatch.snapshot()
	if len(calls) != 1 || strings.Join(calls[0], " ") != "__cancel-owned --job job-7" {
		t.Fatalf("cancel callbacks %v", calls)
	}

	resolved, err := resolveRuntime(d, "codex")
	if err != nil || resolved.Name() != "codex" {
		t.Fatalf("resolved runtime %v, %v", resolved, err)
	}
}

// builtinOperations is the surface the external contract mirrors: every
// built-in answers the shared verbs, and each offers exactly the optional
// ones its registration provides.
func TestBuiltinOperationsSurface(t *testing.T) {
	t.Parallel()
	surface := builtinOperations()
	names := make([]string, 0, len(surface))
	for name := range surface {
		names = append(names, name)
	}
	sort.Strings(names)
	if strings.Join(names, " ") != strings.Join(Runtimes(), " ") {
		t.Fatalf("surface runtimes %v, supervised %v", names, Runtimes())
	}
	for name, ops := range surface {
		have := map[string]bool{}
		for _, op := range ops {
			if have[op] {
				t.Errorf("%s lists %s twice", name, op)
			}
			have[op] = true
		}
		for _, op := range []string{"signature", "local-config-paths", "wait-delivery", "cancel",
			"identity", "config-identity", "contract", "probe", "output-stream-file", "selftest"} {
			if !have[op] {
				t.Errorf("%s surface lacks %s: %v", name, op, ops)
			}
		}
		if _, declared := runtimes.EnforcementMapJSON(name); have["enforcement-map"] != declared {
			t.Errorf("%s enforcement-map listed=%v, declared=%v", name, have["enforcement-map"], declared)
		}
	}
}

func TestLocalConfigManifestIsTheSortedUnion(t *testing.T) {
	t.Parallel()
	manifest, err := LocalConfigManifest(Deps{})
	if err != nil {
		t.Fatal(err)
	}
	if !sort.StringsAreSorted(manifest) {
		t.Fatalf("manifest is not sorted: %v", manifest)
	}
	listed := map[string]bool{}
	for _, path := range manifest {
		if listed[path] {
			t.Fatalf("manifest repeats %s: %v", path, manifest)
		}
		listed[path] = true
	}
	want := map[string]bool{}
	for _, name := range runtimes.WithAdapter() {
		paths, _ := runtimes.LocalConfigPaths(name)
		for _, path := range paths {
			want[path] = true
		}
	}
	if len(want) == 0 || len(want) != len(listed) {
		t.Fatalf("manifest %v, declared %v", manifest, want)
	}
	for path := range want {
		if !listed[path] {
			t.Fatalf("manifest lacks declared %s", path)
		}
	}
}

func TestIdentityFieldsNamesTheMissingMember(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ doc, missing string }{
		{`{}`, "cliVersion"},
		{`{"cliVersion":"1"}`, "configHash"},
		{`{"cliVersion":"1","configHash":"h"}`, "configKeyHashes"},
	} {
		if _, _, _, err := identityFields(row.doc); err == nil || !strings.Contains(err.Error(), "lacks "+row.missing) {
			t.Errorf("identityFields(%s) = %v, want lacks %s", row.doc, err, row.missing)
		}
	}
	version, hash, keys, err := identityFields(`{"cliVersion":"1.2","configHash":"abc","configKeyHashes":{"k":"v"}}`)
	if err != nil || version != "1.2" || hash != "abc" || keys != `{"k":"v"}` {
		t.Fatalf("identityFields = %q %q %q %v", version, hash, keys, err)
	}
}

// Turn.Handshake records only through a shared layer that records in
// place, only for an observed session, and only once.
func TestTurnHandshakeRecordsOnce(t *testing.T) {
	t.Parallel()
	var recorded []string
	record := func(e Events) bool { recorded = append(recorded, e.Session); return true }
	if (&Turn{}).Handshake(Events{Session: "s1"}) {
		t.Fatal("a turn without an in-place recorder claimed the handshake")
	}
	turn := &Turn{handshake: record}
	if turn.Handshake(Events{}) {
		t.Fatal("a handshake without a session was recorded")
	}
	if !turn.Handshake(Events{Session: "s1"}) {
		t.Fatal("an observed session was not recorded")
	}
	done := &Turn{handshake: record, HandshakeDone: true}
	if done.Handshake(Events{Session: "s2"}) {
		t.Fatal("a second handshake was recorded after the first")
	}
	if strings.Join(recorded, ",") != "s1" {
		t.Fatalf("recorded handshakes %v", recorded)
	}
}

func TestTerminationHookRoutesAndRemoves(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	delivered := make(chan os.Signal, 1)
	remove := HookTermination(root, func(c chan<- os.Signal) { c <- syscall.SIGTERM })
	stop := notifyTermination(root, delivered)
	stop()
	if sig := <-delivered; sig != syscall.SIGTERM || signalExitCode(sig) != 143 {
		t.Fatalf("hooked delivery %v → %d", sig, signalExitCode(sig))
	}
	remove()
	if _, ok := signalHooks.Load(root); ok {
		t.Fatal("the removed hook still routes the root's signals")
	}
	if signalExitCode(os.Interrupt) != 130 {
		t.Fatalf("interrupt status %d, want 130", signalExitCode(os.Interrupt))
	}
}

// exitStatus is the shell's status: the exit code, 128 plus the signal for
// a signalled child, and 127 for a command that never started.
func TestExitStatusMapsLikeTheShell(t *testing.T) {
	t.Parallel()
	if code := exitStatus(nil); code != 0 {
		t.Fatalf("success = %d", code)
	}
	if code := exitStatus(errors.New("never started")); code != 127 {
		t.Fatalf("unstarted = %d, want 127", code)
	}
	if code := waitStatusCode(nil); code != 127 {
		t.Fatalf("no state = %d, want 127", code)
	}
	if code := exitStatus(exec.Command("/bin/sh", "-c", "exit 7").Run()); code != 7 {
		t.Fatalf("exit 7 = %d", code)
	}
	if code := exitStatus(exec.Command("/bin/sh", "-c", "kill -KILL $$").Run()); code != 128+int(syscall.SIGKILL) {
		t.Fatalf("killed child = %d, want %d", code, 128+int(syscall.SIGKILL))
	}
}

func TestDepsSelfFallsBackToTheEngine(t *testing.T) {
	t.Parallel()
	if got := (Deps{Engine: "/engine"}).self(); got != "/engine" {
		t.Fatalf("self without Self = %q", got)
	}
	if got := (Deps{Engine: "/engine", Self: "/self"}).self(); got != "/self" {
		t.Fatalf("self = %q", got)
	}
}

// The entry's argument contract for the small verbs: each verb answers its
// own output, a malformed invocation is the usage text with status 2, and a
// failing runtime fact is status 1 with the cause.
func TestMainSmallVerbArgumentContract(t *testing.T) {
	t.Parallel()
	f := newProbeFixture(t, "claude")
	run := func(args ...string) (int, string, string) {
		f.stdout.Reset()
		f.stderr.Reset()
		code := Main(args, func(string) Deps { return f.deps() })
		return code, f.stdout.String(), f.stderr.String()
	}
	root := f.root

	if code, out, _ := run("claude", "identity", "--root", root); code != 0 || !strings.HasPrefix(out, "3.4.5 ") || len(strings.Fields(out)) != 2 {
		t.Fatalf("identity = %d %q", code, out)
	}
	if code, out, _ := run("claude", "config-identity", "--root", root); code != 0 || !strings.Contains(out, `"cliVersion":"3.4.5"`) {
		t.Fatalf("config-identity = %d %q", code, out)
	}
	if code, out, _ := run("claude", "contract", "--root", root); code != 0 || !strings.Contains(out, `"runtime": "claude"`) {
		t.Fatalf("contract = %d %q", code, out)
	}
	if code, out, _ := run("claude", "enforcement-map", "--root", root); code != 0 || !strings.Contains(out, "writeRoots") {
		t.Fatalf("enforcement-map = %d %q", code, out)
	}
	if code, out, _ := run("claude", "output-stream", "--root", root, "--round-dir", "/rounds/1/"); code != 0 || out != "/rounds/1/claude-stream.jsonl\n" {
		t.Fatalf("output-stream = %d %q", code, out)
	}
	if code, out, errText := run("claude", "--help"); code != 0 || !strings.Contains(out+errText, "Usage:") {
		t.Fatalf("--help = %d %q %q", code, out, errText)
	}
	if code, _, errText := run("claude"); code != 2 || !strings.Contains(errText, "usage: metasystem") {
		t.Fatalf("no verb = %d %q", code, errText)
	}
	if code, _, errText := run("no-such-runtime", "signature", "--root", root); code != 2 || !strings.Contains(errText, "not installed") {
		t.Fatalf("unknown runtime = %d %q", code, errText)
	}

	for _, args := range [][]string{
		{"claude", "signature", "--root", "relative/root"},
		{"claude", "no-such-verb", "--root", root},
		{"claude", "identity", "--root", root, "extra"},
		{"claude", "config-identity", "--root", root, "extra"},
		{"claude", "contract", "--root", root, "extra"},
		{"claude", "signature", "--root", root, "extra"},
		{"claude", "local-config-paths", "--root", root, "extra"},
		{"claude", "enforcement-map", "--root", root, "extra"},
		{"claude", "output-stream", "--root", root, "--round-dir", "relative"},
		{"claude", "cancel", "--root", root, "--job"},
		{"claude", "selftest", "--root", root, "extra"},
		{"claude", "wait-delivery", "--root", root, "--wait-id"},
		{"claude", "wait-delivery", "--root", root, "--unknown", "x"},
		{"fake", "enforcement-map", "--root", root},
	} {
		if code, _, _ := run(args...); code != 2 {
			t.Errorf("%v = %d, want the usage status 2", args, code)
		}
	}

	if err := os.Remove(filepath.Join(f.stubs, "claude")); err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"identity", "config-identity"} {
		if code, _, errText := run("claude", verb, "--root", root); code != 1 || !strings.Contains(errText, "claude CLI is not installed") {
			t.Errorf("%s without the CLI = %d %q", verb, code, errText)
		}
	}
}

// The fifo pair: fresh names per attempt, both ends made or neither left,
// and removal leaves nothing behind.
func TestACPPipesLifecycle(t *testing.T) {
	t.Parallel()
	if _, err := TokenHex(0); err == nil {
		t.Fatal("a zero-byte token was accepted")
	}
	token, err := TokenHex(4)
	if err != nil || len(token) != 8 || strings.Trim(token, "0123456789abcdef") != "" {
		t.Fatalf("token %q, %v", token, err)
	}
	dir := t.TempDir()
	first, err := ACPPipeNames(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ACPPipeNames(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first.ServerOut == second.ServerOut || filepath.Dir(first.ServerIn) != dir ||
		!strings.HasSuffix(first.ServerOut, "-out") || !strings.HasSuffix(first.ServerIn, "-in") {
		t.Fatalf("pipe names %+v %+v", first, second)
	}
	if err := first.Make(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{first.ServerOut, first.ServerIn} {
		if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeNamedPipe == 0 {
			t.Fatalf("%s is not a fifo: %v", path, err)
		}
	}
	first.Remove()
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("removal left %v", entries)
	}

	half := &ACPPipes{ServerOut: filepath.Join(dir, "acp-half-out"), ServerIn: filepath.Join(dir, "missing", "acp-half-in")}
	if err := half.Make(); err == nil || !strings.Contains(err.Error(), "mkfifo "+half.ServerIn) {
		t.Fatalf("half-made pair = %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("a half-made pair left %v", entries)
	}
	none := &ACPPipes{ServerOut: filepath.Join(dir, "missing", "out"), ServerIn: filepath.Join(dir, "missing", "in")}
	if err := none.Make(); err == nil || !strings.Contains(err.Error(), "mkfifo "+none.ServerOut) {
		t.Fatalf("unmakeable pair = %v", err)
	}
	var absent *ACPPipes
	absent.Remove() // a nil pair has nothing to remove
}

// builtinOperations reports, per built-in runtime, the operations its Go
// implementation provides — the surface the external contract mirrors.
func builtinOperations() map[string][]string {
	out := map[string][]string{}
	for name, a := range registry {
		ops := []string{"signature", "local-config-paths", "wait-delivery", "cancel"}
		if a.configIdentity != nil {
			ops = append(ops, "identity", "config-identity")
		}
		if _, ok := runtimes.EnforcementMapJSON(name); ok {
			ops = append(ops, "enforcement-map")
		}
		if a.contract != nil {
			ops = append(ops, "contract")
		}
		if a.probe != nil {
			ops = append(ops, "probe")
		}
		if a.outputStream != nil {
			ops = append(ops, "output-stream-file")
		}
		if a.supervise != nil {
			ops = append(ops, "supervise")
		}
		if a.selftest != nil {
			ops = append(ops, "selftest")
		}
		out[name] = ops
	}
	return out
}
