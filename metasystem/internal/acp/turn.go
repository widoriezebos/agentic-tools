package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// jsonMarshal is indirected for conn.go's mustMarshal.
var jsonMarshal = json.Marshal

// Row is the failure-matrix classification of a turn's outcome —
// one row per matrix entry the protocol level can reach (custody
// rows belong to the script and the proof owner).
type Row string

const (
	RowDelivered       Row = "delivered"
	RowVersionMismatch Row = "version-mismatch"
	RowAuthRequired    Row = "auth-required"
	RowSetupError      Row = "setup-error"
	RowProtocolError   Row = "protocol-error"
	RowTurnFailed      Row = "turn-failed"
	RowCancelled       Row = "cancelled"
	RowRefused         Row = "refused"
	RowIncomplete      Row = "incomplete"
)

// Outcome is the typed result the script acts on. Candidate is
// nil for every row but delivered; Violations counts correlation-
// gate and unsolicited-request events (recorded, never fatal by
// themselves); UsageResult holds the PromptResponse usage member
// verbatim for the usage owner's journal-based accounting.
type Outcome struct {
	Row         Row
	StopReason  string
	SessionID   string
	Candidate   []byte
	UsageResult json.RawMessage
	Violations  int
	Detail      string
}

// TurnConfig drives one prompt attempt. ModeID is the DIALECT-
// RESOLVED session mode (the adapter maps the envelope's tools
// grade to its runtime's mode identifier; this package never
// hardcodes one). LoadSessionID selects session/load; empty means
// session/new. The envelope must already have passed PreflightACP.
type TurnConfig struct {
	ExpectedProtocolVersion int64
	WorkspaceDir            string
	LoadSessionID           string
	ModeID                  string
	Prompt                  string
	Envelope                Envelope
	HandshakeTimeout        time.Duration
	PromptTimeout           time.Duration
	LateFrameWindow         time.Duration
}

type initializeResult struct {
	ProtocolVersion int64 `json:"protocolVersion"`
	AuthMethods     []any `json:"authMethods"`
}

type newSessionResult struct {
	SessionID string `json:"sessionId"`
}

type promptResult struct {
	StopReason string          `json:"stopReason"`
	Usage      json.RawMessage `json:"usage"`
}

// RunTurn drives one prompt attempt over an established
// connection: initialize (version verified), session/new or load,
// optional set-mode, the sequence-bounded prompt window, and stop-
// reason mapping. Custody, spawning, and killing are the calling
// script's; RunTurn only ever touches the wire. The prompt window
// is the OPEN sequence interval between the last setup response
// and the PromptResponse — replay before it and stragglers after
// it are journaled evidence that can never become the candidate.
func RunTurn(ctx context.Context, conn *Conn, cfg TurnConfig) Outcome {
	handshake, cancelHandshake := context.WithTimeout(ctx, cfg.HandshakeTimeout)
	defer cancelHandshake()

	initFrame, err := conn.CallSeq(handshake, "initialize", map[string]any{
		"protocolVersion": cfg.ExpectedProtocolVersion,
		"clientCapabilities": map[string]any{
			// Advertise nothing: no client fs, no terminal — a
			// second side-effect path would bypass the envelope.
			"fs": map[string]any{"readTextFile": false, "writeTextFile": false},
		},
		"clientInfo": map[string]any{"name": "metasystem-acp", "version": "1"},
	})
	if err != nil {
		return Outcome{Row: RowProtocolError, Detail: fmt.Sprintf("initialize: %v", err)}
	}
	if initFrame.Msg.Error != nil {
		return Outcome{Row: RowSetupError, Detail: fmt.Sprintf("initialize refused: %d %s", initFrame.Msg.Error.Code, initFrame.Msg.Error.Message)}
	}
	var initBody initializeResult
	if err := json.Unmarshal(initFrame.Msg.Result, &initBody); err != nil {
		return Outcome{Row: RowProtocolError, Detail: "initialize result unreadable"}
	}
	if initBody.ProtocolVersion != cfg.ExpectedProtocolVersion {
		return Outcome{Row: RowVersionMismatch, Detail: fmt.Sprintf("negotiated %d, expected %d", initBody.ProtocolVersion, cfg.ExpectedProtocolVersion)}
	}
	authAdvertised := len(initBody.AuthMethods) > 0

	sessionID := cfg.LoadSessionID
	var setupSeq uint64
	if sessionID == "" {
		frame, err := conn.CallSeq(handshake, "session/new", map[string]any{
			"cwd": cfg.WorkspaceDir, "mcpServers": []any{},
		})
		outcome, id := classifySetup(frame, err, authAdvertised, "session/new")
		if outcome != nil {
			return *outcome
		}
		sessionID = id
		setupSeq = frame.Seq
	} else {
		frame, err := conn.CallSeq(handshake, "session/load", map[string]any{
			"sessionId": sessionID, "cwd": cfg.WorkspaceDir, "mcpServers": []any{},
		})
		if outcome, _ := classifySetup(frame, err, authAdvertised, "session/load"); outcome != nil {
			return *outcome
		}
		setupSeq = frame.Seq
	}

	if cfg.ModeID != "" {
		frame, err := conn.CallSeq(handshake, "session/set_mode", map[string]any{
			"sessionId": sessionID, "modeId": cfg.ModeID,
		})
		if err != nil || frame.Msg.Error != nil {
			// The mode IS the enforcement lever (probe steps C–E):
			// a mode that cannot be set means the envelope cannot
			// be honored, so the turn must not proceed.
			return Outcome{Row: RowSetupError, SessionID: sessionID, Detail: "set_mode failed; the envelope's grade cannot be applied"}
		}
		setupSeq = frame.Seq
	}

	assembler := NewAssembler(sessionID)

	promptCtx, cancelPrompt := context.WithTimeout(ctx, cfg.PromptTimeout)
	defer cancelPrompt()
	promptDone := make(chan Frame, 1)
	promptErr := make(chan error, 1)
	go func() {
		frame, err := conn.CallSeq(promptCtx, "session/prompt", map[string]any{
			"sessionId": sessionID,
			"prompt":    []any{map[string]any{"type": "text", "text": cfg.Prompt}},
		})
		if err != nil {
			promptErr <- err
			return
		}
		promptDone <- frame
	}()

	violations := 0
	for {
		select {
		case frame, ok := <-conn.Notifications():
			if !ok {
				continue
			}
			if frame.Msg.Method == "session/update" {
				assembler.Consume(frame.Seq, frame.Msg.Params)
			}
			// Every other notification kind is journaled by the
			// wire layer and advisory here (accelerator ruling).
		case frame, ok := <-conn.Requests():
			if !ok {
				continue
			}
			violations += answerServerRequest(conn, frame.Msg, sessionID)
		case respFrame := <-promptDone:
			return settlePrompt(conn, respFrame, assembler, setupSeq, sessionID, violations, cfg.LateFrameWindow)
		case err := <-promptErr:
			return Outcome{Row: promptFailureRow(err, conn), SessionID: sessionID, Violations: violations, Detail: fmt.Sprintf("prompt: %v", err)}
		case <-conn.Done():
			return Outcome{Row: promptFailureRow(conn.Err(), conn), SessionID: sessionID, Violations: violations, Detail: fmt.Sprintf("connection died mid-prompt: %v", conn.Err())}
		}
	}
}

// promptFailureRow separates the matrix's protocol deaths
// (malformed, oversize, torn, timeout, mismatched id) from a clean
// peer close before the response (turn failed; chunks stay
// evidence).
func promptFailureRow(err error, conn *Conn) Row {
	if errors.Is(err, io.EOF) {
		return RowTurnFailed
	}
	if err != nil {
		return RowProtocolError
	}
	if connErr := conn.Err(); connErr != nil && !errors.Is(connErr, io.EOF) {
		return RowProtocolError
	}
	return RowTurnFailed
}

// classifySetup maps a setup-phase response to its matrix row, or
// returns the session id on success.
func classifySetup(frame Frame, err error, authAdvertised bool, phase string) (*Outcome, string) {
	if err != nil {
		return &Outcome{Row: RowProtocolError, Detail: fmt.Sprintf("%s: %v", phase, err)}, ""
	}
	if frame.Msg.Error != nil {
		if authAdvertised {
			// The matrix's auth-required row: the server advertised
			// auth methods and refused an unauthenticated session.
			// Never interactive auth inside a job.
			return &Outcome{Row: RowAuthRequired, Detail: fmt.Sprintf("%s refused with auth advertised: %d %s", phase, frame.Msg.Error.Code, frame.Msg.Error.Message)}, ""
		}
		return &Outcome{Row: RowSetupError, Detail: fmt.Sprintf("%s: %d %s", phase, frame.Msg.Error.Code, frame.Msg.Error.Message)}, ""
	}
	var body newSessionResult
	if err := json.Unmarshal(frame.Msg.Result, &body); err != nil || (phase == "session/new" && body.SessionID == "") {
		return &Outcome{Row: RowProtocolError, Detail: phase + " result unreadable"}, ""
	}
	return nil, body.SessionID
}

// answerServerRequest applies the correlation gate and the strict
// permission posture; the return value counts violations.
func answerServerRequest(conn *Conn, request *Message, sessionID string) int {
	if request.Method != "session/request_permission" {
		// Unsolicited server→client effect call: this client
		// advertised nothing, so anything else fails closed.
		conn.RespondError(request.ID, -32601, "client capability not advertised")
		return 1
	}
	var params struct {
		SessionID string             `json:"sessionId"`
		Options   []PermissionOption `json:"options"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil || params.SessionID != sessionID {
		// The correlation gate: wrong-session or unreadable
		// requests are protocol violations, answered cancelled,
		// never normalized (r4 F6).
		conn.Respond(request.ID, PermissionAnswer{Outcome: outcomeCancelled}.WireResult())
		return 1
	}
	// Strict-refusal posture until a captured dialect wires the
	// normalizer + Decide (probe steps C–E: no request fired in
	// any envelope-relevant mode, so this is the defensive
	// backstop, not the enforcement lever).
	conn.Respond(request.ID, StrictAnswer(params.Options).WireResult())
	return 0
}

// settlePrompt maps the PromptResponse to its row. Queued
// notifications drain into the assembler first — the sequence
// filter, not drain timing, decides what was in the window — and
// the late-frame window is drained as journaled evidence.
func settlePrompt(conn *Conn, respFrame Frame, assembler *Assembler, setupSeq uint64, sessionID string, violations int, lateWindow time.Duration) Outcome {
	for {
		select {
		case frame, ok := <-conn.Notifications():
			if !ok {
				break
			}
			if frame.Msg.Method == "session/update" {
				assembler.Consume(frame.Seq, frame.Msg.Params)
			}
			continue
		default:
		}
		break
	}
	resp := respFrame.Msg
	if resp.Error != nil {
		return Outcome{Row: RowTurnFailed, SessionID: sessionID, Violations: violations, Detail: fmt.Sprintf("prompt error: %d %s", resp.Error.Code, resp.Error.Message)}
	}
	var body promptResult
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		return Outcome{Row: RowProtocolError, SessionID: sessionID, Violations: violations, Detail: "prompt result unreadable"}
	}
	drainLate(conn, lateWindow)
	base := Outcome{StopReason: body.StopReason, SessionID: sessionID, UsageResult: body.Usage, Violations: violations}
	switch body.StopReason {
	case "end_turn":
		candidate, err := assembler.Candidate(setupSeq, respFrame.Seq)
		if err != nil {
			base.Row = RowIncomplete
			base.Detail = err.Error()
			return base
		}
		base.Row = RowDelivered
		base.Candidate = candidate
		return base
	case "cancelled":
		base.Row = RowCancelled
		return base
	case "refusal":
		base.Row = RowRefused
		return base
	case "max_tokens", "max_turn_requests":
		base.Row = RowIncomplete
		return base
	}
	base.Row = RowProtocolError
	base.Detail = fmt.Sprintf("unknown stop reason %q — never silent success", body.StopReason)
	return base
}

// drainLate consumes frames for the bounded late window after the
// PromptResponse — the step-A capture proved late frames are real.
// They are journaled evidence; the sequence filter already
// excludes them from the candidate.
func drainLate(conn *Conn, window time.Duration) {
	if window <= 0 {
		return
	}
	deadline := time.After(window)
	for {
		select {
		case <-deadline:
			return
		case _, ok := <-conn.Notifications():
			if !ok {
				return
			}
		case frame, ok := <-conn.Requests():
			if ok && frame.Msg != nil {
				conn.RespondError(frame.Msg.ID, -32601, "turn already settled")
			}
		case <-conn.Done():
			return
		}
	}
}
