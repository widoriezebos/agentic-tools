package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Task is one background task the transcript launched, in launch order.
type Task struct {
	ID        string `json:"id"`
	ToolUseID string `json:"toolUseId"`
	Kind      string `json:"kind"`
	Asked     string `json:"asked"`
	Output    string `json:"output"`
	InFlight  bool   `json:"inFlight"`
	Status    string `json:"status,omitempty"`
}

// Notice is one thing the walk observed that the caller prints.
type Notice struct {
	TaskID string `json:"taskId,omitempty"`
	Text   string `json:"text"`
}

type terminalStatusRow struct {
	status string
	cited  string
}

var terminalTaskStatus = []terminalStatusRow{
	{status: "completed", cited: "9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:220"},
	{status: "failed", cited: "c53cbef5-7be0-48fa-83f1-876f40a3c193.jsonl:839"},
	{status: "killed", cited: "2c27f5cc-245a-4735-a402-dafc351e4caf.jsonl:135"},
	{status: "stopped", cited: "9b4a48c5-e0ce-46bb-99b1-ca3644a08ec4.jsonl:28259"},
}

type taskLaunch struct {
	toolUseID string
	kind      string
	asked     string
	order     int
}

type taskState struct {
	task   Task
	events int
	seen   map[string]bool
	order  int
}

// InFlightTasks returns every launched task in launch order, with InFlight
// describing its state after the last event in the transcript.
func InFlightTasks(transcript string, opts ReadOptions) ([]Task, []Notice, error) {
	states, notices, err := walkTaskTranscript(transcript, opts)
	if err != nil || len(states) == 0 {
		return nil, notices, err
	}
	tasks := make([]Task, len(states))
	for index, state := range states {
		tasks[index] = state.task
	}
	return tasks, notices, nil
}

func walkTaskTranscript(transcript string, opts ReadOptions) ([]*taskState, []Notice, error) {
	if transcript == "" {
		transcript = opts.Transcript
	}
	if transcript == "" {
		return nil, nil, nil
	}
	stream, err := os.Open(transcript)
	observeCallOpen(transcript)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot open task transcript %s: %w", transcript, err)
	}
	defer stream.Close()

	walk := taskWalk{
		launches:     make(map[string]taskLaunch),
		sendMessages: make(map[string]bool),
		byToolUseID:  make(map[string]*taskState),
		byTaskID:     make(map[string]*taskState),
	}
	reader := bufio.NewReader(observedCallReader{reader: stream})
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > maxCallLineBytes {
			return nil, nil, fmt.Errorf("cannot scan task transcript %s: token too long", transcript)
		}
		line = bytes.TrimSuffix(line, []byte{'\n'})
		if len(line) > 0 {
			walk.record(decodeTaskLine(line))
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, nil, fmt.Errorf("cannot read task transcript %s: %w", transcript, readErr)
		}
	}
	sort.Slice(walk.states, func(i, j int) bool { return walk.states[i].order < walk.states[j].order })
	return walk.states, walk.notices, nil
}

type taskWalk struct {
	launches     map[string]taskLaunch
	sendMessages map[string]bool
	byToolUseID  map[string]*taskState
	byTaskID     map[string]*taskState
	states       []*taskState
	notices      []Notice
	nextLaunch   int
}

func decodeTaskLine(line []byte) map[string]any {
	if callJSONDecodes != nil {
		callJSONDecodes()
	}
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil
	}
	return raw
}

func (walk *taskWalk) record(raw map[string]any) {
	if raw == nil {
		return
	}
	message, _ := raw["message"].(map[string]any)
	items, _ := message["content"].([]any)
	if textValue(raw["type"]) == "assistant" {
		walk.rememberRequests(items)
	}
	if textValue(raw["type"]) == "user" {
		for _, item := range items {
			result, _ := item.(map[string]any)
			if textValue(result["type"]) == "tool_result" {
				walk.acceptResult(textValue(result["tool_use_id"]), result, raw["toolUseResult"])
			}
		}
	}
	if notification := taskNotification(raw); notification != "" {
		walk.acceptNotification(notification)
	}
}

func (walk *taskWalk) rememberRequests(items []any) {
	for _, item := range items {
		request, _ := item.(map[string]any)
		if textValue(request["type"]) != "tool_use" {
			continue
		}
		id, name := textValue(request["id"]), textValue(request["name"])
		input, _ := request["input"].(map[string]any)
		switch name {
		case "Agent", "Bash":
			walk.launches[id] = taskLaunch{toolUseID: id, kind: strings.ToLower(name), asked: textValue(input["description"]), order: walk.nextLaunch}
			walk.nextLaunch++
		case "SendMessage":
			walk.sendMessages[id] = true
		}
	}
}

func (walk *taskWalk) acceptResult(toolUseID string, result map[string]any, rawResult any) {
	identity, ok := rawResult.(map[string]any)
	if !ok {
		return
	}
	if launch, ok := walk.launches[toolUseID]; ok && walk.byToolUseID[toolUseID] == nil {
		id := textValue(identity["backgroundTaskId"])
		asked := launch.asked
		output := resultOutput(result, "Output is being written to: ", true)
		if launch.kind == "agent" {
			if textValue(identity["status"]) != "async_launched" {
				return
			}
			id, asked = textValue(identity["agentId"]), textValue(identity["description"])
			output = resultOutput(result, "output_file: ", false)
			if output == "" {
				output = textValue(identity["outputFile"])
			}
		}
		if id != "" {
			state := &taskState{task: Task{ID: id, ToolUseID: toolUseID, Kind: launch.kind, Asked: asked, Output: output, InFlight: true}, seen: make(map[string]bool), order: launch.order}
			walk.states = append(walk.states, state)
			walk.byToolUseID[toolUseID], walk.byTaskID[id] = state, state
		}
	}
	if walk.sendMessages[toolUseID] && identity["success"] == true {
		if state := walk.byTaskID[textValue(identity["resumedAgentId"])]; state != nil {
			state.task.InFlight = true
			walk.byToolUseID[toolUseID] = state
		}
	}
}

func resultOutput(result map[string]any, label string, trimPeriod bool) string {
	content := result["content"]
	var texts []string
	if text, ok := content.(string); ok {
		texts = append(texts, text)
	}
	if items, ok := content.([]any); ok {
		for _, item := range items {
			part, _ := item.(map[string]any)
			texts = append(texts, textValue(part["text"]))
		}
	}
	for _, text := range texts {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			labelAt := strings.Index(line, label)
			if labelAt < 0 {
				continue
			}
			output := strings.TrimSpace(line[labelAt+len(label):])
			if before, _, found := strings.Cut(output, ". You will be notified"); found {
				return before
			}
			if trimPeriod {
				output = strings.TrimSuffix(output, ".")
			}
			return output
		}
	}
	return ""
}

func taskNotification(raw map[string]any) string {
	kind := textValue(raw["type"])
	if kind == "queue-operation" && textValue(raw["operation"]) == "enqueue" {
		if content := textValue(raw["content"]); strings.HasPrefix(content, "<task-notification>") {
			return content
		}
	}
	if kind == "user" {
		message, _ := raw["message"].(map[string]any)
		if content := textValue(message["content"]); strings.HasPrefix(content, "<task-notification>") {
			return content
		}
	}
	if kind == "attachment" {
		attachment, _ := raw["attachment"].(map[string]any)
		if textValue(attachment["type"]) == "queued_command" && textValue(attachment["commandMode"]) == "task-notification" {
			if prompt := textValue(attachment["prompt"]); strings.HasPrefix(prompt, "<task-notification>") {
				return prompt
			}
		}
	}
	return ""
}

func (walk *taskWalk) acceptNotification(text string) {
	taskID, toolUseID, status := taskTag(text, "task-id"), taskTag(text, "tool-use-id"), taskTag(text, "status")
	state := walk.byToolUseID[toolUseID]
	if state == nil || taskID == "" || status == "" {
		return
	}
	if taskID != state.task.ID {
		walk.notices = append(walk.notices, Notice{Text: fmt.Sprintf("notification for tool-use %s names task %s, not %s", toolUseID, taskID, state.task.ID)})
		return
	}
	eventKey := toolUseID + "\x00" + status
	if state.seen[eventKey] {
		return
	}
	state.seen[eventKey], state.events, state.task.Status = true, state.events+1, status
	state.task.InFlight = !terminalTask(status)
	if state.task.InFlight {
		walk.notices = append(walk.notices, Notice{TaskID: taskID, Text: fmt.Sprintf("task %s status %s: treated as in flight", taskID, status)})
	}
}

func taskTag(text, tag string) string {
	open, close := "<"+tag+">", "</"+tag+">"
	start := strings.Index(text, open)
	if start < 0 {
		return ""
	}
	rest := text[start+len(open):]
	end := strings.Index(rest, close)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func terminalTask(status string) bool {
	for _, row := range terminalTaskStatus {
		if row.status == status {
			return true
		}
	}
	return false
}
