package launch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Process struct {
	Ref  identity.Ref
	Argv []string
}
type ProcessScanner interface{ Scan() ([]Process, error) }
type KernelProcessScanner struct{ Prober identity.Prober }

func (scanner KernelProcessScanner) Scan() ([]Process, error) {
	pids, err := identity.AllPids()
	if err != nil {
		return nil, err
	}
	var processes []Process
	for _, pid := range pids {
		exact, state, _ := scanner.Prober.Probe(pid)
		if state == identity.Alive && exact.ArgvKnown {
			processes = append(processes, Process{Ref: exact.Ref(), Argv: exact.Argv})
		}
	}
	return processes, nil
}

type CodexExec struct {
	Binary, Model, Effort, SessionsRoot, CommonTemplate string
	Now                                                 func() time.Time
	Scanner                                             ProcessScanner
}

func (adapter CodexExec) Command(record Record, stateDir string) (Command, error) {
	brief := readString(record.AdapterData, "brief")
	data, err := os.ReadFile(brief)
	if err != nil {
		return Command{}, err
	}
	directory := record.WorkingDirectory
	if record.Kind == "critique" {
		directory, err = adapter.prepareCritique(record)
		if err != nil {
			return Command{}, err
		}
	}
	model, effort := adapter.Model, adapter.Effort
	if value := readString(record.AdapterData, "model"); value != "" {
		model = value
	}
	if value := readString(record.AdapterData, "effort"); value != "" {
		effort = value
	}
	window := int64(200000)
	if value := readInt64(record.AdapterData, "window"); value > 0 {
		window = value
	}
	return Command{Program: adapter.Binary, Directory: directory, Stdin: string(data),
		Args: []string{"exec", "-m", model, "-c", "model_reasoning_effort=" + effort, "-C", directory,
			"-s", "workspace-write", "-o", filepath.Join(stateDir, "last-message.txt"), "-"},
		Environment: []string{fmt.Sprintf("CODEX_CONTEXT_WINDOW=%d", window)}, LogPath: filepath.Join(stateDir, "exec.log")}, nil
}
func (adapter CodexExec) prepareCritique(record Record) (string, error) {
	if record.Tag == "" || len(record.Inputs) < 3 {
		return "", fmt.Errorf("critique requires --tag and two --input files")
	}
	metasystem := filepath.Join(record.WorkingDirectory, "metasystem")
	if info, err := os.Stat(metasystem); err != nil || !info.IsDir() {
		return "", fmt.Errorf("critique worktree has no metasystem directory: %s", metasystem)
	}
	common := adapter.CommonTemplate
	if common == "" {
		common = filepath.Join("scripts", "agents", "templates", "design-common.md")
	}
	copies := [][2]string{
		{record.Inputs[1].Path, filepath.Join(metasystem, "plans", filepath.Base(record.Inputs[1].Path))},
		{record.Inputs[2].Path, filepath.Join(metasystem, "artifacts", "reports", record.Tag+"-design-brief-r1.md")},
		{common, filepath.Join(metasystem, "artifacts", "reports", "design-common.md")},
	}
	for _, pair := range copies {
		if _, err := atomicfile.CopyFile(pair[0], pair[1], record.WorkingDirectory); err != nil {
			return "", err
		}
	}
	return metasystem, nil
}

var sessionLine = regexp.MustCompile(`(?im)^session id:\s*([a-z0-9-]+)\s*$`)
var materialLine = regexp.MustCompile(`(?i)material:?\s*yes`)
var verdictLine = regexp.MustCompile(`(?i)^VERDICT\b`)

func (adapter CodexExec) Measure(record Record, stateDir string) (Measurement, []Output, map[string]json.RawMessage, error) {
	measurement := Measurement{}
	var outputs []Output
	var pageErr error
	if record.Kind == "design" {
		measurement, outputs, pageErr = measurePage(record, stateDir)
	}
	last, _ := os.ReadFile(filepath.Join(stateDir, "last-message.txt"))
	measurement.ResultWords = len(strings.Fields(string(last)))
	if len(last) > 0 {
		measurement.ResultLines = strings.Count(string(last), "\n")
		if last[len(last)-1] != '\n' {
			measurement.ResultLines++
		}
	}
	log, err := os.ReadFile(filepath.Join(stateDir, "exec.log"))
	if err != nil {
		return measurement, outputs, nil, err
	}
	match := sessionLine.FindSubmatch(log)
	if len(match) != 2 {
		return measurement, outputs, nil, fmt.Errorf("codex exec log has no session id")
	}
	sessionID := string(match[1])
	day := adapter.Now().UTC()
	pattern := filepath.Join(adapter.SessionsRoot, day.Format("2006"), day.Format("01"), day.Format("02"), "rollout-*"+sessionID+".jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil || len(files) != 1 {
		return measurement, outputs, nil, fmt.Errorf("rollout for session %s: found %d: %w", sessionID, len(files), err)
	}
	if err := measureRollout(files[0], &measurement); err != nil {
		return measurement, outputs, nil, err
	}
	if pageErr != nil {
		return measurement, outputs, nil, pageErr
	}
	if record.Kind == "critique" {
		output, err := copyCritique(record, stateDir, &measurement)
		if err != nil {
			return measurement, nil, nil, err
		}
		outputs = append(outputs, output)
	}
	patch := map[string]json.RawMessage{}
	setString(patch, "sessionID", sessionID)
	return measurement, outputs, patch, nil
}
func measureRollout(path string, measurement *Measurement) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 128*1024*1024)
	for scanner.Scan() {
		var event struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
				Info struct {
					Last struct {
						InputTokens int64 `json:"input_tokens"`
					} `json:"last_token_usage"`
					Total struct {
						InputTokens           int64 `json:"input_tokens"`
						CachedInputTokens     int64 `json:"cached_input_tokens"`
						CacheWriteInputTokens int64 `json:"cache_write_input_tokens"`
						OutputTokens          int64 `json:"output_tokens"`
					} `json:"total_token_usage"`
				} `json:"info"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return err
		}
		if event.Type == "compacted" {
			measurement.Compactions++
		}
		if event.Type == "response_item" && (event.Payload.Type == "custom_tool_call" || event.Payload.Type == "function_call") {
			measurement.ToolCalls++
		}
		if event.Type == "event_msg" && event.Payload.Type == "token_count" {
			measurement.Calls++
			measurement.InputTokens = event.Payload.Info.Total.InputTokens
			measurement.CacheReadTokens = event.Payload.Info.Total.CachedInputTokens
			measurement.CacheCreationTokens = event.Payload.Info.Total.CacheWriteInputTokens
			measurement.OutputTokens = event.Payload.Info.Total.OutputTokens
			context := event.Payload.Info.Last.InputTokens
			if context > measurement.PeakContext {
				measurement.PeakContext = context
			}
			if context > 200000 {
				measurement.CallsAbove200++
			}
		}
	}
	return scanner.Err()
}
func copyCritique(record Record, stateDir string, measurement *Measurement) (Output, error) {
	source := filepath.Join(record.WorkingDirectory, "metasystem", "artifacts", "reports", record.Tag+"-critique-r1.md")
	target := filepath.Join(stateDir, "outputs", filepath.Base(source))
	if _, err := atomicfile.CopyFile(source, target, stateDir); err != nil {
		return Output{}, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return Output{}, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if materialLine.MatchString(line) {
			measurement.MaterialCount++
		}
		if verdictLine.MatchString(line) {
			measurement.Verdict = strings.TrimSpace(line)
		}
	}
	return Output{Path: target, Bytes: int64(len(data))}, nil
}
func (adapter CodexExec) Strays() ([]string, error) {
	processes, err := adapter.Scanner.Scan()
	if err != nil {
		return nil, err
	}
	jobs := map[string]bool{}
	for _, process := range processes {
		if hasScriptCommand(process.Argv, "codex-companion.mjs", "task-worker") {
			jobs[flagValue(process.Argv, "--cwd")] = true
		}
	}
	var lines []string
	for _, process := range processes {
		if !hasScriptCommand(process.Argv, "app-server-broker.mjs", "serve") {
			continue
		}
		cwd := flagValue(process.Argv, "--cwd")
		if !jobs[cwd] {
			lines = append(lines, fmt.Sprintf("idle-plugin-broker pid=%d cwd=%s", process.Ref.Pid, cwd))
		}
	}
	return lines, nil
}
func hasScriptCommand(argv []string, script, command string) bool {
	for index, value := range argv {
		if strings.HasSuffix(value, "/"+script) || value == script {
			return index+1 < len(argv) && argv[index+1] == command
		}
	}
	return false
}
func flagValue(argv []string, flag string) string {
	for index := range argv {
		if argv[index] == flag && index+1 < len(argv) {
			return argv[index+1]
		}
	}
	return ""
}
