// Package partner is the Project Partner: a read-only conversation with an
// agent, over the Agent Client Protocol, behind one seam.
//
// The seam is ACP and nothing else. Every runtime this seat admits speaks ACP
// as a server over stdio — Devin natively, Claude Code and Codex through their
// adapters — and a runtime is a command PLUS the configuration that makes the
// session read-only. A command alone certifies nothing about what a server
// will do, so a runtime this file does not name is refused rather than
// started.
//
// Read-only is established below the protocol, and the protocol is the
// backstop. Each runtime below names, in its own words, the options that
// forbid writes and command execution, disable external tools, MCP servers and
// network access, and refuse every approval inherited from the seat's own
// settings or from an earlier session. Above that, the permission point in
// permission.go refuses every request it cannot classify as a read of a path
// inside the checkout. The Partner therefore has no shell and no network.
package partner

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// Runtime is one admitted agent runtime, resolved for one checkout.
//
// Argv is the command that starts its ACP server; Env is what this seat adds
// to that process's environment; SessionMeta is the `_meta` object that rides
// session/new, which is where two of the three runtimes take their read-only
// configuration. ReadOnly is the same contract in one sentence, so that what
// the seat believes it configured can be read in `ui status` and in the
// activity line rather than inferred from a command line.
type Runtime struct {
	// Name is the admitted name: claude, codex or devin.
	Name string
	// Model is the model this seat asked for, or empty for the runtime's own
	// default. It is selected at session setup through the model config
	// option, and a runtime that cannot select it refuses the turn.
	Model string
	Argv  []string
	Env   []string
	// SessionMeta is marshalled as the session/new `_meta` member. Nil sends
	// none.
	SessionMeta map[string]any
	// ReadOnly says what makes this runtime read-only, in the runtime's own
	// vocabulary.
	ReadOnly string
	// Install is the line a human runs when the command is not there. Only
	// the two adapters have one; Devin is installed as a CLI, not by us.
	Install string
	// Tools is the interface's own read tools, handed to the session at
	// session/new. Nil hands none, which is the walkthrough's canned server
	// and nothing else.
	Tools *ToolServer
}

// ToolServer is the one tool server this Partner is given: the engine itself,
// over stdio, answering the eight read operations from the same readers the
// pages are composed from.
//
// It is handed over through the protocol's own tool-server hand-off rather
// than configured into the runtime, because the runtime's configuration is the
// read-only contract and must not be the place a tool arrives. Claude's
// adapter merges what session/new names into its own server map; Codex's
// adapter takes stdio servers; Devin's release notes name the same hand-off.
// Every one of them starts the command below itself and closes it with the
// session.
type ToolServer struct {
	// Name is what the server is called on the wire, which is also how a
	// runtime composes the tool names it presents to the model.
	Name string
	// Command and Args are the process the runtime starts.
	Command string
	Args    []string
	// Env is what this seat adds to that process's environment, as NAME=value.
	Env []string
}

// ToolsFor names the tool server for one checkout: this executable, with the
// hidden verb that serves the interface's read tools over stdio.
//
// It is this executable rather than a configured path because the tools ARE
// the engine: a second binary could answer a different ledger than the pages
// do, and the whole point of the hand-off is that it cannot.
func ToolsFor(checkout, installation string) (*ToolServer, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("this seat cannot name its own executable, so the Partner gets no read tools: %w", err)
	}
	// Both roots are named, because the runtime starts this process from
	// whatever directory it happens to be in: a tool server that derived its
	// installation from its own working directory would be a tool server that
	// worked in a test and failed the moment an adapter started it.
	return &ToolServer{
		Name:    uitools.ServerName,
		Command: executable,
		Args:    []string{"ui", "tools", "--root", checkout, "--metasystem-root", installation},
	}, nil
}

// wire is the tool server as session/new carries it: a stdio server, with no
// `type` member at all, which is the shape every adapter reads as stdio.
func (t *ToolServer) wire() []any {
	if t == nil {
		return []any{}
	}
	env := make([]any, 0, len(t.Env))
	for _, entry := range t.Env {
		name, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		env = append(env, map[string]any{"name": name, "value": value})
	}
	args := make([]any, 0, len(t.Args))
	for _, argument := range t.Args {
		args = append(args, argument)
	}
	return []any{map[string]any{
		"name": t.Name, "command": t.Command, "args": args, "env": env,
	}}
}

// Names are the runtimes this slice admits. A fourth is a new file and a new
// read-only contract, never a new configuration value.
const (
	RuntimeClaude = "claude"
	RuntimeCodex  = "codex"
	RuntimeDevin  = "devin"
)

// modelConfigID is the session config option every ACP server names its model
// selection with. It is the protocol's own id rather than a runtime's, which
// is why it is here and not in the three runtime files.
const modelConfigID = "model"

// DefaultModels is the model each runtime is asked for when the seat names
// none. They are the design's, per runtime.
var DefaultModels = map[string]string{
	RuntimeClaude: "claude-opus-5-5",
	RuntimeCodex:  "gpt-6-sol",
	RuntimeDevin:  "",
}

// DefaultCommands is the binary each runtime is started as when the seat names
// no command. They are the published entry points of the two adapters and
// Devin's own ACP verb.
var DefaultCommands = map[string][]string{
	RuntimeClaude: {"claude-agent-acp"},
	RuntimeCodex:  {"codex-acp"},
	RuntimeDevin:  {"devin", "acp"},
}

// Admitted is every runtime name this build will start, in a stable order, so
// a refusal can name them.
func Admitted() []string {
	names := make([]string, 0, len(DefaultCommands))
	for name := range DefaultCommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Admit resolves one runtime for one checkout, or refuses.
//
// command is `ui.partner.command.<runtime>` split into an argv, empty for the
// runtime's own default; model is `ui.partner.model`, empty for the runtime's
// default. An unknown name is refused here and nowhere else: this is the one
// place a command becomes a process, and a name nobody wrote a read-only
// contract for has no contract to be started under.
func Admit(name string, command []string, model string, checkout string) (Runtime, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Runtime{}, fmt.Errorf("no Partner runtime is configured")
	}
	if _, known := DefaultCommands[name]; !known {
		return Runtime{}, fmt.Errorf("%q is not a Partner runtime this build admits; it admits %s",
			name, strings.Join(Admitted(), ", "))
	}
	argv := command
	if len(argv) == 0 {
		argv = append([]string{}, DefaultCommands[name]...)
	}
	if model == "" {
		model = DefaultModels[name]
	}
	switch name {
	case RuntimeClaude:
		return claudeRuntime(argv, model, checkout), nil
	case RuntimeCodex:
		return codexRuntime(argv, model, checkout)
	default:
		return devinRuntime(argv, model, checkout)
	}
}

// jsonEnv is how two of the three runtimes take a structured configuration:
// as one JSON object in one environment variable. Marshalling cannot fail for
// the shapes this package builds, and a failure would be a programming error
// rather than a wire event, so it yields an empty object and the runtime's own
// refusal follows.
func jsonEnv(key string, value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return key + "={}"
	}
	return key + "=" + string(body)
}
