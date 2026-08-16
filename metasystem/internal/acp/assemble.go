package acp

import (
	"encoding/json"
	"errors"
)

// The watermark rule, proven necessary by direct capture (probe
// step B): session/load REPLAYS history as live-looking
// agent_message_chunk notifications, so an unwatermarked assembler
// would adopt a prior turn's answer as the new candidate. Assembly
// consumes only chunks for the matching session that arrive after
// the watermark opens (load/new response seen, prompt sent) and
// before the matched PromptResponse closes the window.

// maxCandidateBytes bounds the assembled candidate; a breach
// disqualifies delivery (evidence, not candidate).
const maxCandidateBytes = 8 * 1024 * 1024

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

// Assembler accumulates message chunks inside one prompt window.
// It is NOT safe for concurrent use; the connection's read loop is
// the single caller.
type Assembler struct {
	sessionID string
	open      bool
	messages  [][]byte
	current   []byte
	currentID string
	total     int
	oversize  bool
}

// NewAssembler builds an assembler for one session's prompt window.
func NewAssembler(sessionID string) *Assembler {
	return &Assembler{sessionID: sessionID}
}

// OpenWindow marks the watermark: everything consumed before this
// call was replay or setup noise and is dropped.
func (a *Assembler) OpenWindow() {
	a.open = true
	a.messages = nil
	a.current = nil
	a.currentID = ""
	a.total = 0
	a.oversize = false
}

// Consume feeds one session/update notification's params. Returns
// true when the chunk entered the candidate (window open, session
// match, text message chunk) — the caller journals regardless.
func (a *Assembler) Consume(params json.RawMessage) bool {
	if !a.open {
		return false
	}
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
		// Message-ID grouping (spec: assembly rules): a change of
		// non-empty identity is a message boundary; chunks without
		// identity continue the current message.
		if body.MessageID != "" && a.currentID != "" && body.MessageID != a.currentID {
			a.breakMessage()
		}
		if body.MessageID != "" {
			a.currentID = body.MessageID
		}
		a.total += len(body.Content.Text)
		if a.total > maxCandidateBytes {
			a.oversize = true
			return false
		}
		a.current = append(a.current, body.Content.Text...)
		return true
	case "agent_thought_chunk":
		// The thought stream is verbose (23 chunks for a pong in
		// the step-A capture) and never enters the candidate.
		return false
	case "user_message_chunk":
		// A user chunk inside an open window means replay bled past
		// the watermark or a second prompt is interleaving; it
		// breaks the current message so a later agent message
		// starts fresh.
		a.breakMessage()
		return false
	}
	return false
}

// breakMessage closes the in-progress message; used at message
// boundaries so "the final complete message wins" is decidable.
func (a *Assembler) breakMessage() {
	if len(a.current) > 0 {
		a.messages = append(a.messages, a.current)
		a.current = nil
	}
	a.currentID = ""
}

// Candidate closes the window and returns the final complete
// message — the candidate under the final-message-wins rule —
// with earlier messages remaining journaled evidence only. An
// oversize window disqualifies entirely.
func (a *Assembler) Candidate() ([]byte, error) {
	a.breakMessage()
	a.open = false
	if a.oversize {
		return nil, ErrCandidateTooLarge
	}
	if len(a.messages) == 0 {
		return nil, nil
	}
	return a.messages[len(a.messages)-1], nil
}
