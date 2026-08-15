package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// BuildDevinConfig writes a Devin delegate's job config and its provenance. The
// CLI's --config replaces the user configuration rather than layering onto it,
// so the job config is the user's own file with only the permissions block
// swapped for the job's workspace-scoped grant: the organisation id and
// onboarding marker survive (a config missing them makes the CLI print a
// welcome banner into the turn's stdout), any sandbox declaration is dropped,
// and replacing the permissions member is the only safe direction because
// merging could only widen what the job may attempt. The provenance records
// which members were replaced and which were inherited unchanged.
func BuildDevinConfig(recordPath, outputPath, provenancePath string) error {
	requested, err := requestedPermissions(recordPath)
	if err != nil {
		return err
	}
	readRoots := stringList(requested["readRoots"])
	writeRoots := stringList(requested["writeRoots"])

	allow := []any{"read", "grep", "glob", "exec"}
	for _, root := range readRoots {
		allow = append(allow, fmt.Sprintf("Read(%s/**)", root))
	}
	deny := []any{"Fetch(*)", "mcp__*"}
	if len(writeRoots) > 0 {
		allow = append(allow, "edit")
		for _, root := range writeRoots {
			allow = append(allow, fmt.Sprintf("Write(%s/**)", root))
		}
	} else {
		deny = append(deny, "edit", "Write(**)")
	}

	userPath := userDevinConfigPath()
	value := loadObjectOrEmpty(userPath)

	var replaced []string
	for _, key := range []string{"permissions", "sandbox"} {
		if _, ok := value[key]; ok {
			replaced = append(replaced, key)
		}
	}
	sort.Strings(replaced)
	delete(value, "sandbox")
	value["permissions"] = map[string]any{"allow": allow, "ask": []any{}, "deny": deny}

	var inherited []string
	for key := range value {
		if key != "permissions" {
			inherited = append(inherited, key)
		}
	}
	sort.Strings(inherited)

	if err := atomicWriteJSON(outputPath, value); err != nil {
		return fmt.Errorf("write devin config: %w", err)
	}
	provenance := map[string]any{
		"userConfig":       userPath,
		"replacedMembers":  stringSlice(replaced),
		"inheritedMembers": stringSlice(inherited),
	}
	if err := atomicWriteJSON(provenancePath, provenance); err != nil {
		return fmt.Errorf("write devin config provenance: %w", err)
	}
	return nil
}

// userDevinConfigPath is the caller's Devin config location, honoring
// XDG_CONFIG_HOME and defaulting to ~/.config.
func userDevinConfigPath() string {
	home := os.Getenv("XDG_CONFIG_HOME")
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(userHome, ".config")
		}
	}
	return filepath.Join(home, "devin", "config.json")
}

// DevinSessionCorrelate identifies this turn's Devin session. A hook signal, if
// one carries a session id, is authoritative. Otherwise the session is the one
// that appears in the current listing, is absent from the pre-launch baseline,
// and runs in this workspace. Exactly one such candidate is this turn's
// session; none means keep waiting; more than one is ambiguous and returned for
// the caller to refuse rather than guess a peer's session. The candidate slice
// is set only on the ambiguous case.
func DevinSessionCorrelate(beforePath, currentPath, signalPath, workspace string) (string, []string) {
	if object, ok := loadAny(signalPath).(map[string]any); ok {
		if sid := firstString(object, "session_id", "sessionId", "id"); sid != "" {
			return sid, nil
		}
	}

	before := map[string]bool{}
	for _, record := range sessionRecords(loadAny(beforePath)) {
		if sid := recordSessionID(record); sid != "" {
			before[sid] = true
		}
	}

	var workspaceResolved string
	if workspace != "" {
		workspaceResolved = resolve(workspace)
	}

	seen := map[string]bool{}
	var candidates []string
	for _, record := range sessionRecords(loadAny(currentPath)) {
		sid := recordSessionID(record)
		if sid == "" || before[sid] || seen[sid] {
			continue
		}
		if !recordInWorkspace(record, workspaceResolved) {
			continue
		}
		seen[sid] = true
		candidates = append(candidates, sid)
	}
	sort.Strings(candidates)
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return "", candidates
}

// sessionRecords walks a session listing, however it is nested, and returns
// every object that carries a session-identifying key.
func sessionRecords(value any) []map[string]any {
	var out []map[string]any
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			if hasSessionKey(typed) {
				out = append(out, typed)
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(value)
	return out
}

// hasSessionKey reports whether a record names a session at all, even when that
// name is null or non-string.
func hasSessionKey(record map[string]any) bool {
	for _, key := range []string{"session_id", "sessionId", "id"} {
		if _, ok := record[key]; ok {
			return true
		}
	}
	return false
}

// recordSessionID returns a record's session identifier, the first of the
// spellings that holds a non-empty string.
func recordSessionID(record map[string]any) string {
	return firstString(record, "session_id", "sessionId", "id")
}

// recordInWorkspace reports whether a session record's working directory
// resolves to this turn's workspace. With no workspace to scope to, every
// record qualifies; a record without a usable directory never does.
func recordInWorkspace(record map[string]any, workspaceResolved string) bool {
	if workspaceResolved == "" {
		return true
	}
	directory := firstString(record, "working_directory", "workingDirectory")
	if directory == "" {
		return false
	}
	return resolve(directory) == workspaceResolved
}

// DevinPermissionMode decides the Devin CLI's --permission-mode. Every
// dispatch runs `dangerous` (auto-approve all tools) under the human
// waiver of 2026-08-15 (D61): Devin already runs uncontained by the
// human's earlier ruling, and the graded modes turned envelope
// refusals into sessions that ended without delivering — swe-1-7
// worked nine minutes and a confirmation-blocked tool call ate the
// return (D57). The record read stays: an unreadable or malformed
// record must still refuse the launch rather than default open.
func DevinPermissionMode(recordPath string) (string, error) {
	if _, err := requestedPermissions(recordPath); err != nil {
		return "", err
	}
	return "dangerous", nil
}

func WriteUnavailableUsage(outputPath string) error {
	value := map[string]any{
		"availability":      "unavailable",
		"inputTokens":       nil,
		"cachedInputTokens": nil,
		"outputTokens":      nil,
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     nil,
	}
	if err := atomicWriteJSON(outputPath, value); err != nil {
		return fmt.Errorf("write unavailable usage: %w", err)
	}
	return nil
}

// DevinSettle adjudicates the exported transcript against the correlated
// session and derives the effective model (review script-adapters-07,
// replacing three shell judgments). The transcript is authoritative: a
// correlated session it contradicts — or fails to name — is not certified,
// and the disagreement artifact says why. The model is the transcript's
// display name canonicalised; absent or unreadable records "unobserved",
// never the requested value the handshake seeded. requireTranscript is the
// repair shape: no transcript at all means session and model are
// unconfirmable, and no model is derived. Record writes stay with the
// caller (D24).
func DevinSettle(transcriptPath, correlatedSession, roundDir string, requireTranscript bool) (model string, certified bool, err error) {
	disagreement := filepath.Join(roundDir, "session-disagreement.txt")
	if requireTranscript {
		info, statErr := os.Stat(transcriptPath)
		if statErr != nil || info.Size() == 0 {
			err := os.WriteFile(disagreement,
				[]byte("repair produced no transcript; session and model are unconfirmable\n"), 0o644)
			return "", false, err
		}
	}
	transcript, readErr := readObject(transcriptPath)
	if readErr != nil {
		transcript = map[string]any{}
	}
	agent, _ := transcript["agent"].(map[string]any)
	display, _ := agent["model_name"].(string)
	model = config.CanonicalModel(display)
	if model == "" {
		model = "unobserved"
	}
	if correlatedSession == "" {
		// Nothing was correlated (no handshake) — nothing to settle.
		return model, true, nil
	}
	exported, _ := transcript["session_id"].(string)
	if exported == "" {
		err := os.WriteFile(disagreement,
			fmt.Appendf(nil, "correlated session %s but the transcript names no session\n", correlatedSession), 0o644)
		return model, false, err
	}
	if exported != correlatedSession {
		err := os.WriteFile(disagreement,
			fmt.Appendf(nil, "transcript session %s disagrees with correlated session %s\n", exported, correlatedSession), 0o644)
		return model, false, err
	}
	return model, true, nil
}

// DevinPrompt writes the schema-augmented prompt copy the Devin CLI reads
// (script-adapters-08/D28): this runtime has no schema flag, so the schema
// goes in the prompt or the model invents field names. The dispatcher's
// prompt file stays untouched as evidence. One writer replaces the two
// hand-maintained copies (adapter round turns and host turns) whose
// line-break placement had already drifted.
// returnFile, when non-empty, names a SECOND delivery channel the prompt
// instructs the model to use alongside printing: swe-1-7 finishes work by
// writing files rather than emitting a final message (D62's evidence: a
// schema-perfect return written to /tmp while stdout stayed empty, in
// both graded and dangerous permission modes), so the adapter names a
// deterministic path inside the round's evidence and reads it whenever
// stdout comes back empty.
func DevinPrompt(promptPath, schemaPath, outputPath, returnFile string) error {
	prompt, err := os.ReadFile(promptPath)
	if err != nil {
		return fmt.Errorf("cannot read the prompt: %w", err)
	}
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("cannot read the schema: %w", err)
	}
	var text strings.Builder
	text.Write(prompt)
	text.WriteString("\n\n# Return schema, exact\n\n")
	text.WriteString("Your reply must be ONE JSON object valid against this schema and nothing else:\n")
	text.WriteString("no prose before or after it, no code fence, and no property this schema\n")
	text.WriteString("does not name. Every property listed in \"required\" must be present.\n\n")
	text.Write(schema)
	if returnFile != "" {
		text.WriteString("\n\n# Delivery, exact\n\n")
		text.WriteString("Write that ONE JSON object to this exact file path, and also print it\n")
		text.WriteString("as your final message. Do not choose a different path:\n\n")
		text.WriteString(returnFile)
		text.WriteString("\n")
	}
	return os.WriteFile(outputPath, []byte(text.String()), 0o644)
}
