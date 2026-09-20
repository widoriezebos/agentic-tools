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
	"syscall"
	"time"
)

type Process struct {
	Ref  identity.Ref
	Argv []string
}
type ProcessScanner interface{ Scan() ([]Process, error) }
type ProcessSignaler interface {
	Signal(int64, syscall.Signal) error
}
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
	Location                                            *time.Location
	Scanner                                             ProcessScanner
}

func (adapter CodexExec) Command(record Record, stateDir string) (Command, error) {
	// window 0 means no cap: the child keeps Codex's own auto-compact limit for
	// its model. Only a positive value is passed through.
	window := readInt64(record.AdapterData, "window")
	if window < 0 {
		return Command{}, fmt.Errorf(`AdapterData key "window" must not be negative`)
	}
	brief := readString(record.AdapterData, "brief")
	data, err := os.ReadFile(brief)
	if err != nil {
		return Command{}, err
	}
	if diff := readString(record.AdapterData, "readDiff"); diff != "" {
		data = append(data, []byte("\nDiff: "+diff+"\n")...)
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
	args := []string{"exec", "-m", model, "-c", "model_reasoning_effort=" + effort, "-C", directory,
		"-s", "workspace-write", "-o", filepath.Join(stateDir, "last-message.txt")}
	if window > 0 {
		args = append(args, "-c", fmt.Sprintf("model_auto_compact_token_limit=%d", window))
	}
	args = append(args, "-")
	return Command{Program: adapter.Binary, Directory: directory, Stdin: string(data),
		Args: args, LogPath: filepath.Join(stateDir, "exec.log")}, nil
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
	directories, err := adapter.rolloutDirectories(record)
	if err != nil {
		return measurement, outputs, nil, err
	}
	var files []string
	for _, directory := range directories {
		pattern := filepath.Join(directory, "rollout-*"+sessionID+".jsonl")
		matches, globErr := filepath.Glob(pattern)
		if globErr != nil {
			return measurement, outputs, nil, fmt.Errorf("rollout for session %s: search %s: %w", sessionID, pattern, globErr)
		}
		files = append(files, matches...)
	}
	if len(files) != 1 {
		return measurement, outputs, nil, fmt.Errorf("rollout for session %s: found %d", sessionID, len(files))
	}
	if err := measureRollout(files[0], &measurement); err != nil {
		return measurement, outputs, nil, err
	}
	if record.Kind == "read" {
		measurement.Verdict = readVerdict(record)
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

func (adapter CodexExec) rolloutDirectories(record Record) ([]string, error) {
	now := adapter.Now()
	started := now
	if record.StartedAt != "" {
		var err error
		started, err = time.Parse(time.RFC3339Nano, record.StartedAt)
		if err != nil {
			return nil, fmt.Errorf("launch start time %q: %w", record.StartedAt, err)
		}
	}
	location := adapter.Location
	if location == nil {
		location = time.Local
	}
	seen := map[string]bool{}
	var directories []string
	add := func(day time.Time) {
		directory := filepath.Join(adapter.SessionsRoot, day.Format("2006"), day.Format("01"), day.Format("02"))
		if !seen[directory] {
			seen[directory] = true
			directories = append(directories, directory)
		}
	}
	localStart := started.In(location)
	localEnd := now.In(location)
	day := time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, location)
	end := time.Date(localEnd.Year(), localEnd.Month(), localEnd.Day(), 0, 0, 0, 0, location)
	for !day.After(end) {
		add(day)
		day = day.AddDate(0, 0, 1)
	}
	add(started.UTC())
	add(now.UTC())
	return directories, nil
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

func (adapter CodexExec) ReapIdleBrokers(prober identity.Prober, signaler ProcessSignaler) ([]string, error) {
	processes, err := adapter.Scanner.Scan()
	if err != nil {
		return nil, err
	}
	companions := map[string]bool{}
	for _, process := range processes {
		if hasScript(process.Argv, "codex-companion.mjs") {
			companions[flagValue(process.Argv, "--cwd")] = true
		}
	}
	var lines []string
	for _, process := range processes {
		if !hasScriptCommand(process.Argv, "app-server-broker.mjs", "serve") {
			continue
		}
		cwd := flagValue(process.Argv, "--cwd")
		if companions[cwd] {
			continue
		}
		if prober == nil || identity.AliveRef(prober, process.Ref) != identity.Alive {
			lines = append(lines, fmt.Sprintf("skipped idle-plugin-broker pid=%d: identity changed", process.Ref.Pid))
			continue
		}
		if signaler == nil {
			return lines, fmt.Errorf("signal idle-plugin-broker pid %d: process signaler is unavailable", process.Ref.Pid)
		}
		if err := signaler.Signal(process.Ref.Pid, syscall.SIGTERM); err != nil {
			return lines, fmt.Errorf("signal idle-plugin-broker pid %d: %w", process.Ref.Pid, err)
		}
		lines = append(lines, fmt.Sprintf("reaped idle-plugin-broker pid=%d cwd=%s", process.Ref.Pid, cwd))
	}
	return lines, nil
}

func hasScript(argv []string, script string) bool {
	for _, value := range argv {
		if strings.HasSuffix(value, "/"+script) || value == script {
			return true
		}
	}
	return false
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
