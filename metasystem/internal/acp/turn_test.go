package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// stubServer speaks the wire over pipes from a script: each step
// matches an expected client method and answers with notifications
// then a response (or a scripted misbehavior). It is the fixture
// engine for every matrix row the protocol level can reach.
type stubServer struct {
	t      *testing.T
	reader *Reader
	writer *Writer
	closer io.Closer
}

type stubStep struct {
	expectMethod  string
	notifications []string // raw params JSON, sent as session/update before responding
	rawFrames     []string // raw frames sent verbatim before responding (misbehavior)
	result        string   // raw result JSON; empty means use errorCode
	errorCode     int64
	errorMessage  string
	request       string // raw params: a server->client request sent before responding
	dropAfter     bool   // close the pipe instead of responding
	silent        bool   // never respond (blocked-read fixture)
}

func newStubTurn(t *testing.T, steps []stubStep) (*Conn, func()) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, nil)
	server := &stubServer{
		t:      t,
		reader: NewReader(serverReads, nil),
		writer: NewWriter(serverWrites, nil),
		closer: serverWrites,
	}
	go server.run(steps)
	return conn, func() { clientWrites.Close(); serverWrites.Close() }
}

func (s *stubServer) run(steps []stubStep) {
	requestID := int64(1000)
	for _, step := range steps {
		msg, err := s.reader.Next()
		if err != nil {
			return
		}
		if msg.Method != step.expectMethod {
			s.t.Errorf("stub expected %s got %s", step.expectMethod, msg.Method)
			return
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
		result:       fmt.Sprintf(`{"protocolVersion":1,"agentCapabilities":{"loadSession":true},"authMethods":%s}`, authMethods),
	}
}

func newSessionStep() stubStep {
	return stubStep{expectMethod: "session/new", result: `{"sessionId":"s-1"}`}
}

func chunkFor(session, text string) string {
	return fmt.Sprintf(`{"sessionId":%q,"update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":%q}}}`, session, text)
}

func TestTurnDelivered(t *testing.T) {
	conn, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{
			expectMethod:  "session/prompt",
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
}

func TestTurnVersionMismatch(t *testing.T) {
	conn, cleanup := newStubTurn(t, []stubStep{
		{expectMethod: "initialize", result: `{"protocolVersion":2,"authMethods":[]}`},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowVersionMismatch {
		t.Fatalf("%+v", outcome)
	}
}

// The auth-required row fires only when auth methods were
// advertised; the same refusal without them is a plain setup error.
func TestTurnAuthClassification(t *testing.T) {
	conn, cleanup := newStubTurn(t, []stubStep{
		initStep(`[{"id":"browser"}]`),
		{expectMethod: "session/new", errorCode: 401, errorMessage: "sign in"},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowAuthRequired {
		t.Fatalf("%+v", outcome)
	}

	conn, cleanup = newStubTurn(t, []stubStep{
		initStep("[]"),
		{expectMethod: "session/new", errorCode: -32000, errorMessage: "boom"},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowSetupError {
		t.Fatalf("%+v", outcome)
	}
}

// A mode that cannot be set means the envelope's grade cannot be
// applied — the turn must not proceed (the mode IS the lever).
func TestTurnSetModeFailure(t *testing.T) {
	conn, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/set_mode", errorCode: -32602, errorMessage: "no such mode"},
	})
	defer cleanup()
	cfg := baseConfig()
	cfg.ModeID = "ask"
	if outcome := RunTurn(context.Background(), conn, cfg); outcome.Row != RowSetupError {
		t.Fatalf("%+v", outcome)
	}
}

// The end-to-end watermark: replay queued before the load response
// never enters the candidate; only the post-prompt chunk does —
// probe step B as a protocol-level fixture.
func TestTurnLoadReplayWatermark(t *testing.T) {
	conn, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		{
			expectMethod:  "session/load",
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
		conn, cleanup := newStubTurn(t, []stubStep{
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
	conn, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", errorCode: -32603, errorMessage: "inference exploded"},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowTurnFailed {
		t.Fatalf("prompt error: %+v", outcome)
	}

	// EOF before the response: turn failed, chunks stay evidence.
	conn, cleanup = newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", notifications: []string{chunkFor("s-1", "partial")}, dropAfter: true},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowTurnFailed {
		t.Fatalf("EOF mid-prompt: %+v", outcome)
	}

	// A malformed frame mid-prompt is a protocol death.
	conn, cleanup = newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", rawFrames: []string{"garbage-not-json"}, silent: true},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowProtocolError {
		t.Fatalf("malformed mid-prompt: %+v", outcome)
	}

	// A silent server (blocked read) hits the prompt deadline.
	conn, cleanup = newStubTurn(t, []stubStep{
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

// Unsolicited server→client requests fail closed and count as
// violations; the turn itself continues.
func TestTurnUnsolicitedRequestFailsClosed(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, nil)
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	writer := NewWriter(serverWrites, nil)
	reader := NewReader(serverReads, nil)
	go func() {
		msg, _ := reader.Next() // initialize
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next() // session/new
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		msg, _ = reader.Next() // session/prompt
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
// in-session one takes the strict deny branch by exact option id.
func TestTurnPermissionGateAndStrictAnswer(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, nil)
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
		writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(1), Method: "session/request_permission", Params: json.RawMessage(`{"sessionId":"SOMEONE-ELSE","options":[{"optionId":"r1","kind":"reject_once"}]}`)})
		reply, _ := reader.Next()
		answers <- string(reply.Result)
		writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(2), Method: "session/request_permission", Params: json.RawMessage(`{"sessionId":"s-1","options":[{"optionId":"a1","kind":"allow_once"},{"optionId":"r1","kind":"reject_once"}]}`)})
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
