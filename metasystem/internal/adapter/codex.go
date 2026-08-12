package adapter

import (
	"encoding/json"
	"fmt"
)

// CodexEventField extracts a session or turn identifier from Codex's JSONL
// event stream: the first thread- or session-created event's id for "session",
// the first turn-started event's id for "turn". It reports whether a value was
// found so a caller can keep polling an in-progress stream rather than treating
// a not-yet-emitted id as an error.
func CodexEventField(eventsPath, field string) (string, bool) {
	for _, event := range jsonlObjects(eventsPath) {
		kind, _ := event["type"].(string)
		switch field {
		case "session":
			switch kind {
			case "thread.started", "thread.created", "session.created":
				if id := firstString(event, "thread_id", "session_id", "id"); id != "" {
					return id, true
				}
			}
		case "turn":
			switch kind {
			case "turn.started", "turn.created":
				if id := firstString(event, "turn_id", "id"); id != "" {
					return id, true
				}
			}
		}
	}
	return "", false
}

// CodexUsageValue derives the typed usage for a Codex turn in memory, from
// the last usage block its event stream reports. Codex spells the same
// counter more than one way across builds, so each field takes the first
// present spelling. Codex reports no cost or provider units, so both stay
// null. Callers that must never write — the mission aggregator recovering a
// killed round's spend from its dead event stream — read this value directly.
func CodexUsageValue(eventsPath string) map[string]any {
	var last map[string]any
	for _, event := range jsonlObjects(eventsPath) {
		if usage, ok := event["usage"].(map[string]any); ok {
			last = usage
		}
	}
	return map[string]any{
		"availability":      "native",
		"inputTokens":       firstPresent(last, "input_tokens", "inputTokens"),
		"cachedInputTokens": firstPresent(last, "cached_input_tokens", "cachedInputTokens"),
		"outputTokens":      firstPresent(last, "output_tokens", "outputTokens"),
		"reasoningTokens":   firstPresent(last, "reasoning_output_tokens", "reasoning_tokens", "reasoningTokens"),
		"cost":              nil,
		"providerUnits":     nil,
	}
}

// CodexUsage writes the typed usage for a Codex turn to its round artifact —
// the adapter's own capture, the one writer of usage.json.
func CodexUsage(eventsPath, outputPath string) error {
	if err := atomicWriteJSON(outputPath, CodexUsageValue(eventsPath)); err != nil {
		return fmt.Errorf("write codex usage: %w", err)
	}
	return nil
}

// BuildCodexCommand assembles the argv for a Codex delegate turn. A dispatch
// starts a fresh thread with an explicit sandbox mode and workspace directory; a
// follow-up resumes an existing thread, which has no --sandbox or -C flags, so
// the thread inherits its cwd and config and carries the supported per-turn
// overrides through -c settings instead. network is the bare TOML boolean the
// sandbox honors.
func BuildCodexCommand(verb, model, workspace, schema, output, sandbox, network, session string) ([]string, error) {
	switch verb {
	case "dispatch":
		return []string{
			"codex", "exec", "--json",
			"-m", model,
			"--sandbox", sandbox,
			"-C", workspace,
			"-c", `approval_policy="never"`,
			"-c", "sandbox_workspace_write.network_access=" + network,
			"--output-schema", schema,
			"-o", output,
			"-",
		}, nil
	case "follow-up":
		if session == "" {
			return nil, fmt.Errorf("a codex follow-up requires a session to resume")
		}
		return []string{
			"codex", "exec", "resume", "--json",
			"-c", "model=" + quoteTOML(model),
			"-c", "sandbox_mode=" + quoteTOML(sandbox),
			"-c", `approval_policy="never"`,
			"-c", "sandbox_workspace_write.network_access=" + network,
			"--output-schema", schema,
			"-o", output,
			session,
			"-",
		}, nil
	default:
		return nil, fmt.Errorf("unknown codex verb %q", verb)
	}
}

// quoteTOML renders a string as a TOML basic string, which a -c override that
// carries a model or sandbox mode needs. A JSON string literal is a valid TOML
// basic string.
func quoteTOML(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
