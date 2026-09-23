package partner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/acp"
)

// The host: the many-turn owner on one acp.Conn.
//
// The kit's own client is single-turn by construction — a prompt exhausts the
// session, a repeated run re-initialises, and a cancel abandons the pending
// prompt so that its eventual response kills the connection. None of that is
// wrong for a job; all of it is wrong for a conversation. So this file owns
// the other lifecycle: initialise once, open one session in the checkout and
// keep it, service updates and permission requests for the life of the
// process, prompt one turn at a time, and settle a cancellation — wait for the
// cancelled prompt's own response — before admitting the next turn. The
// connection survives Stop, which is the whole point: the next question
// continues the same conversation.
//
// Timers here are the server's, which the frontend rules allow and the design
// names: a prompt is bounded, a cancellation's settlement is bounded, and a
// process that nobody has spoken to for an hour is torn down.

// The host's three clocks. They are variables rather than constants so a test
// can run the whole lifecycle in milliseconds; nothing outside this package
// changes them.
var (
	// handshakeTimeout bounds initialize, session/new and the model selection
	// together — everything between the process starting and it being ready
	// for a question.
	handshakeTimeout = 2 * time.Minute
	// promptTimeout bounds one answer. A model that has said nothing for ten
	// minutes is a turn that failed, not a turn still thinking.
	promptTimeout = 10 * time.Minute
	// settleGrace bounds the wait for a cancelled prompt's own response. Past
	// it the process is torn down rather than left half-cancelled.
	settleGrace = 30 * time.Second
	// idleLife is how long a process with nothing to do is kept. The session
	// carries the conversation while it lives, so this is also how long the
	// Partner's memory of the exchange survives without the transcript.
	idleLife = time.Hour
)

// Update is one beat of a turn as it happens.
type Update struct {
	// Kind is text, activity or look: the answer itself, what the Partner is
	// doing while it composes one, and one completed read with its outcome.
	Kind string
	Text string
	// Look is set on a look and nowhere else.
	Look *Look
}

// The four update kinds.
//
// Doing and activity are different things, and the difference is what makes
// the conversation quiet. Doing is what the Partner is at THIS moment — one
// line, replaced by the next, kept nowhere — and a tool call is doing
// something. Activity is what a human has to be told and must still be able to
// read afterwards: a refusal, a session that had to be opened fresh. Nothing
// says a thing twice: a tool call becomes a look when it completes, and the
// line that announced it goes.
const (
	UpdateText     = "text"
	UpdateActivity = "activity"
	UpdateDoing    = "doing"
	UpdateLook     = "look"
)

// Look is one thing the Partner read, with its completion.
//
// A count is not an account: "looked at three things" can hide a failed read
// and a partial one behind a number that sounds like success. So a look names
// what was read, the reading it was of — the accepted tip and the moment it
// was observed, or a file's revision as it stands — how it ended, and enough
// of what came back to check it against.
type Look struct {
	What   string `json:"what"`
	Source string `json:"source,omitempty"`
	// Outcome is read, partial or failed. A failed read is never counted as a
	// look, and is still listed.
	Outcome string `json:"outcome"`
	Excerpt string `json:"excerpt,omitempty"`
	// Page marks the one reading the Partner did not choose: the page the
	// human was looking at. It is listed first and separately, and it is not
	// one of the things the count says the Partner looked at.
	Page bool `json:"page,omitempty"`
}

// The three outcomes a look can have.
const (
	LookRead    = "read"
	LookPartial = "partial"
	LookFailed  = "failed"
)

// Counted reports whether this look is one of the N the page counts: a read
// the Partner chose that actually happened.
func (l Look) Counted() bool { return !l.Page && l.Outcome != LookFailed }

// maxExcerpt is how much of one read the conversation keeps. It is what a
// human checks an answer against, not the read itself: the whole of it went to
// the Partner, and the Partner is the thing being checked.
const maxExcerpt = 1200

// Result is how one turn ended.
type Result struct {
	// Outcome is complete, stopped, failed or refused.
	Outcome string
	// Detail is the server's own words where the turn did not complete.
	Detail string
}

// The four outcomes a turn can end in. They are the transcript's vocabulary
// too: what the page renders is what the host decided.
const (
	OutcomeComplete = "complete"
	OutcomeStopped  = "stopped"
	OutcomeFailed   = "failed"
	OutcomeRefused  = "refused"
)

// StartError is a runtime that could not be started, initialised, or signed
// in. Reason is the server's own words — or the operating system's, for a
// command that is not there — and Install is the line that installs the
// runtime where this build knows one. It is the 503 the route answers with.
type StartError struct {
	Reason  string
	Install string
}

func (e *StartError) Error() string {
	if e.Install == "" {
		return e.Reason
	}
	return e.Reason + "; install it with: " + e.Install
}

// Endpoint is one started runtime's wire, and the way to end it. It is
// indirected so a test can drive the host over an in-process pipe against a
// fake ACP server, and so the walkthrough can do the same with a canned one:
// nothing in this file spawns a real agent for a test.
type Endpoint struct {
	// Reader and Writer are the server's stdout and stdin.
	Reader io.Reader
	Writer io.Writer
	// Journal receives every frame in both directions; nil journals nothing.
	Journal io.Writer
	// Close ends the process and releases the wire. It is called once.
	Close func()
	// Words reports what the process said on its error stream, which is where
	// a runtime that refuses to start says why.
	Words func() string
}

// Host is the many-turn owner. One host serves one human's conversation on one
// checkout.
type Host struct {
	runtime  Runtime
	checkout string
	// start opens one endpoint. The real one spawns the runtime's command;
	// a test's opens a pipe.
	start func(context.Context) (Endpoint, error)

	mu      sync.Mutex
	live    *live
	turning bool
	// settled closes when the running turn's own response has landed, which
	// is what Stop waits for rather than polling.
	settled chan struct{}
	idle    *time.Timer
}

// live is one started process with one open session on it.
type live struct {
	endpoint Endpoint
	conn     *acp.Conn
	// session is written once by the handshake and read by the pump, so it is
	// behind a lock: the pump runs from the moment the connection opens, which
	// is before the session has a name.
	sessionMu sync.RWMutex
	sessionID string
	closed    chan struct{}
	closeOnce sync.Once

	// fence and fenced settle one turn against the pump.
	//
	// The connection's channels lose ordering BETWEEN them: a chunk that
	// arrived before the prompt's response can be selected after it, so a turn
	// that stopped listening the moment its response landed would drop the
	// last of its own answer. The read loop routes frames in order, so every
	// frame older than the response is already in a buffer when the response
	// arrives; the turn asks the pump to drain what is buffered and waits for
	// it to say it has.
	fence  chan struct{}
	fenced chan struct{}

	// sink is the current turn's listener, swapped under the host's lock. The
	// pump reads it for every update it services, so a beat that arrives
	// between turns reaches nobody rather than the wrong turn.
	sinkMu sync.Mutex
	sink   func(Update)

	// calls is what each running tool call is called, by its id. A completion
	// carries the id and often nothing else, and "what was read" is the title
	// the call started with.
	callsMu sync.Mutex
	calls   map[string]string
}

// NewHost builds a host for one runtime and checkout, spawning the runtime's
// own command. journal, when it names a file, receives every frame of the
// process's life; the file is truncated when a process starts, so what it
// holds is one process's wire and not a year of them.
func NewHost(runtime Runtime, checkout, journal string) *Host {
	host := &Host{runtime: runtime, checkout: checkout}
	host.start = func(ctx context.Context) (Endpoint, error) {
		return spawn(ctx, runtime, checkout, journal)
	}
	return host
}

// NewHostOn builds a host over an endpoint a caller opens, which is how the
// tests and the walkthrough drive a fake ACP server.
func NewHostOn(runtime Runtime, checkout string, open func(context.Context) (Endpoint, error)) *Host {
	return &Host{runtime: runtime, checkout: checkout, start: open}
}

// Runtime is what this host was admitted with.
func (h *Host) Runtime() Runtime { return h.runtime }

// Ready starts the process and opens the session when none lives, and reports
// whether the session the caller is about to prompt is a fresh one — which is
// what the conversation owner needs in order to supply the history the lost
// session carried.
func (h *Host) Ready(ctx context.Context) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.live != nil && !h.live.dead() {
		return false, nil
	}
	h.live = nil
	session, err := h.open(ctx)
	if err != nil {
		return false, err
	}
	h.live = session
	h.restartIdleLocked()
	return true, nil
}

// open starts one process and takes it through initialize, session/new and the
// model selection. Every refusal on the way is a StartError carrying the
// server's own words.
func (h *Host) open(ctx context.Context) (*live, error) {
	endpoint, err := h.start(ctx)
	if err != nil {
		return nil, &StartError{Reason: err.Error(), Install: h.runtime.Install}
	}
	session := &live{
		endpoint: endpoint, closed: make(chan struct{}),
		fence: make(chan struct{}), fenced: make(chan struct{}),
		calls: map[string]string{},
	}
	session.conn = acp.NewConn(endpoint.Reader, endpoint.Writer, endpoint.Journal)
	go session.pump(h.checkout)

	handshake, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	if err := h.handshake(handshake, session); err != nil {
		session.close()
		return nil, err
	}
	return session, nil
}

// handshake is initialize, session/new and the model selection, in that order.
func (h *Host) handshake(ctx context.Context, session *live) error {
	frame, err := session.conn.Call(ctx, "initialize", map[string]any{
		"protocolVersion": int64(1),
		"clientCapabilities": map[string]any{
			// Advertise nothing: no client filesystem, no terminal. A second
			// side-effect path would go round the runtime's own read-only
			// configuration and round the permission point both.
			"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]any{"name": "metasystem-partner", "version": "1"},
	})
	if err != nil {
		return h.startFailure("the Partner's runtime did not answer initialize", err, session)
	}
	if frame.Error != nil {
		return &StartError{Reason: frame.Error.Message, Install: h.runtime.Install}
	}
	var initialized struct {
		ProtocolVersion int64 `json:"protocolVersion"`
	}
	if err := json.Unmarshal(frame.Result, &initialized); err != nil {
		return &StartError{Reason: "the Partner's runtime answered initialize with something this client cannot read"}
	}
	if initialized.ProtocolVersion != 1 {
		return &StartError{Reason: fmt.Sprintf(
			"the Partner's runtime speaks ACP %d and this client speaks 1", initialized.ProtocolVersion)}
	}

	// The one tool server this session is given, through the protocol's own
	// hand-off. It is named here rather than in the runtime's configuration
	// because the configuration is the read-only contract: a tool that arrives
	// through it would be a tool the contract cannot speak about.
	params := map[string]any{"cwd": h.checkout, "mcpServers": h.runtime.Tools.wire()}
	if h.runtime.SessionMeta != nil {
		params["_meta"] = h.runtime.SessionMeta
	}
	frame, err = session.conn.Call(ctx, "session/new", params)
	if err != nil {
		return h.startFailure("the Partner's runtime did not open a session", err, session)
	}
	if frame.Error != nil {
		return &StartError{Reason: frame.Error.Message, Install: h.runtime.Install}
	}
	var opened struct {
		SessionID     string `json:"sessionId"`
		ConfigOptions []struct {
			ID string `json:"id"`
		} `json:"configOptions"`
	}
	if err := json.Unmarshal(frame.Result, &opened); err != nil || opened.SessionID == "" {
		return &StartError{Reason: "the Partner's runtime opened a session this client cannot name"}
	}
	session.setSession(opened.SessionID)

	if h.runtime.Model == "" {
		return nil
	}
	offered := false
	for _, option := range opened.ConfigOptions {
		if option.ID == modelConfigID {
			offered = true
		}
	}
	if !offered {
		return &StartError{Reason: fmt.Sprintf(
			"the Partner's runtime offers no model selection, so %q cannot be selected; clear ui.partner.model to use the runtime's own default",
			h.runtime.Model)}
	}
	frame, err = session.conn.Call(ctx, "session/set_config_option", map[string]any{
		"sessionId": session.session(), "configId": modelConfigID, "value": h.runtime.Model,
	})
	if err != nil {
		return h.startFailure("the Partner's runtime did not answer the model selection", err, session)
	}
	if frame.Error != nil {
		// The model IS the configuration: a session running a model nobody
		// chose is not the session this seat asked for, so the turn does not
		// proceed and the server's sentence is what the human reads.
		return &StartError{Reason: frame.Error.Message}
	}
	return nil
}

// startFailure dresses a transport failure with whatever the process said on
// its error stream, which for a runtime that is not signed in is the only
// place the reason appears.
func (h *Host) startFailure(what string, err error, session *live) error {
	reason := what + ": " + err.Error()
	if words := strings.TrimSpace(session.endpoint.words()); words != "" {
		reason = words
	}
	return &StartError{Reason: reason, Install: h.runtime.Install}
}

// Prompt runs one turn and streams it to sink. It is one at a time: a second
// caller waits, which cannot happen through the routes because the
// conversation owner answers 409 first, and must still be true here.
//
// The context bounds the caller's wait, never the turn: Stop is the lever that
// ends a turn, and a browser that went away must not cancel an answer the
// transcript is going to keep.
func (h *Host) Prompt(ctx context.Context, text string, sink func(Update)) (Result, error) {
	h.mu.Lock()
	if h.turning {
		h.mu.Unlock()
		return Result{}, errors.New("a turn is already running")
	}
	session := h.live
	if session == nil || session.dead() {
		h.mu.Unlock()
		return Result{}, &StartError{Reason: "the Partner's session is not open", Install: h.runtime.Install}
	}
	h.turning = true
	settled := make(chan struct{})
	h.settled = settled
	h.stopIdleLocked()
	h.mu.Unlock()

	session.listen(sink)
	defer func() {
		session.listen(nil)
		h.mu.Lock()
		h.turning = false
		h.settled = nil
		h.restartIdleLocked()
		h.mu.Unlock()
		close(settled)
	}()

	promptCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), promptTimeout)
	defer cancel()
	frame, err := session.conn.Call(promptCtx, "session/prompt", map[string]any{
		"sessionId": session.session(),
		"prompt":    []any{map[string]any{"type": "text", "text": text}},
	})
	// The answer settles against the pump before the turn is read: a chunk
	// that arrived before the response must not be lost to the channel
	// hand-off that selected the response first.
	session.settle()
	if err != nil {
		// The connection died or the turn ran past its bound. Either way this
		// process cannot carry another turn, so it goes; the next question
		// opens a fresh session and is told so.
		words := strings.TrimSpace(session.endpoint.words())
		h.drop(session)
		if words == "" {
			words = err.Error()
		}
		return Result{Outcome: OutcomeFailed, Detail: words}, nil
	}
	if frame.Error != nil {
		return Result{Outcome: OutcomeFailed, Detail: frame.Error.Message}, nil
	}
	var answered struct {
		StopReason string `json:"stopReason"`
	}
	if err := json.Unmarshal(frame.Result, &answered); err != nil {
		return Result{Outcome: OutcomeFailed, Detail: "the Partner's runtime answered with something this client cannot read"}, nil
	}
	switch answered.StopReason {
	case "end_turn":
		return Result{Outcome: OutcomeComplete}, nil
	case "cancelled":
		return Result{Outcome: OutcomeStopped}, nil
	case "refusal":
		return Result{Outcome: OutcomeRefused, Detail: "the Partner's runtime refused the turn"}, nil
	case "max_tokens", "max_turn_requests":
		return Result{Outcome: OutcomeStopped, Detail: "the Partner reached its limit for one answer (" + answered.StopReason + ")"}, nil
	default:
		return Result{Outcome: OutcomeFailed, Detail: "the Partner's runtime ended the turn with an unknown reason: " + answered.StopReason}, nil
	}
}

// Stop cancels the running turn through the protocol and lets Prompt settle
// it. The cancellation notification is all that is sent here: the prompt call
// is deliberately left pending, so its own response arrives, matches, and
// leaves the connection alive for the next question. A process that does not
// answer within the grace is torn down, and Prompt reports the failure.
func (h *Host) Stop(ctx context.Context) error {
	h.mu.Lock()
	session := h.live
	running := h.turning
	settled := h.settled
	h.mu.Unlock()
	if session == nil || !running || settled == nil {
		return nil
	}
	notify, cancel := context.WithTimeout(ctx, settleGrace)
	defer cancel()
	if err := session.conn.Notify(notify, "session/cancel", map[string]any{"sessionId": session.session()}); err != nil {
		h.drop(session)
		return err
	}
	// Settle: the turn ends when its own response lands. Past the grace the
	// process is not going to answer, and holding the conversation open on it
	// would leave the page busy for ever.
	settle := time.NewTimer(settleGrace)
	defer settle.Stop()
	select {
	case <-settled:
		return nil
	case <-settle.C:
		h.drop(session)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Busy reports whether a turn is running.
func (h *Host) Busy() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.turning
}

// Session is the live session's id, or empty when no session stands. It is
// what the conversation records beside its transcript.
func (h *Host) Session() string {
	h.mu.Lock()
	session := h.live
	h.mu.Unlock()
	if session == nil {
		return ""
	}
	return session.session()
}

// Alive reports whether a session stands.
func (h *Host) Alive() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.live != nil && !h.live.dead()
}

// Close ends the process and the session. The host is reusable afterwards: the
// next Ready opens a fresh one.
func (h *Host) Close() {
	h.mu.Lock()
	session := h.live
	h.live = nil
	h.stopIdleLocked()
	h.mu.Unlock()
	if session != nil {
		session.close()
	}
}

// drop ends one process, if it is still the host's.
func (h *Host) drop(session *live) {
	h.mu.Lock()
	if h.live == session {
		h.live = nil
		h.stopIdleLocked()
	}
	h.mu.Unlock()
	session.close()
}

// restartIdleLocked arms the idle teardown. The host's lock is held.
func (h *Host) restartIdleLocked() {
	h.stopIdleLocked()
	if h.live == nil {
		return
	}
	session := h.live
	h.idle = time.AfterFunc(idleLife, func() { h.drop(session) })
}

func (h *Host) stopIdleLocked() {
	if h.idle != nil {
		h.idle.Stop()
		h.idle = nil
	}
}

// session and setSession guard the one field the handshake writes and the
// pump reads.
func (l *live) session() string {
	l.sessionMu.RLock()
	defer l.sessionMu.RUnlock()
	return l.sessionID
}

func (l *live) setSession(id string) {
	l.sessionMu.Lock()
	l.sessionID = id
	l.sessionMu.Unlock()
}

// settle asks the pump to hand over everything the connection routed before
// the turn's response, and waits for it to say it has. A pump that is gone
// answers nothing, and the bound is what keeps the turn from waiting on it.
func (l *live) settle() {
	select {
	case l.fence <- struct{}{}:
	case <-l.closed:
		return
	case <-time.After(settleGrace):
		return
	}
	select {
	case <-l.fenced:
	case <-l.closed:
	case <-time.After(settleGrace):
	}
}

// listen swaps the turn's listener.
func (l *live) listen(sink func(Update)) {
	l.sinkMu.Lock()
	l.sink = sink
	l.sinkMu.Unlock()
}

func (l *live) emit(update Update) {
	l.sinkMu.Lock()
	sink := l.sink
	l.sinkMu.Unlock()
	if sink != nil {
		sink(update)
	}
}

func (l *live) dead() bool {
	select {
	case <-l.closed:
		return true
	default:
	}
	select {
	case <-l.conn.Done():
		return true
	default:
		return false
	}
}

func (l *live) close() {
	l.closeOnce.Do(func() {
		close(l.closed)
		if l.endpoint.Close != nil {
			l.endpoint.Close()
		}
	})
}

// pump services updates and permission requests for the life of the process —
// not for the life of one call. That is the difference between a job's driver
// and a conversation's host: a server that asks a question between turns is
// still answered, and a notification that arrives while nothing is running is
// still read rather than left to fill a channel and wedge the read loop.
func (l *live) pump(checkout string) {
	for {
		select {
		case frame, open := <-l.conn.Notifications():
			if !open {
				l.close()
				return
			}
			if frame.Msg.Method == "session/update" {
				l.update(frame.Msg.Params)
			}
		case frame, open := <-l.conn.Requests():
			if !open {
				l.close()
				return
			}
			l.answer(frame, checkout)
		case <-l.fence:
			// Everything the read loop routed before the turn's response is
			// already in these buffers, so draining them here is draining
			// exactly the turn's own remainder. Nothing blocks: what is not
			// buffered yet belongs to no settled turn.
			l.drain(checkout)
			select {
			case l.fenced <- struct{}{}:
			case <-l.closed:
				return
			}
		case <-l.closed:
			return
		}
	}
}

// drain handles everything already routed, and stops the moment there is
// nothing left rather than waiting for more.
func (l *live) drain(checkout string) {
	for {
		select {
		case frame, open := <-l.conn.Notifications():
			if !open {
				return
			}
			if frame.Msg.Method == "session/update" {
				l.update(frame.Msg.Params)
			}
		case frame, open := <-l.conn.Requests():
			if !open {
				return
			}
			l.answer(frame, checkout)
		default:
			return
		}
	}
}

// update turns one session/update into what the page shows: the answer's text
// as it streams, and a muted line for the work the Partner does on the way.
// The thought stream is not the answer and never reaches the page.
func (l *live) update(params json.RawMessage) {
	var envelope struct {
		SessionID string          `json:"sessionId"`
		Update    json.RawMessage `json:"update"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		return
	}
	if named := l.session(); named != "" && envelope.SessionID != named {
		return
	}
	var named struct {
		Kind string `json:"sessionUpdate"`
	}
	if err := json.Unmarshal(envelope.Update, &named); err != nil {
		return
	}
	// The three shapes are parsed separately, and deliberately. A tool call's
	// content is an array of blocks and a message chunk's is one block, so one
	// struct over both reads neither: the array fails to unmarshal into the
	// object and the whole update is dropped — which is how the completion of
	// every tool call went missing before this slice.
	switch named.Kind {
	case "agent_message_chunk":
		var body struct {
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(envelope.Update, &body); err != nil {
			return
		}
		if body.Content.Type == "text" && body.Content.Text != "" {
			l.emit(Update{Kind: UpdateText, Text: body.Content.Text})
		}
	case "tool_call", "tool_call_update":
		var body toolCall
		if err := json.Unmarshal(envelope.Update, &body); err != nil {
			return
		}
		l.tool(named.Kind == "tool_call", body)
	}
}

// toolCall is the part of a tool call, and of its later completion, this host
// reads: what it is, what it asked for, how it ended, and what came back.
type toolCall struct {
	ToolCallID string `json:"toolCallId"`
	Title      string `json:"title"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	// Name is the tool's own name where the runtime carries one, which is how
	// a call to the interface's tool server is told from any other.
	Name     string          `json:"name"`
	RawInput json.RawMessage `json:"rawInput"`
	Meta     struct {
		ClaudeCode struct {
			ToolName string `json:"toolName"`
		} `json:"claudeCode"`
	} `json:"_meta"`
	Content []struct {
		Type    string `json:"type"`
		Content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"content"`
}

// tool folds one tool call and its completion into what the page shows: a
// muted line while it runs, and a look when it ends.
func (l *live) tool(started bool, body toolCall) {
	id := strings.TrimSpace(body.ToolCallID)
	what := strings.TrimSpace(body.Title)
	if started {
		if what == "" {
			what = strings.TrimSpace(body.Name)
		}
		if id != "" {
			l.callsMu.Lock()
			l.calls[id] = what
			l.callsMu.Unlock()
		}
		if what != "" {
			l.emit(Update{Kind: UpdateDoing, Text: what})
		}
		return
	}
	if what == "" && id != "" {
		l.callsMu.Lock()
		what = l.calls[id]
		l.callsMu.Unlock()
	}
	switch body.Status {
	case "completed", "failed":
	default:
		// A call that is still running says nothing new: what it is was said
		// when it started, and how it ended is what a look is.
		return
	}
	if id != "" {
		l.callsMu.Lock()
		delete(l.calls, id)
		l.callsMu.Unlock()
	}
	l.emit(Update{Kind: UpdateLook, Look: lookAt(what, body)})
}

// lookAt reads one completion into a look. The source and the outcome are the
// tool server's own words where it answered: it stamps every result with what
// it read from and says how much of the whole it supplied, so neither has to
// be inferred here.
func lookAt(what string, body toolCall) *Look {
	text := ""
	for _, block := range body.Content {
		if block.Type == "content" && block.Content.Type == "text" {
			text += block.Content.Text
		}
	}
	look := &Look{What: what, Outcome: LookRead}
	if look.What == "" {
		look.What = "an unnamed tool call"
	}
	if body.Status == "failed" {
		look.Outcome = LookFailed
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "Source: "):
			look.Source = strings.TrimSpace(strings.TrimPrefix(line, "Source: "))
		case strings.HasPrefix(line, "Outcome: this read failed"):
			look.Outcome = LookFailed
		case strings.HasPrefix(line, "More remains:"):
			if look.Outcome == LookRead {
				look.Outcome = LookPartial
			}
		}
	}
	look.Excerpt = text
	if len(look.Excerpt) > maxExcerpt {
		look.Excerpt = look.Excerpt[:maxExcerpt] + "…"
	}
	return look
}

// answer applies the permission point to one server request. Anything that is
// not a permission request fails closed with the JSON-RPC error for a
// capability this client never advertised; a permission request is judged, and
// its refusal becomes an activity line.
func (l *live) answer(frame acp.Frame, checkout string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if frame.Msg.Method != "session/request_permission" {
		_ = l.conn.RespondError(ctx, frame.Msg.ID, -32601, "client capability not advertised")
		l.emit(Update{Kind: UpdateActivity, Text: "Refused: the Partner asked for " + frame.Msg.Method + ", which this client does not offer"})
		return
	}
	decided := judge(frame.Msg.Params, l.session(), checkout)
	if err := l.conn.Respond(ctx, frame.Msg.ID, decided.answer.WireResult()); err != nil {
		// The answer never reached the wire; the turn will fail on its own,
		// and a refusal nobody was told of must not be shown as one.
		return
	}
	l.emit(Update{Kind: UpdateActivity, Text: decided.activity})
}

func (e Endpoint) words() string {
	if e.Words == nil {
		return ""
	}
	return e.Words()
}

// spawn starts the runtime's own command in the checkout and hands back its
// wire. The process is given this server's environment plus the runtime's own
// read-only configuration, and nothing else: it has no browser session, no
// cookie, and no way to obtain one.
func spawn(ctx context.Context, runtime Runtime, checkout, journal string) (Endpoint, error) {
	if len(runtime.Argv) == 0 {
		return Endpoint{}, errors.New("this runtime has no command")
	}
	stopped, stop := context.WithCancel(context.WithoutCancel(ctx))
	command := exec.CommandContext(stopped, runtime.Argv[0], runtime.Argv[1:]...)
	command.Dir = checkout
	command.Env = append(os.Environ(), runtime.Env...)
	stdin, err := command.StdinPipe()
	if err != nil {
		stop()
		return Endpoint{}, err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		stop()
		return Endpoint{}, err
	}
	said := &words{}
	command.Stderr = said
	var log *os.File
	if journal != "" {
		log, _ = os.OpenFile(journal, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	}
	if err := command.Start(); err != nil {
		stop()
		if log != nil {
			_ = log.Close()
		}
		return Endpoint{}, err
	}
	var journalWriter io.Writer
	if log != nil {
		journalWriter = log
	}
	return Endpoint{
		Reader:  stdout,
		Writer:  stdin,
		Journal: journalWriter,
		Close: func() {
			stop()
			_ = stdin.Close()
			_ = command.Wait()
			if log != nil {
				_ = log.Close()
			}
		},
		Words: said.read,
	}, nil
}

// words is the tail of what a process said on its error stream: bounded,
// because a runtime that logs every frame must not become this server's
// memory, and kept because it is the only place some refusals appear.
type words struct {
	mu   sync.Mutex
	held []byte
}

const maxWords = 8 << 10

func (w *words) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.held = append(w.held, b...)
	if len(w.held) > maxWords {
		w.held = w.held[len(w.held)-maxWords:]
	}
	return len(b), nil
}

func (w *words) read() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.held)
}
