package partner

// Codex, through the @agentclientprotocol/codex-acp adapter.
//
// The adapter takes its configuration from the environment: `CODEX_CONFIG` is
// a JSON object merged into the Codex session configuration, and
// `INITIAL_AGENT_MODE` picks the approval and sandbox preset every turn runs
// under.
//
//   - `sandbox_mode: "read-only"` is the enforcement lever: Codex runs its own
//     tools inside a sandbox, and in read-only that sandbox denies every write
//     and every network connection. `approval_policy: "on-request"` is what
//     turns an attempt to leave it into an ACP permission request rather than
//     a silent escalation, and the permission point refuses it.
//   - `INITIAL_AGENT_MODE=read-only` is the same pair as the session's initial
//     mode, so the first turn runs under it and not under the adapter's own
//     default, which is the agent preset.
//   - `tools.web_search: false` removes the one tool that reaches the network
//     without leaving the sandbox.
//   - `mcp_servers: {}` refuses every MCP server the user's config.toml would
//     otherwise have supplied, which is also where an inherited approval would
//     live.
//   - `projects` trust is left alone deliberately: this session is read-only,
//     so there is nothing for a trust level to unlock.
//
// The adapter's `read-only` preset is a preset of the adapter's, not of the
// protocol's, and its sandbox is only as strong as the Codex binary beneath
// it. That is why the permission point above it refuses everything it cannot
// classify as a read inside the checkout, and why the proof this slice carries
// asks a live Codex to write a file and to run a command.

const codexInstall = "npm install -g @agentclientprotocol/codex-acp"

const codexReadOnly = "sandbox_mode read-only and approval_policy on-request, so an escalation becomes a permission request the client refuses; INITIAL_AGENT_MODE read-only; web search off; no MCP servers; " + confinement

func codexRuntime(argv []string, model string, checkout string) (Runtime, error) {
	config := map[string]any{
		"sandbox_mode":    "read-only",
		"approval_policy": "on-request",
		"tools":           map[string]any{"web_search": false},
		"mcp_servers":     map[string]any{},
	}
	if model != "" {
		config["model"] = model
	}
	confined, kept := confine(argv, checkout)
	if !kept {
		// Codex cannot keep the contract by itself, so a platform that cannot
		// keep it for Codex admits no Codex.
		return Runtime{}, unconfined(RuntimeCodex)
	}
	return Runtime{
		Name:  RuntimeCodex,
		Model: model,
		Argv:  confined,
		Env: []string{
			jsonEnv("CODEX_CONFIG", config),
			"INITIAL_AGENT_MODE=read-only",
			"NO_BROWSER=1",
		},
		ReadOnly: codexReadOnly,
		Install:  codexInstall,
	}, nil
}
