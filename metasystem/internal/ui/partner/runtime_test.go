package partner

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// A runtime nobody wrote a read-only contract for is refused, because a
// command alone certifies nothing about what a server will do.
func TestAdmitRefusesAnUnknownRuntime(t *testing.T) {
	t.Parallel()
	_, err := Admit("gemini", nil, "", "/work")
	testutil.Require(t, "refused", err != nil, true)
	testutil.Expect(t, "names the three", strings.Contains(err.Error(), "claude, codex, devin"), true)
}

func TestAdmitRefusesNoRuntime(t *testing.T) {
	t.Parallel()
	_, err := Admit("", nil, "", "/work")
	testutil.Require(t, "refused", err != nil, true)
}

// Claude's read-only configuration is the session options the adapter
// forwards: a three-tool allowlist, the denylist beneath it, no inherited
// settings, no MCP, and no bypass.
func TestClaudeIsStartedReadOnly(t *testing.T) {
	t.Parallel()
	admitted, err := Admit(RuntimeClaude, nil, "", "/work")
	testutil.Require(t, "admitted", err, nil)
	options := claudeOptions(t, admitted)
	testutil.Expect(t, "tools", options["tools"], []any{"Read", "Glob", "Grep"})
	testutil.Expect(t, "settings are not inherited", options["settingSources"], []any{})
	testutil.Expect(t, "no MCP", options["mcpServers"], map[string]any{})
	testutil.Expect(t, "strict MCP", options["strictMcpConfig"], true)
	testutil.Expect(t, "no bypass", options["allowDangerouslySkipPermissions"], false)
	testutil.Expect(t, "the model is asked for", options["model"], "claude-opus-5-5")
	forbidden, _ := json.Marshal(options["disallowedTools"])
	for _, tool := range []string{"Bash", "Write", "Edit", "WebFetch", "WebSearch", "Task"} {
		testutil.Expect(t, "forbids "+tool, strings.Contains(string(forbidden), `"`+tool+`"`), true)
	}
}

// Codex takes its read-only configuration from the environment, and the seat
// says which sandbox and which approval policy.
func TestCodexIsStartedReadOnly(t *testing.T) {
	t.Parallel()
	admitted, err := Admit(RuntimeCodex, nil, "gpt-6-sol", "/work")
	if runtime.GOOS != "darwin" {
		testutil.Require(t, "unconfined platforms refuse codex", err != nil, true)
		return
	}
	testutil.Require(t, "admitted", err, nil)
	config := map[string]any{}
	testutil.Require(t, "the config is JSON", json.Unmarshal([]byte(valueOf(admitted.Env, "CODEX_CONFIG")), &config), nil)
	testutil.Expect(t, "sandbox", config["sandbox_mode"], "read-only")
	testutil.Expect(t, "approvals", config["approval_policy"], "on-request")
	testutil.Expect(t, "no MCP", config["mcp_servers"], map[string]any{})
	testutil.Expect(t, "no web search", config["tools"], map[string]any{"web_search": false})
	testutil.Expect(t, "the initial mode", valueOf(admitted.Env, "INITIAL_AGENT_MODE"), "read-only")
	testutil.Expect(t, "the model", config["model"], "gpt-6-sol")
}

// Devin's lever is its permission mode, and the seat sets it whether or not
// the configured command already names one.
func TestDevinIsStartedUnderTheAutoPermissionMode(t *testing.T) {
	t.Parallel()
	admitted, err := Admit(RuntimeDevin, nil, "", "/work")
	if runtime.GOOS != "darwin" {
		testutil.Require(t, "unconfined platforms refuse devin", err != nil, true)
		return
	}
	testutil.Require(t, "admitted", err, nil)
	testutil.Expect(t, "the flag is before the verb",
		strings.Contains(strings.Join(admitted.Argv, " "), "devin --permission-mode auto acp"), true)
	testutil.Expect(t, "the environment says it too",
		valueOf(admitted.Env, "DEVIN_PERMISSION_MODE"), "auto")
}

func TestDevinKeepsAPermissionModeTheSeatAlreadyNamed(t *testing.T) {
	t.Parallel()
	admitted, err := Admit(RuntimeDevin, []string{"devin", "--permission-mode", "auto", "acp"}, "", "/work")
	if runtime.GOOS != "darwin" {
		testutil.Require(t, "unconfined platforms refuse devin", err != nil, true)
		return
	}
	testutil.Require(t, "admitted", err, nil)
	testutil.Expect(t, "the flag appears once",
		strings.Count(strings.Join(admitted.Argv, " "), "--permission-mode"), 1)
}

// Every admitted runtime is started inside the platform's sandbox with writes
// to the checkout denied, because two of the three write the checkout when
// they are asked to and their own options do not stop them.
func TestEveryRuntimeIsConfinedToReadTheCheckout(t *testing.T) {
	t.Parallel()
	for _, name := range Admitted() {
		admitted, err := Admit(name, nil, "", "/work/checkout")
		if runtime.GOOS != "darwin" {
			if name != RuntimeClaude {
				testutil.Expect(t, name+" is refused unconfined", err != nil, true)
			}
			continue
		}
		testutil.Require(t, name+" is admitted", err, nil)
		line := strings.Join(admitted.Argv, " ")
		testutil.Expect(t, name+" runs in the sandbox", strings.HasPrefix(line, sandboxExec+" -p "), true)
		testutil.Expect(t, name+" denies writes to the checkout",
			strings.Contains(line, `(deny file-write* (subpath "/work/checkout"))`), true)
	}
}

// The seat's own command replaces the runtime's default, and keeps the same
// read-only configuration around it.
func TestAdmitTakesTheSeatsOwnCommand(t *testing.T) {
	t.Parallel()
	admitted, err := Admit(RuntimeClaude, []string{"/opt/acp/claude-agent-acp", "--verbose"}, "", "/work")
	testutil.Require(t, "admitted", err, nil)
	testutil.Expect(t, "the command is the seat's",
		strings.Contains(strings.Join(admitted.Argv, " "), "/opt/acp/claude-agent-acp --verbose"), true)
}

func claudeOptions(t *testing.T, admitted Runtime) map[string]any {
	t.Helper()
	body, err := json.Marshal(admitted.SessionMeta)
	testutil.Require(t, "the session meta is JSON", err, nil)
	var meta struct {
		ClaudeCode struct {
			Options map[string]any `json:"options"`
		} `json:"claudeCode"`
	}
	testutil.Require(t, "the session meta reads back", json.Unmarshal(body, &meta), nil)
	return meta.ClaudeCode.Options
}

func valueOf(environment []string, key string) string {
	for _, entry := range environment {
		if name, value, found := strings.Cut(entry, "="); found && name == key {
			return value
		}
	}
	return ""
}
