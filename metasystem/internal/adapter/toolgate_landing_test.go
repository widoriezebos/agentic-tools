package adapter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// For the landing agent the gate fails closed: none of the context gate's
// fail-open paths (no clock, a subagent's call, an unresolvable root, an
// unreadable call) lets a call through, and a denial that cannot be written
// is an error the boundary turns into the blocking status.
func TestToolGateLandingAgentFailsClosed(t *testing.T) {
	t.Parallel()
	subagentPush := `{"session_id":"s","agent_id":"sub-1","cwd":"/","tool_name":"Bash","tool_input":{"command":"git push"}}`
	for name, opts := range map[string]ToolGateOptions{
		"no call":           {},
		"unreadable call":   {Stdin: strings.NewReader("{not json"), Installation: t.TempDir()},
		"subagent push":     {Stdin: strings.NewReader(subagentPush), Installation: t.TempDir()},
		"no installation":   {Stdin: strings.NewReader(subagentPush)},
		"not in a checkout": {Stdin: strings.NewReader(`{"tool_name":"Read","tool_input":{}}`), Installation: t.TempDir()},
	} {
		stdout := &bytes.Buffer{}
		opts.LandingAgent, opts.Stdout = true, stdout
		if err := RunToolGate(opts); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var answer struct {
			HookSpecificOutput struct{ PermissionDecision string } `json:"hookSpecificOutput"`
		}
		if json.Unmarshal(stdout.Bytes(), &answer) != nil || answer.HookSpecificOutput.PermissionDecision != "deny" {
			t.Errorf("%s: stdout %q, want a denial", name, stdout.String())
		}
	}
	if err := RunToolGate(ToolGateOptions{LandingAgent: true}); err == nil {
		t.Fatal("a denial with nowhere to go returned no error")
	}
}
