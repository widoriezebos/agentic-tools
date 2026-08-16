package acp

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestWireRoundTripAndClassify(t *testing.T) {
	var buffer, journal bytes.Buffer
	writer := NewWriter(&buffer, &journal)
	id := int64(7)
	if err := writer.Send(&Message{JSONRPC: "2.0", ID: &id, Method: "initialize"}); err != nil {
		t.Fatal(err)
	}
	reader := NewReader(&buffer, &journal)
	msg, err := reader.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Classify() != KindServerRequest || *msg.ID != 7 {
		t.Fatalf("round trip broke: %+v", msg)
	}
	if !strings.Contains(journal.String(), "> {") || !strings.Contains(journal.String(), "< {") {
		t.Fatalf("both directions must be journaled: %q", journal.String())
	}

	for raw, want := range map[string]Kind{
		`{"jsonrpc":"2.0","id":1,"result":{}}`:                           Kind(KindResponse),
		`{"jsonrpc":"2.0","id":2,"method":"session/request_permission"}`: KindServerRequest,
		`{"jsonrpc":"2.0","method":"session/update"}`:                    KindNotification,
		`{"jsonrpc":"2.0"}`: KindMalformed,
	} {
		reader := NewReader(strings.NewReader(raw+"\n"), nil)
		msg, err := reader.Next()
		if err != nil {
			t.Fatal(err)
		}
		if msg.Classify() != want {
			t.Fatalf("%s classified %v want %v", raw, msg.Classify(), want)
		}
	}
}

func TestWireMalformedAndLimits(t *testing.T) {
	reader := NewReader(strings.NewReader("not-json\n"), nil)
	if _, err := reader.Next(); err == nil || !strings.Contains(err.Error(), "not-json") {
		t.Fatalf("malformed frame must error with the raw bytes as evidence: %v", err)
	}

	reader = NewReader(strings.NewReader(""), nil)
	if _, err := reader.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("EOF must surface as io.EOF: %v", err)
	}

	huge := strings.Repeat("x", maxFrameBytes+2)
	reader = NewReader(strings.NewReader(huge), nil)
	if _, err := reader.Next(); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("over-ceiling frame must be ErrFrameTooLarge: %v", err)
	}
}
