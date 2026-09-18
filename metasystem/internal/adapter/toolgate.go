package adapter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
)

// Call is the part of a tool hook payload needed for classification.
type Call struct {
	Tool  string          `json:"tool_name"`
	Input json.RawMessage `json:"tool_input"`
}

// ClassificationKind names the policy class of a tool call.
type ClassificationKind string

const (
	NeverDenied      ClassificationKind = "never-denied"
	AllowedAtTrigger ClassificationKind = "allowed-at-trigger"
	MemoryNote       ClassificationKind = "memory-note"
	MemoryUnresolved ClassificationKind = "memory-unresolved"
	Other            ClassificationKind = "other"
)

type toolGateMatch uint8

const (
	matchTool toolGateMatch = iota
	matchToolPrefix
	matchCommand
	matchLandingScript
	matchMemoryPath
)

type toolGateRow struct {
	pattern string
	kind    ClassificationKind
	match   toolGateMatch
	words   []string
	trigger bool
	ceiling bool
}

// toolGateRows is the complete allow policy. Grammar code only identifies a
// row; the row's two decision columns determine whether the call may proceed.
var toolGateRows = []toolGateRow{
	{pattern: "Agent", kind: NeverDenied, match: matchTool, words: []string{"Agent"}, trigger: true, ceiling: true},
	{pattern: "SendMessage", kind: NeverDenied, match: matchTool, words: []string{"SendMessage"}, trigger: true, ceiling: true},
	{pattern: "Monitor", kind: NeverDenied, match: matchTool, words: []string{"Monitor"}, trigger: true, ceiling: true},
	{pattern: "TaskStop", kind: NeverDenied, match: matchTool, words: []string{"TaskStop"}, trigger: true, ceiling: true},
	{pattern: "Task*", kind: NeverDenied, match: matchToolPrefix, words: []string{"Task"}, trigger: true, ceiling: true},
	{pattern: "metasystem landing <verb>", kind: NeverDenied, match: matchCommand, words: []string{"metasystem", "landing", "<verb>"}, trigger: true, ceiling: true},
	{pattern: "metasystem goal land-ready", kind: NeverDenied, match: matchCommand, words: []string{"metasystem", "goal", "land-ready"}, trigger: true, ceiling: true},
	{pattern: "metasystem wait", kind: NeverDenied, match: matchCommand, words: []string{"metasystem", "wait"}, trigger: true, ceiling: true},
	{pattern: "metasystem job watch", kind: NeverDenied, match: matchCommand, words: []string{"metasystem", "job", "watch"}, trigger: true, ceiling: true},
	{pattern: "scripts/agents/land.sh", kind: NeverDenied, match: matchLandingScript, trigger: true, ceiling: true},
	{pattern: "metasystem context handoff", kind: AllowedAtTrigger, match: matchCommand, words: []string{"metasystem", "context", "handoff"}, trigger: true, ceiling: true},
	{pattern: "metasystem context resume", kind: AllowedAtTrigger, match: matchCommand, words: []string{"metasystem", "context", "resume"}, trigger: true, ceiling: true},
	{pattern: "metasystem context status", kind: AllowedAtTrigger, match: matchCommand, words: []string{"metasystem", "context", "status"}, trigger: true, ceiling: true},
	{pattern: "metasystem context verify", kind: AllowedAtTrigger, match: matchCommand, words: []string{"metasystem", "context", "verify"}, trigger: true, ceiling: true},
	{pattern: "metasystem delegate", kind: AllowedAtTrigger, match: matchCommand, words: []string{"metasystem", "delegate"}, trigger: true, ceiling: true},
	{pattern: "metasystem steward revive", kind: AllowedAtTrigger, match: matchCommand, words: []string{"metasystem", "steward", "revive"}, trigger: true, ceiling: true},
	{pattern: "metasystem steward status", kind: AllowedAtTrigger, match: matchCommand, words: []string{"metasystem", "steward", "status"}, trigger: true, ceiling: true},
	{pattern: "Write under memoryDir", kind: MemoryNote, match: matchMemoryPath, words: []string{"Write"}, trigger: true, ceiling: true},
	{pattern: "Edit under memoryDir", kind: MemoryNote, match: matchMemoryPath, words: []string{"Edit"}, trigger: true, ceiling: true},
}

// Classification records the policy row and normalized Bash command that
// explain a call's class. Row is nil when no table pattern matched.
type Classification struct {
	Kind    ClassificationKind
	Row     *toolGateRow
	Command string
}

// Decision is the complete allow or deny result for one classified call.
type Decision struct {
	Deny   bool
	Cause  string
	Reason string
}

// Classify identifies a policy row without reading token state or files.
func Classify(call Call, memoryDir string) Classification {
	for index := range toolGateRows {
		row := &toolGateRows[index]
		switch row.match {
		case matchTool:
			if call.Tool == row.words[0] {
				return Classification{Kind: row.kind, Row: row}
			}
		case matchToolPrefix:
			if strings.HasPrefix(call.Tool, row.words[0]) {
				return Classification{Kind: row.kind, Row: row}
			}
		case matchMemoryPath:
			if call.Tool != row.words[0] {
				continue
			}
			if memoryDir == "" {
				return Classification{Kind: MemoryUnresolved, Row: row}
			}
			var input struct {
				FilePath string `json:"file_path"`
			}
			if json.Unmarshal(call.Input, &input) == nil && pathUnder(memoryDir, input.FilePath) {
				return Classification{Kind: row.kind, Row: row}
			}
		}
	}
	if call.Tool != "Bash" {
		return Classification{Kind: Other}
	}
	var input struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(call.Input, &input) != nil || input.Command == "" {
		return Classification{Kind: Other}
	}
	return classifyBash(input.Command)
}

// Decide applies the matched row at the configured context thresholds.
func Decide(class Classification, tokens int64, budget config.Budget, installation string) Decision {
	if tokens < budget.Trigger {
		return Decision{Cause: "under-trigger"}
	}
	if class.Kind == MemoryUnresolved {
		return Decision{Cause: "memory-unresolved"}
	}
	if class.Row != nil {
		allowed := class.Row.trigger
		if tokens >= budget.Ceiling {
			allowed = class.Row.ceiling
		}
		if allowed {
			cause := "allowlisted"
			if class.Kind == MemoryNote {
				cause = "memory-note"
			}
			return Decision{Cause: cause}
		}
	}
	reason := fmt.Sprintf(
		"CONTEXT AT %dK (trigger %dK): this call is denied; run metasystem context handoff --root %s alone, or launch a delegate",
		tokens/1000, budget.Trigger/1000, installation,
	)
	return Decision{Deny: true, Cause: string(class.Kind), Reason: reason}
}

// Output returns the hook response. An allow is represented by silence so the
// runtime's ordinary permission flow remains in control.
func (d Decision) Output() []byte {
	if !d.Deny {
		return nil
	}
	payload := struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}{}
	payload.HookSpecificOutput.HookEventName = "PreToolUse"
	payload.HookSpecificOutput.PermissionDecision = "deny"
	payload.HookSpecificOutput.PermissionDecisionReason = d.Reason
	encoded, _ := json.Marshal(payload)
	return encoded
}

func pathUnder(directory, path string) bool {
	if directory == "" || path == "" {
		return false
	}
	directory = filepath.Clean(directory)
	path = filepath.Clean(path)
	prefix := directory
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(path, prefix)
}

type shellCommand struct {
	words []string
}

func classifyBash(command string) Classification {
	commands, err := parseShellCommands(command)
	if err != nil || len(commands) == 0 {
		return Classification{Kind: Other}
	}
	var matched Classification
	for _, command := range commands {
		row := commandRow(command.words)
		if row != nil {
			if matched.Row != nil {
				return Classification{Kind: Other}
			}
			matched = Classification{Kind: row.kind, Row: row, Command: strings.Join(command.words, " ")}
			continue
		}
		if matched.Row == nil || !isOutputFilter(command.words) {
			return Classification{Kind: Other}
		}
	}
	if matched.Row == nil {
		return Classification{Kind: Other}
	}
	return matched
}

func commandRow(words []string) *toolGateRow {
	for index := range toolGateRows {
		row := &toolGateRows[index]
		switch row.match {
		case matchCommand:
			if commandPrefixMatches(words, row.words) {
				return row
			}
		case matchLandingScript:
			if len(words) > 0 && landingScriptCommand(words[0]) {
				return row
			}
		}
	}
	return nil
}

func commandPrefixMatches(command, pattern []string) bool {
	if len(command) < len(pattern) || len(command) == 0 || filepath.Base(command[0]) != pattern[0] {
		return false
	}
	for index := 1; index < len(pattern); index++ {
		if pattern[index] != "<verb>" && command[index] != pattern[index] {
			return false
		}
	}
	return true
}

func landingScriptCommand(word string) bool {
	cleaned := filepath.ToSlash(filepath.Clean(word))
	return cleaned == "scripts/agents/land.sh" || strings.HasSuffix(cleaned, "/scripts/agents/land.sh")
}

func isOutputFilter(words []string) bool {
	if len(words) == 0 {
		return false
	}
	switch filepath.Base(words[0]) {
	case "head", "tail", "grep", "tee", "wc", "cut", "sed":
		return true
	default:
		return false
	}
}

func parseShellCommands(command string) ([]shellCommand, error) {
	simple, err := splitSimpleCommands(command)
	if err != nil {
		return nil, err
	}
	if len(simple) > 0 {
		words, splitErr := shellWords(simple[0])
		if splitErr != nil {
			return nil, splitErr
		}
		if len(words) == 2 && words[0] == "cd" {
			simple = simple[1:]
		}
	}
	var commands []shellCommand
	for _, raw := range simple {
		normalized, normalizeErr := normalizeShellCommand(raw)
		if normalizeErr != nil {
			return nil, normalizeErr
		}
		commands = append(commands, normalized...)
	}
	return commands, nil
}

func normalizeShellCommand(raw string) ([]shellCommand, error) {
	words, err := shellWords(raw)
	if err != nil {
		return nil, err
	}
	for len(words) > 0 && shellAssignment(words[0]) {
		words = words[1:]
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("simple command has no command word")
	}
	command := filepath.Base(words[0])
	if command != "bash" && command != "sh" {
		return []shellCommand{{words: words}}, nil
	}
	if len(words) < 2 {
		return nil, fmt.Errorf("shell wrapper has no command")
	}
	if words[1] == "-c" || words[1] == "-lc" {
		if len(words) != 3 {
			return nil, fmt.Errorf("shell command wrapper has unexpected arguments")
		}
		return parseShellCommands(words[2])
	}
	if strings.HasPrefix(words[1], "-") {
		return nil, fmt.Errorf("unsupported shell wrapper option")
	}
	return []shellCommand{{words: words[1:]}}, nil
}

func shellAssignment(word string) bool {
	equals := strings.IndexByte(word, '=')
	if equals < 1 {
		return false
	}
	for index := 0; index < equals; index++ {
		character := word[index]
		if index == 0 {
			if character != '_' && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') {
				return false
			}
			continue
		}
		if character != '_' && (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func splitSimpleCommands(command string) ([]string, error) {
	var commands []string
	start := 0
	quote := byte(0)
	escaped := false
	flush := func(end int) error {
		part := strings.TrimSpace(command[start:end])
		if part == "" {
			return fmt.Errorf("empty simple command")
		}
		commands = append(commands, part)
		return nil
	}
	for index := 0; index < len(command); index++ {
		character := command[index]
		if escaped {
			escaped = false
			continue
		}
		if quote != '\'' && character == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			continue
		}
		separatorLength := 0
		switch character {
		case '\n', ';', '|':
			separatorLength = 1
			if character == '|' && index+1 < len(command) && command[index+1] == '|' {
				separatorLength = 2
			}
		case '&':
			if index+1 >= len(command) || command[index+1] != '&' {
				return nil, fmt.Errorf("unsupported background command")
			}
			separatorLength = 2
		}
		if separatorLength == 0 {
			continue
		}
		if err := flush(index); err != nil {
			return nil, err
		}
		index += separatorLength - 1
		start = index + 1
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("unterminated shell word")
	}
	if err := flush(len(command)); err != nil {
		return nil, err
	}
	return commands, nil
}

func shellWords(command string) ([]string, error) {
	var words []string
	var word strings.Builder
	quote := byte(0)
	escaped := false
	started := false
	finish := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	for index := 0; index < len(command); index++ {
		character := command[index]
		if escaped {
			word.WriteByte(character)
			started = true
			escaped = false
			continue
		}
		if quote != '\'' && character == '\\' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if character == quote {
				quote = 0
			} else {
				word.WriteByte(character)
			}
			started = true
			continue
		}
		if character == '\'' || character == '"' {
			quote = character
			started = true
			continue
		}
		if character == ' ' || character == '\t' || character == '\r' || character == '\n' {
			finish()
			continue
		}
		word.WriteByte(character)
		started = true
	}
	if escaped || quote != 0 {
		return nil, fmt.Errorf("unterminated shell word")
	}
	finish()
	return words, nil
}
