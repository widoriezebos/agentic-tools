// Package acp implements the Agent Client Protocol transport core:
// newline-delimited JSON-RPC framing, the permission decision over
// the job envelope, and watermarked candidate assembly. The package
// is runtime-neutral — dialect facts (mode identifiers, launch argv,
// wire-to-effect mappings) are supplied by adapter-owned tables, and
// nothing here names an agent runtime. Design: plans/
// acp-transport-design.md (the D81 spec); wire evidence: plans/
// acp-wire-probe.md.
package acp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// maxFrameBytes bounds a single wire frame. The probe observed
// tens-of-KB single frames (the config-option catalog) on a healthy
// wire; the ceiling is client-owned because ACP pins no maximum.
const maxFrameBytes = 16 * 1024 * 1024

// ErrFrameTooLarge reports a frame past the client-owned ceiling —
// a protocol error that fails the turn, never a silent truncation.
var ErrFrameTooLarge = errors.New("acp: frame exceeds the client ceiling")

// Message is one decoded JSON-RPC frame. Exactly one of the three
// shapes holds: a response (ID, no Method), a server request (ID and
// Method), or a notification (Method, no ID).
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *WireError      `json:"error,omitempty"`
}

// WireError is the JSON-RPC error member; codes may be standard or
// arbitrary per the schema, so the code is evidence, not a switch.
type WireError struct {
	Code    int64           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Kind classifies a decoded frame.
type Kind int

const (
	KindResponse Kind = iota
	KindServerRequest
	KindNotification
	KindMalformed
)

// Classify reports which JSON-RPC shape a message has. A frame with
// neither ID nor Method is malformed.
func (m *Message) Classify() Kind {
	switch {
	case m.ID != nil && m.Method == "":
		return KindResponse
	case m.ID != nil:
		return KindServerRequest
	case m.Method != "":
		return KindNotification
	}
	return KindMalformed
}

// Reader decodes newline-delimited frames and journals every raw
// line before parsing, so malformed bytes are evidence even when
// they cannot be a Message.
type Reader struct {
	scanner *bufio.Scanner
	journal io.Writer
}

// NewReader wraps the wire's read side. journal receives every raw
// inbound line ('<' prefix, newline-terminated); nil disables
// journaling (tests only — production always journals).
func NewReader(r io.Reader, journal io.Writer) *Reader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxFrameBytes)
	return &Reader{scanner: scanner, journal: journal}
}

// Next returns the next frame. A too-long line surfaces as
// ErrFrameTooLarge; EOF surfaces as io.EOF; a line that is not JSON
// returns a *Message with KindMalformed classification and the raw
// bytes in the error for evidence.
func (r *Reader) Next() (*Message, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				return nil, ErrFrameTooLarge
			}
			return nil, err
		}
		return nil, io.EOF
	}
	line := r.scanner.Bytes()
	if r.journal != nil {
		r.journal.Write([]byte("< "))
		r.journal.Write(line)
		r.journal.Write([]byte("\n"))
	}
	var msg Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("acp: malformed frame %q: %w", string(line), err)
	}
	return &msg, nil
}

// Writer encodes outbound frames, journaling each ('>' prefix).
type Writer struct {
	w       io.Writer
	journal io.Writer
}

// NewWriter wraps the wire's write side.
func NewWriter(w io.Writer, journal io.Writer) *Writer {
	return &Writer{w: w, journal: journal}
}

// Send marshals and writes one frame with the trailing newline.
func (w *Writer) Send(m *Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(b) > maxFrameBytes {
		return ErrFrameTooLarge
	}
	if w.journal != nil {
		w.journal.Write([]byte("> "))
		w.journal.Write(b)
		w.journal.Write([]byte("\n"))
	}
	_, err = w.w.Write(append(b, '\n'))
	return err
}
