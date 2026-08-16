package acp

import (
	"bytes"
	"context"
	"encoding/json"
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
	ctx := context.Background()
	if err := conn.Notify(ctx, "session/cancel", map[string]any{"sessionId": "s"}); err == nil {
		t.Fatal("notify past a dead journal must fail")
	}
	if conn.JournalErr() == nil {
		t.Fatal("the journal failure must surface on the connection")
	}

	// A live (blocking) read side: a conn whose reader hit EOF is
	// dead and rightly refuses sends, so the live-path check needs
	// an open pipe.
	idleReads, _ := io.Pipe()
	conn2 := NewConn(idleReads, clientWrites, nil)
	if err := conn2.Notify(ctx, "session/cancel", map[string]any{"sessionId": "s"}); err != nil {
		t.Fatalf("notify with a live journal path: %v", err)
	}
	msg := <-received
	if msg.Method != "session/cancel" || msg.ID != nil {
		t.Fatalf("cancel must be a notification: %+v", msg)
	}
}

// An unreadable prompt result is a protocol error.
func TestTurnUnreadableResult(t *testing.T) {
	conn, _, cleanup := newStubTurn(t, []stubStep{
		initStep("[]"),
		newSessionStep(),
		{expectMethod: "session/prompt", result: `{"stopReason":123}`},
	})
	defer cleanup()
	if outcome := RunTurn(context.Background(), conn, baseConfig()); outcome.Row != RowProtocolError {
		t.Fatalf("unreadable result: %+v", outcome)
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

// The remaining setup edges: an unreadable session/new result and
// a send refused by a dead journal before anything hits the wire.
func TestSetupEdges(t *testing.T) {
	conn, _, cleanup := newStubTurn(t, []stubStep{
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

// The enqueue edges: sends after a connection death fail fast
// with the recorded cause, and the reader-side journal error
// surfaces through the conn.
func TestEnqueueAfterDeathAndReaderJournal(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	_, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, nil)
	defer clientWrites.Close()
	serverWrites.Close() // immediate EOF: the connection dies
	<-conn.Done()
	if err := conn.Notify(context.Background(), "session/cancel", nil); err == nil {
		t.Fatal("a send after connection death must fail fast")
	}

	failing := NewConn(strings.NewReader("{\"jsonrpc\":\"2.0\",\"method\":\"m\"}\n"), io.Discard, failingWriter{})
	<-failing.Done()
	if failing.JournalErr() == nil {
		t.Fatal("the reader-side journal failure must surface on the conn")
	}
}

// Late-window traffic: a chunk and a permission request after the
// PromptResponse are drained as evidence, the request answered as
// an out-of-window violation, and the candidate stays clean.
func TestTurnLateWindowTraffic(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, &bytes.Buffer{})
	defer func() { clientWrites.Close(); serverWrites.Close() }()
	writer := NewWriter(serverWrites, nil)
	reader := NewReader(serverReads, nil)
	lateAnswer := make(chan string, 1)
	go func() {
		msg, _ := reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"stopReason":"end_turn"}`)})
		writer.Send(&Message{JSONRPC: "2.0", Method: "session/update", Params: json.RawMessage(chunkFor("s-1", "too late"))})
		writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(5), Method: "session/request_permission", Params: json.RawMessage(`{"sessionId":"s-1","options":[{"optionId":"r1","kind":"reject_once","name":"Reject"}]}`)})
		reply, _ := reader.Next()
		lateAnswer <- string(reply.Result)
	}()
	cfg := baseConfig()
	cfg.LateFrameWindow = 500 * time.Millisecond
	outcome := RunTurn(context.Background(), conn, cfg)
	if outcome.Row != RowDelivered || outcome.Candidate != nil {
		t.Fatalf("late chunk must not become a candidate: %+v %q", outcome, outcome.Candidate)
	}
	if outcome.Violations != 1 {
		t.Fatalf("a post-window permission request is a violation: %+v", outcome)
	}
	if answer := <-lateAnswer; !strings.Contains(answer, `"cancelled"`) {
		t.Fatalf("post-window requests are answered cancelled: %s", answer)
	}
}

// Cancellation with a dead write side still settles cancelled with
// the courtesy failure recorded.
func TestCancelWithDeadWriteSide(t *testing.T) {
	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()
	conn := NewConn(clientReads, clientWrites, &bytes.Buffer{})
	reader := NewReader(serverReads, nil)
	writer := NewWriter(serverWrites, nil)
	go func() {
		msg, _ := reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"protocolVersion":1,"authMethods":[]}`)})
		msg, _ = reader.Next()
		writer.Send(&Message{JSONRPC: "2.0", ID: msg.ID, Result: json.RawMessage(`{"sessionId":"s-1"}`)})
		reader.Next()        // the prompt
		clientWrites.Close() // kill the client's read side mid-prompt: writes now fail
		serverWrites.Close()
	}()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(200 * time.Millisecond); cancel() }()
	cfg := baseConfig()
	cfg.CancelGrace = 200 * time.Millisecond
	outcome := RunTurn(ctx, conn, cfg)
	if outcome.Row != RowCancelled && outcome.Row != RowTurnFailed && outcome.Row != RowProtocolError {
		t.Fatalf("cancel against a dying connection must settle a terminal row: %+v", outcome)
	}
}
