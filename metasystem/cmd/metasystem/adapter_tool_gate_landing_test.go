package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/agentgate"
)

// The recorded context gate (`internal adapter claude-tool-gate`) fails open
// on its own errors for a seat. For the landing agent it is the landing
// gate, and every one of those errors denies (unit A-b, TestHookErrorDenies'
// adapter half): an unresolvable root, bad flags, an unreadable call.
func TestAdapterToolGateDeniesForTheLandingAgent(t *testing.T) {
	t.Setenv(agentgate.LineageEnv, agentgate.Lineage)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	top, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	module := filepath.Join(top, "metasystem")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "lane/b1"}, {"add", "."}, {"-c", "user.name=f", "-c", "user.email=f@invalid", "commit", "-qm", "f"}} {
		command := exec.Command("git", append([]string{"-C", top}, args...)...)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + top, "GIT_CONFIG_NOSYSTEM=1"}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	call := func(command string) string {
		encoded, err := json.Marshal(map[string]any{"session_id": "landing-1", "cwd": top, "tool_name": "Bash",
			"tool_input": map[string]any{"command": command}})
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	run := func(args []string, input string) (int, string) {
		var code int
		var stdout string
		withStdin(t, input, func() {
			code, stdout, _ = runOnOwnStreams(func(stdout, stderr io.Writer) int {
				return runAdapterClaudeToolGate(args, stdout, stderr)
			})
		})
		return code, stdout
	}
	denied := func(code int, stdout string) bool {
		var answer struct {
			HookSpecificOutput struct{ PermissionDecision, PermissionDecisionReason string } `json:"hookSpecificOutput"`
		}
		return code == 0 && json.Unmarshal([]byte(stdout), &answer) == nil && answer.HookSpecificOutput.PermissionDecision == "deny" &&
			strings.Count(answer.HookSpecificOutput.PermissionDecisionReason, "\n") == 1
	}
	for name, test := range map[string]struct {
		args  []string
		input string
	}{
		"unresolvable root": {[]string{"--root", filepath.Join(top, "gone")}, call("git status")},
		"bad flags":         {[]string{"--bogus"}, call("git status")},
		"no root":           {nil, call("git status")},
		"unreadable call":   {[]string{"--root", module}, "{not json"},
		"push":              {[]string{"--root", module}, call("git push origin HEAD:main")},
	} {
		if code, stdout := run(test.args, test.input); !denied(code, stdout) {
			t.Errorf("%s: code %d stdout %q, want a denial", name, code, stdout)
		}
	}
	if code, stdout := run([]string{"--root", module}, call("git status")); code != 0 || stdout != "" {
		t.Fatalf("an admitted call: code %d stdout %q", code, stdout)
	}
}
