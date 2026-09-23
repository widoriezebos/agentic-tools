package partner

// Claude Code, through the @agentclientprotocol/claude-agent-acp adapter.
//
// The adapter forwards `_meta.claudeCode.options` to the Claude Agent SDK's
// own options, and those options are where read-only is established:
//
//   - `tools` is the whole set of built-in tools the session loads. Naming the
//     three reading tools means Bash, Write, Edit, NotebookEdit, WebFetch,
//     WebSearch and Task are never loaded at all, so there is no shell and no
//     network, and nothing to approve.
//   - `disallowedTools` names them again anyway, because a preset that grows a
//     tool in a later release must not grow this session one.
//   - `settingSources: []` is the no-inherited-approvals lever: without it the
//     SDK resolves the user's, the project's and the local settings files, and
//     an `allow` rule a human once wrote for their own terminal would decide
//     what this session may do.
//   - `mcpServers: {}` with `strictMcpConfig: true` refuses every MCP server,
//     including any the resolved settings would have supplied.
//   - `allowDangerouslySkipPermissions: false` is what the adapter reads to
//     decide whether the bypass mode may be offered at all; false removes it
//     from the session's mode catalogue.
//   - `plugins: []` and `skills: []` keep third-party instructions and their
//     tools out of a conversation that is supposed to read this checkout and
//     explain it.
//
// The model rides the same options, and is ALSO selected through the protocol
// at session setup, which is what refuses the turn when the seat asked for a
// model this account cannot use.

const claudeInstall = "npm install -g @agentclientprotocol/claude-agent-acp"

// claudeReadOnly is the contract in one sentence, in the adapter's own
// vocabulary.
const claudeReadOnly = "tools limited to Read, Glob and Grep; Bash, Write, Edit, NotebookEdit, WebFetch, WebSearch and Task disallowed; settingSources empty so no inherited approval applies; no MCP servers under strictMcpConfig; no plugins or skills; bypass refused"

// claudeConfinedReadOnly is the same contract with the seat's own sandbox
// beneath it. Claude keeps the contract by itself — asked to write a file and
// run a command it refused both, because neither tool exists in the session —
// so the sandbox here is depth rather than the load-bearing half, and a
// platform without one still admits Claude.
const claudeConfinedReadOnly = claudeReadOnly + "; " + confinement

// claudeReadingTools is the whole tool surface a read-only Partner has.
var claudeReadingTools = []string{"Read", "Glob", "Grep"}

// claudeForbiddenTools is every built-in that writes, executes, reaches the
// network or delegates, named again beneath the allowlist.
var claudeForbiddenTools = []string{
	"Bash", "BashOutput", "KillShell", "Write", "Edit", "MultiEdit",
	"NotebookEdit", "WebFetch", "WebSearch", "Task", "SlashCommand",
	"TodoWrite", "AskUserQuestion",
}

func claudeRuntime(argv []string, model string, checkout string) Runtime {
	options := map[string]any{
		"settingSources":                  []string{},
		"tools":                           claudeReadingTools,
		"disallowedTools":                 claudeForbiddenTools,
		"allowDangerouslySkipPermissions": false,
		"mcpServers":                      map[string]any{},
		"strictMcpConfig":                 true,
		"plugins":                         []string{},
		"skills":                          []string{},
	}
	if model != "" {
		options["model"] = model
	}
	confined, kept := confine(argv, checkout)
	readOnly := claudeReadOnly
	if kept {
		readOnly = claudeConfinedReadOnly
	}
	return Runtime{
		Name:  RuntimeClaude,
		Model: model,
		Argv:  confined,
		// The adapter opens no browser of its own for authentication: a server
		// that needs a human to sign in says so on the wire, which is what the
		// drawer shows.
		Env:         []string{"NO_BROWSER=1"},
		SessionMeta: map[string]any{"claudeCode": map[string]any{"options": options}},
		ReadOnly:    readOnly,
		Install:     claudeInstall,
	}
}
