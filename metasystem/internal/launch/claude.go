package launch

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const DefaultClaudeAutoCompactWindow = "1000000"

var errClaudeResultUnreadable = errors.New("result-unreadable")
var errClaudeResultError = errors.New("result-error")

type ClaudeHeadless struct {
	Binary, ProjectsRoot string
	Scanner              ProcessScanner
}

func (adapter ClaudeHeadless) Command(record Record, stateDir string) (Command, error) {
	brief, err := os.ReadFile(readString(record.AdapterData, "brief"))
	if err != nil {
		return Command{}, err
	}
	model := readString(record.AdapterData, "model")
	if model == "" {
		switch record.Kind {
		case "design":
			model = "claude-fable-5-1"
		case "read":
			model = "claude-opus-5"
		}
	}
	args := []string{"-p", "--model", model, "--dangerously-skip-permissions", "--output-format", "json", "--name", record.Kind + "-" + record.Tag}
	if session := readString(record.AdapterData, "resumeSession"); session != "" {
		args = append(args, "--resume", session)
	}
	return Command{
		Program: adapter.Binary, Directory: record.WorkingDirectory, Stdin: string(brief), Args: args,
		Environment: []string{"CLAUDE_CODE_AUTO_COMPACT_WINDOW=" + DefaultClaudeAutoCompactWindow},
		StdoutPath:  filepath.Join(stateDir, "result.json"), LogPath: filepath.Join(stateDir, "stderr.log"),
	}, nil
}

func (adapter ClaudeHeadless) Measure(record Record, stateDir string) (Measurement, []Output, map[string]json.RawMessage, error) {
	measurement, outputs, pageErr := measurePage(record, stateDir)
	resultPath := filepath.Join(stateDir, "result.json")
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return measurement, outputs, nil, errClaudeResultUnreadable
	}
	var result struct {
		SessionID string `json:"session_id"`
		IsError   *bool  `json:"is_error"`
		Turns     int    `json:"num_turns"`
		Result    string `json:"result"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return measurement, outputs, nil, errClaudeResultUnreadable
	}
	if result.IsError == nil {
		return measurement, outputs, nil, errClaudeResultUnreadable
	}
	measurement.Turns = result.Turns
	measurement.ResultLines, measurement.ResultWords, measurement.ResultTail = textMeasure(result.Result)
	patch := map[string]json.RawMessage{}
	setString(patch, "sessionID", result.SessionID)
	if record.Kind == "read" {
		measurement.Verdict = readVerdict(record)
	}
	if *result.IsError {
		return measurement, outputs, patch, errClaudeResultError
	}
	if pageErr != nil {
		return measurement, outputs, patch, pageErr
	}
	if result.SessionID == "" {
		return measurement, outputs, patch, errClaudeResultUnreadable
	}
	if err := adapter.measureTranscript(result.SessionID, &measurement); err != nil {
		return measurement, outputs, patch, err
	}
	return measurement, outputs, patch, nil
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

func (adapter ClaudeHeadless) measureTranscript(sessionID string, measurement *Measurement) error {
	var paths []string
	err := filepath.WalkDir(adapter.ProjectsRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == sessionID+".jsonl" {
			paths = append(paths, path)
		}
		return nil
	})
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
			ID    string          `json:"id"`
			Usage json.RawMessage `json:"usage"`
		}
		if err := json.Unmarshal(row.Message, &message); err != nil {
			return err
		}
		if message.ID == "" || len(message.Usage) == 0 || string(message.Usage) == "null" || seen[message.ID] {
			continue
		}
		var usage struct {
			Input         int64 `json:"input_tokens"`
			CacheRead     int64 `json:"cache_read_input_tokens"`
			CacheCreation int64 `json:"cache_creation_input_tokens"`
		}
		if err := json.Unmarshal(message.Usage, &usage); err != nil {
			return err
		}
		seen[message.ID] = true
		measurement.Calls++
		context := usage.Input + usage.CacheRead + usage.CacheCreation
		if context > measurement.PeakContext {
			measurement.PeakContext = context
		}
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
