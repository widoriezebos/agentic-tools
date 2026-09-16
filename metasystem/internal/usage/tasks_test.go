package usage

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type taskExpectation struct {
	id       string
	inFlight bool
	status   string
}

func TestInFlightTasksStateMachine(t *testing.T) {
	tests := []struct {
		fixture string
		want    []taskExpectation
	}{
		{"agent-running", []taskExpectation{{"ab3e7e704e393e416", true, ""}}},
		{"agent-completed-user-delivery", []taskExpectation{{"ab3e7e704e393e416", false, "completed"}}},
		{"agent-completed-attachment-delivery", []taskExpectation{{"a62bd12fa4c1c585d", false, "completed"}}},
		{"notification-without-enqueue", []taskExpectation{{"ab3e7e704e393e416", false, "completed"}}},
		{"repeated-notification-one-id", []taskExpectation{{"ab3e7e704e393e416", false, "completed"}}},
		{"resumed-then-completed", []taskExpectation{{"a72a593da23f0651e", false, "completed"}}},
		{"unknown-status", []taskExpectation{{"ab3e7e704e393e416", true, "paused"}}},
		{"bash-running", []taskExpectation{{"b2rs8m65o", true, ""}}},
		{"bash-completed", []taskExpectation{{"b2rs8m65o", false, "completed"}}},
		{"sync-agent-not-a-task", nil},
		{"agent-killed", []taskExpectation{{"a279c21fe213df9a0", false, "killed"}}},
		{"bash-failed", []taskExpectation{{"bvf001bva", false, "failed"}}},
		{"agent-killed-then-resumed", []taskExpectation{{"a279c21fe213df9a0", true, "killed"}}},
		{"agent-stopped", []taskExpectation{{"a279c21fe213df9a0", false, "stopped"}}},
		{"agent-async-without-flag", []taskExpectation{{"aa5f34f38119dbd41", true, ""}}},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			tasks, _, err := InFlightTasks(taskFixture(test.fixture), ReadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != len(test.want) {
				t.Fatalf("tasks = %d, want %d: %#v", len(tasks), len(test.want), tasks)
			}
			for index, want := range test.want {
				got := tasks[index]
				if got.ID != want.id || got.InFlight != want.inFlight || got.Status != want.status {
					t.Errorf("task = {ID:%q InFlight:%v Status:%q}, want %#v", got.ID, got.InFlight, got.Status, want)
				}
			}
		})
	}
	stopped := bytes.ReplaceAll(taskFixtureBytes(t, "agent-killed-then-resumed"), []byte("<status>killed</status>"), []byte("<status>stopped</status>"))
	tasks, _ := tasksFromBytes(t, stopped)
	if len(tasks) != 1 || !tasks[0].InFlight || tasks[0].Status != "stopped" {
		t.Fatalf("stopped then resumed task = %#v, want stopped task in flight", tasks)
	}
}

func TestInFlightTasksCountsACompletionOnce(t *testing.T) {
	for _, fixture := range []string{"agent-completed-user-delivery", "agent-completed-attachment-delivery", "repeated-notification-one-id"} {
		t.Run(fixture, func(t *testing.T) {
			states, _, err := walkTaskTranscript(taskFixture(fixture), ReadOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(states) != 1 || states[0].events != 1 || states[0].task.InFlight {
				t.Fatalf("states = %#v, want one ended task with one completion event", states)
			}
		})
	}
}

func TestInFlightTasksTreatsUnknownStatusAsInFlight(t *testing.T) {
	tasks, notices, err := InFlightTasks(taskFixture("unknown-status"), ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantNotice := Notice{TaskID: "ab3e7e704e393e416", Text: "task ab3e7e704e393e416 status paused: treated as in flight"}
	if len(tasks) != 1 || !tasks[0].InFlight || tasks[0].Status != "paused" {
		t.Fatalf("tasks = %#v, want paused task in flight", tasks)
	}
	if !reflect.DeepEqual(notices, []Notice{wantNotice}) {
		t.Fatalf("notices = %#v, want %#v", notices, []Notice{wantNotice})
	}
}

func TestInFlightTasksJoinsByToolUseId(t *testing.T) {
	tests := []struct {
		fixture string
		want    Task
	}{
		{"agent-running", Task{ID: "ab3e7e704e393e416", ToolUseID: "toolu_01G9AiFhA8vhPnef4Up5ETkV", Kind: "agent", Asked: "Token diagnosis for 2026-09-15", Output: "/private/tmp/claude-501/<slug>/<session>/tasks/ab3e7e704e393e416.output", InFlight: true}},
		{"bash-running", Task{ID: "b2rs8m65o", ToolUseID: "toolu_01M6nAxpne47YJcRc94sqi7W", Kind: "bash", Asked: "Show largest top-level home directory items", Output: "/private/tmp/claude-501/<slug>/<session>/tasks/b2rs8m65o.output", InFlight: true}},
	}
	for _, test := range tests {
		tasks, _, err := InFlightTasks(taskFixture(test.fixture), ReadOptions{})
		if err != nil || len(tasks) != 1 || tasks[0] != test.want {
			t.Errorf("%s: tasks = %#v, err = %v, want %#v", test.fixture, tasks, err, test.want)
		}
	}
	competing := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"agent-use","name":"Agent","input":{"description":"request text"}},{"type":"tool_use","id":"bash-use","name":"Bash","input":{"description":"bash asked"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"bash-use","content":"Output is being written to: /tmp/bash.output."}]},"toolUseResult":{"backgroundTaskId":"bash-id"}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"agent-use","content":"output_file: /tmp/agent.out."}]},"toolUseResult":{"status":"async_launched","agentId":"agent-id","description":"agent result asked"}}
`)
	tasks, _ := tasksFromBytes(t, competing)
	wantTasks := []Task{
		{ID: "agent-id", ToolUseID: "agent-use", Kind: "agent", Asked: "agent result asked", Output: "/tmp/agent.out.", InFlight: true},
		{ID: "bash-id", ToolUseID: "bash-use", Kind: "bash", Asked: "bash asked", Output: "/tmp/bash.output", InFlight: true},
	}
	if !reflect.DeepEqual(tasks, wantTasks) {
		t.Fatalf("competing launch tasks = %#v, want launch-ordered %#v", tasks, wantTasks)
	}
	data := taskFixtureBytes(t, "agent-running")
	data = append(data, []byte("{\"type\":\"queue-operation\",\"operation\":\"enqueue\",\"content\":\"<task-notification>\\n<task-id>wrong-task</task-id>\\n<tool-use-id>toolu_01G9AiFhA8vhPnef4Up5ETkV</tool-use-id>\\n<status>completed</status>\\n</task-notification>\"}\n")...)
	tasks, notices := tasksFromBytes(t, data)
	want := "notification for tool-use toolu_01G9AiFhA8vhPnef4Up5ETkV names task wrong-task, not ab3e7e704e393e416"
	if len(tasks) != 1 || !tasks[0].InFlight || tasks[0].Status != "" || len(notices) != 1 || notices[0].Text != want {
		t.Fatalf("mismatch tasks = %#v, notices = %#v, want unchanged task and %q", tasks, notices, want)
	}
}

func TestInFlightTasksTerminalTable(t *testing.T) {
	want := map[string]string{
		"completed": "9292cf37-9f88-4700-a7f9-1fe2e45bf000.jsonl:220",
		"failed":    "c53cbef5-7be0-48fa-83f1-876f40a3c193.jsonl:839",
		"killed":    "2c27f5cc-245a-4735-a402-dafc351e4caf.jsonl:135",
		"stopped":   "9b4a48c5-e0ce-46bb-99b1-ca3644a08ec4.jsonl:28259",
	}
	if len(terminalTaskStatus) != len(want) {
		t.Fatalf("terminal statuses = %d, want %d", len(terminalTaskStatus), len(want))
	}
	for _, row := range terminalTaskStatus {
		if row.cited == "" || want[row.status] != row.cited {
			t.Errorf("terminal row = %#v, want citation %q", row, want[row.status])
		}
	}
}

func TestInFlightTasksResumeNeedsSuccess(t *testing.T) {
	base := taskFixtureBytes(t, "agent-killed-then-resumed")
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"success", base, true},
		{"success false", bytes.ReplaceAll(base, []byte(`"success":true`), []byte(`"success":false`)), false},
		{"input to without result", base[:bytes.LastIndexByte(base[:len(base)-1], '\n')+1], false},
		{"agentId only", bytes.ReplaceAll(base, []byte(`"resumedAgentId":"a279c21fe213df9a0"`), []byte(`"agentId":"a279c21fe213df9a0"`)), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tasks, _ := tasksFromBytes(t, test.data)
			if len(tasks) != 1 || tasks[0].InFlight != test.want || tasks[0].Status != "killed" {
				t.Fatalf("task = %#v, want InFlight %v with killed status", tasks, test.want)
			}
		})
	}
}

func taskFixture(name string) string {
	return filepath.Join("testdata", "tasks", name+".jsonl")
}

func taskFixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(taskFixture(name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func tasksFromBytes(t *testing.T, data []byte) ([]Task, []Notice) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	tasks, notices, err := InFlightTasks(path, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return tasks, notices
}

func TestTaskFixturesHaveOneSourceEach(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "tasks", "SOURCES.md"))
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(strings.TrimSpace(string(data)), "\n") + 1; lines != 15 {
		t.Fatalf("source lines = %d, want 15", lines)
	}
}

func TestInFlightTasksTranscriptSelection(t *testing.T) {
	var opened []string
	previous := callFileOpens
	callFileOpens = func(path string) { opened = append(opened, path) }
	t.Cleanup(func() { callFileOpens = previous })
	path := taskFixture("agent-running")
	tasks, _, err := InFlightTasks("", ReadOptions{Transcript: path})
	if err != nil || len(tasks) != 1 || !reflect.DeepEqual(opened, []string{path}) {
		t.Fatalf("fallback tasks = %#v, opened = %#v, err = %v", tasks, opened, err)
	}
	tasks, notices, err := InFlightTasks("", ReadOptions{})
	if err != nil || tasks != nil || notices != nil || len(opened) != 1 {
		t.Fatalf("empty transcript = (%#v, %#v, %v), opened = %#v", tasks, notices, err, opened)
	}
	missing := filepath.Join(t.TempDir(), "missing.jsonl")
	if _, _, err := InFlightTasks(missing, ReadOptions{}); err == nil || !strings.Contains(err.Error(), "cannot open task transcript") {
		t.Fatalf("missing transcript error = %v", err)
	}
	if !reflect.DeepEqual(opened, []string{path, missing}) {
		t.Fatalf("opened = %#v, want each attempted path once", opened)
	}
}
