package agentgate

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// HookTimeoutSeconds is the landing gate hook's timeout. The runtime lets a
// call through when it kills a hook at its timeout, so the bound is generous
// against a decision that takes milliseconds.
const HookTimeoutSeconds = 30

// failClosed is what the landing gate hook answers when the engine cannot:
// exit status 2 is the runtime's blocking answer whatever stdout holds, so a
// missing, crashed or half-written engine still denies the call.
const failClosed = `{ printf '%s\n' 'the landing tool gate could not run, so this call is denied' 'run: metasystem landing status' >&2; exit 2; }`

// HookCommand is the landing agent's own PreToolUse command: enter the lane
// checkout, run the lane engine's hook entry, and deny when either fails.
// The engine and checkout are absolute paths spelled literally, so no PATH
// lookup or Git discovery stands between the runtime and the engine; the
// shell's own errors are dropped so the denial reads as its two lines.
func HookCommand(engine, checkout string) (string, error) {
	if err := literalPath(engine); err != nil {
		return "", err
	}
	if err := literalPath(checkout); err != nil {
		return "", err
	}
	return `{ cd '` + checkout + `' && '` + engine + `' internal hook claude tool; } 2>/dev/null || ` + failClosed, nil
}

// literalPath refuses a path the command could not spell in single quotes.
func literalPath(path string) error {
	if !filepath.IsAbs(path) || strings.ContainsAny(path, "'\n\r") {
		return fmt.Errorf("the landing gate hook needs an absolute path without quotes or newlines, not %q", path)
	}
	return nil
}

// ClaudeSettings is the settings file the landing launch passes to Claude
// Code with --settings, beside the checkout's project settings: one
// PreToolUse handler for every tool. The runtime runs every matching handler
// and a denial from any of them holds.
func ClaudeSettings(engine, checkout string) ([]byte, error) {
	command, err := HookCommand(engine, checkout)
	if err != nil {
		return nil, err
	}
	settings := map[string]any{"hooks": map[string]any{"PreToolUse": []any{map[string]any{
		"hooks": []any{map[string]any{"type": "command", "command": command, "timeout": HookTimeoutSeconds}},
	}}}}
	encoded, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
