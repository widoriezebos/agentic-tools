package agentgate

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
// checkout, run the lane engine's hook entry as the landing lineage (so the
// gate governs even if the session's environment lost it), and deny when
// either fails.
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
	return `{ cd '` + checkout + `' && ` + LineageEnv + `=` + Lineage + ` '` + engine + `' internal hook claude tool; } 2>/dev/null || ` + failClosed, nil
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

// SettingsFileName is the name WriteClaudeSettings gives the file.
const SettingsFileName = "landing-tool-gate.json"

// WriteClaudeSettings is what the landing launch calls before it starts a
// landing session: it writes ClaudeSettings(engine, checkout) as
// stateDir/landing-tool-gate.json (mode 0600, replaced atomically) and
// returns that path, which the launch records under AdapterData "settings"
// so Claude runs with --settings PATH. stateDir is the launch's own state
// directory: absolute and outside the lane checkout, which the agent may
// edit. engine is the lane's absolute bin/metasystem and checkout the lane
// checkout's absolute Git toplevel.
func WriteClaudeSettings(stateDir, engine, checkout string) (string, error) {
	data, err := ClaudeSettings(engine, checkout)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(stateDir) {
		return "", fmt.Errorf("the landing gate settings need an absolute state directory, not %q", stateDir)
	}
	resolvedState, err := resolveExisting(filepath.Clean(stateDir))
	if err != nil {
		return "", err
	}
	resolvedCheckout, err := resolveExisting(filepath.Clean(checkout))
	if err != nil {
		return "", err
	}
	if relative, err := filepath.Rel(resolvedCheckout, resolvedState); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("the landing gate settings must live outside the lane checkout %s, not in %s", checkout, stateDir)
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(stateDir, SettingsFileName+".*")
	if err != nil {
		return "", err
	}
	name := temporary.Name()
	_, writeErr := temporary.Write(data)
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, closeErr, os.Chmod(name, 0o600)); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	path := filepath.Join(stateDir, SettingsFileName)
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return path, nil
}
