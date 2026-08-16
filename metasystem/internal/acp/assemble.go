package acp

import (
	"encoding/json"
	"errors"
)

// The watermark rule, proven necessary by direct capture (probe
// step B): session/load REPLAYS history as live-looking
// agent_message_chunk notifications, and channel hand-off loses
// arrival order between channels — the linux race in slice two
// proved a post-response chunk can be SELECTED before the response
// it followed. So the prompt window is a pure SEQUENCE RANGE: the
// assembler buffers every candidate-relevant event with its wire
// sequence, and Candidate assembles exactly the events strictly
// inside (afterSeq, beforeSeq). No drains, no races.

// maxCandidateBytes bounds the assembled candidate; a breach
// disqualifies delivery (evidence, not candidate).
const maxCandidateBytes = 8 * 1024 * 1024

// maxBufferedBytes caps the assembler's event buffer; the journal
// keeps the full stream, so past this point the window is already
// disqualified and buffering more proves nothing.
const maxBufferedBytes = 2 * maxCandidateBytes

// ErrCandidateTooLarge reports a disqualified oversized candidate.
var ErrCandidateTooLarge = errors.New("acp: assembled candidate exceeds the ceiling")

// sessionUpdate is the envelope of a session/update notification.
type sessionUpdate struct {
	SessionID string          `json:"sessionId"`
	Update    json.RawMessage `json:"update"`
}

// updateBody is the union of the update fields assembly cares
// about; unknown kinds are journaled by the connection and ignored
// here (spec: failure outcomes — unknown notifications never fail
// the turn). MessageID is the schema-generic message identity;
// dialect-specific identities living in _meta belong to the
// adapter's extractor, never to this package.
type updateBody struct {
	SessionUpdate string `json:"sessionUpdate"`
	MessageID     string `json:"messageId"`
	Content       struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type assemblyEvent struct {
	seq       uint64
	userBreak bool
	messageID string
	text      string
}

// Assembler buffers candidate-relevant events for one session.
// It is NOT safe for concurrent use; the turn driver is the single
// caller.
type Assembler struct {
	sessionID string
	events    []assemblyEvent
	buffered  int
	overflow  bool
}

// NewAssembler builds an assembler for one session.
func NewAssembler(sessionID string) *Assembler {
	return &Assembler{sessionID: sessionID}
}

// Consume feeds one session/update notification's params with its
// wire sequence. Returns true when the event was retained as
// candidate-relevant — the caller journals regardless.
func (a *Assembler) Consume(seq uint64, params json.RawMessage) bool {
	var envelope sessionUpdate
	if err := json.Unmarshal(params, &envelope); err != nil {
		return false
	}
	if envelope.SessionID != a.sessionID {
		return false
	}
	var body updateBody
	if err := json.Unmarshal(envelope.Update, &body); err != nil {
		return false
	}
	switch body.SessionUpdate {
	case "agent_message_chunk":
		if body.Content.Type != "text" {
			// Non-text blocks are journaled evidence, never
			// candidate bytes.
			return false
		}
		a.buffered += len(body.Content.Text)
		if a.buffered > maxBufferedBytes {
			a.overflow = true
			return false
		}
		a.events = append(a.events, assemblyEvent{seq: seq, messageID: body.MessageID, text: body.Content.Text})
		return true
	case "agent_thought_chunk":
		// The thought stream is verbose (23 chunks for a pong in
		// the step-A capture) and never enters the candidate.
		return false
	case "user_message_chunk":
		// A message boundary: a later agent message starts fresh.
		a.events = append(a.events, assemblyEvent{seq: seq, userBreak: true})
		return false
	}
	return false
}

// Candidate assembles the window (afterSeq, beforeSeq): chunks in
// arrival order, message-ID grouping (a change of non-empty
// identity is a boundary), user chunks as boundaries, the FINAL
// complete message winning, earlier ones remaining journaled
// evidence. Oversize disqualifies the whole window.
func (a *Assembler) Candidate(afterSeq, beforeSeq uint64) ([]byte, error) {
	if a.overflow {
		return nil, ErrCandidateTooLarge
	}
	var messages [][]byte
	var current []byte
	currentID := ""
	total := 0
	flush := func() {
		if len(current) > 0 {
			messages = append(messages, current)
			current = nil
		}
		currentID = ""
	}
	for _, event := range a.events {
		if event.seq <= afterSeq || event.seq >= beforeSeq {
			continue
		}
		if event.userBreak {
			flush()
			continue
		}
		if event.messageID != "" && currentID != "" && event.messageID != currentID {
			flush()
		}
		if event.messageID != "" {
			currentID = event.messageID
		}
		total += len(event.text)
		if total > maxCandidateBytes {
			return nil, ErrCandidateTooLarge
		}
		current = append(current, event.text...)
	}
	flush()
	if len(messages) == 0 {
		return nil, nil
	}
	return messages[len(messages)-1], nil
}
