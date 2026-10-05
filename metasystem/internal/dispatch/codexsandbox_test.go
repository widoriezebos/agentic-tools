package dispatch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// noEnvironment is a configuration environment with nothing set, so a test
// reads only its own settings files.
func noEnvironment(string) (string, bool) { return "", false }

// sandboxInstallation is a checkout whose local settings hold local, or none
// when local is empty.
func sandboxInstallation(t *testing.T, local string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude,codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if local != "" {
		if err := os.WriteFile(filepath.Join(root, "metasystem.conf.local"), []byte(local), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func sandboxRequested() map[string]any {
	return map[string]any{
		"readRoots": []any{"/repo"}, "writeRoots": []any{},
		"network": "deny", "approvals": "deny", "tools": "read-only", "preset": "read-only",
	}
}

// buildSandboxRecord admits one fresh job using the dispatch's settings and
// returns its permissions.requested.
func buildSandboxRecord(t *testing.T, root, settingsFile, runtime string) map[string]any {
	t.Helper()
	dir := t.TempDir()
	permissions := writeJSONFile(t, dir, "permissions.json", sandboxRequested())
	capResolution := writeJSONFile(t, dir, "cap.json", map[string]any{
		"capMin": 30, "capDeadline": nil,
		"source": map[string]any{"rule": "fixture", "origin": "fixture", "truncatedBy": nil},
	})
	output := filepath.Join(dir, "record.json")
	err := buildRecord(BuildRecordParams{
		LookupEnv: noEnvironment, Output: output, Job: "sandbox-record", Role: "code-critic", Root: root, SettingsFile: settingsFile,
		Runtime: runtime, Workspace: root, CapResolution: capResolution, Model: "gpt-fixture",
		Permissions: permissions, Fallbacks: "[]", ReasoningEffort: "medium",
		DestructiveReach: HazardMechanical, LaunchMode: LaunchModeSharedCheckout,
		OutputStream: filepath.Join(dir, "record.jsonl"),
	}, recordFacts(t, root, 1, ""))
	if err != nil {
		t.Fatal(err)
	}
	return readJSONFile(t, output)["permissions"].(map[string]any)["requested"].(map[string]any)
}

// A Codex job admitted on a host whose launch.codex.sandbox is
// danger-full-access records the envelope the host enforces, named by
// widenedBy; approvals, tools and the preset stay as requested. Under the
// default, and for another runtime, the request is recorded unchanged.
func TestBuildRecordWidensACodexRequestUnderFullAccess(t *testing.T) {
	t.Parallel()
	fullAccess := sandboxInstallation(t, "launch.codex.sandbox=danger-full-access\n")
	widened := buildSandboxRecord(t, fullAccess, "", "codex")
	want := map[string]any{
		"readRoots": []any{"/"}, "writeRoots": []any{"/"},
		"network": "allow", "approvals": "deny", "tools": "read-only", "preset": "read-only",
		"widenedBy": "launch.codex.sandbox=danger-full-access",
	}
	if !reflect.DeepEqual(widened, want) {
		t.Fatalf("requested under full access = %v\nwant %v", widened, want)
	}
	for _, test := range []struct{ name, local, runtime string }{
		{"default", "", "codex"},
		{"workspace-write", "launch.codex.sandbox=workspace-write\n", "codex"},
		{"another runtime", "launch.codex.sandbox=danger-full-access\n", "fake"},
	} {
		if got := buildSandboxRecord(t, sandboxInstallation(t, test.local), "", test.runtime); !reflect.DeepEqual(got, sandboxRequested()) {
			t.Fatalf("%s: requested = %v, want the request unchanged", test.name, got)
		}
	}
}

func TestBuildRecordReadsTheServingInstallationsCodexSandbox(t *testing.T) {
	t.Parallel()
	root := sandboxInstallation(t, "")
	serving := sandboxInstallation(t, "")
	settingsFile := filepath.Join(serving, "metasystem.conf")
	if err := os.WriteFile(settingsFile, []byte("launch.codex.sandbox=danger-full-access\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := buildSandboxRecord(t, root, settingsFile, "codex"); !reflect.DeepEqual(got, WidenForCodexSandbox(sandboxRequested())) {
		t.Fatalf("serving full access = %v, want the widened request", got)
	}
	if err := os.WriteFile(settingsFile+".local", []byte("launch.codex.sandbox=workspace-write\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := buildSandboxRecord(t, root, settingsFile, "codex"); !reflect.DeepEqual(got, sandboxRequested()) {
		t.Fatalf("serving local workspace-write = %v, want the request unchanged", got)
	}
}

func TestBuildRecordWithoutSettingsFileReadsTheRootsCodexSandbox(t *testing.T) {
	t.Parallel()
	root := sandboxInstallation(t, "")
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("launch.codex.sandbox=danger-full-access\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := buildSandboxRecord(t, root, "", "codex"); !reflect.DeepEqual(got, WidenForCodexSandbox(sandboxRequested())) {
		t.Fatalf("root full access without a settings file = %v, want the widened request", got)
	}
}

// A value outside the two modes refuses the admission, naming both.
func TestAdmittedRequestRefusesAnUnknownSandbox(t *testing.T) {
	t.Parallel()
	root := sandboxInstallation(t, "launch.codex.sandbox=other\n")
	_, err := admittedRequest(noEnvironment, root, "", "codex", sandboxRequested())
	if err == nil || !strings.Contains(err.Error(), "workspace-write") || !strings.Contains(err.Error(), "danger-full-access") {
		t.Fatalf("an unknown sandbox = %v, want a refusal naming both modes", err)
	}
}

// The handshake passes a widened request against the effective envelope
// materialized from it, and still refuses an effective network allow against
// a requested deny when nothing widened the request.
func TestHandshakePassesTheWidenedPairOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	output := filepath.Join(dir, "out.json")
	widened := WidenForCodexSandbox(sandboxRequested())
	record := writeJSONFile(t, dir, "widened.json", map[string]any{
		"jobId": "j", "permissions": map[string]any{"requested": widened, "effective": nil, "enforcementSnapshot": "snap.json"},
	})
	effective := writeJSONFile(t, dir, "widened-effective.json", widened)
	if err := HandshakeEval(record, effective, "sess-1", "", "model-x", true, output); err != nil {
		t.Fatal(err)
	}
	if result := readJSONFile(t, output); result["target"] != "running" {
		t.Fatalf("the widened pair = %v, want running", result)
	}

	contained := writeJSONFile(t, dir, "contained.json", map[string]any{
		"jobId": "j", "permissions": map[string]any{"requested": sandboxRequested(), "effective": nil, "enforcementSnapshot": "snap.json"},
	})
	open := sandboxRequested()
	open["network"] = "allow"
	wider := writeJSONFile(t, dir, "contained-effective.json", open)
	if err := HandshakeEval(contained, wider, "sess-1", "", "model-x", true, output); err != nil {
		t.Fatal(err)
	}
	result := readJSONFile(t, output)
	if patch, _ := result["patch"].(map[string]any); result["target"] != "failed" || patch["error"] != "permissions_mismatch:network" {
		t.Fatalf("network allow against deny = %v, want permissions_mismatch:network", result)
	}
}

// Widening copies the request: the caller's map is left as it was.
func TestWidenForCodexSandboxLeavesTheRequestAlone(t *testing.T) {
	t.Parallel()
	requested := sandboxRequested()
	WidenForCodexSandbox(requested)
	if !reflect.DeepEqual(requested, sandboxRequested()) {
		t.Fatalf("widening changed the request: %v", requested)
	}
}
