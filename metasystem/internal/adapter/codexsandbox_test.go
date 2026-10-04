package adapter

import (
	"path/filepath"
	"strings"
	"testing"
)

// The host's sandbox mode against both envelope shapes: workspace-write keeps
// the envelope's own mapping, danger-full-access replaces it, a read-only
// envelope included.
func TestCodexPermissionSettingsUnderTheHostSandbox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	readOnly := filepath.Join(dir, "read-only.json")
	writeFile(t, readOnly, `{"writeRoots":[],"network":"deny"}`)
	workspaceWrite := filepath.Join(dir, "workspace-write.json")
	writeFile(t, workspaceWrite, `{"writeRoots":["/ws"],"network":"deny"}`)
	for _, test := range []struct {
		envelope, mode, want string
	}{
		{readOnly, "workspace-write", "read-only"},
		{workspaceWrite, "workspace-write", "workspace-write"},
		{readOnly, "danger-full-access", "danger-full-access"},
		{workspaceWrite, "danger-full-access", "danger-full-access"},
	} {
		sandbox, _, err := CodexPermissionSettings(test.envelope, "", test.mode)
		if err != nil || sandbox != test.want {
			t.Fatalf("%s under %s = %q, %v; want %q", filepath.Base(test.envelope), test.mode, sandbox, err, test.want)
		}
	}
}

// A delegate round runs under the mode its record was admitted with: only the
// launch.codex.sandbox widening in the requested envelope means full access.
func TestCodexAdmittedSandboxFollowsTheRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, test := range []struct {
		name, record, want string
	}{
		{"widened", `{"permissions":{"requested":{"writeRoots":["/"],"network":"allow","widenedBy":"launch.codex.sandbox=danger-full-access"}}}`, "danger-full-access"},
		{"contained", `{"permissions":{"requested":{"writeRoots":["/ws"],"network":"deny"}}}`, "workspace-write"},
		{"other-widening", `{"permissions":{"requested":{"writeRoots":["/ws"],"widenedBy":"something-else"}}}`, "workspace-write"},
	} {
		path := filepath.Join(dir, test.name+".json")
		writeFile(t, path, test.record)
		if got := CodexAdmittedSandbox(path); got != test.want {
			t.Fatalf("%s: admitted sandbox = %q, want %q", test.name, got, test.want)
		}
	}
	if got := CodexAdmittedSandbox(filepath.Join(dir, "missing.json")); got != "workspace-write" {
		t.Fatalf("an unreadable record's admitted sandbox = %q, want workspace-write", got)
	}
}

// Under the default every argv is the one Codex has always been given: the
// golden fresh thread and resume, extra write roots and effort included.
func TestBuildCodexCommandDefaultArgvIsGolden(t *testing.T) {
	t.Parallel()
	extra := []string{"/repo/.git/worktrees/j"}
	fresh, err := BuildCodexCommand("dispatch", "m", "/ws", "/s.json", "/o.json", "workspace-write", "false", "", "job-tag", "xhigh", extra)
	if err != nil {
		t.Fatal(err)
	}
	wantFresh := []string{
		"codex", "exec", "--json", "-m", "m", "--sandbox", "workspace-write", "-C", "/ws",
		"-c", `approval_policy="never"`, "-c", "sandbox_workspace_write.network_access=false",
		"-c", `metasystem_instance_tag="job-tag"`, "-c", `model_reasoning_effort="xhigh"`,
		"--add-dir", "/repo/.git/worktrees/j", "--output-schema", "/s.json", "-o", "/o.json", "-",
	}
	if strings.Join(fresh, "\x00") != strings.Join(wantFresh, "\x00") {
		t.Fatalf("fresh argv:\n got %q\nwant %q", fresh, wantFresh)
	}
	resume, err := BuildCodexCommand("follow-up", "m", "/ws", "/s.json", "/o.json", "read-only", "true", "sess", "job-tag", "", extra)
	if err != nil {
		t.Fatal(err)
	}
	wantResume := []string{
		"codex", "exec", "resume", "--json", "-c", `model="m"`, "-c", `sandbox_mode="read-only"`,
		"-c", `approval_policy="never"`, "-c", "sandbox_workspace_write.network_access=true",
		"-c", `metasystem_instance_tag="job-tag"`,
		"-c", `sandbox_workspace_write.writable_roots=["/repo/.git/worktrees/j"]`,
		"--output-schema", "/s.json", "-o", "/o.json", "sess", "-",
	}
	if strings.Join(resume, "\x00") != strings.Join(wantResume, "\x00") {
		t.Fatalf("resume argv:\n got %q\nwant %q", resume, wantResume)
	}
}

// Under full access the fresh thread gets --sandbox danger-full-access and the
// resume sandbox_mode="danger-full-access"; neither carries the network
// override or the extra write roots, which configure workspace-write alone.
func TestBuildCodexCommandUnderFullAccess(t *testing.T) {
	t.Parallel()
	extra := []string{"/", "/repo/.git/worktrees/j"}
	fresh, err := BuildCodexCommand("dispatch", "m", "/ws", "/s.json", "/o.json", "danger-full-access", "true", "", "job-tag", "", extra)
	if err != nil {
		t.Fatal(err)
	}
	wantFresh := []string{
		"codex", "exec", "--json", "-m", "m", "--sandbox", "danger-full-access", "-C", "/ws",
		"-c", `approval_policy="never"`, "-c", `metasystem_instance_tag="job-tag"`,
		"--output-schema", "/s.json", "-o", "/o.json", "-",
	}
	if strings.Join(fresh, "\x00") != strings.Join(wantFresh, "\x00") {
		t.Fatalf("fresh argv:\n got %q\nwant %q", fresh, wantFresh)
	}
	resume, err := BuildCodexCommand("follow-up", "m", "/ws", "/s.json", "/o.json", "danger-full-access", "true", "sess", "job-tag", "", extra)
	if err != nil {
		t.Fatal(err)
	}
	wantResume := []string{
		"codex", "exec", "resume", "--json", "-c", `model="m"`, "-c", `sandbox_mode="danger-full-access"`,
		"-c", `approval_policy="never"`, "-c", `metasystem_instance_tag="job-tag"`,
		"--output-schema", "/s.json", "-o", "/o.json", "sess", "-",
	}
	if strings.Join(resume, "\x00") != strings.Join(wantResume, "\x00") {
		t.Fatalf("resume argv:\n got %q\nwant %q", resume, wantResume)
	}
}

// widenedRecord is a Codex job admitted under full access: its request names
// the whole host and the setting that widened it.
const widenedRecord = `{
  "permissions": {
    "requested": {
      "readRoots": ["/"],
      "writeRoots": ["/"],
      "network": "allow",
      "approvals": "deny",
      "tools": "runtime-default",
      "preset": "workspace",
      "widenedBy": "launch.codex.sandbox=danger-full-access"
    }
  }
}`

// A widened request keeps its roots through the write-scope pin, and the
// effective envelope materialized from it passes the widening check; an
// envelope without widenedBy is still pinned to the workspace.
func TestRewriteWriteScopeLeavesAWidenedEnvelope(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	workspace := filepath.Join(dir, "repo")
	writeFile(t, filepath.Join(workspace, ".keep"), "")

	widened, widenedEffective := filepath.Join(dir, "widened.json"), filepath.Join(dir, "widened-effective.json")
	writeFile(t, widened, widenedRecord)
	if err := MaterializeEffective(widened, widenedEffective); err != nil {
		t.Fatal(err)
	}
	if err := RewriteWriteScope(widenedEffective, workspace); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, widenedEffective)
	if roots, _ := got["writeRoots"].([]any); len(roots) != 1 || roots[0] != "/" || got["widenedBy"] != "launch.codex.sandbox=danger-full-access" {
		t.Fatalf("the widened envelope was rewritten: %v", got)
	}
	if mismatch, err := ComparePermissions(widened, widenedEffective); err != nil || mismatch != "" {
		t.Fatalf("the widened pair = %q, %v; want it to pass", mismatch, err)
	}

	contained, containedEffective := filepath.Join(dir, "contained.json"), filepath.Join(dir, "contained-effective.json")
	writeFile(t, contained, requestedRecord)
	if err := MaterializeEffective(contained, containedEffective); err != nil {
		t.Fatal(err)
	}
	if err := RewriteWriteScope(containedEffective, workspace); err != nil {
		t.Fatal(err)
	}
	if roots, _ := readJSONFile(t, containedEffective)["writeRoots"].([]any); len(roots) != 1 || roots[0] != resolve(workspace) {
		t.Fatalf("an envelope without widenedBy was not pinned to the workspace: %v", roots)
	}
}

// The comparison has no exception for the sandbox: an effective network allow
// against a requested deny is still refused.
func TestComparePermissionsStillRefusesAnUnwidenedNetwork(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	record, effective := filepath.Join(dir, "job.json"), filepath.Join(dir, "effective.json")
	writeFile(t, record, requestedRecord)
	writeFile(t, effective, `{"readRoots":["/a"],"writeRoots":["/w"],"network":"allow","approvals":"ask","tools":"read-only"}`)
	if mismatch, err := ComparePermissions(record, effective); err != nil || mismatch != "network" {
		t.Fatalf("network allow against deny = %q, %v; want network", mismatch, err)
	}
}
