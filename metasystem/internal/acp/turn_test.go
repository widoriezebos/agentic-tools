package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// stubServer speaks the wire over pipes from a script: each step
// matches an expected client method, optionally validates params,
// and answers with notifications then a response (or a scripted
// misbehavior). Consumed steps are counted so a fixture cannot
// pass on canned responses it never earned.
type stubServer struct {
	t        *testing.T
	reader   *Reader
	writer   *Writer
	closer   io.Closer
	consumed chan int
}

type stubStep struct {
	expectMethod  string
	expectParams  []string // substrings that must appear in the params
	notifications []string // raw params JSON, sent as session/update before responding
	rawFrames     []string // raw frames sent verbatim before responding (misbehavior)
	result        string   // raw result JSON; empty means use errorCode
	errorCode     int64
	errorMessage  string
	request       string // raw params: a server->client request sent before responding
	dropAfter     bool   // close the pipe instead of responding
	silent        bool   // never respond (blocked-read fixture)
	seen          chan<- struct{}
}

func newStubTurn(t *testing.T, steps []stubStep) (*Conn, *stubServer, func()) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, &bytes.Buffer{})
	server := &stubServer{
		t:        t,
		reader:   NewReader(serverReads, nil),
		writer:   NewWriter(serverWrites, nil),
		closer:   serverWrites,
		consumed: make(chan int, 1),
	}
	go server.run(steps)
	return conn, server, func() { clientWrites.Close(); serverWrites.Close() }
}

func (s *stubServer) run(steps []stubStep) {
	requestID := int64(1000)
	count := 0
	defer func() { s.consumed <- count }()
	for _, step := range steps {
		msg, err := s.reader.Next()
		if err != nil {
			return
		}
		if msg.Method != step.expectMethod {
			s.t.Errorf("stub expected %s got %s", step.expectMethod, msg.Method)
			return
		}
		for _, want := range step.expectParams {
			if !strings.Contains(string(msg.Params), want) {
				s.t.Errorf("%s params %s missing %q", step.expectMethod, msg.Params, want)
			}
		}
		count++
		if step.seen != nil {
			step.seen <- struct{}{}
		}
		for _, raw := range step.rawFrames {
			s.writer.w.Write([]byte(raw + "\n"))
		}
		for _, params := range step.notifications {
			s.writer.Send(&Message{JSONRPC: "2.0", Method: "session/update", Params: json.RawMessage(params)})
		}
		if step.request != "" {
			requestID++
			s.writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(requestID), Method: "session/request_permission", Params: json.RawMessage(step.request)})
			// The client's answer arrives before it can settle the
			// prompt; consume it so the pipe does not block.
			if _, err := s.reader.Next(); err != nil {
				return
			}
		}
		if step.dropAfter {
			s.closer.Close()
			return
		}
		if step.silent {
			continue
		}
		response := &Message{JSONRPC: "2.0", ID: msg.ID}
		if step.result != "" {
			response.Result = json.RawMessage(step.result)
		} else {
			response.Error = &WireError{Code: step.errorCode, Message: step.errorMessage}
		}
		s.writer.Send(response)
	}
}

// requireConsumed asserts the stub actually served every scripted
// step — a green fixture must have earned its responses.
func (s *stubServer) requireConsumed(want int) {
	select {
	case got := <-s.consumed:
		if got != want {
			s.t.Fatalf("stub consumed %d steps, fixture requires %d", got, want)
		}
	case <-time.After(5 * time.Second):
		s.t.Fatal("stub never finished")
	}
}

func baseConfig() TurnConfig {
	return TurnConfig{
		ExpectedProtocolVersion: 1,
		WorkspaceDir:            "/work",
		Prompt:                  "do the thing",
		HandshakeTimeout:        5 * time.Second,
		PromptTimeout:           5 * time.Second,
		LateFrameWindow:         50 * time.Millisecond,
	}
}

func initStep(authMethods string) stubStep {
	return stubStep{
		expectMethod: "initialize",
		expectParams: []string{`"protocolVersion":1`, `"readTextFile":false`},
		result:       fmt.Sprintf(`{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":%s}`, authMethods),
	}
}

func newSessionStep() stubStep {
	return stubStep{expectMethod: "session/new", expectParams: []string{`"cwd":"/work"`}, result: `{"sessionId":"s-1"}`}
}

func chunkFor(session, text string) string {
	return fmt.Sprintf(`{"sessionId":%q,"update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":%q}}}`, session, text)
}

func TestTurnDelivered(t *testing.T) {
	conn, server, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{
			expectMethod:  "session/prompt",
			expectParams:  []string{`"sessionId":"s-1"`, "do the thing"},
			notifications: []string{chunkFor("s-1", "the "), chunkFor("s-1", "answer")},
			result:        `{"stopReason":"end_turn","usage":{"inputTokens":10,"outputTokens":2,"totalTokens":12}}`,
		},
	})
	defer cleanup()
	outcome := RunTurn(context.Background(), conn, baseConfig())
	if outcome.Row != RowDelivered || string(outcome.Candidate) != "the answer" {
		t.Fatalf("%+v", outcome)
	}
	if !strings.Contains(string(outcome.UsageResult), "totalTokens") {
		t.Fatalf("PromptResponse.usage must be captured verbatim: %s", outcome.UsageResult)
	}
	if outcome.SessionID != "s-1" || outcome.Violations != 0 {
		t.Fatalf("%+v", outcome)
	}
	cleanup()
	server.requireConsumed(3)
}

func TestTurnVersionMismatch(t *testing.T) {
	conn, _, cleanup := newStubTurn(t, []stubStep{
		{expectMethod: "initialize", result: `{"protocolVersion":2,"authMethods":[]}`},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowVersionMismatch {
		t.Fatalf("%+v", outcome)
	}
}

// Auth classification keys on the pinned schema's -32000 code —
// advertisement alone proves nothing (the live probe created
// sessions with methods advertised).
func TestTurnAuthClassification(t *testing.T) {
	conn, _, cleanup := newStubTurn(t, []stubStep{
		initStep(`[{"id":"browser","name":"Log in with browser"}]`),
		{expectMethod: "session/new", errorCode: authRequiredCode, errorMessage: "sign in required"},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowAuthRequired {
		t.Fatalf("%+v", outcome)
	}

	// A non-auth error stays a setup error even with methods
	// advertised.
	conn, _, cleanup = newStubTurn(t, []stubStep{
		initStep(`[{"id":"browser","name":"Log in with browser"}]`),
		{expectMethod: "session/new", errorCode: -32603, errorMessage: "boom"},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowSetupError {
		t.Fatalf("%+v", outcome)
	}
}

// Requesting a load from a server that never declared loadSession
// is refused client-side.
func TestTurnLoadCapabilityGate(t *testing.T) {
	conn, _, cleanup := newStubTurn(t, []stubStep{
		{expectMethod: "initialize", result: `{"protocolVersion":1,"agentCapabilities":{"loadSession":false},"authMethods":[]}`},
	})
	defer cleanup()
	cfg := baseConfig()
	cfg.LoadSessionID = "old"
	outcome := RunTurn(context.Background(), conn, cfg)
	if outcome.Row != RowSetupError || !strings.Contains(outcome.Detail, "loadSession") {
		t.Fatalf("%+v", outcome)
	}
}

// A mode that cannot be set means the envelope's grade cannot be
// applied — the turn must not proceed (the mode IS the lever).
func TestTurnSetModeFailure(t *testing.T) {
	conn, _, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/set_mode", expectParams: []string{`"modeId":"ask"`}, errorCode: -32602, errorMessage: "no such mode"},
	})
	defer cleanup()
	cfg := baseConfig()
	cfg.ModeID = "ask"
	if outcome := RunTurn(context.Background(), conn, cfg); outcome.Row != RowSetupError {
		t.Fatalf("%+v", outcome)
	}
}

// The end-to-end fence: replay delivered before the prompt was
// sent never enters the candidate; only the post-prompt chunk
// does — the captured replay wire as a protocol-level fixture.
func TestTurnLoadReplayWatermark(t *testing.T) {
	conn, _, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		{
			expectMethod:  "session/load",
			expectParams:  []string{`"sessionId":"old-session"`},
			notifications: []string{chunkFor("old-session", "stale prior answer")},
			result:        `{}`,
		},
		{
			expectMethod:  "session/prompt",
			notifications: []string{chunkFor("old-session", "fresh answer")},
			result:        `{"stopReason":"end_turn"}`,
		},
	})
	defer cleanup()
	cfg := baseConfig()
	cfg.LoadSessionID = "old-session"
	outcome := RunTurn(context.Background(), conn, cfg)
	if outcome.Row != RowDelivered || string(outcome.Candidate) != "fresh answer" {
		t.Fatalf("the stale answer must not win: %+v candidate=%q", outcome, outcome.Candidate)
	}
}

func TestTurnStopReasonRows(t *testing.T) {
	for stop, want := range map[string]Row{
		"cancelled":         RowCancelled,
		"refusal":           RowRefused,
		"max_tokens":        RowIncomplete,
		"max_turn_requests": RowIncomplete,
		"martian":           RowProtocolError,
	} {
		conn, _, cleanup := newStubTurn(t, []stubStep{
			initStep("[]"),
			newSessionStep(),
			{expectMethod: "session/prompt", result: fmt.Sprintf(`{"stopReason":%q}`, stop)},
		})
		outcome := RunTurn(context.Background(), conn, baseConfig())
		cleanup()
		if outcome.Row != want {
			t.Fatalf("stop %s: got %+v want %s", stop, outcome, want)
		}
	}
}

func TestTurnPromptErrorAndDeath(t *testing.T) {
	conn, _, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", errorCode: -32603, errorMessage: "inference exploded"},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowTurnFailed {
		t.Fatalf("prompt error: %+v", outcome)
	}

	// EOF before the response: turn failed, chunks stay evidence.
	conn, _, cleanup = newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", notifications: []string{chunkFor("s-1", "partial")}, dropAfter: true},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowTurnFailed {
		t.Fatalf("EOF mid-prompt: %+v", outcome)
	}

	// A malformed frame mid-prompt is a protocol death.
	conn, _, cleanup = newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", rawFrames: []string{"garbage-not-json"}, silent: true},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowProtocolError {
		t.Fatalf("malformed mid-prompt: %+v", outcome)
	}

	// A silent server (blocked read) hits the prompt deadline.
	conn, _, cleanup = newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", silent: true},
	})
	defer cleanup()
	cfg := baseConfig()
	cfg.PromptTimeout = 200 * time.Millisecond
	if outcome := RunTurn(context.Background(), conn, cfg); outcome.Row != RowProtocolError {
		t.Fatalf("deadline: %+v", outcome)
	}
}

// Parent cancellation sends the courtesy cancel and the turn
// settles cancelled (the kill path owns the rest).
func TestTurnParentCancellation(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, &bytes.Buffer{})
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	writer := NewWriter(serverWrites, nil)
	reader := NewReader(serverReads, nil)
	sawCancel := make(chan bool, 1)
	promptSeen := make(chan struct{})
	go func() {
		msg, _ := reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		reader.Next() // the prompt; never answered
		close(promptSeen)
		msg, err := reader.Next()
		sawCancel <- err == nil && msg.Method == "session/cancel" && msg.ID == nil
	}()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-promptSeen
		cancel()
	}()
	cfg := baseConfig()
	cfg.CancelGrace = 300 * time.Millisecond
	outcome := RunTurn(ctx, conn, cfg)
	if outcome.Row != RowCancelled {
		t.Fatalf("%+v", outcome)
	}
	if !<-sawCancel {
		t.Fatal("the courtesy session/cancel notification must reach the wire")
	}
}

// The wedge scenarios: a server that requires its request
// answered BEFORE it responds to setup, and a notification flood
// past the channel buffer — neither may wedge the turn, because
// the driver services both queues while its own call waits.
func TestTurnSetupServicesQueues(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, &bytes.Buffer{})
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	writer := NewWriter(serverWrites, nil)
	reader := NewReader(serverReads, nil)
	go func() {
		msg, _ := reader.Next() // initialize
		initID := msg.ID
		// The server demands an answer before it will respond.
		writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(2000), Method: "session/request_permission", Params: json.RawMessage(`{"sessionId":"nobody","options":[]}`)})
		if reply, err := reader.Next(); err != nil || reply.ID.Key() != "2000" {
			panic("the driver must answer during setup")
		}
		// And floods notifications past the 4096 buffer.
		for i := 0; i < 4200; i++ {
			writer.Send(&Message{JSONRPC: "2.0", Method: "session/update", Params: json.RawMessage(chunkFor("s-1", "n"))})
		}
		writer.Send(&Message{JSONRPC: "2.0", ID: initID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"stopReason":"end_turn"}`)})
	}()
	cfg := baseConfig()
	cfg.HandshakeTimeout = 15 * time.Second
	outcome := RunTurn(context.Background(), conn, cfg)
	if outcome.Row != RowDelivered || outcome.Violations != 1 {
		t.Fatalf("%+v", outcome)
	}
}

// Unsolicited server→client requests fail closed and count as
// violations; the turn itself continues.
func TestTurnUnsolicitedRequestFailsClosed(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, &bytes.Buffer{})
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	writer := NewWriter(serverWrites, nil)
	reader := NewReader(serverReads, nil)
	go func() {
		msg, _ := reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		msg, _ = reader.Next()
		promptID := msg.ID
		// An fs read this client never advertised.
		writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(7777), Method: "fs/read_text_file", Params: json.RawMessage(`{"path":"/etc/passwd"}`)})
		reply, _ := reader.Next()
		if reply.Error == nil || reply.ID.Key() != "7777" {
			panic("unsolicited request must be answered with a JSON-RPC error")
		}
		writer.Send(&Message{JSONRPC: "2.0", ID: promptID, Result: json.RawMessage(`{"stopReason":"end_turn"}`)})
	}()
	outcome := RunTurn(context.Background(), conn, baseConfig())
	if outcome.Row != RowDelivered || outcome.Violations != 1 {
		t.Fatalf("%+v", outcome)
	}
}

// The correlation gate and the strict posture: a wrong-session
// permission request is a violation answered cancelled; an
// in-window one takes the strict deny branch by exact option id.
// The fixtures carry the schema's required option names and a
// toolCall member so a green test reflects real request shapes.
func TestTurnPermissionGateAndStrictAnswer(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, &bytes.Buffer{})
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	writer := NewWriter(serverWrites, nil)
	reader := NewReader(serverReads, nil)
	answers := make(chan string, 2)
	go func() {
		msg, _ := reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		msg, _ = reader.Next()
		promptID := msg.ID
		writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(1), Method: "session/request_permission", Params: json.RawMessage(`{"sessionId":"SOMEONE-ELSE","toolCall":{"toolCallId":"t1"},"options":[{"optionId":"r1","kind":"reject_once","name":"Reject"}]}`)})
		reply, _ := reader.Next()
		answers <- string(reply.Result)
		writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(2), Method: "session/request_permission", Params: json.RawMessage(`{"sessionId":"s-1","toolCall":{"toolCallId":"t2"},"options":[{"optionId":"a1","kind":"allow_once","name":"Allow"},{"optionId":"r1","kind":"reject_once","name":"Reject"}]}`)})
		reply, _ = reader.Next()
		answers <- string(reply.Result)
		writer.Send(&Message{JSONRPC: "2.0", ID: promptID, Result: json.RawMessage(`{"stopReason":"end_turn"}`)})
	}()
	outcome := RunTurn(context.Background(), conn, baseConfig())
	if outcome.Row != RowDelivered || outcome.Violations != 1 {
		t.Fatalf("%+v", outcome)
	}
	foreign := <-answers
	if !strings.Contains(foreign, `"cancelled"`) {
		t.Fatalf("wrong-session request must be cancelled: %s", foreign)
	}
	strict := <-answers
	if !strings.Contains(strict, `"selected"`) || !strings.Contains(strict, `"r1"`) {
		t.Fatalf("strict answer must select reject_once by id: %s", strict)
	}
}

// The journal survives concurrent directions intact: every line
// parses as one well-formed prefixed frame.
func TestJournalIntegrityUnderConcurrency(t *testing.T) {
	var journal bytes.Buffer
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, &journal)
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	writer := NewWriter(serverWrites, nil)
	reader := NewReader(serverReads, nil)
	go func() {
		msg, _ := reader.Next()
		// Flood notifications while the response goes out.
		for i := 0; i < 200; i++ {
			writer.Send(&Message{JSONRPC: "2.0", Method: "session/update", Params: json.RawMessage(chunkFor("s-1", "x"))})
		}
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"stopReason":"end_turn"}`)})
	}()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowDelivered {
		t.Fatalf("%+v", outcome)
	}
	for lineNumber, line := range strings.Split(strings.TrimSuffix(journal.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "> ") && !strings.HasPrefix(line, "< ") && !strings.HasPrefix(line, "<!") {
			t.Fatalf("journal line %d corrupt: %q", lineNumber+1, line)
		}
		payload := line[2:]
		if strings.HasPrefix(line, "<!") {
			continue
		}
		var check map[string]any
		if err := json.Unmarshal([]byte(payload), &check); err != nil {
			t.Fatalf("journal line %d is not one frame: %q", lineNumber+1, line)
		}
	}
}

// A server that stops reading cannot defeat deadlines: the write
// hand-off is context-bounded even while the physical write is
// wedged.
func TestBlockedWriteIsBounded(t *testing.T) {
	clientReads, _ := io.Pipe()
	_, clientWrites := io.Pipe() // nobody ever reads this side
	conn := NewConn(clientReads, clientWrites, nil)
	defer clientWrites.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := conn.CallSeq(ctx, "initialize", map[string]any{"big": strings.Repeat("x", 1024*1024)})
	if err != context.DeadlineExceeded {
		t.Fatalf("blocked write must surface the deadline: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("the wait was not bounded by the context")
	}
}

// The session file lands at setup success — the adapter's early
// handshake — long before the prompt settles.
func TestTurnSessionFileEarlyHandshake(t *testing.T) {
	dir := t.TempDir()
	sessionFile := dir + "/session-id"
	conn, _, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", result: `{"stopReason":"end_turn"}`},
	})
	defer cleanup()
	cfg := baseConfig()
	cfg.SessionFile = sessionFile
	outcome := RunTurn(context.Background(), conn, cfg)
	if outcome.Row != RowDelivered {
		t.Fatalf("%+v", outcome)
	}
	body, err := os.ReadFile(sessionFile)
	if err != nil || strings.TrimSpace(string(body)) != "s-1" {
		t.Fatalf("session file must carry the id: %q err %v", body, err)
	}
}
