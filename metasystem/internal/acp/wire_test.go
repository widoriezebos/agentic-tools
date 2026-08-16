package acp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestWireRoundTripAndClassify(t *testing.T) {
	var buffer, journal bytes.Buffer
	writer := NewWriter(&buffer, &journal)
	if err := writer.Send(&Message{JSONRPC: "2.0", ID: NewRequestID(7), Method: "initialize"}); err != nil {
		t.Fatal(err)
	}
	reader := NewReader(&buffer, &journal)
	msg, err := reader.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Classify() != KindServerRequest || msg.ID.Key() != "7" {
		t.Fatalf("round trip broke: %+v", msg)
	}
	if !strings.Contains(journal.String(), "> {") || !strings.Contains(journal.String(), "< {") {
		t.Fatalf("both directions must be journaled: %q", journal.String())
	}

	for raw, want := range map[string]Kind{
		`{"jsonrpc":"2.0","id":1,"result":{}}`:                                  KindResponse,
		`{"jsonrpc":"2.0","id":"str-id","result":{}}`:                           KindResponse,
		`{"jsonrpc":"2.0","id":null,"method":"x"}`:                              KindServerRequest,
		`{"jsonrpc":"2.0","id":2,"method":"session/request_permission"}`:        KindServerRequest,
		`{"jsonrpc":"2.0","method":"session/update"}`:                           KindNotification,
		`{"jsonrpc":"2.0","id":3,"result":{},"error":{"code":1,"message":"x"}}`: KindMalformed,
		`{"jsonrpc":"2.0","id":4}`:                                              KindMalformed,
		`{"jsonrpc":"1.0","id":5,"result":{}}`:                                  KindMalformed,
		`{"jsonrpc":"2.0"}`:                                                     KindMalformed,
	} {
		reader := NewReader(strings.NewReader(raw+"\n"), nil)
		msg, err := reader.Next()
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if msg.Classify() != want {
			t.Fatalf("%s classified %v want %v", raw, msg.Classify(), want)
		}
	}
}

// A null id is a PRESENT id (a request), distinguishable from an
// absent one (a notification), and echoes back as null.
func TestRequestIDShapes(t *testing.T) {
	reader := NewReader(strings.NewReader(`{"jsonrpc":"2.0","id":null,"method":"m"}`+"\n"), nil)
	msg, err := reader.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID == nil || msg.ID.Key() != "null" {
		t.Fatalf("null id lost: %+v", msg.ID)
	}
	echoed, _ := msg.ID.MarshalJSON()
	if string(echoed) != "null" {
		t.Fatalf("null id must echo as null, got %s", echoed)
	}

	reader = NewReader(strings.NewReader(`{"jsonrpc":"2.0","id":"abc","method":"m"}`+"\n"), nil)
	msg, err = reader.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID.Key() != `"abc"` {
		t.Fatalf("string id lost: %q", msg.ID.Key())
	}

	reader = NewReader(strings.NewReader(`{"jsonrpc":"2.0","id":{"bad":1},"method":"m"}`+"\n"), nil)
	if _, err = reader.Next(); err == nil {
		t.Fatal("an object id must be rejected as malformed")
	}
}

// The ceiling is symmetric: a frame the writer accepts must round
// trip through the reader.
func TestMaxFrameRoundTrip(t *testing.T) {
	padding := strings.Repeat("x", maxFrameBytes-100)
	frame := fmt.Sprintf(`{"jsonrpc":"2.0","method":"m","params":{"p":%q}}`, padding)
	if len(frame) > maxFrameBytes {
		t.Fatal("test construction error")
	}
	reader := NewReader(strings.NewReader(frame+"\n"), nil)
	msg, err := reader.Next()
	if err != nil {
		t.Fatalf("a writer-legal frame failed to read: %v", err)
	}
	if msg.Classify() != KindNotification {
		t.Fatal("frame mangled in transit")
	}
}

func TestWireMalformedLimitsAndEvidence(t *testing.T) {
	reader := NewReader(strings.NewReader("not-json\n"), nil)
	if _, err := reader.Next(); err == nil || !strings.Contains(err.Error(), "not-json") {
		t.Fatalf("malformed frame must error with the raw bytes as evidence: %v", err)
	}

	reader = NewReader(strings.NewReader(""), nil)
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("EOF must surface as io.EOF: %v", err)
	}

	// Oversize: the consumed prefix is journaled as evidence, the
	// line is discarded, and the NEXT frame is still readable.
	var journal bytes.Buffer
	huge := strings.Repeat("x", maxFrameBytes+10)
	reader = NewReader(strings.NewReader(huge+"\n"+`{"jsonrpc":"2.0","method":"after"}`+"\n"), &journal)
	if _, err := reader.Next(); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("over-ceiling frame must be ErrFrameTooLarge: %v", err)
	}
	if !strings.Contains(journal.String(), "<!oversize") {
		t.Fatal("the oversize prefix must be journaled as evidence")
	}
	msg, err := reader.Next()
	if err != nil || msg.Method != "after" {
		t.Fatalf("the stream must recover at the next frame boundary: %v %+v", err, msg)
	}

	// A torn tail at EOF is evidence, never a frame.
	journal.Reset()
	reader = NewReader(strings.NewReader(`{"jsonrpc":"2.0","method":"cut`), &journal)
	if _, err := reader.Next(); !errors.Is(err, ErrPartialFrame) {
		t.Fatalf("unterminated final bytes must be ErrPartialFrame: %v", err)
	}
	if !strings.Contains(journal.String(), "<!torn") {
		t.Fatal("the torn tail must be journaled as evidence")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// Journal failures are never silent: the writer refuses to send
// past a dead journal, and the reader surfaces stickiness.
func TestJournalFailuresSurface(t *testing.T) {
	var wire bytes.Buffer
	writer := NewWriter(&wire, failingWriter{})
	if err := writer.Send(&Message{JSONRPC: "2.0", Method: "m"}); err == nil {
		t.Fatal("a send past a dead journal must fail — evidence precedes action")
	}
	if writer.JournalErr() == nil {
		t.Fatal("the journal error must be sticky on the writer")
	}
	if wire.Len() != 0 {
		t.Fatal("no bytes may reach the wire after the journal refused")
	}

	reader := NewReader(strings.NewReader(`{"jsonrpc":"2.0","method":"m"}`+"\n"), failingWriter{})
	if _, err := reader.Next(); err != nil {
		t.Fatalf("reading continues past a dead journal (caller's choice): %v", err)
	}
	if reader.JournalErr() == nil {
		t.Fatal("the journal error must be sticky on the reader")
	}
}

// The remaining edge branches: nil-ID marshal/key, empty and
// invalid ID parses, a multi-buffer oversize line whose remainder
// must be discarded to the boundary, and a writer-side oversize
// refusal.
func TestWireEdgeBranches(t *testing.T) {
	var nilID *RequestID
	if b, _ := nilID.MarshalJSON(); string(b) != "null" {
		t.Fatalf("nil id marshals as null: %s", b)
	}
	if nilID.Key() != "" {
		t.Fatal("nil id key must be empty")
	}
	if _, err := parseRequestID([]byte("  ")); err == nil {
		t.Fatal("blank id must be refused")
	}
	if _, err := parseRequestID([]byte("[1]")); err == nil {
		t.Fatal("array id must be refused")
	}

	// Oversize spanning many reader buffers: the discard loop must
	// consume to the line boundary so the next frame is intact.
	var journal bytes.Buffer
	huge := strings.Repeat("y", maxFrameBytes+256*1024)
	reader := NewReader(strings.NewReader(huge+"\n"+`{"jsonrpc":"2.0","method":"tail"}`+"\n"), &journal)
	if _, err := reader.Next(); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("multi-buffer oversize: %v", err)
	}
	msg, err := reader.Next()
	if err != nil || msg.Method != "tail" {
		t.Fatalf("discard must stop at the boundary: %v %+v", err, msg)
	}

	writer := NewWriter(&bytes.Buffer{}, nil)
	big := strings.Repeat("z", maxFrameBytes)
	if err := writer.Send(&Message{JSONRPC: "2.0", Method: "m", Params: json.RawMessage(fmt.Sprintf("%q", big))}); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("writer must refuse over-ceiling frames: %v", err)
	}
}
