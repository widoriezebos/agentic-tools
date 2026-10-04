package adapter

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
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
	if err := wiredoc.WriteFile(outputPath, usage.CodexUsageValue(eventsPath)); err != nil {
		return fmt.Errorf("write codex usage: %w", err)
	}
	return nil
}

// BuildCodexCommand assembles the argv for a Codex delegate turn. A dispatch
// starts a fresh thread with an explicit sandbox mode and workspace directory; a
// follow-up resumes an existing thread, which has no --sandbox or -C flags, so
// the thread inherits its cwd and config and carries the supported per-turn
// overrides through -c settings instead. network is the bare TOML boolean the
// sandbox honors. Under danger-full-access neither the network override nor the
// extra write roots are passed: both configure workspace-write alone, and the
// unsandboxed turn already reaches the network and every path.
func BuildCodexCommand(verb, model, workspace, schema, output, sandbox, network, session, instanceTag, reasoningEffort string, extraDirs []string) ([]string, error) {
	if instanceTag == "" {
		return nil, fmt.Errorf("a codex delegate command requires an instance tag")
	}
	tagSetting := "metasystem_instance_tag=" + quoteTOML(instanceTag)
	fullAccess := sandbox == config.CodexSandboxFullAccess
	if fullAccess {
		extraDirs = nil
	}
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
		}
		if !fullAccess {
			command = append(command, "-c", "sandbox_workspace_write.network_access="+network)
		}
		command = append(command, "-c", tagSetting)
		if reasoningEffort != "" {
			command = append(command, "-c", "model_reasoning_effort="+quoteTOML(reasoningEffort))
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
		}
		if !fullAccess {
			command = append(command, "-c", "sandbox_workspace_write.network_access="+network)
		}
		command = append(command, "-c", tagSetting)
		if reasoningEffort != "" {
			command = append(command, "-c", "model_reasoning_effort="+quoteTOML(reasoningEffort))
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
// so it is decided here, not pre-chewed in shell. sandboxMode is the host's
// launch.codex.sandbox: danger-full-access replaces the mapping whatever the
// envelope says, a read-only envelope included; any other mode keeps it.
func CodexPermissionSettings(permissionsPath, recordPath, sandboxMode string) (sandbox, network string, err error) {
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
	if sandboxMode == config.CodexSandboxFullAccess {
		sandbox = config.CodexSandboxFullAccess
	}
	network = "false"
	if networkValue, _ := envelope["network"].(string); networkValue == "allow" {
		network = "true"
	}
	return sandbox, network, nil
}

// CodexAdmittedSandbox is the sandbox mode a delegate round was admitted
// under: danger-full-access when its record's requested envelope carries the
// launch.codex.sandbox widening, workspace-write otherwise (an unreadable
// record included, so the envelope's own mapping decides).
func CodexAdmittedSandbox(recordPath string) string {
	record, err := readObject(recordPath)
	if err != nil {
		return config.CodexSandboxWorkspaceWrite
	}
	permissions, _ := record["permissions"].(map[string]any)
	requested, _ := permissions["requested"].(map[string]any)
	if widenedBy, _ := requested["widenedBy"].(string); widenedBy == config.CodexSandboxWidening {
		return config.CodexSandboxFullAccess
	}
	return config.CodexSandboxWorkspaceWrite
}

// CodexExtraWriteRoots lists the envelope's write roots that fall OUTSIDE
// the workspace — the worktree git metadata a commit needs (issue #5) —
// for explicit sandbox grants. Roots inside the workspace are already
// covered by workspace-write; an empty workspace grants nothing extra.
func CodexExtraWriteRoots(permissionsPath, recordPath, workspace string, cacheDirs []string) ([]string, error) {
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
	for _, root := range append(stringList(envelope["writeRoots"]), cacheDirs...) {
		if (workspace == "" || !strings.HasPrefix(root+"/", workspace+"/")) && !slices.Contains(outside, root) {
			outside = append(outside, root)
		}
	}
	return outside, nil
}
