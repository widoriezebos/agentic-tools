package adapter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BuildClaudeSettings writes the settings file a Claude delegate launches
// under. The tool allow/deny lists and OS sandbox are derived from the job's
// requested permissions, and the SessionStart hook signals session
// establishment back to the adapter by running the metasystem session-signal
// verb. metasystemBin is the binary that command invokes. scratch is the
// delegate's private writable directory; an empty value grants no scratch.
func BuildClaudeSettings(recordPath, outputPath, metasystemBin, scratch string) error {
	requested, err := requestedPermissions(recordPath)
	if err != nil {
		return err
	}
	record, err := readObject(recordPath)
	if err != nil {
		return err
	}
	writeRoots, _ := requested["writeRoots"].([]any)
	if writeRoots == nil {
		writeRoots = []any{}
	}
	allowWrite := append([]any{}, writeRoots...)
	if scratch != "" {
		allowWrite = append(allowWrite, scratch)
	}
	network, _ := requested["network"].(string)
	if network == "" {
		network = "deny"
	}

	allow := []any{"Read", "Glob", "Grep"}
	deny := []any{}
	if network == "allow" {
		allow = append(allow, "WebFetch", "WebSearch")
	} else {
		deny = append(deny, "WebFetch", "WebSearch")
	}
	if len(writeRoots) > 0 {
		allow = append(allow, "Bash", "Edit", "Write", "NotebookEdit")
	} else {
		if isClaudeCriticRole(record) {
			allow = append(allow, "Bash")
		} else {
			deny = append(deny, "Bash")
		}
		deny = append(deny, "Edit", "Write", "NotebookEdit")
	}

	// An empty allowlist with an empty denylist permits ordinary egress; the
	// non-resolving sentinel makes every usable destination unavailable.
	networkSandbox := map[string]any{"allowedDomains": []any{"metasystem.invalid"}, "deniedDomains": []any{}}
	if network == "allow" {
		networkSandbox = map[string]any{"allowedDomains": []any{}, "deniedDomains": []any{}}
	}

	filesystemSandbox := map[string]any{"allowWrite": allowWrite}
	readRoots, err := ClaudeReadRoots(recordPath)
	if err != nil {
		return err
	}
	resolvedWriteRoots := []string{}
	for _, root := range stringList(requested["writeRoots"]) {
		if root != "" {
			resolvedWriteRoots = append(resolvedWriteRoots, resolve(root))
		}
	}
	workspace, _ := record["workspaceRoot"].(string)
	denyWrite := []any{}
	seen := map[string]bool{}
	// Claude's sandbox makes the working directory and every --add-dir root
	// writable, and a denied ancestor overrides a nested allow. Ancestors of a
	// write root therefore stay open while each existing sibling entry is
	// denied. A delegate can still create new entries directly in worktree
	// ancestors such as the repository root, metasystem/, artifacts/, agents/,
	// and worktrees/, because those directories must remain writable for the
	// nested write root; every existing entry outside its path is denied.
	for _, root := range append([]string{workspace}, readRoots...) {
		if root == "" {
			continue
		}
		root = resolve(root)
		if seen[root] {
			continue
		}
		seen[root] = true
		denied, err := claudeDenyWritePaths(root, resolvedWriteRoots)
		if err != nil {
			return err
		}
		for _, path := range denied {
			denyWrite = append(denyWrite, path)
		}
	}
	if len(denyWrite) > 0 {
		filesystemSandbox["denyWrite"] = denyWrite
	}

	settings := map[string]any{
		"permissions": map[string]any{"allow": allow, "ask": []any{}, "deny": deny},
		"sandbox": map[string]any{
			"enabled":                  true,
			"failIfUnavailable":        true,
			"autoAllowBashIfSandboxed": true,
			"allowUnsandboxedCommands": false,
			"filesystem":               filesystemSandbox,
			"network":                  networkSandbox,
		},
		"hooks": map[string]any{
			"SessionStart": []any{map[string]any{
				"matcher": "startup|resume",
				"hooks": []any{map[string]any{
					"type":    "command",
					"command": shellQuote(metasystemBin) + " adapter claude-session-signal",
					"timeout": 5,
				}},
			}},
		},
	}
	if err := atomicWriteJSON(outputPath, settings); err != nil {
		return fmt.Errorf("write claude settings: %w", err)
	}
	return nil
}

func claudeDenyWritePaths(root string, writeRoots []string) ([]string, error) {
	for _, writeRoot := range writeRoots {
		if pathContains(writeRoot, root) {
			return nil, nil
		}
	}

	containsWriteRoot := false
	for _, writeRoot := range writeRoots {
		if pathContains(root, writeRoot) {
			containsWriteRoot = true
			break
		}
	}
	if !containsWriteRoot {
		return []string{root}, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read Claude write-root ancestor %s: %w", root, err)
	}
	var denied []string
	for _, entry := range entries {
		child := filepath.Join(root, entry.Name())
		childDenied, err := claudeDenyWritePaths(child, writeRoots)
		if err != nil {
			return nil, err
		}
		denied = append(denied, childDenied...)
	}
	return denied, nil
}

// ClaudeUsage extracts the native token counts and cost a Claude result
// document reports into the typed usage the mission fence meters. An absent or
// malformed result records the shape with null counts rather than failing.
func ClaudeUsage(resultPath, outputPath string) error {
	document, _ := readObject(resultPath)
	usage, _ := document["usage"].(map[string]any)
	value := map[string]any{
		"availability":      "native",
		"inputTokens":       usage["input_tokens"],
		"cachedInputTokens": usage["cache_read_input_tokens"],
		"outputTokens":      usage["output_tokens"],
		"reasoningTokens":   usage["reasoning_tokens"],
		"cost":              nil,
		"providerUnits":     nil,
	}
	if cost := document["total_cost_usd"]; isNumber(cost) {
		value["cost"] = map[string]any{"amount": cost, "currency": "USD"}
	}
	if err := atomicWriteJSON(outputPath, value); err != nil {
		return fmt.Errorf("write claude usage: %w", err)
	}
	return nil
}

// ClaudeResultField reads a field from a Claude result document. The "model"
// field is derived from the modelUsage map rather than read directly: a single
// model collapses to its key, none to "unobserved", and several to a
// "multi-model:" list. Any other field is read as-is; a present null yields
// nothing to print, an absent field an empty string. An unreadable document is
// an error. The boolean reports whether the value should be printed.
func ClaudeResultField(resultPath, field string) (string, bool, error) {
	document, err := readObject(resultPath)
	if err != nil {
		return "", false, err
	}
	if field == "model" {
		models, _ := document["modelUsage"].(map[string]any)
		keys := sortedStringKeys(models)
		switch len(keys) {
		case 1:
			return keys[0], true, nil
		case 0:
			return "unobserved", true, nil
		default:
			return "multi-model:" + strings.Join(keys, ","), true, nil
		}
	}
	raw, present := document[field]
	if !present {
		return "", true, nil
	}
	if raw == nil {
		return "", false, nil
	}
	return scalarString(raw), true, nil
}

// ClaudeReadRoots lists the requested read roots other than the workspace root:
// the extra directories a read-only Claude turn is granted with --add-dir.
func ClaudeReadRoots(recordPath string) ([]string, error) {
	record, err := readObject(recordPath)
	if err != nil {
		return nil, err
	}
	permissions, _ := record["permissions"].(map[string]any)
	requested, _ := permissions["requested"].(map[string]any)
	workspace, _ := record["workspaceRoot"].(string)
	var extra []string
	for _, root := range stringList(requested["readRoots"]) {
		if root != workspace {
			extra = append(extra, root)
		}
	}
	return extra, nil
}

// ClaudeAppendResult appends a Claude result document to the flight-recorder
// events file as one compact line. An absent or malformed result is a no-op, so
// a failed turn leaves the events file as it stands.
func ClaudeAppendResult(resultPath, eventsPath string) error {
	value, err := decodeJSON(resultPath)
	if err != nil {
		return nil
	}
	if err := appendJSONLine(eventsPath, value); err != nil {
		return fmt.Errorf("append claude result event: %w", err)
	}
	return nil
}

// ClaudeSessionSignal turns a Claude SessionStart hook payload, read from r,
// into the adapter's session-establishment signal: it writes the signal file
// the supervisor polls and appends a session-init event to the flight recorder,
// then returns the established session id for the hook to echo as runtime
// context. A payload without a usable session id is refused.
func ClaudeSessionSignal(r io.Reader, signalPath, eventsPath string) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	value, err := decodeJSONBytes(data)
	if err != nil {
		return "", fmt.Errorf("SessionStart payload is not JSON: %w", err)
	}
	payload, _ := value.(map[string]any)
	sessionID, _ := payload["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("SessionStart payload has no session_id")
	}
	signal := map[string]any{
		"session_id": sessionID,
		"model":      payload["model"],
		"source":     payload["source"],
	}
	if err := atomicWriteJSON(signalPath, signal); err != nil {
		return "", fmt.Errorf("write session signal: %w", err)
	}
	event := map[string]any{
		"type":       "system",
		"subtype":    "init",
		"session_id": sessionID,
		"source":     "SessionStart-hook",
	}
	if err := appendJSONLine(eventsPath, event); err != nil {
		return "", fmt.Errorf("append session-init event: %w", err)
	}
	return sessionID, nil
}

// The Claude argv, permission-mode/tool-list mapping, and native budget
// policy: one home, so the adapter and host copies cannot fork. The
// codex pattern:
// one builder, NUL-separated tokens on the wire, both shells read it back.

// claudeFullTools is the read-write tool list; the critic and read-only lists
// are the envelope's narrowings of it.
const claudeFullTools = "Bash,Edit,Write,Read,Glob,Grep,NotebookEdit"
const claudeCriticTools = "Bash,Read,Glob,Grep"
const claudeReadOnlyTools = "Read,Glob,Grep"

func isClaudeCriticRole(record map[string]any) bool {
	role, _ := record["role"].(string)
	return role == "code-critic" || role == "design-critic" || role == "warden"
}

// ClaudeBudget validates the native budget and turn-limit policy from the
// environment. The --max-budget-usd flag is omitted unless the operator sets
// METASYSTEM_CLAUDE_MAX_BUDGET_USD. The worker is bounded by its wall-clock cap
// and its turn limit, with the turn limit serving as the surviving in-process
// coarse dollar bound. A set budget is validated, and a malformed value is the
// invalid_native_budget protocol error. The turn limit defaults to 150, and a
// malformed value is the invalid_native_turn_limit protocol error.
func ClaudeBudget(lookupEnv func(string) (string, bool)) (budget, turns string, err error) {
	if value, ok := lookupEnv("METASYSTEM_CLAUDE_MAX_BUDGET_USD"); ok {
		budget = value
		if !regexp.MustCompile(`^[0-9]+([.][0-9]+)?$`).MatchString(budget) || budget == "0" || budget == "0.0" {
			return "", "", fmt.Errorf("invalid_native_budget")
		}
	}
	turns = "150"
	if value, ok := lookupEnv("METASYSTEM_CLAUDE_MAX_TURNS"); ok {
		turns = value
	}
	if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(turns) {
		return "", "", fmt.Errorf("invalid_native_turn_limit")
	}
	return budget, turns, nil
}

// BuildClaudeCommand assembles the claude -p argv. Adapter mode (recordPath
// non-empty) derives the permission envelope: an empty requested writeRoots
// means dontAsk with either the critic or read-only tools plus --add-dir for
// every extra read root; anything else means acceptEdits with the full tools.
// Host mode (recordPath empty) is the orchestrator's own turn: acceptEdits
// with the full tools, no settings file, no add-dirs.
// ClaudeOutputModes are the two argv output shapes: "json" (the
// blocking single document — the host launcher's mode, unchanged)
// and "stream-json" (the dispatch stream; the CLI refuses print-mode
// stream-json without --verbose, so the flag rides along).
func BuildClaudeCommand(recordPath, model, schemaJSON, settings, session, budget, turns, outputMode string) ([]string, error) {
	permissionMode := "acceptEdits"
	tools := claudeFullTools
	var addDirs []string
	instanceTag := ""
	if recordPath != "" {
		record, err := readObject(recordPath)
		if err != nil {
			return nil, err
		}
		instanceTag, _ = record["instanceTag"].(string)
		if instanceTag == "" {
			return nil, fmt.Errorf("a Claude delegate command requires an instance tag")
		}
		permissions, _ := record["permissions"].(map[string]any)
		requested, _ := permissions["requested"].(map[string]any)
		writeRoots := stringList(requested["writeRoots"])
		if len(writeRoots) == 0 {
			permissionMode = "dontAsk"
			// A critic can use Bash safely for evidence: the settings deny every
			// write to the workspace and read roots, the only writable place is the
			// private scratch directory, and the role packet still forbids editing.
			if isClaudeCriticRole(record) {
				tools = claudeCriticTools
			} else {
				tools = claudeReadOnlyTools
			}
			if addDirs, err = ClaudeReadRoots(recordPath); err != nil {
				return nil, err
			}
		}
	}
	switch outputMode {
	case "", "json":
		outputMode = "json"
	case "stream-json":
	default:
		return nil, fmt.Errorf("claude output mode %q is not one of json, stream-json", outputMode)
	}
	command := []string{
		"claude", "-p", "--output-format", outputMode, "--model", model,
		"--json-schema", schemaJSON,
		"--permission-mode", permissionMode,
		"--tools", tools,
		"--allowedTools", tools,
	}
	if settings != "" {
		command = append(command, "--settings", settings)
	}
	if instanceTag != "" {
		command = append(command, "--name", instanceTag)
	}
	if budget != "" {
		command = append(command, "--max-budget-usd", budget)
	}
	command = append(command, "--max-turns", turns)
	for _, dir := range addDirs {
		command = append(command, "--add-dir", dir)
	}
	if session != "" {
		command = append(command, "--resume", session)
	}
	if outputMode == "stream-json" {
		command = append(command, "--verbose")
	}
	return command, nil
}

// ClaudeDeriveResult writes claude-result.json from a streamed
// round: the LAST result-typed line of the stream artifact, raw
// bytes verbatim — every existing consumer reads exactly the
// document the blocking mode would have produced. A stream without
// a result-typed line is the missing-result failure: an error, and
// no document is invented from partial events.
func ClaudeDeriveResult(streamPath, resultPath string) error {
	data, err := os.ReadFile(streamPath)
	if err != nil {
		return err
	}
	var last []byte
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), MaxEventArtifactBytes+1)
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			continue
		}
		if head.Type == "result" {
			last = append(last[:0], raw...)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(last) == 0 {
		return fmt.Errorf("stream %s carries no result-typed line; the turn has no result document", filepath.Base(streamPath))
	}
	return os.WriteFile(resultPath, append(last, '\n'), 0o644)
}
