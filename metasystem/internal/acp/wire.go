// Package acp implements the Agent Client Protocol transport core:
// newline-delimited JSON-RPC framing, the permission decision over
// the job envelope, and watermarked candidate assembly. The package
// is runtime-neutral — dialect facts (mode identifiers, launch argv,
// wire-to-effect mappings) are supplied by adapter-owned tables, and
// nothing here names an agent runtime. Design: plans/
// acp-transport-design.md (the D81 spec); wire evidence: plans/
// acp-wire-probe.md; message shapes: the pinned schema artifact in
// schema/ (see schema/PIN.md).
package acp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// maxFrameBytes bounds a single wire frame's payload (the newline
// delimiter is not counted). The probe observed tens-of-KB single
// frames on a healthy wire; the ceiling is client-owned because ACP
// pins no maximum.
const maxFrameBytes = 16 * 1024 * 1024

// ErrFrameTooLarge reports a frame past the client-owned ceiling —
// a protocol error that fails the turn, never a silent truncation.
var ErrFrameTooLarge = errors.New("acp: frame exceeds the client ceiling")

// ErrPartialFrame reports bytes after the final delimiter at EOF —
// evidence of a torn stream, never a frame.
var ErrPartialFrame = errors.New("acp: partial frame at EOF")

// RequestID is a JSON-RPC id: string, integer, or null, preserved
// verbatim so responses echo exactly what arrived. A nil *RequestID
// on Message means the id member was ABSENT (a notification); a
// RequestID holding "null" is a present null id and stays a
// request/response id.
type RequestID struct {
	raw json.RawMessage
}

// NewRequestID builds an integer id for client-originated requests.
func NewRequestID(n int64) *RequestID {
	return &RequestID{raw: json.RawMessage(fmt.Sprintf("%d", n))}
}

// parseRequestID validates the shapes the pinned schema allows:
// string, integer, null. (Presence detection lives on Message —
// encoding/json never invokes an Unmarshaler through a nil-able
// pointer for JSON null, so the Message decodes id as RawMessage.)
func parseRequestID(b []byte) (*RequestID, error) {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return nil, errors.New("acp: empty request id")
	}
	switch trimmed[0] {
	case '"', 'n', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return &RequestID{raw: append(json.RawMessage(nil), trimmed...)}, nil
	}
	return nil, fmt.Errorf("acp: request id %q is not string, integer, or null", string(b))
}

// MarshalJSON echoes the verbatim wire bytes.
func (r *RequestID) MarshalJSON() ([]byte, error) {
	if r == nil || len(r.raw) == 0 {
		return []byte("null"), nil
	}
	return r.raw, nil
}

// Key returns a comparable identity for pending-call matching.
func (r *RequestID) Key() string {
	if r == nil {
		return ""
	}
	return string(r.raw)
}

// Message is one decoded JSON-RPC frame. Exactly one of the three
// shapes holds: a response (ID, no Method, exactly one of Result or
// Error), a server request (ID and Method), or a notification
// (Method, no ID member).
type Message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *RequestID      `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *WireError      `json:"error,omitempty"`
}

// UnmarshalJSON decodes through a shadow so a PRESENT null id (a
// request/response id) stays distinguishable from an ABSENT id (a
// notification): the RawMessage shadow field holds "null" for the
// former and nil for the latter.
func (m *Message) UnmarshalJSON(b []byte) error {
	var shadow struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		Result  json.RawMessage `json:"result"`
		Error   *WireError      `json:"error"`
	}
	if err := json.Unmarshal(b, &shadow); err != nil {
		return err
	}
	m.JSONRPC = shadow.JSONRPC
	m.Method = shadow.Method
	m.Params = shadow.Params
	m.Result = shadow.Result
	m.Error = shadow.Error
	m.ID = nil
	if len(shadow.ID) > 0 {
		id, err := parseRequestID(shadow.ID)
		if err != nil {
			return err
		}
		m.ID = id
	}
	return nil
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

// Classify reports which JSON-RPC 2.0 shape a message has. Frames
// without jsonrpc "2.0", with neither ID nor Method, or responses
// carrying both or neither of result and error are malformed.
func (m *Message) Classify() Kind {
	if m.JSONRPC != "2.0" {
		return KindMalformed
	}
	switch {
	case m.ID != nil && m.Method == "":
		hasResult := len(m.Result) > 0
		hasError := m.Error != nil
		if hasResult == hasError {
			return KindMalformed
		}
		return KindResponse
	case m.ID != nil:
		return KindServerRequest
	case m.Method != "":
		return KindNotification
	}
	return KindMalformed
}

// Reader decodes newline-delimited frames. Every consumed byte is
// journaled BEFORE parsing — including oversize prefixes and torn
// tails — because the journal is settlement evidence and a frame
// that cannot parse is still evidence. Journal failures are sticky
// and surfaced via JournalErr; processing that continues past a
// dead journal is the caller's deliberate choice, never a silent
// one.
type Reader struct {
	reader     *bufio.Reader
	journal    io.Writer
	journalErr error
}

// NewReader wraps the wire's read side. journal receives every raw
// inbound line ('<' prefix); nil disables journaling (tests only —
// production always journals).
func NewReader(r io.Reader, journal io.Writer) *Reader {
	return &Reader{reader: bufio.NewReaderSize(r, 64*1024), journal: journal}
}

// JournalErr reports the first journal write failure, if any.
func (r *Reader) JournalErr() error { return r.journalErr }

func (r *Reader) journalLine(prefix string, line []byte) {
	if r.journal == nil || r.journalErr != nil {
		return
	}
	// One Write per line: a shared, serialized sink then cannot
	// interleave directions into invalid evidence.
	buffer := make([]byte, 0, len(prefix)+len(line)+1)
	buffer = append(buffer, prefix...)
	buffer = append(buffer, line...)
	buffer = append(buffer, '\n')
	if _, err := r.journal.Write(buffer); err != nil {
		r.journalErr = err
	}
}

// Next returns the next frame. An over-ceiling line journals its
// consumed prefix and returns ErrFrameTooLarge with the remainder
// of the line discarded (bounded, so the stream can fail cleanly);
// bytes after the final delimiter at EOF journal as a torn tail and
// return ErrPartialFrame; EOF at a frame boundary is io.EOF; a line
// that is not JSON journals and returns the raw bytes in the error.
func (r *Reader) Next() (*Message, error) {
	var line []byte
	for {
		chunk, err := r.reader.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > maxFrameBytes+1 {
			r.journalLine("<!oversize ", line[:4096])
			r.discardLine(errors.Is(err, bufio.ErrBufferFull))
			return nil, ErrFrameTooLarge
		}
		if err == nil {
			break
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) {
			if len(line) == 0 {
				return nil, io.EOF
			}
			r.journalLine("<!torn ", line)
			return nil, ErrPartialFrame
		}
		return nil, err
	}
	line = bytes.TrimSuffix(line, []byte("\n"))
	r.journalLine("< ", line)
	var msg Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return nil, fmt.Errorf("acp: malformed frame %q: %w", string(line), err)
	}
	return &msg, nil
}

// discardLine drains the remainder of an oversize line so the next
// call starts at a frame boundary.
func (r *Reader) discardLine(more bool) {
	for more {
		_, err := r.reader.ReadSlice('\n')
		if err == nil || !errors.Is(err, bufio.ErrBufferFull) {
			return
		}
	}
}

// Writer encodes outbound frames, journaling each ('>' prefix). A
// journal failure fails the send — evidence precedes action.
type Writer struct {
	w          io.Writer
	journal    io.Writer
	journalErr error
}

// NewWriter wraps the wire's write side.
func NewWriter(w io.Writer, journal io.Writer) *Writer {
	return &Writer{w: w, journal: journal}
}

// JournalErr reports the first journal write failure, if any.
func (w *Writer) JournalErr() error { return w.journalErr }

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
		buffer := make([]byte, 0, len(b)+3)
		buffer = append(buffer, '>', ' ')
		buffer = append(buffer, b...)
		buffer = append(buffer, '\n')
		if _, err := w.journal.Write(buffer); err != nil {
			w.journalErr = err
			return fmt.Errorf("acp: journal write failed before send: %w", err)
		}
	}
	_, err = w.w.Write(append(b, '\n'))
	return err
}
