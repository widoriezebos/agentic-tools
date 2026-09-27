package hostturn

import (
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

func init() {
	register("codex", hostRuntime{cli: "codex", run: runCodex})
}

// runCodex is the Codex host turn. `codex exec resume` takes no -C, so the
// CLI runs in the checkout on both paths.
func runCodex(t *Turn) int {
	if !t.requireCLI("codex") {
		return 3
	}
	raw, returnPath := t.Path("raw.out"), t.Path("return.json")
	events, usagePath := t.Path("events.jsonl"), t.Path("usage.json")
	model, ok := t.turnModel()
	if !ok {
		return 1
	}
	verb := "dispatch"
	if t.ResumeSession != "" {
		verb = "follow-up"
	}
	command, err := supervisor.CodexCommand(verb, model, t.d.Root, t.schema(), raw, t.InstanceTag, "", t.permissions(), "", t.ResumeSession)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 2
	}
	status := t.runCLI(command, events, false)
	ensureFile(raw)
	ports, err := delegate.PortsFor("codex")
	if err != nil || ports.Usage == nil {
		fmt.Fprintln(t.d.Stderr, "codex delegate ports are not registered")
		return 1
	}
	if err := ports.Usage(events, usagePath); err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	if info, err := os.Stat(raw); err == nil && info.Size() > 0 {
		copyOrEmpty(raw, returnPath)
	}
	session, _ := adapter.CodexEventField(events, "session")
	return t.finish(session, usagePath, raw, returnPath, "", status, false, "")
}
