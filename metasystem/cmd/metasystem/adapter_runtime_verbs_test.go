package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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
			// A template installation is its own state root, so the gate
			// resolves both roots without a Git repository.
			root := filepath.Join(t.TempDir(), "metasystem")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.template=true\ncontext.toolgate.mode="+test.mode+"\n"), 0o644); err != nil {
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
				code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
					return runAdapterClaudeToolGate([]string{"--root", root}, stdout, stderr)
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

// TestAdapterClaudeToolGateVerbAppliesTheInstallationThresholds drives the
// verb on an installation whose metasystem.conf sets a 120K ceiling and a 40K
// margin, a trigger of 80K where the defaults put it at 105K. The --root flag
// names either the installation or the checkout that contains it; both reach
// the installation's thresholds and keep the decision row under its
// artifacts/agents/context.
func TestAdapterClaudeToolGateVerbAppliesTheInstallationThresholds(t *testing.T) {
	birth := time.Date(2026, 10, 3, 9, 0, 0, 0, time.UTC)
	previousBirth, previousClock := toolGateProcessBirth, toolGateClock
	toolGateProcessBirth = func(int64) (time.Time, bool) { return birth, true }
	toolGateClock = func() time.Time { return birth.Add(time.Millisecond) }
	t.Cleanup(func() {
		toolGateProcessBirth, toolGateClock = previousBirth, previousClock
	})

	for _, flag := range []string{"installation", "checkout"} {
		t.Run(flag, func(t *testing.T) {
			checkout := t.TempDir()
			installation := filepath.Join(checkout, "metasystem")
			if err := os.MkdirAll(installation, 0o755); err != nil {
				t.Fatal(err)
			}
			conf := "metasystem.template=true\ncontext.toolgate.mode=deny\ncontext.ceiling.tokens=120000\ncontext.handoff.margin.tokens=40000\n"
			if err := os.WriteFile(filepath.Join(installation, "metasystem.conf"), []byte(conf), 0o644); err != nil {
				t.Fatal(err)
			}
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			line := `{"type":"assistant","requestId":"tool-gate","timestamp":"2026-10-03T09:00:00Z","message":{"usage":{"input_tokens":85000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}` + "\n"
			if err := os.WriteFile(transcript, []byte(line), 0o644); err != nil {
				t.Fatal(err)
			}
			payload, err := json.Marshal(map[string]any{
				"session_id": "threshold-session", "transcript_path": transcript,
				"tool_name": "Bash", "tool_input": map[string]string{"command": "rm x"}, "cwd": checkout,
			})
			if err != nil {
				t.Fatal(err)
			}
			root := installation
			if flag == "checkout" {
				root = checkout
			}
			var code int
			var stdout, stderr string
			withStdin(t, string(payload), func() {
				code, stdout, stderr = runOnOwnStreams(func(stdout, stderr io.Writer) int {
					return runAdapterClaudeToolGate([]string{"--root", root}, stdout, stderr)
				})
			})
			want := "CONTEXT AT 85K (trigger 80K): this call is denied; run metasystem session handoff --root " + installation + " alone"
			if code != 0 || stderr != "" || !strings.Contains(stdout, `"permissionDecision":"deny"`) || !strings.Contains(stdout, want) {
				t.Fatalf("code=%d stdout=%q stderr=%q, want a denial naming %q", code, stdout, stderr, want)
			}
			rows, err := os.ReadFile(filepath.Join(installation, "artifacts", "agents", "context", "tool-gate.jsonl"))
			if err != nil || strings.Count(string(rows), "\n") != 1 || !strings.Contains(string(rows), `"decision":"deny"`) {
				t.Fatalf("installation rows = %q (err %v), want the one denial", rows, err)
			}
			if _, err := os.Stat(filepath.Join(checkout, "artifacts")); !os.IsNotExist(err) {
				t.Fatalf("the containing checkout gained artifacts/ (err %v); the gate keeps its rows in the installation", err)
			}
		})
	}
}

// TestToolGatePeerOffersTheSeatsOldestMessage (R26, U10f-2's binding): the
// gate's Peer is the shared offer over this seat's board with Claude's
// declared tool field as the room: it returns the oldest pending message
// after its preface and a marker that records the delivery; with no message
// on the board it reads no enrollment; a runtime that declares no field
// offers nothing.
func TestToolGatePeerOffersTheSeatsOldestMessage(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), ".metasystem")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	resolved := 0
	resolve := func(string) (string, error) { resolved++; return "m1b", nil }
	claims := func() (board.Ownership, error) { return board.Ownership{}, nil }
	now := func() time.Time { return time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC) }
	peer, release := toolGatePeer(home, "/repo", noLaunchKind, resolve, claims, 10000, io.Discard, now)
	if text, mark := peer(); text != "" || mark != nil || resolved != 0 {
		t.Fatalf("an empty board: %q, marker %v, %d enrollment reads", text, mark != nil, resolved)
	}
	release()
	published, err := board.Publish(home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1a"}, To: board.Address{Machine: "m1b"}, Text: "is it green?"}, now())
	if err != nil {
		t.Fatal(err)
	}
	peer, release = toolGatePeer(home, "/repo", noLaunchKind, resolve, claims, 10000, io.Discard, now)
	text, mark := peer()
	if !strings.HasPrefix(text, "[peer message from m1a to m1b, id "+published.Message.ID+":") || !strings.HasSuffix(text, "\n> is it green?\n[end of peer message "+published.Message.ID+"]") || mark == nil {
		t.Fatalf("the offer = %q", text)
	}
	if err := mark(); err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := os.Stat(filepath.Join(board.Dir(home), "m1b", "mailbox", "delivered", published.Message.ID, "m1b.json")); err != nil {
		t.Fatalf("the marker: %v", err)
	}
	if _, err := board.Publish(home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1a"}, To: board.Address{Machine: "m1b"}, Text: "still pending"}, now()); err != nil {
		t.Fatal(err)
	}
	peer, release = toolGatePeer(home, "/repo", noLaunchKind, resolve, claims, 0, io.Discard, now)
	defer release()
	if text, _ := peer(); text != "" {
		t.Fatalf("a runtime with no declared field was offered %q", text)
	}
}

// noLaunchKind is the environment of a session started by hand: it names no
// launch kind.
func noLaunchKind(string) (string, bool) { return "", false }

// TestToolGatePeerOffersALaunchedStepNothing: the gate of a session the
// launcher started for a step resolves the seat's machine but offers none of
// its mail and marks none; the message stays pending, and the gate of the
// seat's session and of a session started by hand still receive it.
func TestToolGatePeerOffersALaunchedStepNothing(t *testing.T) {
	t.Parallel()
	resolve := func(string) (string, error) { return "m1b", nil }
	claims := func() (board.Ownership, error) { return board.Ownership{}, nil }
	now := func() time.Time { return time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC) }
	kind := func(value string) func(string) (string, bool) {
		return func(name string) (string, bool) {
			if name == launch.KindEnv {
				return value, true
			}
			return "", false
		}
	}
	for _, receiver := range []struct {
		name   string
		lookup func(string) (string, bool)
	}{{"the seat's", kind("seat")}, {"a hand-started", noLaunchKind}} {
		home := filepath.Join(t.TempDir(), ".metasystem")
		if err := os.MkdirAll(home, 0o700); err != nil {
			t.Fatal(err)
		}
		published, err := board.Publish(home, board.Request{Kind: board.KindAsk, From: board.Sender{Machine: "m1a"}, To: board.Address{Machine: "m1b"}, Text: "is it green?"}, now())
		if err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(board.Dir(home), "m1b", "mailbox", "delivered", published.Message.ID, "m1b.json")
		peer, release := toolGatePeer(home, "/repo", kind("build"), resolve, claims, 10000, io.Discard, now)
		text, mark := peer()
		release()
		if text != "" || mark != nil {
			t.Fatalf("a build session's gate was offered %q, marker %v", text, mark != nil)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("a build session's gate marked the machine's message: %v", err)
		}
		peer, release = toolGatePeer(home, "/repo", receiver.lookup, resolve, claims, 10000, io.Discard, now)
		text, mark = peer()
		if !strings.Contains(text, "id "+published.Message.ID+":") || mark == nil {
			release()
			t.Fatalf("%s gate was offered %q", receiver.name, text)
		}
		err = mark()
		release()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("%s gate's marker: %v", receiver.name, err)
		}
	}
}
