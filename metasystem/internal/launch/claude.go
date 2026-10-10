package launch

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

var errClaudeResultUnreadable = errors.New("result-unreadable")
var errClaudeResultError = errors.New("result-error")

type ClaudeHeadless struct {
	Binary, ProjectsRoot string
	Scanner              ProcessScanner
}

func (adapter ClaudeHeadless) Command(record Record, stateDir string) (Command, error) {
	// window 0 means no cap: the child keeps Claude Code's own window for its
	// model. Only a positive value is passed through.
	window := readInt64(record.AdapterData, "window")
	if window < 0 {
		return Command{}, fmt.Errorf(`AdapterData key "window" must not be negative`)
	}
	brief, err := os.ReadFile(readString(record.AdapterData, "brief"))
	if err != nil {
		return Command{}, err
	}
	brief = appendReadPacket(brief, record)
	model := readString(record.AdapterData, "model")
	if model == "" {
		return Command{}, fmt.Errorf("a claude %s lane needs a model; set launch.%s.model.claude", record.Kind, record.Kind)
	}
	args := []string{"-p", "--model", model}
	// The launch's recorded effort reaches the CLI; a record without one
	// keeps Claude Code's own default.
	if effort := readString(record.AdapterData, "effort"); effort != "" {
		args = append(args, "--effort", effort)
	}
	args = append(args, "--dangerously-skip-permissions", "--output-format", "json", "--name", record.Kind+"-"+record.Tag)
	if session := readString(record.AdapterData, "resumeSession"); session != "" {
		args = append(args, "--resume", session)
	} else if session := readString(record.AdapterData, "sessionID"); session != "" && record.Kind == LandingKind {
		// A landing session's id is fixed before it starts, so its
		// transcript is found even when it is cancelled before it answers.
		args = append(args, "--session-id", session)
	}
	var environment []string
	if window > 0 {
		environment = []string{"CLAUDE_CODE_AUTO_COMPACT_WINDOW=" + fmt.Sprint(window)}
	}
	return Command{
		Program: adapter.Binary, Directory: record.WorkingDirectory, Stdin: string(brief), Args: args,
		Environment: environment,
		StdoutPath:  filepath.Join(stateDir, "result.json"), LogPath: filepath.Join(stateDir, "stderr.log"),
	}, nil
}

func (adapter ClaudeHeadless) Measure(record Record, stateDir string) (Measurement, []Output, map[string]json.RawMessage, error) {
	measurement, outputs, pageErr := measurePage(record, stateDir)
	// The session's id is known before it runs when the launch fixed it
	// (a landing session); its result names it once the session ends.
	sessionID := readString(record.AdapterData, "sessionID")
	var patch map[string]json.RawMessage
	var result struct {
		SessionID string `json:"session_id"`
		IsError   *bool  `json:"is_error"`
		Turns     int    `json:"num_turns"`
		Result    string `json:"result"`
	}
	resultErr := errClaudeResultUnreadable
	if data, err := os.ReadFile(filepath.Join(stateDir, "result.json")); err == nil && json.Unmarshal(data, &result) == nil && result.IsError != nil {
		resultErr = nil
		measurement.Turns = result.Turns
		measurement.ResultLines, measurement.ResultWords, measurement.ResultTail = textMeasure(result.Result)
		patch = map[string]json.RawMessage{}
		setString(patch, "sessionID", result.SessionID)
		if record.Kind == "read" {
			measurement.Verdict = readVerdict(record)
		}
		if result.SessionID != "" {
			sessionID = result.SessionID
		}
		if *result.IsError {
			resultErr = errClaudeResultError
		}
	}
	// The usage is read from the transcript whenever the session is known,
	// also when the session failed or left no readable result: every ended
	// session is measured (K10).
	usageErr := fmt.Errorf("claude session of launch %s is not known, so its transcript can't be read", record.ID)
	if sessionID != "" {
		usageErr = adapter.measureTranscript(sessionID, &measurement, transcriptFiles)
		measurement.UsageRead = usageErr == nil
	}
	switch {
	case resultErr != nil:
		return measurement, outputs, patch, resultErr
	case pageErr != nil:
		return measurement, outputs, patch, pageErr
	case result.SessionID == "":
		return measurement, outputs, patch, errClaudeResultUnreadable
	case usageErr != nil:
		return measurement, outputs, patch, usageErr
	}
	return measurement, outputs, patch, nil
}

// TranscriptUsage reads an ended session's usage from its transcript alone,
// by the session id its launch recorded: the reconciliation of a launch
// whose own measure was skipped.
func (adapter ClaudeHeadless) TranscriptUsage(record Record) (Measurement, error) {
	sessionID := readString(record.AdapterData, "sessionID")
	if sessionID == "" {
		return Measurement{}, fmt.Errorf("launch %s recorded no claude session, so its transcript can't be found", record.ID)
	}
	var measurement Measurement
	if err := adapter.measureTranscript(sessionID, &measurement, transcriptFiles); err != nil {
		return Measurement{}, err
	}
	measurement.UsageRead = true
	return measurement, nil
}

func (ClaudeHeadless) Outcome(exitCode int, measureErr error) (State, string) {
	switch {
	case errors.Is(measureErr, errClaudeResultUnreadable):
		return Failed, "result-unreadable"
	case errors.Is(measureErr, errClaudeResultError):
		return Failed, "result-error"
	case exitCode != 0:
		return Failed, fmt.Sprintf("exit-%d", exitCode)
	default:
		return Completed, ""
	}
}

func (ClaudeHeadless) StopCause(_ Record, stateDir string) string {
	if _, _, ok := outage.ClassifyLogs(filepath.Join(stateDir, "stderr.log")); ok {
		return outage.ProviderLimit
	}
	if _, _, ok := outage.ClassifyProviderResult(filepath.Join(stateDir, "result.json")); ok {
		return outage.ProviderLimit
	}
	return ""
}

func measurePage(record Record, stateDir string) (Measurement, []Output, error) {
	var measurement Measurement
	page := readString(record.AdapterData, "page")
	if page == "" {
		return measurement, nil, nil
	}
	if !filepath.IsAbs(page) {
		page = filepath.Join(record.WorkingDirectory, page)
	}
	data, err := os.ReadFile(page)
	if os.IsNotExist(err) {
		measurement.PageMissing = true
		return measurement, nil, nil
	}
	if err != nil {
		return measurement, nil, err
	}
	measurement.PageLines = strings.Count(string(data), "\n")
	measurement.PageWords = len(strings.Fields(string(data)))
	target := filepath.Join(stateDir, "page", filepath.Base(page))
	if _, err := atomicfile.CopyFile(page, target, stateDir); err != nil {
		return measurement, nil, err
	}
	return measurement, []Output{{Path: target, Bytes: int64(len(data))}}, nil
}

func textMeasure(text string) (int, int, string) {
	lines := 0
	if text != "" {
		lines = strings.Count(text, "\n")
		if !strings.HasSuffix(text, "\n") {
			lines++
		}
	}
	trimmed := strings.TrimSuffix(text, "\n")
	parts := strings.Split(trimmed, "\n")
	if trimmed == "" {
		parts = nil
	}
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return lines, len(strings.Fields(text)), strings.Join(parts, " | ")
}

func (adapter ClaudeHeadless) measureTranscript(sessionID string, measurement *Measurement, files func(string) ([]string, error)) error {
	all, err := files(adapter.ProjectsRoot)
	var paths []string
	for _, path := range all {
		if filepath.Base(path) == sessionID+".jsonl" {
			paths = append(paths, path)
		}
	}
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("claude transcript for session %s was not found", sessionID)
	}
	seen := map[string]bool{}
	for _, path := range paths {
		if err := measureClaudeTranscript(path, measurement, seen); err != nil {
			return err
		}
	}
	return nil
}

func measureClaudeTranscript(path string, measurement *Measurement, seen map[string]bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 128*1024*1024)
	for scanner.Scan() {
		var row struct {
			Type, Subtype string
			Timestamp     string          `json:"timestamp"`
			Message       json.RawMessage `json:"message"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			return err
		}
		if row.Type == "system" && row.Subtype == "compact_boundary" {
			measurement.Compactions++
		}
		if row.Type != "assistant" {
			continue
		}
		var message struct {
			ID      string `json:"id"`
			Content []struct {
				Type string `json:"type"`
			} `json:"content"`
			Usage json.RawMessage `json:"usage"`
		}
		if err := json.Unmarshal(row.Message, &message); err != nil {
			return err
		}
		for _, block := range message.Content {
			if block.Type == "tool_use" {
				measurement.ToolCalls++
			}
		}
		if message.ID == "" || len(message.Usage) == 0 || string(message.Usage) == "null" || seen[message.ID] {
			continue
		}
		var usage struct {
			Input         int64 `json:"input_tokens"`
			CacheRead     int64 `json:"cache_read_input_tokens"`
			CacheCreation int64 `json:"cache_creation_input_tokens"`
			Output        int64 `json:"output_tokens"`
		}
		if err := json.Unmarshal(message.Usage, &usage); err != nil {
			return err
		}
		seen[message.ID] = true
		call := Measurement{Calls: 1, InputTokens: usage.Input, CacheReadTokens: usage.CacheRead, CacheCreationTokens: usage.CacheCreation, OutputTokens: usage.Output, PeakContext: usage.Input + usage.CacheRead + usage.CacheCreation}
		if !measurement.observeCall(call, row.Timestamp) {
			continue
		}
		addCall(measurement, call)
		context := usage.Input + usage.CacheRead + usage.CacheCreation
		if context > 200000 {
			measurement.CallsAbove200++
		}
	}
	return scanner.Err()
}

func readVerdict(record Record) string {
	var paths []string
	_ = json.Unmarshal(record.AdapterData["declaredOutputs"], &paths)
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(record.WorkingDirectory, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "VERDICT:") {
				return strings.TrimSpace(line)
			}
		}
	}
	return "none"
}

func (adapter ClaudeHeadless) Strays() ([]string, error) {
	return adapter.StraysFor(nil)
}

func (adapter ClaudeHeadless) StraysFor(records []Record) ([]string, error) {
	processes, err := adapter.Scanner.Scan()
	if err != nil {
		return nil, err
	}
	owned := map[identity.Ref]bool{}
	for _, record := range records {
		if record.State == Running && record.Adapter == "claude-headless" && record.Child != nil {
			owned[*record.Child] = true
		}
	}
	var lines []string
	for _, process := range processes {
		if owned[process.Ref] || len(process.Argv) == 0 || filepath.Base(process.Argv[0]) != "claude" || !hasArg(process.Argv, "-p") {
			continue
		}
		name := flagValue(process.Argv, "--name")
		if !strings.HasPrefix(name, "design-") && !strings.HasPrefix(name, "read-") {
			continue
		}
		lines = append(lines, fmt.Sprintf("stray-claude-headless pid=%d name=%s", process.Ref.Pid, name))
	}
	return lines, nil
}

func hasArg(args []string, wanted string) bool {
	for _, arg := range args {
		if arg == wanted {
			return true
		}
	}
	return false
}
