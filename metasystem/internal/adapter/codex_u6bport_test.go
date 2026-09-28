package adapter

import (
	"reflect"
	"testing"
)

// mission-runner scenario, runner-codex: a resumed host turn opens with
// `exec resume --json`, carries the model, sandbox, never-approve policy and
// network access as -c overrides, names the session, and never re-enters
// the workspace with -C (resume takes no -C; the host enters the workspace
// before invocation instead).
func TestU6bPortCodexResumeArgvKeepsTheTurnBoundary(t *testing.T) {
	t.Parallel()
	resume, err := BuildCodexCommand("follow-up", "gpt-5-fixture", "/repo", "/schema.json", "/turn/raw.out",
		"workspace-write", "true", "codex-fixture-session", "metasystem-host-t2", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"codex", "exec", "resume", "--json",
		"-c", `model="gpt-5-fixture"`,
		"-c", `sandbox_mode="workspace-write"`,
		"-c", `approval_policy="never"`,
		"-c", "sandbox_workspace_write.network_access=true",
		"-c", `metasystem_instance_tag="metasystem-host-t2"`,
		"--output-schema", "/schema.json", "-o", "/turn/raw.out",
		"codex-fixture-session", "-",
	}
	if !reflect.DeepEqual(resume, want) {
		t.Fatalf("resume argv:\n got %q\nwant %q", resume, want)
	}
	for _, arg := range resume {
		if arg == "-C" {
			t.Fatal("the resumed argv re-entered the workspace flag")
		}
	}
}
