package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// The file-driven prompt attempt: one ACP turn over already-created pipe
// paths (the supervisor owns the fifo pair, the server child, custody and
// killing), with the raw wire journal and the typed outcome. This is the
// body of the former `acp turn` verb, called in process by the delegate
// supervisor and the Devin host turn.

// FileTurnConfig names the pipes, files and deadlines of one attempt.
type FileTurnConfig struct {
	ServerOut, ServerIn string // the server's stdout pipe (read side) and stdin pipe (write side)
	JournalPath         string // created, append-only raw wire journal
	Workspace           string // session cwd (absolute)
	EnvelopePath        string // the expanded five-field envelope JSON
	PromptFile          string
	LoadSession         string // a session to load instead of creating one
	Mode                string // dialect-resolved session mode (empty: leave default)
	ExpectedProtocol    int64
	HandshakeTimeout    time.Duration // zero: DefaultHandshakeTimeout
	PromptTimeout       time.Duration // zero: 1800s
	LateFrameWindow     time.Duration // zero: 2s
	SessionFile         string        // receives the session id at setup success
}

// FileTurnOutcome is the typed outcome document, the wire shape the
// supervisor stores as acp-outcome.json.
type FileTurnOutcome struct {
	Row          string          `json:"row"`
	StopReason   string          `json:"stopReason,omitempty"`
	SessionID    string          `json:"sessionId,omitempty"`
	Candidate    *string         `json:"candidate,omitempty"`
	Usage        json.RawMessage `json:"usage,omitempty"`
	Violations   int             `json:"violations"`
	Detail       string          `json:"detail,omitempty"`
	JournalError string          `json:"journalError,omitempty"`
}

type envelopeDocument struct {
	ReadRoots  []string `json:"readRoots"`
	WriteRoots []string `json:"writeRoots"`
	Network    string   `json:"network"`
	Approvals  string   `json:"approvals"`
	Tools      string   `json:"tools"`
}

// LoadEnvelopeFile reads an expanded five-field envelope.
func LoadEnvelopeFile(path string) (Envelope, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Envelope{}, err
	}
	var parsed envelopeDocument
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Envelope{}, fmt.Errorf("envelope %s: %w", path, err)
	}
	return Envelope(parsed), nil
}

// RunFileTurn drives one prompt attempt and returns its typed outcome. An
// error means no attempt ran (a refused envelope, an unreadable prompt, a
// journal path collision, an unopenable pipe); the cancellation of ctx lands
// as the courtesy session/cancel and the typed cancelled outcome.
func RunFileTurn(ctx context.Context, cfg FileTurnConfig) (FileTurnOutcome, error) {
	envelope, err := LoadEnvelopeFile(cfg.EnvelopePath)
	if err != nil {
		return FileTurnOutcome{}, err
	}
	if reason := PreflightACP(envelope); reason != "" {
		return FileTurnOutcome{}, fmt.Errorf("preflight refused: %s", reason)
	}
	prompt, err := os.ReadFile(cfg.PromptFile)
	if err != nil {
		return FileTurnOutcome{}, err
	}
	// One attempt, one journal: an existing file is a path collision that
	// would concatenate stale sessions into settlement evidence — refuse,
	// never append or truncate.
	journal, err := os.OpenFile(cfg.JournalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return FileTurnOutcome{}, fmt.Errorf("journal create: %w", err)
	}
	// Open order is the launch contract: the read side first (the server
	// opens its write side symmetrically), then the write side. Regular
	// files work too, which the tests use.
	readEnd, err := os.OpenFile(cfg.ServerOut, os.O_RDONLY, 0)
	if err != nil {
		journal.Close()
		return FileTurnOutcome{}, fmt.Errorf("server-out open: %w", err)
	}
	writeEnd, err := os.OpenFile(cfg.ServerIn, os.O_WRONLY, 0)
	if err != nil {
		readEnd.Close()
		journal.Close()
		return FileTurnOutcome{}, fmt.Errorf("server-in open: %w", err)
	}
	handshake, promptTimeout, late := cfg.HandshakeTimeout, cfg.PromptTimeout, cfg.LateFrameWindow
	if handshake <= 0 {
		handshake = DefaultHandshakeTimeout
	}
	if promptTimeout <= 0 {
		promptTimeout = 1800 * time.Second
	}
	if late <= 0 {
		late = 2 * time.Second
	}
	expected := cfg.ExpectedProtocol
	if expected == 0 {
		expected = 1
	}
	conn := NewConn(readEnd, writeEnd, journal)
	outcome := RunTurn(ctx, conn, TurnConfig{
		ExpectedProtocolVersion: expected,
		WorkspaceDir:            cfg.Workspace,
		LoadSessionID:           cfg.LoadSession,
		ModeID:                  cfg.Mode,
		Prompt:                  string(prompt),
		Envelope:                envelope,
		HandshakeTimeout:        handshake,
		PromptTimeout:           promptTimeout,
		LateFrameWindow:         late,
		SessionFile:             cfg.SessionFile,
	})
	// Quiesce before sampling journal health: close the pipe ends so the
	// read loop terminates, wait for it (bounded), then sync and close the
	// journal — the settlement journal must be whole BEFORE the outcome
	// claims it is.
	writeEnd.Close()
	readEnd.Close()
	select {
	case <-conn.Done():
	case <-time.After(3 * time.Second):
	}
	journalIssue := conn.JournalErr()
	if err := journal.Sync(); err != nil && journalIssue == nil {
		journalIssue = err
	}
	if err := journal.Close(); err != nil && journalIssue == nil {
		journalIssue = err
	}
	wire := FileTurnOutcome{
		Row:        string(outcome.Row),
		StopReason: outcome.StopReason,
		SessionID:  outcome.SessionID,
		Usage:      outcome.UsageResult,
		Violations: outcome.Violations,
		Detail:     outcome.Detail,
	}
	if outcome.Candidate != nil {
		text := string(outcome.Candidate)
		wire.Candidate = &text
	}
	if journalIssue != nil {
		// A thinned journal must be visible to settlement even when the
		// turn otherwise delivered.
		wire.JournalError = journalIssue.Error()
	}
	return wire, nil
}

// WriteFileTurnOutcome writes the outcome document as one JSON line, the
// bytes the former verb printed.
func WriteFileTurnOutcome(path string, outcome FileTurnOutcome) error {
	payload, err := json.Marshal(outcome)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}
