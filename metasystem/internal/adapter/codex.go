package adapter

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
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

// CodexUsage writes the typed usage for a Codex turn to its round artifact —
// the adapter's own capture, the one writer of usage.json.
func CodexUsage(eventsPath, outputPath string) error {
	if err := atomicWriteJSON(outputPath, usage.CodexUsageValue(eventsPath)); err != nil {
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
func BuildCodexCommand(verb, model, workspace, schema, output, sandbox, network, session string, extraDirs []string) ([]string, error) {
	// Write roots OUTSIDE the workspace — the worktree's git metadata a
	// commit needs (issue #5) — ride --add-dir on dispatch and the
	// equivalent writable-roots override on resume (which has no
	// positional flags). Without this the envelope's derived roots were
	// silently dropped and every worktree codex implementer's commit
	// died read-only.
	switch verb {
	case "dispatch":
		command := []string{
			"codex", "exec", "--json",
			"-m", model,
			"--sandbox", sandbox,
			"-C", workspace,
			"-c", `approval_policy="never"`,
			"-c", "sandbox_workspace_write.network_access=" + network,
		}
		for _, dir := range extraDirs {
			command = append(command, "--add-dir", dir)
		}
		return append(command,
			"--output-schema", schema,
			"-o", output,
			"-",
		), nil
	case "follow-up":
		if session == "" {
			return nil, fmt.Errorf("a codex follow-up requires a session to resume")
		}
		command := []string{
			"codex", "exec", "resume", "--json",
			"-c", "model=" + quoteTOML(model),
			"-c", "sandbox_mode=" + quoteTOML(sandbox),
			"-c", `approval_policy="never"`,
			"-c", "sandbox_workspace_write.network_access=" + network,
		}
		if len(extraDirs) > 0 {
			command = append(command, "-c", "sandbox_workspace_write.writable_roots="+tomlStringArray(extraDirs))
		}
		return append(command,
			"--output-schema", schema,
			"-o", output,
			session,
			"-",
		), nil
	default:
		return nil, fmt.Errorf("unknown codex verb %q", verb)
	}
}

// tomlStringArray renders a TOML array of basic strings for a -c override.
func tomlStringArray(items []string) string {
	quoted := make([]string, 0, len(items))
	for _, item := range items {
		quoted = append(quoted, quoteTOML(item))
	}
	return "[" + strings.Join(quoted, ",") + "]"
}

// quoteTOML renders a string as a TOML basic string, which a -c override that
// carries a model or sandbox mode needs. A JSON string literal is a valid TOML
// basic string.
func quoteTOML(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// CodexPermissionSettings derives the sandbox/network pair from a
// permission envelope: an empty writeRoots
// means read-only, anything else workspace-write; network "allow" means
// true. recordPath reads the record's requested envelope; otherwise
// permissionsPath is the envelope JSON itself. The envelope-to-flag
// mapping is the security-relevant half of command construction,
// so it is decided here, not pre-chewed in shell.
func CodexPermissionSettings(permissionsPath, recordPath string) (sandbox, network string, err error) {
	var envelope map[string]any
	if recordPath != "" {
		record, err := readObject(recordPath)
		if err != nil {
			return "", "", err
		}
		permissions, _ := record["permissions"].(map[string]any)
		envelope, _ = permissions["requested"].(map[string]any)
	} else {
		value, err := readObject(permissionsPath)
		if err != nil {
			return "", "", err
		}
		envelope = value
	}
	sandbox = "workspace-write"
	if len(stringList(envelope["writeRoots"])) == 0 {
		sandbox = "read-only"
	}
	network = "false"
	if networkValue, _ := envelope["network"].(string); networkValue == "allow" {
		network = "true"
	}
	return sandbox, network, nil
}

// CodexExtraWriteRoots lists the envelope's write roots that fall OUTSIDE
// the workspace — the worktree git metadata a commit needs (issue #5) —
// for explicit sandbox grants. Roots inside the workspace are already
// covered by workspace-write; an empty workspace grants nothing extra.
func CodexExtraWriteRoots(permissionsPath, recordPath, workspace string) ([]string, error) {
	var envelope map[string]any
	if recordPath != "" {
		record, err := readObject(recordPath)
		if err != nil {
			return nil, err
		}
		permissions, _ := record["permissions"].(map[string]any)
		envelope, _ = permissions["requested"].(map[string]any)
	} else {
		value, err := readObject(permissionsPath)
		if err != nil {
			return nil, err
		}
		envelope = value
	}
	var outside []string
	for _, root := range stringList(envelope["writeRoots"]) {
		if workspace == "" || !strings.HasPrefix(root+"/", workspace+"/") {
			outside = append(outside, root)
		}
	}
	return outside, nil
}
