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

// A response nobody asked for is a mismatched ID — a protocol
// death that fails every pending call.
func TestConnUnknownResponseIDIsProtocolDeath(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	_, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, nil)
	defer clientWrites.Close()
	writer := NewWriter(serverWrites, nil)
	go writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(999), Result: json.RawMessage(`{}`)})
	<-conn.Done()
	if err := conn.Err(); err == nil || !strings.Contains(err.Error(), "unknown id") {
		t.Fatalf("mismatched response id must kill the connection: %v", err)
	}
}

func TestConnCallCancelAndClose(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, nil)
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	go func() {
		reader := NewReader(serverReads, nil)
		for {
			if _, err := reader.Next(); err != nil {
				return
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := conn.Call(ctx, "never/answered", nil); err != context.DeadlineExceeded {
		t.Fatalf("a cancelled call must surface the context error: %v", err)
	}

	// Peer close with a call in flight: the call fails with EOF.
	callErr := make(chan error, 1)
	go func() {
		_, err := conn.Call(context.Background(), "also/never", nil)
		callErr <- err
	}()
	time.Sleep(50 * time.Millisecond)
	serverWrites.Close()
	if err := <-callErr; err != io.EOF {
		t.Fatalf("peer close must fail pending calls with EOF: %v", err)
	}
}

func TestConnNotifyAndJournalSurface(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	conn := NewConn(clientReads, clientWrites, failingWriter{})
	received := make(chan *Message, 1)
	go func() {
		reader := NewReader(serverReads, nil)
		msg, _ := reader.Next()
		received <- msg
	}()
	// The journal is dead, so Notify must refuse (evidence precedes
	// action) and the failure must surface on the conn.
	if err := conn.Notify("session/cancel", map[string]any{"sessionId": "s"}); err == nil {
		t.Fatal("notify past a dead journal must fail")
	}
	if conn.JournalErr() == nil {
		t.Fatal("the journal failure must surface on the connection")
	}

	conn2 := NewConn(io.NopCloser(strings.NewReader("")), clientWrites, nil)
	if err := conn2.Notify("session/cancel", map[string]any{"sessionId": "s"}); err != nil {
		t.Fatalf("notify with a live journal path: %v", err)
	}
	msg := <-received
	if msg.Method != "session/cancel" || msg.ID != nil {
		t.Fatalf("cancel must be a notification: %+v", msg)
	}
}

// An unreadable prompt result is a protocol error, and late frames
// after the response stay out of the candidate while late requests
// get the settled refusal.
func TestTurnUnreadableResultAndLateWindow(t *testing.T) {
	conn, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", result: `{"stopReason":123}`},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowProtocolError {
		t.Fatalf("unreadable result: %+v", outcome)
	}

	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn2 := NewConn(clientReads, clientWrites, nil)
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	writer := NewWriter(serverWrites, nil)
	reader := NewReader(serverReads, nil)
	lateAnswer := make(chan *Message, 1)
	go func() {
		msg, _ := reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"stopReason":"end_turn"}`)})
		// A late chunk and a late request inside the drain window.
		writer.Send(&Message{JSONRPC: "2.0", Method: "session/update", Params: json.RawMessage(chunkFor("s-1", "too late"))})
		writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(5), Method: "session/request_permission", Params: json.RawMessage(`{"sessionId":"s-1","options":[]}`)})
		reply, _ := reader.Next()
		lateAnswer <- reply
	}()
	cfg := baseConfig()
	cfg.LateFrameWindow = 400 * time.Millisecond
	outcome := RunTurn(context.Background(), conn2, cfg)
	if outcome.Row != RowDelivered || outcome.Candidate != nil {
		t.Fatalf("an empty window with late-only chunks delivers no candidate: %+v %q", outcome, outcome.Candidate)
	}
	reply := <-lateAnswer
	if reply.Error == nil {
		t.Fatalf("a request after settlement gets the settled refusal: %+v", reply)
	}
}

// mustMarshal panics only on programming errors and passes nil
// through.
func TestMustMarshal(t *testing.T) {
	if mustMarshal(nil) != nil {
		t.Fatal("nil params stay nil")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("unmarshalable params are a programming error")
		}
	}()
	mustMarshal(map[string]any{"bad": func() {}})
}

func TestPromptFailureRowMapping(t *testing.T) {
	dead := &Conn{err: fmt.Errorf("malformed")}
	if promptFailureRow(io.EOF, dead) != RowTurnFailed {
		t.Fatal("EOF is a turn failure, chunks stay evidence")
	}
	if promptFailureRow(nil, dead) != RowProtocolError {
		t.Fatal("a non-EOF connection death is a protocol error")
	}
	clean := &Conn{err: io.EOF}
	if promptFailureRow(nil, clean) != RowTurnFailed {
		t.Fatal("a clean close without a call error is a turn failure")
	}
}

// The remaining setup edges: an unreadable session/new result and
// a send refused by a dead journal before anything hits the wire.
func TestSetupEdges(t *testing.T) {
	conn, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		{expectMethod: "session/new", result: `{"sessionId":42}`},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowProtocolError {
		t.Fatalf("unreadable session id: %+v", outcome)
	}

	_, clientWrites := io.Pipe()
	defer clientWrites.Close()
	dead := NewConn(strings.NewReader(""), clientWrites, failingWriter{})
	if _, err := dead.Call(context.Background(), "initialize", nil); err == nil {
		t.Fatal("a call past a dead journal must fail at send")
	}
}
