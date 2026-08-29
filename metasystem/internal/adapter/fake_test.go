package adapter

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- canned usage ---

func TestWriteFakeUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	if err := WriteFakeUsage(path); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, path)
	if got["availability"] != "native" || got["inputTokens"] != float64(11) ||
		got["cachedInputTokens"] != float64(2) || got["outputTokens"] != float64(7) {
		t.Fatalf("unexpected fake usage %v", got)
	}
	if got["reasoningTokens"] != nil || got["cost"] != nil {
		t.Fatalf("reasoningTokens and cost must be explicit nulls: %v", got)
	}
	units, ok := got["providerUnits"].(map[string]any)
	if !ok || units["name"] != "fake-unit" || units["value"] != float64(1) {
		t.Fatalf("unexpected provider units %v", got["providerUnits"])
	}
}

// --- canned role returns ---

func fakeReturnFixture(t *testing.T, role, extraRecordFields, prompt string) map[string]any {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	promptPath := filepath.Join(dir, "prompt.md")
	output := filepath.Join(dir, "return.json")
	writeFile(t, record, `{
	  "jobId": "job-1", "round": 2, "role": "`+role+`",
	  "sessionId": "fake-session-job-1", "requestedModel": "fake-model",
	  "effectiveModel": null`+extraRecordFields+`
	}`)
	writeFile(t, promptPath, prompt)
	if err := WriteFakeReturn(record, promptPath, output); err != nil {
		t.Fatal(err)
	}
	return readJSONFile(t, output)
}

func TestWriteFakeReturnCommonShape(t *testing.T) {
	got := fakeReturnFixture(t, "verifier", "", "Job-Id: job-1\nWorking Mode: verify\n")
	if got["schemaVersion"] != float64(2) || got["jobId"] != "job-1" || got["round"] != float64(2) {
		t.Fatalf("identity fields wrong: %v", got)
	}
	if got["runtime"] != "fake" || got["sessionId"] != "fake-session-job-1" {
		t.Fatalf("session identity wrong: %v", got)
	}
	model, ok := got["model"].(map[string]any)
	if !ok || model["requested"] != "fake-model" || model["effective"] != "unobserved" {
		t.Fatalf("a null effective model must read as unobserved: %v", got["model"])
	}
	if got["mode"] != "verify" {
		t.Fatalf("mode was not read from the prompt: %v", got["mode"])
	}
	claimed, ok := got["claimed"].(map[string]any)
	if !ok || claimed["sessionId"] != nil || claimed["model"] != nil {
		t.Fatalf("claimed must be present with both members null: %v", got["claimed"])
	}
	if _, present := claimed["sessionId"]; !present {
		t.Fatal("claimed.sessionId must be an explicit null")
	}
	if got["whatWasDone"] != "simulated verification" || got["riskiestPart"] != "fake boundary" {
		t.Fatalf("verifier fields wrong: %v", got)
	}
}

func TestWriteFakeReturnDefaultsModeWithoutHeader(t *testing.T) {
	got := fakeReturnFixture(t, "implementer", "", "no mode header here\n")
	if got["mode"] != "implement" {
		t.Fatalf("a prompt without a Working Mode header must default to implement: %v", got["mode"])
	}
	boundary, ok := got["diffBoundary"].([]any)
	if !ok || len(boundary) != 0 {
		t.Fatalf("implementer diffBoundary must be an empty array: %v", got["diffBoundary"])
	}
}

func TestWriteFakeReturnPerRole(t *testing.T) {
	code := fakeReturnFixture(t, "code-critic", "", "Working Mode: implement\n")
	if code["schemaVersion"] != float64(3) || code["reviewedTree"] != strings.Repeat("0", 40) || code["verdictMaterialCount"] != float64(0) {
		t.Fatalf("code-critic fields wrong: %v", code)
	}
	if rigor, ok := code["rigor"].([]any); !ok || len(rigor) != 0 {
		t.Fatalf("fake critic must return empty version-three rigor: %v", code["rigor"])
	}
	warden := fakeReturnFixture(t, "warden", "", "Working Mode: implement\n")
	if warden["schemaVersion"] != float64(3) || warden["reviewedTree"] != strings.Repeat("0", 40) {
		t.Fatalf("warden fields wrong: %v", warden)
	}

	investigator := fakeReturnFixture(t, "investigator", "", "Working Mode: investigate\n")
	theories, ok := investigator["theories"].([]any)
	if !ok || len(theories) != 1 {
		t.Fatalf("investigator theories wrong: %v", investigator["theories"])
	}
	stopLoss, ok := investigator["stopLoss"].(map[string]any)
	if !ok || stopLoss["triggered"] != false || stopLoss["trigger"] != nil {
		t.Fatalf("investigator stop-loss wrong: %v", investigator["stopLoss"])
	}
}

func TestWriteFakeReturnDesignCriticReviewsHead(t *testing.T) {
	workspace := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", workspace}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	git("init", "-q")
	writeFile(t, filepath.Join(workspace, "file.txt"), "content\n")
	git("add", ".")
	git("-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-qm", "base")

	got := fakeReturnFixture(t, "design-critic",
		`, "workspaceRoot": "`+workspace+`"`, "Working Mode: design\n")
	commit, ok := got["reviewedCommit"].(string)
	if !ok || len(commit) != 40 {
		t.Fatalf("reviewedCommit is not a commit hash: %v", got["reviewedCommit"])
	}
}

func TestWriteFakeReturnRefusesUnknownRole(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeFile(t, record, `{"jobId": "j", "round": 1, "role": "orchestrator",
	  "sessionId": null, "requestedModel": "m", "effectiveModel": null}`)
	writeFile(t, prompt, "Working Mode: design\n")
	err := WriteFakeReturn(record, prompt, filepath.Join(dir, "return.json"))
	if err == nil || !strings.Contains(err.Error(), "unsupported fake role") {
		t.Fatalf("expected an unsupported-role refusal, got %v", err)
	}
}

// --- effective-permissions edit ---

func TestSetEffectiveNetwork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "effective.json")
	writeFile(t, path, `{"readRoots": [], "writeRoots": ["/w"], "network": "deny"}`)
	if err := SetEffectiveNetwork(path, "allow"); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, path)
	if got["network"] != "allow" {
		t.Fatalf("network was not rewritten: %v", got)
	}
	roots, ok := got["writeRoots"].([]any)
	if !ok || len(roots) != 1 || roots[0] != "/w" {
		t.Fatalf("other fields must survive the edit: %v", got)
	}
}

// --- guarded write and network mechanism ---

func TestFakeGuardedWriteDeniedAndAllowed(t *testing.T) {
	dir := t.TempDir()
	permissions := filepath.Join(dir, "permissions.json")
	target := filepath.Join(dir, "probe.txt")

	writeFile(t, permissions, `{"readRoots": [], "writeRoots": [], "network": "deny"}`)
	allowed, err := FakeGuardedWrite(permissions, target)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("a deny-all envelope must refuse the write")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("a refused write must leave no file behind")
	}

	writeFile(t, permissions, `{"writeRoots": ["`+dir+`"]}`)
	allowed, err = FakeGuardedWrite(permissions, target)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("a write under a granted root must be allowed")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "fake envelope write probe\n" {
		t.Fatalf("probe content wrong: %q err=%v", data, err)
	}

	// A sibling of the granted root must not be reachable through a
	// prefix-looking name.
	sibling := dir + "-outside"
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sibling)
	allowed, err = FakeGuardedWrite(permissions, filepath.Join(sibling, "escape.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("a path outside every granted root must be refused")
	}
}

func TestFakeGuardedNetworkDeniedAndAllowed(t *testing.T) {
	dir := t.TempDir()
	permissions := filepath.Join(dir, "permissions.json")

	writeFile(t, permissions, `{"network": "deny"}`)
	allowed, err := FakeGuardedNetwork(permissions, "127.0.0.1", "9")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("a denied envelope must refuse the call without dialing")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	received := make(chan string, 1)
	go func() {
		connection, err := listener.Accept()
		if err != nil {
			received <- ""
			return
		}
		defer connection.Close()
		buffer := make([]byte, 256)
		n, _ := connection.Read(buffer)
		received <- string(buffer[:n])
	}()

	writeFile(t, permissions, `{"network": "allow"}`)
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	allowed, err = FakeGuardedNetwork(permissions, "127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("an allowed envelope must make the call")
	}
	select {
	case request := <-received:
		if !strings.Contains(request, "fake-envelope-probe") {
			t.Fatalf("unexpected probe request %q", request)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the probe request never arrived")
	}
}

// --- fake capability snapshot ---

func TestWriteFakeCapabilitySnapshotProfiles(t *testing.T) {
	restore := now
	now = func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) }
	defer func() { now = restore }()
	dir := t.TempDir()

	path, err := WriteFakeCapabilitySnapshot(dir, "current", 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(path); base != "fake-fake-1-fake-config-v1-20260810-001.json" {
		t.Fatalf("unexpected snapshot name %q", base)
	}
	got := readJSONFile(t, path)
	if got["capturedAt"] != "2026-08-10T12:00:00Z" || got["profile"] != "current" {
		t.Fatalf("capture fields wrong: %v", got)
	}
	capabilities := got["capabilities"].(map[string]any)
	if capabilities["resume"] != true || capabilities["sessionEstablishedTimeoutSec"] != float64(6) {
		t.Fatalf("current profile must enable capabilities: %v", capabilities)
	}
	envelope := got["envelopeEnforcement"].(map[string]any)
	if envelope["writeRoots"] != "mapped" || envelope["readRoots"] != "notEnforced" || envelope["network"] != "mapped" {
		t.Fatalf("unexpected enforcement declaration %v", envelope)
	}

	// The same day advances the sequence instead of overwriting.
	next, err := WriteFakeCapabilitySnapshot(dir, "current", 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(next); base != "fake-fake-1-fake-config-v1-20260810-002.json" {
		t.Fatalf("second snapshot did not advance the sequence: %q", base)
	}

	// An aged snapshot is dated in the past, name and capture time agreeing.
	old, err := WriteFakeCapabilitySnapshot(dir, "old", 40, 6)
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(old); base != "fake-fake-1-fake-config-v1-20260701-001.json" {
		t.Fatalf("aged snapshot name wrong: %q", base)
	}
	got = readJSONFile(t, old)
	if got["capturedAt"] != "2026-07-01T12:00:00Z" {
		t.Fatalf("aged capture time wrong: %v", got["capturedAt"])
	}
	if got["capabilities"].(map[string]any)["resume"] != false {
		t.Fatal("the old profile must disable capabilities")
	}

	// The unverified-network profile declares network enforcement unproven.
	unverified, err := WriteFakeCapabilitySnapshot(dir, "unverified-network", 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	got = readJSONFile(t, unverified)
	if got["envelopeEnforcement"].(map[string]any)["network"] != "notEnforced" {
		t.Fatalf("unverified-network must declare network notEnforced: %v", got)
	}
	perms := got["permissions"].(map[string]any)
	list, ok := perms["unverified"].([]any)
	if !ok || len(list) != 1 || list[0] != "network" {
		t.Fatalf("unverified list wrong: %v", perms)
	}

	if _, err := WriteFakeCapabilitySnapshot(dir, "sideways", 0, 6); err == nil {
		t.Fatal("an unknown profile must be refused")
	}
}

// --- selftest pass record ---

func TestWriteFakeSelftestRecord(t *testing.T) {
	restore := now
	now = func() time.Time { return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC) }
	defer func() { now = restore }()

	path := filepath.Join(t.TempDir(), "selftest.json")
	if err := WriteFakeSelftestRecord(path, "fake-selftest-1"); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, path)
	if got["runtime"] != "fake" || got["job"] != "fake-selftest-1" || got["passedAt"] != "2026-08-10T12:00:00Z" {
		t.Fatalf("unexpected selftest record %v", got)
	}
	proven, ok := got["provenBehaviorally"].([]any)
	if !ok || len(proven) != 7 || proven[0] != "dispatch" || proven[6] != "denied-network" {
		t.Fatalf("proven behaviors wrong: %v", got["provenBehaviorally"])
	}
	constructed, ok := got["constructedOnly"].([]any)
	if !ok || len(constructed) != 3 {
		t.Fatalf("constructed-only list wrong: %v", got["constructedOnly"])
	}

	if err := WriteFakeSelftestRecord(path, ""); err == nil {
		t.Fatal("an empty job id must be refused")
	}
}
