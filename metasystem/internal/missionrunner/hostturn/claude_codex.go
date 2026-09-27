package hostturn

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"

	// The host ports (claude-result, devin-return, the fake results)
	// register from internal/host.
	_ "github.com/widoriezebos/agentic-tools/metasystem/internal/host"
)

func init() {
	register("claude", hostRuntime{cli: "claude", run: runClaude})
	register("codex", hostRuntime{cli: "codex", run: runCodex})
}

// runCLI runs the runtime CLI in the installation root: stdin from the
// prompt, stdout into stdoutPath, stderr into the turn's host log (appended,
// or truncated first when truncateLog). It returns the shell status.
func (t *Turn) runCLI(argv []string, stdoutPath string, truncateLog bool) int {
	command := exec.Command(argv[0], argv[1:]...)
	if path, err := t.d.LookPath(argv[0]); err == nil {
		command.Path = path
	}
	command.Dir = t.d.Root
	command.Env = t.d.Environ
	stdin, err := os.Open(t.Prompt)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer stdin.Close()
	command.Stdin = stdin
	stdout, err := os.OpenFile(stdoutPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer stdout.Close()
	command.Stdout = stdout
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	if truncateLog {
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	log, err := os.OpenFile(t.Path("host.log"), flags, 0o644)
	if err != nil {
		fmt.Fprintln(t.d.Stderr, err)
		return 1
	}
	defer log.Close()
	command.Stderr = log
	return shellStatus(command.Run())
}

func shellStatus(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
		return 128 + int(signalOf(exitErr))
	}
	return 127
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

func signalOf(exitErr *exec.ExitError) syscall.Signal {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return status.Signal()
	}
	return 0
}
