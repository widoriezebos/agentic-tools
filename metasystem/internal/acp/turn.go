package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// jsonMarshal is indirected for conn.go's mustMarshal.
var jsonMarshal = json.Marshal

// authRequiredCode is the pinned schema's authentication-required
// JSON-RPC error code; auth classification keys on IT, not on
// whether auth methods were advertised — live servers advertise
// methods while unauthenticated sessions succeed.
const authRequiredCode = -32000

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

// Rows enumerates the whole vocabulary. A new row MUST be added here
// in the same change — the delegate seam's parity pin compares the
// two vocabularies as sets in both directions through this list.
func Rows() []Row {
	return []Row{RowDelivered, RowVersionMismatch, RowAuthRequired,
		RowSetupError, RowProtocolError, RowTurnFailed, RowCancelled,
		RowRefused, RowIncomplete}
}

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
	CancelGrace             time.Duration
	// SessionFile, when set, receives the session id the moment
	// setup succeeds — minutes before the prompt settles — so the
	// adapter can record its handshake inside the dispatcher's
	// deadline instead of after the turn.
	SessionFile string
}

type initializeResult struct {
	ProtocolVersion   int64 `json:"protocolVersion"`
	AgentCapabilities struct {
		LoadSession bool `json:"loadSession"`
	} `json:"agentCapabilities"`
	AuthMethods []any `json:"authMethods"`
}

type newSessionResult struct {
	SessionID string `json:"sessionId"`
}

type promptResult struct {
	StopReason string          `json:"stopReason"`
	Usage      json.RawMessage `json:"usage"`
}

// turnDriver is the single event pump: every call — setup and
// prompt alike — is serviced while notifications and server
// requests keep flowing, so a request flood or a server that
// requires its answer before responding can never wedge the read
// loop. phase governs the correlation gate.
type turnDriver struct {
	conn       *Conn
	assembler  *Assembler
	sessionID  string
	violations int
	inWindow   bool
	failure    error
}

// RunTurn drives one prompt attempt over an established
// connection. Custody, spawning, and killing are the calling
// script's; RunTurn only ever touches the wire. The prompt window
// opens at the fence sampled immediately before the prompt is
// SENT and closes at the PromptResponse's sequence;
// replay before it and stragglers after it are journaled evidence
// that arithmetic, not timing, keeps out of the candidate.
func RunTurn(ctx context.Context, conn *Conn, cfg TurnConfig) Outcome {
	driver := &turnDriver{conn: conn}
	handshake, cancelHandshake := context.WithTimeout(ctx, cfg.HandshakeTimeout)
	defer cancelHandshake()

	initFrame, err := driver.call(handshake, "initialize", map[string]any{
		"protocolVersion": cfg.ExpectedProtocolVersion,
		"clientCapabilities": map[string]any{
			// Advertise nothing: no client fs, no terminal — a
			// second side-effect path would bypass the envelope.
			"fs": map[string]any{"readTextFile": false, "writeTextFile": false},
		},
		"clientInfo": map[string]any{"name": "metasystem-acp", "version": "1"},
	})
	if err != nil {
		return driver.fail("initialize", err)
	}
	if initFrame.Msg.Error != nil {
		return Outcome{Row: RowSetupError, Violations: driver.violations, Detail: fmt.Sprintf("initialize refused: %d %s", initFrame.Msg.Error.Code, initFrame.Msg.Error.Message)}
	}
	var initBody initializeResult
	if err := json.Unmarshal(initFrame.Msg.Result, &initBody); err != nil {
		return Outcome{Row: RowProtocolError, Violations: driver.violations, Detail: "initialize result unreadable"}
	}
	if initBody.ProtocolVersion != cfg.ExpectedProtocolVersion {
		return Outcome{Row: RowVersionMismatch, Violations: driver.violations, Detail: fmt.Sprintf("negotiated %d, expected %d", initBody.ProtocolVersion, cfg.ExpectedProtocolVersion)}
	}

	if cfg.LoadSessionID == "" {
		frame, err := driver.call(handshake, "session/new", map[string]any{
			"cwd": cfg.WorkspaceDir, "mcpServers": []any{},
		})
		if err != nil {
			return driver.fail("session/new", err)
		}
		outcome, id := classifySetup(frame, driver.violations, "session/new")
		if outcome != nil {
			return *outcome
		}
		driver.sessionID = id
	} else {
		// The capability gate: sending load to a server that never
		// declared it is a client bug, not a wire experiment.
		if !initBody.AgentCapabilities.LoadSession {
			return Outcome{Row: RowSetupError, Violations: driver.violations, Detail: "session/load requested but the server did not declare loadSession"}
		}
		frame, err := driver.call(handshake, "session/load", map[string]any{
			"sessionId": cfg.LoadSessionID, "cwd": cfg.WorkspaceDir, "mcpServers": []any{},
		})
		if err != nil {
			return driver.fail("session/load", err)
		}
		if outcome, _ := classifySetup(frame, driver.violations, "session/load"); outcome != nil {
			return *outcome
		}
		driver.sessionID = cfg.LoadSessionID
	}

	if cfg.SessionFile != "" {
		if err := os.WriteFile(cfg.SessionFile, []byte(driver.sessionID+"\n"), 0o644); err != nil {
			return Outcome{Row: RowProtocolError, SessionID: driver.sessionID, Violations: driver.violations, Detail: "session file unwritable: " + err.Error()}
		}
	}

	if cfg.ModeID != "" {
		frame, err := driver.call(handshake, "session/set_mode", map[string]any{
			"sessionId": driver.sessionID, "modeId": cfg.ModeID,
		})
		if err != nil {
			return driver.fail("session/set_mode", err)
		}
		if frame.Msg.Error != nil {
			// The mode IS the enforcement lever:
			// a mode that cannot be set means the envelope cannot
			// be honored, so the turn must not proceed.
			return Outcome{Row: RowSetupError, SessionID: driver.sessionID, Violations: driver.violations, Detail: "set_mode failed; the envelope's grade cannot be applied"}
		}
	}

	// The fence: sampled immediately before the prompt is sent.
	// Anything already routed is setup noise or replay; the
	// assembler refuses to buffer at or below it.
	driver.assembler = NewAssembler(driver.sessionID)
	driver.assembler.SetFence(conn.LastSeq())
	driver.inWindow = true

	promptCtx, cancelPrompt := context.WithTimeout(ctx, cfg.PromptTimeout)
	defer cancelPrompt()
	respFrame, err := driver.call(promptCtx, "session/prompt", map[string]any{
		"sessionId": driver.sessionID,
		"prompt":    []any{map[string]any{"type": "text", "text": cfg.Prompt}},
	})
	if err != nil {
		if ctx.Err() != nil && promptCtx.Err() == context.Canceled {
			// Parent cancellation, not the prompt deadline: send
			// session/cancel as the bounded courtesy and give the
			// server the grace window to settle; a COMPLETE
			// PromptResponse inside it wins (the matrix's
			// cancellation-race row).
			return driver.cancelAndSettle(cfg)
		}
		return driver.fail("prompt", err)
	}
	return driver.settle(respFrame, cfg.LateFrameWindow)
}

// call pumps notifications and server requests while one request
// is in flight, so the read loop never blocks on full queues and
// servers that demand answers before responding are served.
func (d *turnDriver) call(ctx context.Context, method string, params any) (Frame, error) {
	type callResult struct {
		frame Frame
		err   error
	}
	resultCh := make(chan callResult, 1)
	go func() {
		frame, err := d.conn.CallSeq(ctx, method, params)
		resultCh <- callResult{frame, err}
	}()
	for {
		select {
		case result := <-resultCh:
			return result.frame, result.err
		case frame, ok := <-d.conn.Notifications():
			if ok && frame.Msg.Method == "session/update" && d.assembler != nil {
				d.assembler.Consume(frame.Seq, frame.Msg.Params)
			}
		case frame, ok := <-d.conn.Requests():
			if ok {
				if err := d.answer(ctx, frame); err != nil {
					// A mandatory answer that never reached the
					// wire breaks the protocol; remember it so the
					// turn cannot settle as delivered.
					if d.failure == nil {
						d.failure = err
					}
				}
			}
		}
	}
}

// answer applies the correlation gate: permission requests outside
// the open prompt window or for the wrong session are violations
// answered cancelled; in-window ones take the strict posture.
// Anything that is not a permission request fails closed.
func (d *turnDriver) answer(ctx context.Context, frame Frame) error {
	request := frame.Msg
	if request.Method != "session/request_permission" {
		d.violations++
		return d.conn.RespondError(ctx, request.ID, -32601, "client capability not advertised")
	}
	var params struct {
		SessionID string             `json:"sessionId"`
		Options   []PermissionOption `json:"options"`
	}
	inWindow := d.inWindow
	if err := json.Unmarshal(request.Params, &params); err != nil || params.SessionID != d.sessionID || !inWindow {
		// Wrong session, unreadable, or outside the open window:
		// a violation, answered cancelled,
		// never normalized.
		d.violations++
		return d.conn.Respond(ctx, request.ID, PermissionAnswer{Outcome: outcomeCancelled}.WireResult())
	}
	// Strict-refusal posture until a captured dialect wires the
	// normalizer + Decide (no live capture has fired this request in
	// any envelope-relevant mode, so this is the defensive
	// backstop, not the enforcement lever).
	return d.conn.Respond(ctx, request.ID, StrictAnswer(params.Options).WireResult())
}

// fail maps a transport-level call failure to its matrix row.
func (d *turnDriver) fail(phase string, err error) Outcome {
	row := RowProtocolError
	if errors.Is(err, io.EOF) {
		row = RowTurnFailed
	}
	return Outcome{Row: row, SessionID: d.sessionID, Violations: d.violations, Detail: fmt.Sprintf("%s: %v", phase, err)}
}

// cancelAndSettle implements the cancellation race: the courtesy
// cancel goes out, and a complete PromptResponse inside the grace
// window still wins; otherwise the turn is cancelled. The script's
// kill path follows regardless — cancel is never a shutdown
// contract.
func (d *turnDriver) cancelAndSettle(cfg TurnConfig) Outcome {
	grace := cfg.CancelGrace
	if grace <= 0 {
		grace = 2 * time.Second
	}
	graceCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := d.conn.Notify(graceCtx, "session/cancel", map[string]any{"sessionId": d.sessionID}); err != nil {
		return Outcome{Row: RowCancelled, SessionID: d.sessionID, Violations: d.violations, Detail: fmt.Sprintf("cancelled; courtesy cancel failed: %v", err)}
	}
	// The prompt call was already abandoned with its context; the
	// response, if it lands, arrives as a pending-call frame the
	// read loop can no longer match — it kills the connection as an
	// unknown id. That is acceptable teardown: the turn is already
	// cancelled, and the kill path owns the rest. We wait only for
	// quiet.
	select {
	case <-time.After(grace):
	case <-d.conn.Done():
	}
	return Outcome{Row: RowCancelled, SessionID: d.sessionID, Violations: d.violations, Detail: "parent cancellation; courtesy cancel sent"}
}

// classifySetup maps a setup-phase response to its matrix row, or
// returns the session id on success. Auth classification keys on
// the pinned schema's code, never on advertisement.
func classifySetup(frame Frame, violations int, phase string) (*Outcome, string) {
	if frame.Msg.Error != nil {
		if frame.Msg.Error.Code == authRequiredCode {
			// Never interactive auth inside a job.
			return &Outcome{Row: RowAuthRequired, Violations: violations, Detail: fmt.Sprintf("%s: authentication required: %s", phase, frame.Msg.Error.Message)}, ""
		}
		return &Outcome{Row: RowSetupError, Violations: violations, Detail: fmt.Sprintf("%s: %d %s", phase, frame.Msg.Error.Code, frame.Msg.Error.Message)}, ""
	}
	var body newSessionResult
	if err := json.Unmarshal(frame.Msg.Result, &body); err != nil || (phase == "session/new" && body.SessionID == "") {
		return &Outcome{Row: RowProtocolError, Violations: violations, Detail: phase + " result unreadable"}, ""
	}
	return nil, body.SessionID
}

// settle maps the PromptResponse to its row. Queued frames drain
// first — the sequence fence, not drain timing, decides window
// membership — and the bounded late window drains as journaled
// evidence with post-window requests answered as violations.
func (d *turnDriver) settle(respFrame Frame, lateWindow time.Duration) Outcome {
	d.inWindow = false
	settleCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		select {
		case frame, ok := <-d.conn.Notifications():
			if ok && frame.Msg.Method == "session/update" {
				d.assembler.Consume(frame.Seq, frame.Msg.Params)
			}
			if !ok {
				goto drained
			}
			continue
		case frame, ok := <-d.conn.Requests():
			if ok {
				if frame.Seq < respFrame.Seq {
					// Arrived before the response on the wire: a
					// legitimate in-window request, answered under
					// the in-window policy.
					d.inWindow = true
					if err := d.answer(settleCtx, frame); err != nil && d.failure == nil {
						d.failure = err
					}
					d.inWindow = false
				} else {
					if err := d.answer(settleCtx, frame); err != nil && d.failure == nil {
						d.failure = err
					}
				}
			}
			if !ok {
				goto drained
			}
			continue
		default:
		}
		break
	}
drained:
	if d.failure != nil {
		return Outcome{Row: RowProtocolError, SessionID: d.sessionID, Violations: d.violations, Detail: fmt.Sprintf("mandatory answer never reached the wire: %v", d.failure)}
	}
	resp := respFrame.Msg
	if resp.Error != nil {
		return Outcome{Row: RowTurnFailed, SessionID: d.sessionID, Violations: d.violations, Detail: fmt.Sprintf("prompt error: %d %s", resp.Error.Code, resp.Error.Message)}
	}
	var body promptResult
	if err := json.Unmarshal(resp.Result, &body); err != nil {
		return Outcome{Row: RowProtocolError, SessionID: d.sessionID, Violations: d.violations, Detail: "prompt result unreadable"}
	}
	d.drainLate(lateWindow, respFrame.Seq)
	if d.failure != nil {
		return Outcome{Row: RowProtocolError, SessionID: d.sessionID, Violations: d.violations, Detail: fmt.Sprintf("mandatory answer never reached the wire: %v", d.failure)}
	}
	base := Outcome{StopReason: body.StopReason, SessionID: d.sessionID, UsageResult: body.Usage, Violations: d.violations}
	switch body.StopReason {
	case "end_turn":
		candidate, err := d.assembler.Candidate(respFrame.Seq)
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
// PromptResponse — live captures prove late frames are real.
// They are journaled evidence; the fence arithmetic keeps them out
// of the candidate, and post-window requests are violations
// answered cancelled.
func (d *turnDriver) drainLate(window time.Duration, responseSeq uint64) {
	if window <= 0 {
		return
	}
	lateCtx, cancel := context.WithTimeout(context.Background(), window+5*time.Second)
	defer cancel()
	deadline := time.After(window)
	for {
		select {
		case <-deadline:
			return
		case _, ok := <-d.conn.Notifications():
			if !ok {
				return
			}
		case frame, ok := <-d.conn.Requests():
			if !ok {
				return
			}
			if err := d.answer(lateCtx, frame); err != nil && d.failure == nil {
				d.failure = err
			}
			_ = responseSeq
		case <-d.conn.Done():
			return
		}
	}
}
