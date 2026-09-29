package adapter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/usage"
	"golang.org/x/sys/unix"
)

func TestToolGateDeadlineCountsFromTheShellBirth(t *testing.T) {
	birth := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name       string
		now        time.Time
		wantOutput bool
		wantCause  string
	}{
		{name: "before", now: birth.Add(99 * time.Millisecond), wantOutput: true, wantCause: "other"},
		{name: "after", now: birth.Add(101 * time.Millisecond), wantCause: "deadline"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, transcript := toolGateFixture(t, 120000)
			stdout := &bytes.Buffer{}
			opts := toolGateOptions(root, transcript, "deny", birth, func() time.Time { return test.now }, stdout)
			if err := RunToolGate(opts); err != nil {
				t.Fatal(err)
			}
			if (stdout.Len() > 0) != test.wantOutput {
				t.Fatalf("stdout = %q, want output=%t", stdout.String(), test.wantOutput)
			}
			rows := readToolGateRows(t, root)
			if len(rows) != 1 || rows[0].Cause != test.wantCause {
				t.Fatalf("rows = %#v, want cause %q", rows, test.wantCause)
			}
		})
	}
}

func TestToolGateFallsBackToEntryWhenBirthUnreadable(t *testing.T) {
	entry := time.Date(2026, 9, 17, 11, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name       string
		after      time.Duration
		wantOutput bool
		wantCause  string
	}{
		{name: "inside-allowance", after: 74 * time.Millisecond, wantOutput: true, wantCause: "other"},
		{name: "outside-allowance", after: 76 * time.Millisecond, wantCause: "deadline"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, transcript := toolGateFixture(t, 120000)
			calls := 0
			clock := func() time.Time {
				calls++
				if calls == 1 {
					return entry
				}
				return entry.Add(test.after)
			}
			stdout := &bytes.Buffer{}
			opts := toolGateOptions(root, transcript, "deny", time.Time{}, clock, stdout)
			if err := RunToolGate(opts); err != nil {
				t.Fatal(err)
			}
			if (stdout.Len() > 0) != test.wantOutput {
				t.Fatalf("stdout = %q, want output=%t", stdout.String(), test.wantOutput)
			}
			rows := readToolGateRows(t, root)
			if len(rows) != 1 || rows[0].Birth != "unreadable" || rows[0].Cause != test.wantCause || rows[0].ElapsedMs != test.after.Milliseconds() {
				t.Fatalf("fallback row = %#v", rows)
			}
		})
	}
}

func TestToolGateClassifiesBeforeReading(t *testing.T) {
	for _, call := range []Call{
		{Tool: "Agent", Input: json.RawMessage(`{}`)},
		bashToolGateCall("metasystem session handoff --status --root /repo"),
	} {
		root := t.TempDir()
		birth := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
		stdout := &bytes.Buffer{}
		opts := toolGateOptions(root, filepath.Join(root, "missing.jsonl"), "deny", birth, func() time.Time { return birth }, stdout)
		opts.Stdin = bytes.NewReader(toolGatePayloadBytes("allowlisted", opts.StateRoot+"/missing.jsonl", call, ""))
		if err := RunToolGate(opts); err != nil || stdout.Len() != 0 {
			t.Fatalf("call %s: err=%v stdout=%q", call.Tool, err, stdout.String())
		}
		if _, err := os.Stat(toolGateRowsPath(root)); !os.IsNotExist(err) {
			t.Fatalf("call %s created a decision row: %v", call.Tool, err)
		}
	}
}

func TestToolGateReadOptionsAreNonBlocking(t *testing.T) {
	deadline := time.Date(2026, 9, 17, 12, 1, 0, 0, time.UTC)
	clock := func() time.Time { return deadline.Add(-time.Millisecond) }
	want := usage.ReadOptions{
		Transcript: "/tmp/transcript.jsonl", NonBlocking: true, MaxBytes: 262144,
		Deadline: deadline, Clock: clock,
	}
	got := toolGateReadOptions(want.Transcript, deadline, clock)
	if got.Clock == nil || got.Clock() != clock() {
		t.Fatalf("read clock is not the injected clock")
	}
	got.Clock, want.Clock = nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("read options = %#v, want %#v", got, want)
	}
}

func TestToolGateAllowsNativeSubagentCalls(t *testing.T) {
	root, transcript := toolGateFixture(t, 120000)
	birth := time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)
	call := bashToolGateCall("rm x")

	stdout := &bytes.Buffer{}
	opts := toolGateOptions(root, transcript, "deny", birth, func() time.Time { return birth.Add(time.Millisecond) }, stdout)
	opts.Stdin = bytes.NewReader(toolGatePayloadBytes("subagent", transcript, call, "agent-1"))
	if err := RunToolGate(opts); err != nil || stdout.Len() != 0 {
		t.Fatalf("subagent call: err=%v stdout=%q", err, stdout.String())
	}
	if _, err := os.Stat(toolGateRowsPath(root)); !os.IsNotExist(err) {
		t.Fatalf("subagent call created a row: %v", err)
	}

	stdout.Reset()
	opts.Stdout = stdout
	opts.Stdin = bytes.NewReader(toolGatePayloadBytes("main", transcript, call, ""))
	if err := RunToolGate(opts); err != nil || stdout.Len() == 0 {
		t.Fatalf("main call: err=%v stdout=%q", err, stdout.String())
	}
}

func TestToolGateSubagentCallsWriteNoRow(t *testing.T) {
	root, transcript := toolGateFixture(t, 120000)
	birth := time.Date(2026, 9, 17, 13, 1, 0, 0, time.UTC)
	opts := toolGateOptions(root, transcript, "observe", birth, func() time.Time { return birth.Add(time.Millisecond) }, &bytes.Buffer{})
	opts.Stdin = bytes.NewReader(toolGatePayloadBytes("subagent", transcript, bashToolGateCall("rm x"), "agent-1"))
	if err := RunToolGate(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(toolGateRowsPath(root)); !os.IsNotExist(err) {
		t.Fatalf("subagent call created a row: %v", err)
	}
}

func TestToolGateAllowsPastItsDeadline(t *testing.T) {
	root, transcript := toolGateFixture(t, 120000)
	birth := time.Date(2026, 9, 17, 14, 0, 0, 0, time.UTC)
	clockCalls := 0
	clock := func() time.Time {
		clockCalls++
		if clockCalls == 1 {
			return birth
		}
		if clockCalls < 5 {
			return birth.Add(99 * time.Millisecond)
		}
		return birth.Add(101 * time.Millisecond)
	}
	stdout := &bytes.Buffer{}
	opts := toolGateOptions(root, transcript, "deny", birth, clock, stdout)
	if err := RunToolGate(opts); err != nil || stdout.Len() != 0 {
		t.Fatalf("past-deadline call: err=%v stdout=%q", err, stdout.String())
	}
	rows := readToolGateRows(t, root)
	if len(rows) != 1 || rows[0].Cause != "deadline" || rows[0].Decision != "allow" || !rows[0].WouldDeny || rows[0].Tokens != 120000 {
		t.Fatalf("past-deadline row = %#v", rows)
	}
}

func TestToolGateNoDecisionWhenTheCallStoreIsBusy(t *testing.T) {
	for _, test := range []struct {
		name string
		path func(string, string) string
	}{
		{name: "maintenance", path: func(root, _ string) string {
			return filepath.Join(root, "artifacts", "agents", "context", "maintenance.lock")
		}},
		{name: "cursor", path: func(root, session string) string {
			return usage.CursorPath(root, "claude", session) + ".lock"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, transcript := toolGateFixture(t, 120000)
			session := "busy-" + test.name
			lockPath := test.path(root, session)
			if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
				t.Fatal(err)
			}
			lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Close()
			if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
				t.Fatal(err)
			}
			defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)

			birth := time.Date(2026, 9, 17, 15, 0, 0, 0, time.UTC)
			stdout := &bytes.Buffer{}
			opts := toolGateOptions(root, transcript, "deny", birth, func() time.Time { return birth.Add(time.Millisecond) }, stdout)
			opts.Stdin = bytes.NewReader(toolGatePayloadBytes(session, transcript, bashToolGateCall("rm x"), ""))
			if err := RunToolGate(opts); err != nil || stdout.Len() != 0 {
				t.Fatalf("busy call: err=%v stdout=%q", err, stdout.String())
			}
			rows := readToolGateRows(t, root)
			if len(rows) != 1 || rows[0].Cause != "busy" || rows[0].Decision != "allow" {
				t.Fatalf("busy row = %#v", rows)
			}
		})
	}
}

func TestToolGateLeavesTheCursor(t *testing.T) {
	root, transcript := toolGateFixture(t, 10)
	session := "unchanged"
	if _, err := usage.LatestCall(root, "claude", session, usage.ReadOptions{Capability: usage.PerCall, Transcript: transcript}); err != nil {
		t.Fatal(err)
	}
	appendToolGateSample(t, transcript, "second", 120000)
	cursorPath := usage.CursorPath(root, "claude", session)
	samplesPath := usage.SamplesPath(root, "claude", session)
	cursorBefore := readToolGateFile(t, cursorPath)
	samplesBefore := readToolGateFile(t, samplesPath)

	birth := time.Date(2026, 9, 17, 16, 0, 0, 0, time.UTC)
	opts := toolGateOptions(root, transcript, "deny", birth, func() time.Time { return birth.Add(time.Millisecond) }, &bytes.Buffer{})
	opts.Stdin = bytes.NewReader(toolGatePayloadBytes(session, transcript, bashToolGateCall("rm x"), ""))
	if err := RunToolGate(opts); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cursorBefore, readToolGateFile(t, cursorPath)) || !bytes.Equal(samplesBefore, readToolGateFile(t, samplesPath)) {
		t.Fatal("tool gate changed the usage cursor or samples")
	}
}

func TestToolGateWritesDecisionRows(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("context.toolgate.mode=deny\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	birth := time.Date(2026, 9, 17, 17, 0, 0, 0, time.UTC)
	at := birth.Add(20 * time.Millisecond)
	memoryDir := filepath.Join(root, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		session   string
		tokens    int64
		call      Call
		memoryDir string
		cause     string
		decision  string
		wouldDeny bool
	}{
		{session: "deny", tokens: 120000, call: bashToolGateCall("rm x"), memoryDir: memoryDir, cause: "other", decision: "deny", wouldDeny: true},
		{session: "note", tokens: 120000, call: toolGatePathCall("Write", filepath.Join(memoryDir, "note.md")), memoryDir: memoryDir, cause: "memory-note", decision: "allow"},
		{session: "unresolved", tokens: 120000, call: toolGatePathCall("Edit", "/unknown/note.md"), cause: "memory-unresolved", decision: "allow"},
	}
	for _, test := range tests {
		transcript := filepath.Join(root, test.session+".jsonl")
		writeToolGateTranscript(t, transcript, "sample", test.tokens)
		opts := toolGateOptions(root, transcript, "deny", birth, func() time.Time { return at }, &bytes.Buffer{})
		opts.MemoryDir = test.memoryDir
		opts.Stdin = bytes.NewReader(toolGatePayloadBytes(test.session, transcript, test.call, ""))
		if err := RunToolGate(opts); err != nil {
			t.Fatal(err)
		}
	}

	under := filepath.Join(root, "under.jsonl")
	writeToolGateTranscript(t, under, "under", 100)
	opts := toolGateOptions(root, under, "deny", birth, func() time.Time { return at }, &bytes.Buffer{})
	opts.Stdin = bytes.NewReader(toolGatePayloadBytes("under", under, bashToolGateCall("rm x"), ""))
	if err := RunToolGate(opts); err != nil {
		t.Fatal(err)
	}
	opts.Stdin = bytes.NewReader(toolGatePayloadBytes("allowlisted", filepath.Join(root, "missing"), Call{Tool: "Agent", Input: json.RawMessage(`{}`)}, ""))
	if err := RunToolGate(opts); err != nil {
		t.Fatal(err)
	}

	rows := readToolGateRows(t, root)
	if len(rows) != len(tests) {
		t.Fatalf("row count = %d, want %d: %#v", len(rows), len(tests), rows)
	}
	for index, want := range tests {
		got := rows[index]
		if got.Session != want.session || got.At != at.Format(time.RFC3339Nano) || got.Tokens != want.tokens || got.Tool != want.call.Tool || got.Mode != "deny" || got.Decision != want.decision || got.WouldDeny != want.wouldDeny || got.Cause != want.cause || got.ElapsedMs != 20 || got.Birth != birth.Format(time.RFC3339Nano) || got.Reason != "" {
			t.Fatalf("row %d = %#v, fixture = %#v", index, got, want)
		}
	}
	file, err := os.Open(toolGateRowsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var fields map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 10 {
			t.Fatalf("row fields = %v", fields)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestToolGateObserveModeAllowsAndRecordsTheDenyDecision(t *testing.T) {
	root, transcript := toolGateFixture(t, 120000)
	birth := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)
	stdout := &bytes.Buffer{}
	opts := toolGateOptions(root, transcript, "observe", birth, func() time.Time { return birth.Add(time.Millisecond) }, stdout)
	if err := RunToolGate(opts); err != nil || stdout.Len() != 0 {
		t.Fatalf("observe call: err=%v stdout=%q", err, stdout.String())
	}
	rows := readToolGateRows(t, root)
	if len(rows) != 1 || rows[0].Mode != "observe" || rows[0].Decision != "allow" || !rows[0].WouldDeny || rows[0].Reason == "" {
		t.Fatalf("observe row = %#v", rows)
	}
	data := readToolGateFile(t, toolGateRowsPath(root))
	var fields map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 11 {
		t.Fatalf("observe row fields = %v", fields)
	}
}

func TestToolGateDenyModeDenies(t *testing.T) {
	root, transcript := toolGateFixture(t, 120000)
	birth := time.Date(2026, 9, 17, 19, 0, 0, 0, time.UTC)
	stdout := &bytes.Buffer{}
	opts := toolGateOptions(root, transcript, "deny", birth, func() time.Time { return birth.Add(time.Millisecond) }, stdout)
	if err := RunToolGate(opts); err != nil {
		t.Fatal(err)
	}
	reason := fmt.Sprintf("CONTEXT AT 120K (trigger 105K): this call is denied; run metasystem session handoff --root %s alone, or launch a delegate", root)
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"` + reason + `"}}` + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func toolGateFixture(t *testing.T, tokens int64) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("context.toolgate.mode=deny\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(root, "transcript.jsonl")
	writeToolGateTranscript(t, transcript, "sample", tokens)
	return root, transcript
}

func toolGateOptions(root, transcript, mode string, birth time.Time, clock func() time.Time, stdout *bytes.Buffer) ToolGateOptions {
	return ToolGateOptions{
		ShellStartedAt: birth, Clock: clock, Mode: mode, StateRoot: root, Installation: root,
		Stdin:  bytes.NewReader(toolGatePayloadBytes("session", transcript, bashToolGateCall("rm x"), "")),
		Stdout: stdout, Stderr: &bytes.Buffer{},
	}
}

func toolGatePayloadBytes(session, transcript string, call Call, agentID string) []byte {
	payload := struct {
		SessionID      string          `json:"session_id"`
		TranscriptPath string          `json:"transcript_path"`
		ToolName       string          `json:"tool_name"`
		ToolInput      json.RawMessage `json:"tool_input"`
		Cwd            string          `json:"cwd"`
		AgentID        string          `json:"agent_id,omitempty"`
	}{session, transcript, call.Tool, call.Input, "/repo", agentID}
	encoded, _ := json.Marshal(payload)
	return encoded
}

func writeToolGateTranscript(t *testing.T, path, request string, tokens int64) {
	t.Helper()
	row := fmt.Sprintf(`{"type":"assistant","requestId":%q,"timestamp":"2026-09-17T10:00:00Z","message":{"usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`+"\n", request, tokens)
	if err := os.WriteFile(path, []byte(row), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendToolGateSample(t *testing.T, path, request string, tokens int64) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	row := fmt.Sprintf(`{"type":"assistant","requestId":%q,"timestamp":"2026-09-17T10:01:00Z","message":{"usage":{"input_tokens":%d,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`+"\n", request, tokens)
	if _, err := file.WriteString(row); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func readToolGateRows(t *testing.T, root string) []toolGateDecisionRow {
	t.Helper()
	data := readToolGateFile(t, toolGateRowsPath(root))
	var rows []toolGateDecisionRow
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var row toolGateDecisionRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}

func toolGateRowsPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "context", "tool-gate.jsonl")
}

func readToolGateFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// peerProbe is a fake peer offer: it counts the calls, the markers, and
// what the marker saw written before it ran.
type peerProbe struct {
	text     string
	calls    int
	marked   int
	markSeen string
	out      *bytes.Buffer
}

func (p *peerProbe) offer() (string, func() error) {
	p.calls++
	return p.text, func() error {
		p.marked++
		if p.out != nil {
			p.markSeen = p.out.String()
		}
		return nil
	}
}

type failingToolGateWriter struct{}

func (failingToolGateWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("injected: stdout closed")
}

// TestToolGateComposesOneResponse (R26, U10f-2; D14C-08): through RunToolGate
// with Peer set, an allowed call with a pending message emits exactly one
// object, the context object without permissionDecision, and the marker runs
// after the write; so does observe mode's would-deny and every allowed path
// (trivial allow, under the trigger, a busy store, the deadline, no sample);
// an enforced denial emits today's deny object byte for byte, never calls
// Peer and marks nothing; nothing pending emits nothing; a native subagent
// call and a call under the helm never call Peer; a failing Stdout marks
// nothing; the decision row is byte-identical with and without a pending
// message.
func TestToolGateComposesOneResponse(t *testing.T) {
	t.Parallel()
	const text = "[peer message from m1a to m1b, id d-1: information]\nis it green?"
	encoded, _ := json.Marshal(text)
	contextObject := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":` + string(encoded) + `}}` + "\n"
	birth := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	at := func() time.Time { return birth.Add(time.Millisecond) }

	runIn := func(t *testing.T, root, transcript, mode string, call Call, pending string, amend func(*ToolGateOptions)) (*peerProbe, string, string, error) {
		t.Helper()
		stdout := &bytes.Buffer{}
		probe := &peerProbe{text: pending, out: stdout}
		opts := toolGateOptions(root, transcript, mode, birth, at, stdout)
		opts.Stdin = bytes.NewReader(toolGatePayloadBytes("session", transcript, call, ""))
		opts.Peer = probe.offer
		if amend != nil {
			amend(&opts)
		}
		err := RunToolGate(opts)
		rows, _ := os.ReadFile(toolGateRowsPath(root))
		lines := strings.Split(strings.TrimRight(string(rows), "\n"), "\n")
		return probe, stdout.String(), lines[len(lines)-1], err
	}
	run := func(t *testing.T, tokens int64, mode string, call Call, pending string, amend func(*ToolGateOptions)) (*peerProbe, string, string, error) {
		t.Helper()
		root, transcript := toolGateFixture(t, tokens)
		return runIn(t, root, transcript, mode, call, pending, amend)
	}

	for _, test := range []struct {
		name   string
		tokens int64
		mode   string
		call   Call
	}{
		{"observe mode's would-deny", 120000, "observe", bashToolGateCall("rm x")},
		{"a trivial allow", 120000, "deny", Call{Tool: "Agent"}},
		{"under the trigger", 10, "deny", bashToolGateCall("rm x")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			probe, out, _, err := run(t, test.tokens, test.mode, test.call, text, nil)
			if err != nil || out != contextObject || probe.calls != 1 || probe.marked != 1 || probe.markSeen != contextObject {
				t.Fatalf("err=%v stdout=%q calls=%d marked=%d seen-at-mark=%q", err, out, probe.calls, probe.marked, probe.markSeen)
			}
			if strings.Contains(out, "permissionDecision") || strings.Count(out, "\n") != 1 {
				t.Fatalf("the context object decides the call or is not one object: %q", out)
			}
		})
	}
	t.Run("the deadline and no sample", func(t *testing.T) {
		t.Parallel()
		late := 0
		probe, out, _, err := run(t, 120000, "deny", bashToolGateCall("rm x"), text, func(opts *ToolGateOptions) {
			opts.Clock = func() time.Time {
				late++
				if late < 5 {
					return birth.Add(99 * time.Millisecond)
				}
				return birth.Add(101 * time.Millisecond)
			}
		})
		if err != nil || out != contextObject || probe.marked != 1 {
			t.Fatalf("past the deadline: err=%v stdout=%q marked=%d", err, out, probe.marked)
		}
		root, _ := toolGateFixture(t, 120000)
		empty := filepath.Join(root, "empty.jsonl")
		if err := os.WriteFile(empty, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		stdout := &bytes.Buffer{}
		noSample := &peerProbe{text: text, out: stdout}
		opts := toolGateOptions(root, empty, "deny", birth, at, stdout)
		opts.Stdin = bytes.NewReader(toolGatePayloadBytes("no-sample", empty, bashToolGateCall("rm x"), ""))
		opts.Peer = noSample.offer
		if err := RunToolGate(opts); err != nil || stdout.String() != contextObject || noSample.marked != 1 {
			t.Fatalf("no sample: err=%v stdout=%q marked=%d", err, stdout.String(), noSample.marked)
		}
	})
	t.Run("a busy store", func(t *testing.T) {
		t.Parallel()
		root, transcript := toolGateFixture(t, 120000)
		lockPath := filepath.Join(root, "artifacts", "agents", "context", "maintenance.lock")
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			t.Fatal(err)
		}
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			t.Fatal(err)
		}
		defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		stdout := &bytes.Buffer{}
		probe := &peerProbe{text: text, out: stdout}
		opts := toolGateOptions(root, transcript, "deny", birth, at, stdout)
		opts.Stdin = bytes.NewReader(toolGatePayloadBytes("busy", transcript, bashToolGateCall("rm x"), ""))
		opts.Peer = probe.offer
		if err := RunToolGate(opts); err != nil || stdout.String() != contextObject || probe.marked != 1 {
			t.Fatalf("a busy store: err=%v stdout=%q marked=%d", err, stdout.String(), probe.marked)
		}
	})
	t.Run("an enforced denial carries no peer text", func(t *testing.T) {
		t.Parallel()
		root, transcript := toolGateFixture(t, 120000)
		probe, out, rows, err := runIn(t, root, transcript, "deny", bashToolGateCall("rm x"), text, nil)
		_, silent, silentRows, _ := runIn(t, root, transcript, "deny", bashToolGateCall("rm x"), "", nil)
		if err != nil || out != silent || probe.calls != 0 || probe.marked != 0 || !strings.Contains(out, `"permissionDecision":"deny"`) {
			t.Fatalf("denial with a pending message: err=%v stdout=%q (without: %q) calls=%d marked=%d", err, out, silent, probe.calls, probe.marked)
		}
		if rows != silentRows {
			t.Fatalf("the decision row changed with a pending message:\n%s\n%s", rows, silentRows)
		}
	})
	t.Run("nothing pending emits nothing", func(t *testing.T) {
		t.Parallel()
		root, transcript := toolGateFixture(t, 120000)
		probe, out, rows, err := runIn(t, root, transcript, "observe", bashToolGateCall("rm x"), "", nil)
		_, _, pendingRows, _ := runIn(t, root, transcript, "observe", bashToolGateCall("rm x"), text, nil)
		if err != nil || out != "" || probe.calls != 1 || probe.marked != 0 {
			t.Fatalf("nothing pending: err=%v stdout=%q calls=%d marked=%d", err, out, probe.calls, probe.marked)
		}
		if rows != pendingRows {
			t.Fatalf("the decision row changed with a pending message:\n%s\n%s", rows, pendingRows)
		}
	})
	t.Run("a subagent call and the helm never call Peer", func(t *testing.T) {
		t.Parallel()
		probe, out, _, err := run(t, 120000, "observe", bashToolGateCall("rm x"), text, func(opts *ToolGateOptions) {
			transcript := filepath.Join(opts.StateRoot, "transcript.jsonl")
			opts.Stdin = bytes.NewReader(toolGatePayloadBytes("subagent", transcript, bashToolGateCall("rm x"), "agent-1"))
		})
		if err != nil || out != "" || probe.calls != 0 {
			t.Fatalf("subagent: err=%v stdout=%q calls=%d", err, out, probe.calls)
		}
		probe, out, _, err = run(t, 120000, "observe", bashToolGateCall("rm x"), text, func(opts *ToolGateOptions) {
			if err := os.Mkdir(filepath.Join(opts.StateRoot, ".git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := helm.Write(opts.StateRoot, helm.Record{By: "Wido", At: birth.Format(time.RFC3339), Reason: "by hand"}); err != nil {
				t.Fatal(err)
			}
		})
		if err != nil || out != "" || probe.calls != 0 {
			t.Fatalf("helm: err=%v stdout=%q calls=%d", err, out, probe.calls)
		}
	})
	t.Run("a failing stdout marks nothing", func(t *testing.T) {
		t.Parallel()
		probe, _, _, err := run(t, 120000, "observe", bashToolGateCall("rm x"), text, func(opts *ToolGateOptions) { opts.Stdout = failingToolGateWriter{} })
		if err == nil || probe.marked != 0 {
			t.Fatalf("failing stdout: err=%v marked=%d", err, probe.marked)
		}
	})
}
