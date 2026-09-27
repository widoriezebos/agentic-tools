package supervisor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes/external"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The external-runtime witnesses (design verbs-object-action 3.5, R13): an
// adapter executable installed into a fixture installation after this engine
// was built — a wrapper around this test binary, whose
// runExternalAdapterFixture speaks the operations — runs through the
// same shared round the built-ins use, with no Go registration.

// installExternalAdapter writes <root>/adapters/<name> as a wrapper that runs
// this test binary as the adapter in the given fixture mode, names it in the
// configuration when named, and returns the file the adapter logs each
// operation to.
func installExternalAdapter(t *testing.T, root, name, mode string, fileMode os.FileMode, named bool) (path, log string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(root, "adapters", name)
	log = filepath.Join(root, "adapter-"+name+".log")
	wrapper := fmt.Sprintf("#!/bin/sh\nEXTERNAL_FIXTURE_MODE='%s' EXTERNAL_FIXTURE_LOG='%s' exec '%s' \"$@\"\n",
		mode, log, self)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, fileMode); err != nil {
		t.Fatal(err)
	}
	if named {
		conf := filepath.Join(root, "metasystem.conf")
		existing, _ := os.ReadFile(conf)
		mustWrite(t, conf, string(existing)+"adapters."+name+".use=external\n")
	}
	return path, log
}

// operationsLogged reads the adapter's log: one "operation answer" line per
// run, answer being "ok" or "64".
func operationsLogged(t *testing.T, log string) []string {
	t.Helper()
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}

// externalRequest is the part of an operation request the fixture reads.
type externalRequest struct {
	Operation  string `json:"operation"`
	Stage      string `json:"stage"`
	Probe      string `json:"probe"`
	Scratch    string `json:"scratch"`
	Nonce      string `json:"nonce"`
	ReturnPath string `json:"returnPath"`
	Root       string `json:"root"`
	Job        string `json:"job"`
	Usage      string `json:"usage"`
	Turn       struct {
		Role, Verb, Dir, Record, Prompt, Tag, Round, RootJob, Model, ResumeSession, Requested, Effective, Workspace string
	} `json:"turn"`
	Private struct {
		Stdout  string `json:"stdout"`
		Session string `json:"session"`
	} `json:"private"`
}

// runExternalAdapterFixture is the adapter executable's body, entered from
// TestMain before the test environment starts (an adapter run is one short
// process that forks nothing): the operation is the last argument, the
// request is stdin.
func runExternalAdapterFixture(mode string) int {
	operation := os.Args[len(os.Args)-1]
	var request externalRequest
	input, _ := io.ReadAll(os.Stdin)
	if err := json.Unmarshal(input, &request); err != nil || request.Operation != operation {
		fmt.Fprintf(os.Stderr, "bad request for %q: %v\n", operation, err)
		return 90
	}
	answer, code := externalFixture(mode, operation, request)
	logLine := "ok"
	if code == 64 {
		logLine = "64"
	}
	if file, err := os.OpenFile(os.Getenv("EXTERNAL_FIXTURE_LOG"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644); err == nil {
		fmt.Fprintf(file, "%s=%s\n", operation, logLine)
		file.Close()
	}
	if answer != "" {
		fmt.Print(answer)
	}
	return code
}

func jsonText(value any) string {
	data, _ := json.Marshal(value)
	return string(data) + "\n"
}

// externalFixture answers one operation: mode newagent is a complete new
// runtime (a fake-style agent whose CLI is a shell printing its session);
// mode fake-finalize overrides only the fake built-in's finalize.
func externalFixture(mode, operation string, request externalRequest) (string, int) {
	switch mode {
	case "fake-finalize":
		if operation != "finalize" {
			return "", 64
		}
		// The override's one changed behavior: its own usage accounting.
		usage := map[string]any{"availability": "native", "inputTokens": 77, "cachedInputTokens": 0, "outputTokens": 7,
			"reasoningTokens": nil, "cost": nil, "providerUnits": map[string]any{"name": "override-unit", "value": 1}}
		data, _ := json.Marshal(usage)
		if err := os.WriteFile(request.Usage, data, 0o644); err != nil {
			return err.Error(), 1
		}
		return jsonText(map[string]any{"schemaVersion": 1, "candidate": filepath.Join(request.Turn.Dir, "return.json")}), 0
	case "devin-identity":
		// An override of the built-in Devin that changes only its
		// configuration identity (say, a wrapper that pins extra config).
		if operation != "probe" {
			return "", 64
		}
		return jsonText(map[string]any{"schemaVersion": 1, "installed": true, "version": "3000.4.25",
			"configIdentity": map[string]any{"cliVersion": "3000.4.25", "configHash": "devin-override-config", "configKeyHashes": map[string]any{}, "runtime": "devin"}}), 0
	case "newagent":
	default:
		return "unknown mode", 1
	}
	switch operation {
	case "describe":
		return jsonText(map[string]any{
			"schemaVersion": 1, "name": "newagent",
			"capabilities": map[string]any{"resume": true, "followUp": true, "waitDelivery": true, "host": true, "usage": "native"},
			"match":        []string{`^([^[:space:]]*/)?newagent([[:space:]]|$)`},
			"positive":     "/opt/newagent/bin/newagent -p task", "lookalike": "newagent-helper serve",
			"invocations": []map[string]any{{"includes": []string{"newagent", "-p"}, "tagFlag": "--tag"}},
			"configPaths": []string{".newagent/config.json"},
			"enforcement": map[string]string{"writeRoots": "mapped", "readRoots": "mapped", "network": "mapped"},
		}), 0
	case "probe":
		return jsonText(map[string]any{"schemaVersion": 1, "installed": true, "version": "1.0.0",
			"configIdentity": map[string]any{"cliVersion": "1.0.0", "configHash": "newagent-config-v1", "configKeyHashes": map[string]any{}, "runtime": "newagent"}}), 0
	case "prepare":
		turn := request.Turn
		session := "newagent-session-" + turn.RootJob
		if turn.ResumeSession != "" {
			session = turn.ResumeSession
		}
		stdout := filepath.Join(turn.Dir, "newagent.out")
		answer := map[string]any{"schemaVersion": 1,
			// The CLI prints its session, then waits until observe has
			// reported it, so the handshake runs through observe on any
			// schedule.
			"argv": []string{"/bin/sh", "-c", `printf '{"type":"session","id":"%s"}\n' "$1"; while [ ! -e "$2" ]; do sleep 0.01; done`,
				"newagent", session, filepath.Join(turn.Dir, "observed"), "-p", "--tag", turn.Tag},
			"argv0": "newagent", "stdin": turn.Prompt, "stdout": stdout,
			"private": map[string]any{"stdout": stdout, "session": session}}
		if prompt, _ := os.ReadFile(turn.Prompt); strings.Contains(string(prompt), "EXT:effective-wider") {
			answer["effective"] = map[string]any{"readRoots": []string{}, "writeRoots": []string{"/"}, "network": "allow"}
		}
		return jsonText(answer), 0
	case "observe":
		data, _ := os.ReadFile(request.Private.Stdout)
		if !strings.Contains(string(data), request.Private.Session) {
			return "", 0
		}
		if err := os.WriteFile(filepath.Join(request.Turn.Dir, "observed"), nil, 0o644); err != nil {
			return err.Error(), 1
		}
		return jsonText(map[string]any{"event": "handshake", "session": request.Private.Session,
			"turn": "newagent-turn-" + request.Turn.Round, "model": request.Turn.Model}), 0
	case "finalize":
		turn := request.Turn
		raw := filepath.Join(turn.Dir, "raw.out")
		if err := adapter.WriteFakeReturn(turn.Record, turn.Prompt, raw); err != nil {
			return err.Error(), 1
		}
		var value map[string]any
		data, _ := os.ReadFile(raw)
		if json.Unmarshal(data, &value) != nil {
			return "bad return", 1
		}
		value["runtime"] = "newagent"
		// A fake-style agent does what a self-test brief asks: it reads
		// permitted.txt and quotes the line in its evidence.
		if permitted, err := os.ReadFile(filepath.Join(turn.Workspace, "permitted.txt")); err == nil {
			if brief, _ := os.ReadFile(turn.Prompt); strings.Contains(string(brief), "PERMITTED_READ:") {
				value["evidence"] = append(value["evidence"].([]any), map[string]any{
					"command": "read permitted.txt", "observed": strings.TrimSpace(string(permitted)), "level": "ran"})
			}
		}
		data, _ = json.Marshal(value)
		if err := os.WriteFile(raw, data, 0o644); err != nil {
			return err.Error(), 1
		}
		if err := adapter.WriteFakeUsage(request.Usage); err != nil {
			return err.Error(), 1
		}
		return jsonText(map[string]any{"schemaVersion": 1, "candidate": raw, "session": request.Private.Session,
			"turn": "newagent-turn-" + turn.Round, "handshakeModel": turn.Model}), 0
	case "cancel":
		return "", 0
	case "selftest":
		// The custom probe's stages: a scratch fixture, the prompt text,
		// and the evidence check against the nonce.
		switch request.Stage {
		case "prepare-scratch":
			if err := os.WriteFile(filepath.Join(request.Scratch, "probe-"+request.Probe), []byte(request.Nonce), 0o644); err != nil {
				return err.Error(), 1
			}
			return "", 0
		case "prompt-text":
			return jsonText(map[string]any{"schemaVersion": 1, "text": "echo " + request.Nonce}), 0
		case "verify-evidence":
			data, _ := os.ReadFile(request.ReturnPath)
			if !strings.Contains(string(data), request.Nonce) {
				return "", 1
			}
			return "", 0
		}
	}
	return "", 64
}

// externalInstall is a fake-style installation whose job runs on the
// external runtime newagent.
func externalInstall(t *testing.T, opts installOptions) (*fakeInstall, string) {
	t.Helper()
	f := newFakeInstall(t, opts)
	editRecord(filepath.Join(f.agents(), "jobs", f.job+".json"), func(record map[string]any) { record["runtime"] = "newagent" })
	_, log := installExternalAdapter(t, f.root, "newagent", "newagent", 0o755, true)
	return f, log
}

func (f *fakeInstall) superviseAs(runtime string) []string {
	args := f.superviseArgs()
	args[0] = runtime
	return args
}

// TestExternalRuntimeDispatchesEndToEnd: a runtime the engine does not ship,
// installed as an executable and named in the configuration, runs a whole
// delegate round through the shared layer: prepare's argv launched under
// custody, the handshake from observe, finalize's candidate adjudicated to a
// completed record.
func TestExternalRuntimeDispatchesEndToEnd(t *testing.T) {
	t.Parallel()
	f, log := externalInstall(t, installOptions{})
	if code := f.run(f.superviseAs("newagent")...); code != 0 {
		t.Fatalf("exit %d, stderr %s, log %s", code, f.stderr.String(), f.logText())
	}
	calls := f.dispatcher.calls(t)
	names := callNames(calls)
	if len(calls) < 2 || calls[0][0] != "__register-custody" && calls[0][0] != "__handshake" {
		t.Fatalf("calls = %q", calls)
	}
	var handshake []string
	for _, call := range calls {
		if call[0] == "__handshake" {
			handshake = call
		}
	}
	if flagValue(handshake, "--session") != "newagent-session-fake-job-1" || flagValue(handshake, "--turn") != "newagent-turn-1" {
		t.Fatalf("handshake = %q (calls %q)", handshake, names)
	}
	patch := assertCAS(t, casCall(calls), f.job, "running", "completed")
	assertPatch(t, patch, nil, "completed", true)
	if ret := readJSON(t, f.roundFile("return.json")); ret["runtime"] != "newagent" || ret["sessionId"] != "newagent-session-fake-job-1" {
		t.Fatalf("return = %v", ret)
	}
	if got := readText(t, f.roundFile("newagent.out")); !strings.Contains(got, "newagent-session-fake-job-1") {
		t.Fatalf("the CLI's stdout = %q", got)
	}
	logged := strings.Join(operationsLogged(t, log), " ")
	for _, want := range []string{"describe=ok", "prepare=ok", "observe=ok", "finalize=ok"} {
		if !strings.Contains(logged, want) {
			t.Fatalf("adapter operations = %s, missing %s", logged, want)
		}
	}
}

// TestExternalRuntimeResumesAndCancels: a follow-up resumes the recorded
// session through the external prepare, and cancel runs the adapter's own
// cancellation before the shared one.
func TestExternalRuntimeResumesAndCancels(t *testing.T) {
	t.Parallel()
	f, log := externalInstall(t, installOptions{verb: "follow-up", sessionID: "newagent-session-fake-job-1"})
	if code := f.run(f.superviseAs("newagent")...); code != 0 {
		t.Fatalf("follow-up exit %d, stderr %s", code, f.stderr.String())
	}
	for _, call := range f.dispatcher.calls(t) {
		if call[0] == "__handshake" && flagValue(call, "--session") != "newagent-session-fake-job-1" {
			t.Fatalf("the follow-up did not resume the session: %q", call)
		}
	}
	if code := f.run("newagent", "cancel", "--root", f.root, "--job", f.job); code != 0 {
		t.Fatalf("cancel exit %d, stderr %s", code, f.stderr.String())
	}
	calls := f.dispatcher.calls(t)
	if last := calls[len(calls)-1]; !reflect.DeepEqual(last, []string{"__cancel-owned", "--job", f.job}) {
		t.Fatalf("last call = %q, want the shared cancellation", last)
	}
	if logged := strings.Join(operationsLogged(t, log), " "); !strings.Contains(logged, "cancel=ok") {
		t.Fatalf("adapter operations = %s, want its cancel", logged)
	}
}

// TestExternalRuntimeWiderEnvelopeRefused: the runtime reports its own
// effective envelope (VOA-26), and the shared comparison refuses a wider
// grant before anything launches.
func TestExternalRuntimeWiderEnvelopeRefused(t *testing.T) {
	t.Parallel()
	f, _ := externalInstall(t, installOptions{prompt: "Working Mode: implement\n\nEXT:effective-wider\n"})
	if code := f.run(f.superviseAs("newagent")...); code != 1 {
		t.Fatalf("exit %d, want 1; stderr %s", code, f.stderr.String())
	}
	calls := f.dispatcher.calls(t)
	patch := assertCAS(t, casCall(calls), f.job, "pending", "failed")
	if got := readJSON(t, patch)["error"].(string); !strings.HasPrefix(got, "permissions_mismatch:") {
		t.Fatalf("error = %q", got)
	}
	if exists(f.roundFile("newagent.out")) {
		t.Fatal("the CLI ran although its grant was wider than the request")
	}
}

// TestPartialOverrideFallsBackPerExit64: an override of the fake built-in
// that implements only finalize changes that one behavior; every other
// operation exits 64 and runs as the built-in, asked once per process.
func TestPartialOverrideFallsBackPerExit64(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{})
	_, log := installExternalAdapter(t, f.root, "fake", "fake-finalize", 0o755, true)
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d, stderr %s", code, f.stderr.String())
	}
	patch := assertCAS(t, casCall(f.dispatcher.calls(t)), f.job, "running", "completed")
	assertPatch(t, patch, nil, "completed", true)
	if usage := readJSON(t, f.roundFile("usage.json")); usage["inputTokens"] != float64(77) {
		t.Fatalf("usage = %v, want the override's finalize", usage)
	}
	if got := f.logText(); !strings.Contains(got, "fake supervisor started") {
		t.Fatalf("the built-in's prepare did not run: %q", got)
	}
	logged := operationsLogged(t, log)
	counts := map[string]int{}
	for _, entry := range logged {
		counts[entry]++
	}
	if counts["describe=64"] != 1 || counts["prepare=64"] != 1 || counts["observe=64"] != 1 || counts["finalize=ok"] != 1 {
		t.Fatalf("adapter operations = %v", logged)
	}
}

// TestUntrustedAdapterRefusedWithTheFix: a group-writable adapter is never
// executed; the refusal names the file and the command that fixes it. An
// unnamed one names the setting.
func TestUntrustedAdapterRefusedWithTheFix(t *testing.T) {
	t.Parallel()
	f := newFakeInstall(t, installOptions{})
	path, log := installExternalAdapter(t, f.root, "newagent", "newagent", 0o775, true)
	if code := f.run(f.superviseAs("newagent")...); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if got := f.stderr.String(); !strings.Contains(got, "chmod go-w "+path) || !strings.Contains(got, "group- or world-writable") {
		t.Fatalf("stderr = %q, want the refusal and its fix", got)
	}
	if logged := operationsLogged(t, log); len(logged) != 0 {
		t.Fatalf("the untrusted adapter was executed: %v", logged)
	}

	// A named but unsafe override of a built-in refuses running it, with
	// the fix, instead of the built-in running silently (read F3).
	o := newFakeInstall(t, installOptions{})
	overridePath, overrideLog := installExternalAdapter(t, o.root, "fake", "fake-finalize", 0o775, true)
	if code := o.run(); code != 2 {
		t.Fatalf("unsafe override exit %d, want 2", code)
	}
	if got := o.stderr.String(); !strings.Contains(got, "chmod go-w "+overridePath) {
		t.Fatalf("stderr = %q, want the fix", got)
	}
	if logged := operationsLogged(t, overrideLog); len(logged) != 0 || exists(o.roundFile("raw.out")) {
		t.Fatalf("the refused override ran or the built-in ran silently: %v", logged)
	}

	g := newFakeInstall(t, installOptions{})
	_, unnamedLog := installExternalAdapter(t, g.root, "newagent", "newagent", 0o755, false)
	if code := g.run(g.superviseAs("newagent")...); code != 2 {
		t.Fatalf("unnamed exit %d, want 2", code)
	}
	if got := g.stderr.String(); !strings.Contains(got, "adapters.newagent.use=external") {
		t.Fatalf("stderr = %q, want the setting that names it", got)
	}
	if logged := operationsLogged(t, unnamedLog); len(logged) != 0 {
		t.Fatalf("the unnamed adapter was executed: %v", logged)
	}
}

// TestExternalRuntimeSmallVerbs: the verbs dispatch.sh asks before a launch
// answer from the executable: signature (the registry's effective one),
// config-identity and probe (a snapshot under the runtime's name).
func TestExternalRuntimeSmallVerbs(t *testing.T) {
	t.Parallel()
	f, _ := externalInstall(t, installOptions{})
	if code := f.run("newagent", "signature", "--root", f.root); code != 0 || !strings.Contains(f.stdout.String(), "match ^([^[:space:]]*/)?newagent") {
		t.Fatalf("signature exit %d: %q %s", code, f.stdout.String(), f.stderr.String())
	}
	f.stdout = &syncBuffer{}
	if code := f.run("newagent", "config-identity", "--root", f.root); code != 0 || !strings.Contains(f.stdout.String(), "newagent-config-v1") {
		t.Fatalf("config-identity exit %d: %q %s", code, f.stdout.String(), f.stderr.String())
	}
	f.stdout = &syncBuffer{}
	if code := f.run("newagent", "probe", "--root", f.root); code != 0 {
		t.Fatalf("probe exit %d: %s", code, f.stderr.String())
	}
	snapshot := strings.TrimSpace(f.stdout.String())
	if !strings.HasPrefix(filepath.Base(snapshot), "newagent-1.0.0-newagent-config-v1-") {
		t.Fatalf("snapshot = %q", snapshot)
	}
	if value := readJSON(t, snapshot); value["runtime"] != "newagent" {
		t.Fatalf("snapshot = %v", value)
	}
}

// TestExternalSelftestProbeStages: an external runtime's custom self-test
// probe needs no Go registration; its stages run through the selftest
// operation.
func TestExternalSelftestProbeStages(t *testing.T) {
	t.Parallel()
	f, _ := externalInstall(t, installOptions{})
	entry, err := registryEntry(f.deps(), "newagent")
	if err != nil {
		t.Fatal(err)
	}
	probe := newExternalOps(entry).selftestProbe(f.deps(), external.SelftestProbe{Name: "tools", BehaviorLabels: []string{"tool-use"}})
	scratch := t.TempDir()
	if err := probe.PrepareScratch(scratch, "nonce-1"); err != nil || readText(t, filepath.Join(scratch, "probe-tools")) != "nonce-1" {
		t.Fatalf("prepare-scratch = %v", err)
	}
	if got := probe.PromptText("nonce-1"); got != "echo nonce-1" {
		t.Fatalf("prompt-text = %q", got)
	}
	evidence := filepath.Join(scratch, "return.json")
	mustWrite(t, evidence, `{"evidence":"nonce-1"}`)
	if err := probe.VerifyEvidence(evidence, "nonce-1"); err != nil {
		t.Fatalf("verify-evidence refused its evidence: %v", err)
	}
	if err := probe.VerifyEvidence(evidence, "nonce-2"); err == nil {
		t.Fatal("verify-evidence accepted evidence without the nonce")
	}
}

// TestDevinPartialOverrideFallsBackToTheBuiltIn: Devin stays a first-class
// built-in under an override. An override that answers only probe changes
// the configuration identity; describe, prepare, observe and finalize exit
// 64, so a whole legacy-transport round runs through the built-in Devin,
// and the signature keeps its reserved `devin acp` exclusion.
func TestDevinPartialOverrideFallsBackToTheBuiltIn(t *testing.T) {
	t.Parallel()
	f := newDevinFixture(t, "", "dispatch")
	f.legacyHappyStubs()
	_, log := installExternalAdapter(t, f.root, "devin", "devin-identity", 0o755, true)
	if code := f.run("dispatch"); code != 0 {
		t.Fatalf("exit %d\ncalls %v\nstderr %s\nlog %s", code, f.dispatch.verbs(), f.stderr, f.jobLog())
	}
	f.expectTerminal("completed", "")
	if !strings.Contains(f.jobLog(), "devin cli exit status=0") {
		t.Fatalf("the built-in Devin did not run the turn: %s", f.jobLog())
	}
	counts := map[string]int{}
	for _, entry := range operationsLogged(t, log) {
		counts[entry]++
	}
	if counts["describe=64"] != 1 || counts["prepare=64"] != 1 || counts["finalize=64"] != 1 || counts["probe=ok"] != 0 {
		t.Fatalf("adapter operations = %v", counts)
	}
	// Delivery repair stays available: the override left describe to the
	// built-in, whose capabilities (repair included) stand (read F2).
	r := newDevinFixture(t, "", "dispatch")
	r.legacyHappyStubs()
	os.Remove(filepath.Join(r.stubDir, "stdout"))
	r.stub("repair-transcript", r.transcript("sess-1"))
	r.stub("repair-named", r.validReturn)
	r.stub("repair-named-path", filepath.Join(r.roundDir, "devin-return.repair-1.json"))
	installExternalAdapter(t, r.root, "devin", "devin-identity", 0o755, true)
	if code := r.run("dispatch"); code != 0 {
		t.Fatalf("repair round exit %d\ncalls %v\nstderr %s\nlog %s", code, r.dispatch.verbs(), r.stderr, r.jobLog())
	}
	r.expectTerminal("completed", "")
	if !strings.Contains(r.jobLog(), "delivery repair attempt 1: no return was delivered, asking session sess-1 to write") {
		t.Fatalf("the overridden Devin lost its delivery repair: %s", r.jobLog())
	}
	f.stdout.Reset()
	if code := Main([]string{"devin", "config-identity", "--root", f.root}, func(string) Deps { return f.deps() }); code != 0 || !strings.Contains(f.stdout.String(), "devin-override-config") {
		t.Fatalf("config-identity exit %d: %q %s", code, f.stdout.String(), f.stderr)
	}
	f.stdout.Reset()
	if code := Main([]string{"devin", "signature", "--root", f.root}, func(string) Deps { return f.deps() }); code != 0 ||
		!strings.Contains(f.stdout.String(), "exclude ^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)") {
		t.Fatalf("the override's signature lost the reserved devin acp exclusion: %q", f.stdout.String())
	}
}

// TestRelativeRootKeepsExternalsVisible: a relative --root is refused with
// the usage (2), not as an uninstalled external runtime (read F9).
func TestRelativeRootKeepsExternalsVisible(t *testing.T) {
	t.Parallel()
	f, _ := externalInstall(t, installOptions{})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(wd, f.root)
	if err != nil {
		t.Fatal(err)
	}
	if code := f.run("newagent", "signature", "--root", relative); code != 2 {
		t.Fatalf("exit %d, want the usage's 2", code)
	}
	if got := f.stderr.String(); strings.Contains(got, "not installed") || !strings.Contains(got, "Usage:") {
		t.Fatalf("stderr = %q, want the usage", got)
	}
}
