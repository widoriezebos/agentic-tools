package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var errDevinResultUnreadable = errors.New("result-unreadable")

// maxDevinTranscriptBytes bounds the export a launch reads back. The export
// is written by the launch's own child and holds the whole session, so the
// ceiling sits far above a long build; crossing it fails the measurement
// loudly instead of truncating it.
const maxDevinTranscriptBytes = 256 << 20

// DevinPrint runs one launch lane on the Devin CLI in print mode. Devin names
// the reasoning effort inside the model id and has no context-window
// control, so the lane's effort becomes a model suffix and a positive window
// is refused rather than recorded as enforced.
type DevinPrint struct {
	Binary, StateRoot string
	Scanner           ProcessScanner
}

const (
	devinPromptFile     = "prompt.md"
	devinTranscriptFile = "transcript.json"
)

// devinModel is the Devin model id for a lane's model and effort: the effort
// is appended unless the model already ends in it, so one model and effort
// pair means the same thing on every runtime.
func devinModel(model, effort string) string {
	if effort == "" || strings.HasSuffix(model, "-"+effort) {
		return model
	}
	return model + "-" + effort
}

func (adapter DevinPrint) Command(record Record, stateDir string) (Command, error) {
	if err := refuseUngatedLanding(record, "devin-print"); err != nil {
		return Command{}, err
	}
	if window := readInt64(record.AdapterData, "window"); window != 0 {
		return Command{}, fmt.Errorf("devin has no context-window cap to enforce window=%d; set launch.%s.window.tokens=0", window, windowLane(record.Kind))
	}
	model := readString(record.AdapterData, "model")
	if model == "" {
		return Command{}, fmt.Errorf("a devin %s lane needs a model; set launch.%s.model.devin", record.Kind, record.Kind)
	}
	brief, err := os.ReadFile(readString(record.AdapterData, "brief"))
	if err != nil {
		return Command{}, err
	}
	prompt := filepath.Join(stateDir, devinPromptFile)
	if _, err := atomicfile.WriteText(prompt, string(appendReadPacket(brief, record)), stateDir); err != nil {
		return Command{}, err
	}
	args := []string{"-p", "--prompt-file", prompt, "--respect-workspace-trust", "false",
		"--model", devinModel(model, readString(record.AdapterData, "effort")),
		"--permission-mode", "dangerous", "--export", filepath.Join(stateDir, devinTranscriptFile)}
	if session := readString(record.AdapterData, "resumeSession"); session != "" {
		args = append(args, "-r", session)
	}
	return Command{Program: adapter.Binary, Directory: record.WorkingDirectory, Args: args,
		StdoutPath: filepath.Join(stateDir, "result.txt"), LogPath: filepath.Join(stateDir, "stderr.log")}, nil
}

// windowLane names the settings key family of a kind's window: the critique
// lane shares the build lane's window.
func windowLane(kind string) string {
	if kind == "critique" {
		return "build"
	}
	return kind
}

type devinTranscript struct {
	SessionID string `json:"session_id"`
	Steps     []struct {
		Source    string            `json:"source"`
		ModelName string            `json:"model_name"`
		ToolCalls []json.RawMessage `json:"tool_calls"`
		Metrics   *struct {
			Prompt     int64 `json:"prompt_tokens"`
			Completion int64 `json:"completion_tokens"`
			Cached     int64 `json:"cached_tokens"`
			Extra      struct {
				CacheCreation int64 `json:"cache_creation_input_tokens"`
			} `json:"extra"`
		} `json:"metrics"`
		Extra struct {
			Telemetry struct {
				Source    string `json:"source"`
				Operation string `json:"operation"`
			} `json:"telemetry"`
		} `json:"extra"`
	} `json:"steps"`
}

func readDevinTranscript(path string) (devinTranscript, error) {
	var transcript devinTranscript
	file, err := os.Open(path)
	if err != nil {
		return transcript, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxDevinTranscriptBytes+1))
	if err != nil {
		return transcript, err
	}
	if len(data) > maxDevinTranscriptBytes {
		return transcript, fmt.Errorf("devin transcript exceeds %d bytes", maxDevinTranscriptBytes)
	}
	return transcript, json.Unmarshal(data, &transcript)
}

func devinCompaction(source, operation string) bool {
	for _, value := range []string{source, operation} {
		value = strings.ToLower(value)
		if strings.Contains(value, "compact") || strings.Contains(value, "summar") {
			return true
		}
	}
	return false
}

func (adapter DevinPrint) Measure(record Record, stateDir string) (Measurement, []Output, map[string]json.RawMessage, error) {
	measurement, outputs, pageErr := measurePage(record, stateDir)
	result, _ := os.ReadFile(filepath.Join(stateDir, "result.txt"))
	measurement.ResultLines, measurement.ResultWords, measurement.ResultTail = textMeasure(string(result))
	if record.Kind == "read" {
		measurement.Verdict = readVerdict(record)
	}
	transcript, err := readDevinTranscript(filepath.Join(stateDir, devinTranscriptFile))
	if err != nil || transcript.SessionID == "" {
		return measurement, outputs, nil, errDevinResultUnreadable
	}
	patch := map[string]json.RawMessage{}
	setString(patch, "sessionID", transcript.SessionID)
	observed := ""
	for _, step := range transcript.Steps {
		if devinCompaction(step.Extra.Telemetry.Source, step.Extra.Telemetry.Operation) {
			measurement.Compactions++
		}
		if step.Source != "agent" {
			continue
		}
		measurement.Turns++
		measurement.ToolCalls += len(step.ToolCalls)
		if step.ModelName != "" {
			observed = step.ModelName
		}
		if step.Metrics == nil {
			continue
		}
		metrics := step.Metrics
		measurement.Calls++
		measurement.InputTokens += max(0, metrics.Prompt-metrics.Cached-metrics.Extra.CacheCreation)
		measurement.CacheReadTokens += metrics.Cached
		measurement.CacheCreationTokens += metrics.Extra.CacheCreation
		measurement.OutputTokens += metrics.Completion
		measurement.PeakContext = max(measurement.PeakContext, metrics.Prompt)
		if metrics.Prompt > 200000 {
			measurement.CallsAbove200++
		}
	}
	if observed != "" {
		setString(patch, "observedModel", observed)
	}
	if measurement.Turns == 0 {
		return measurement, outputs, patch, errDevinResultUnreadable
	}
	return measurement, outputs, patch, pageErr
}

func (DevinPrint) Outcome(exitCode int, measureErr error) (State, string) {
	switch {
	case errors.Is(measureErr, errDevinResultUnreadable):
		return Failed, "result-unreadable"
	case exitCode != 0:
		return Failed, fmt.Sprintf("exit-%d", exitCode)
	default:
		return Completed, ""
	}
}

func (adapter DevinPrint) Strays() ([]string, error) {
	return adapter.StraysFor(nil)
}

// StraysFor reports print-mode Devin processes exporting into a launch state
// directory that no running record owns. They are reported, never signalled.
func (adapter DevinPrint) StraysFor(records []Record) ([]string, error) {
	if adapter.Scanner == nil || adapter.StateRoot == "" {
		return nil, nil
	}
	processes, err := adapter.Scanner.Scan()
	if err != nil {
		return nil, err
	}
	owned := map[identity.Ref]bool{}
	for _, record := range records {
		if record.State == Running && record.Adapter == "devin-print" && record.Child != nil {
			owned[*record.Child] = true
		}
	}
	root := filepath.Clean(adapter.StateRoot)
	var lines []string
	for _, process := range processes {
		if owned[process.Ref] || len(process.Argv) == 0 || filepath.Base(process.Argv[0]) != "devin" || !hasArg(process.Argv, "-p") {
			continue
		}
		export := flagValue(process.Argv, "--export")
		if export == "" || filepath.Base(export) != devinTranscriptFile || filepath.Dir(filepath.Dir(export)) != root {
			continue
		}
		lines = append(lines, fmt.Sprintf("stray-devin-print pid=%d id=%s", process.Ref.Pid, filepath.Base(filepath.Dir(export))))
	}
	return lines, nil
}
