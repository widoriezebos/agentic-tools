package hostturn

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
)

func init() {
	register("claude", hostRuntime{cli: "claude", run: runClaude})
}

// runClaude is the Claude host turn: one blocking json turn in the checkout
// (host mode is the record-less claude-command call — acceptEdits with the
// full tools).
func runClaude(t *Turn) int {
	if !t.requireCLI("claude") {
		return 3
	}
	raw, returnPath := t.Path("raw.out"), t.Path("return.json")
	provider, usagePath := t.Path("claude-result.json"), t.Path("usage.json")
	model, ok := t.turnModel()
	if !ok {
		return 1
	}
	log, err := os.OpenFile(t.Path("host.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	command, failure := supervisor.ClaudeCommand(t.d, "", model, t.schema(), "", t.ResumeSession, "json", log)
	log.Close()
	if failure != "" {
		fmt.Fprintln(t.d.Stderr, "claude host native budget configuration is invalid")
		return 3
	}
	status := t.runCLI(command, provider, true)
	copyOrEmpty(provider, raw)
	ports, err := delegate.PortsFor("claude")
	if err != nil || ports.HostResult == nil {
		fmt.Fprintln(t.d.Stderr, "claude host ports are not registered")
		return 1
	}
	if err := ports.HostResult(provider, returnPath, usagePath); err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	session := ""
	if data, err := os.ReadFile(provider); err == nil {
		fallback := ""
		session, _ = jsonedit.Get(data, "session_id", &fallback)
	}
	return t.finish(session, usagePath, raw, returnPath, "", status, false, "")
}
