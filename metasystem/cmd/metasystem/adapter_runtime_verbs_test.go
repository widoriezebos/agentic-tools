package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withStdin runs fn with os.Stdin fed from content, so a verb that reads the
// hook payload or a --version stream can be driven from a test.
func withStdin(t *testing.T, content string, fn func()) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdin
	os.Stdin = read
	go func() {
		write.WriteString(content)
		write.Close()
	}()
	fn()
	os.Stdin = saved
}

func TestAdapterClaudeToolGateVerb(t *testing.T) {
	birth := time.Date(2026, 9, 17, 20, 0, 0, 0, time.UTC)
	previousBirth, previousClock := toolGateProcessBirth, toolGateClock
	toolGateProcessBirth = func(int64) (time.Time, bool) { return birth, true }
	toolGateClock = func() time.Time { return birth.Add(time.Millisecond) }
	t.Cleanup(func() {
		toolGateProcessBirth, toolGateClock = previousBirth, previousClock
	})

	for _, test := range []struct {
		name       string
		mode       string
		agentID    string
		wantOutput bool
		wantRows   int
	}{
		{name: "deny", mode: "deny", wantOutput: true, wantRows: 1},
		{name: "observe", mode: "observe", wantRows: 1},
		{name: "native-subagent", mode: "observe", agentID: "agent-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("context.toolgate.mode="+test.mode+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			transcript := filepath.Join(root, "transcript.jsonl")
			line := `{"type":"assistant","requestId":"tool-gate","timestamp":"2026-09-17T20:00:00Z","message":{"usage":{"input_tokens":120000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}` + "\n"
			if err := os.WriteFile(transcript, []byte(line), 0o644); err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(map[string]any{
				"session_id": "verb-session", "transcript_path": transcript,
				"tool_name": "Bash", "tool_input": map[string]string{"command": "rm x"},
				"cwd": root, "agent_id": test.agentID,
			})
			if err != nil {
				t.Fatal(err)
			}
			var code int
			var stdout, stderr string
			withStdin(t, string(payload), func() {
				code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
					return runAdapterClaudeToolGate([]string{"--root", root})
				})
			})
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if (stdout != "") != test.wantOutput {
				t.Fatalf("stdout=%q, want output=%t", stdout, test.wantOutput)
			}
			if test.wantOutput {
				reason := fmt.Sprintf("CONTEXT AT 120K (trigger 105K): this call is denied; run metasystem session handoff --root %s alone, or launch a delegate", root)
				want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"` + reason + `"}}` + "\n"
				if stdout != want {
					t.Fatalf("stdout=%q, want %q", stdout, want)
				}
			}
			rowsPath := filepath.Join(root, "artifacts", "agents", "context", "tool-gate.jsonl")
			rows, readErr := os.ReadFile(rowsPath)
			if test.wantRows == 0 {
				if !os.IsNotExist(readErr) {
					t.Fatalf("subagent row file: bytes=%q err=%v", rows, readErr)
				}
				return
			}
			if readErr != nil {
				t.Fatal(readErr)
			}
			if count := strings.Count(string(rows), "\n"); count != test.wantRows {
				t.Fatalf("row count=%d rows=%q", count, rows)
			}
		})
	}
}
