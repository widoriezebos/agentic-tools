// The native ACP driver: acp.RunTurn conformed to the delegate
// seam's complete-session contract (acp-adapter-seam slice two,
// records/acp/acp-seam-s2-design.md — three critique rounds, landed at the
// declared failsafe). The adapter keeps custody of process, pipes,
// and journal; the driver only speaks. The event tap makes the
// seam's EventStream real; asks stay policy-refused below the seam
// because PreflightACP admits only approvals=deny envelopes, and the
// refused ask is visible on the stream. Sessions are ONE TURN WIDE:
// the claim is consumed by entry, and delegate.ErrSessionExhausted
// is the typed exhaustion.
package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// DefaultHandshakeTimeout is the one handshake default, shared with
// the acp turn verb's flag so the two surfaces cannot drift.
const DefaultHandshakeTimeout = 120 * time.Second

// SpoolCapacity bounds the turn's event spool by COUNT, and
// SpoolByteCapacity bounds it by buffered PARAMS BYTES — either
// bound tripping enters drop mode (contiguous drops, one
// events-dropped gap event at the drain-to-empty transition), and
// the tap never blocks the pump. Two bounds because events are not
// uniform: a flood of tiny updates exhausts count, a few giant ones
// exhaust memory.
const SpoolCapacity = 4096

// SpoolByteCapacity is the spool's params-byte bound (8 MiB).
const SpoolByteCapacity = 8 << 20

// Typed refusal identities (design R3-06); each errors.Is-able.
var (
	ErrPreflightRefused = errors.New("acp: envelope refused by preflight")
	ErrModeUnresolved   = errors.New("acp: session mode unresolved")
	ErrTimeoutUnset     = errors.New("acp: a zero turn timeout is refused; set PromptTimeout, LateWindow, and CancelGrace explicitly")
	ErrAskPolicyRefused = errors.New("acp: asks are answered below the seam by the strict-refusal policy; there is nothing to answer")
)

// NativeDriver is the (runtime-agnostic) native protocol driver.
// Construction data only; per-session state lives on the session.
type NativeDriver struct {
	// HandshakeTimeout defaults to DefaultHandshakeTimeout when zero
	// (the seam's PromptRequest carries no handshake field).
	HandshakeTimeout time.Duration
	// ModeResolver maps the envelope's tools grade to the runtime's
	// dialect mode id; PromptRequest.Mode overrides it when set.
	ModeResolver func(tools string) (string, error)

	expectedProtocol int64
	declaration      delegate.Declaration
}

// Declaration reports the native driver's earned capability surface
// (design: "The declaration, earned").
func (d *NativeDriver) Declaration() delegate.Declaration { return d.declaration }

// Open builds the protocol connection over the pre-opened pipes and
// returns a one-shot session. No protocol traffic happens at Open;
// the wire opens with the turn.
func (d *NativeDriver) Open(_ context.Context, req delegate.OpenRequest) (delegate.Session, error) {
	if req.Endpoint.FromAgent == nil || req.Endpoint.ToAgent == nil || req.Endpoint.Journal == nil {
		return nil, fmt.Errorf("acp: open needs the full endpoint (from-agent, to-agent, journal)")
	}
	return &nativeSession{
		driver:    d,
		conn:      NewConn(req.Endpoint.FromAgent, req.Endpoint.ToAgent, req.Endpoint.Journal),
		workspace: req.Workspace,
		resumeID:  req.ResumeSessionID,
	}, nil
}

type nativeSession struct {
	driver    *NativeDriver
	conn      *Conn
	workspace string
	resumeID  string
	claimed   atomic.Bool
}

// Quiescer is the evidence owner's finalization surface: after the
// owner closes the pipes, Quiesce waits for the connection's read
// loop to stop (bounded by ctx) and reports journal health — the
// same Done-wait + JournalErr sampling the acp turn verb performs.
// The native session implements it; the owner type-asserts.
type Quiescer interface {
	Quiesce(ctx context.Context) error
}

// Quiesce waits for the connection to stop and returns the journal's
// health. Call AFTER closing the endpoint's pipes; evidence is final
// only when Quiesce has returned.
func (s *nativeSession) Quiesce(ctx context.Context) error {
	select {
	case <-s.conn.Done():
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.conn.JournalErr()
}

// PromptTurn claims the session (consumed by entry, not by success),
// refuses ineligible envelopes and unset timeouts before any wire
// traffic, resolves the mode, and starts the one pump goroutine.
func (s *nativeSession) PromptTurn(ctx context.Context, req delegate.PromptRequest) (delegate.Turn, error) {
	if !s.claimed.CompareAndSwap(false, true) {
		return nil, delegate.ErrSessionExhausted
	}
	// The claim is consumed even by this refusal (consumed by entry).
	// The turn deliberately OUTLIVES the caller's context once
	// started — Cancel is the lever after that — but a caller that
	// is already dead must not start a turn at all.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	envelope := Envelope{
		ReadRoots:  req.Envelope.ReadRoots,
		WriteRoots: req.Envelope.WriteRoots,
		Network:    req.Envelope.Network,
		Approvals:  req.Envelope.Approvals,
		Tools:      req.Envelope.Tools,
	}
	if reason := PreflightACP(envelope); reason != "" {
		return nil, fmt.Errorf("%w: %s", ErrPreflightRefused, reason)
	}
	mode := req.Mode
	if mode == "" {
		if s.driver.ModeResolver == nil {
			return nil, fmt.Errorf("%w: no resolver and no explicit mode", ErrModeUnresolved)
		}
		resolved, err := s.driver.ModeResolver(req.Envelope.Tools)
		if err != nil || resolved == "" {
			return nil, fmt.Errorf("%w: tools grade %q (%v)", ErrModeUnresolved, req.Envelope.Tools, err)
		}
		mode = resolved
	}
	if req.PromptTimeout <= 0 || req.LateWindow <= 0 || req.CancelGrace <= 0 {
		return nil, ErrTimeoutUnset
	}
	handshake := s.driver.HandshakeTimeout
	if handshake <= 0 {
		handshake = DefaultHandshakeTimeout
	}
	turnCtx, cancelTurn := context.WithCancel(context.Background())
	turn := &nativeTurn{
		cancelTurn: cancelTurn,
		done:       make(chan struct{}),
	}
	turn.spool.capacity = SpoolCapacity
	turn.spool.byteCapacity = SpoolByteCapacity
	cfg := TurnConfig{
		ExpectedProtocolVersion: s.driver.expectedProtocol,
		WorkspaceDir:            s.workspace,
		LoadSessionID:           s.resumeID,
		ModeID:                  mode,
		Prompt:                  string(req.Prompt),
		Envelope:                envelope,
		HandshakeTimeout:        handshake,
		PromptTimeout:           req.PromptTimeout,
		LateFrameWindow:         req.LateWindow,
		CancelGrace:             req.CancelGrace,
		OnEvent:                 turn.tap,
	}
	go func() {
		outcome := RunTurn(turnCtx, s.conn, cfg)
		turn.mu.Lock()
		turn.outcome = outcome
		turn.finalized = true
		turn.mu.Unlock()
		turn.spool.settle()
		close(turn.done)
	}()
	return turn, nil
}

// nativeTurn is one prompt turn in flight: the spool, the controls,
// and the settled, cached outcome (accessor reads copy their bytes —
// no caller can mutate what another observes).
type nativeTurn struct {
	cancelTurn context.CancelFunc
	done       chan struct{}
	// mu guards outcome and the three lifecycle bools as ONE state:
	// cancel classification must be atomic against both settlement
	// start (CC-S2-003) and finalization (CC-S2-004) — a Cancel that
	// races either must never rewrite what an earlier Result showed.
	mu                sync.Mutex
	outcome           Outcome
	finalized         bool
	cancelRequested   bool
	settlementStarted bool
	spool             eventSpool
}

// tap is the pump's synchronous observer: the internal
// settlement-started beat is consumed here (never projected), and
// everything else is projected into seam vocabulary and spooled.
func (t *nativeTurn) tap(ev TapEvent) {
	if ev.Kind == TapSettlementStarted {
		t.mu.Lock()
		t.settlementStarted = true
		t.mu.Unlock()
		return
	}
	kind, params := "driver/"+ev.Kind, ev.Params
	if !ev.Synthetic {
		var discriminator string
		discriminator, params = projectUpdate(ev.Params)
		kind = "update/" + discriminator
	}
	t.spool.append(delegate.Event{Seq: ev.WireSeq, Kind: kind, Params: append([]byte(nil), params...)})
}

// projectUpdate applies the projection law: the delegate event's
// Kind is the nested sessionUpdate discriminator and its Params are
// the inner update OBJECT verbatim — never the outer notification
// wrapper. The fallbacks preserve WHAT failed instead of inventing a
// kind the server never sent: unparseable params project as
// "malformed" (wrapper bytes kept as evidence), a parseable update
// without the discriminator as "undeclared" — a server that really
// sends "unknown" keeps its own word.
func projectUpdate(params []byte) (string, []byte) {
	var body struct {
		Update json.RawMessage `json:"update"`
	}
	if err := json.Unmarshal(params, &body); err != nil || len(body.Update) == 0 ||
		bytes.Equal(bytes.TrimSpace(body.Update), []byte("null")) {
		// No update object at all — including a literal JSON null,
		// which unmarshals "successfully" into an empty struct and
		// would otherwise masquerade as undeclared while losing the
		// wrapper evidence (CC-R3-002).
		return "malformed", params
	}
	var inner struct {
		SessionUpdate string `json:"sessionUpdate"`
	}
	if err := json.Unmarshal(body.Update, &inner); err != nil {
		return "malformed", params
	}
	if inner.SessionUpdate == "" {
		return "undeclared", body.Update
	}
	return inner.SessionUpdate, body.Update
}

func (t *nativeTurn) EventStream() delegate.EventStream { return &spoolStream{spool: &t.spool} }
func (t *nativeTurn) AskStream() delegate.AskStream     { return emptyAskStream{} }

// Answer types its refusal: asks are policy-answered below the seam.
func (t *nativeTurn) Answer(context.Context, delegate.Answer) error { return ErrAskPolicyRefused }

// Cancel cancels the turn context. The settled row is reported
// VERBATIM (no remap — design R2-03); Result adds the
// cancel-requested Detail prefix when the cancel preceded
// settlement.
func (t *nativeTurn) Cancel(context.Context) error {
	t.mu.Lock()
	if !t.settlementStarted && !t.finalized {
		t.cancelRequested = true
	}
	t.mu.Unlock()
	t.cancelTurn()
	return nil
}

// Usage blocks until settle or its context expires. Available means
// present AND not JSON null; Raw is copied at the boundary.
func (t *nativeTurn) Usage(ctx context.Context) (delegate.Usage, error) {
	if err := t.waitSettled(ctx); err != nil {
		return delegate.Usage{}, err
	}
	t.mu.Lock()
	raw := append([]byte(nil), t.outcome.UsageResult...)
	t.mu.Unlock()
	trimmed := bytes.TrimSpace(raw)
	available := len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
	return delegate.Usage{Available: available, Raw: raw}, nil
}

// Result blocks until settle or its context expires; rows verbatim,
// bytes copied, the cancel-requested prefix when Cancel preceded
// settlement.
func (t *nativeTurn) Result(ctx context.Context) (delegate.Result, error) {
	if err := t.waitSettled(ctx); err != nil {
		return delegate.Result{}, err
	}
	t.mu.Lock()
	outcome := t.outcome
	prefixed := t.cancelRequested
	t.mu.Unlock()
	detail := outcome.Detail
	if prefixed {
		detail = "cancel-requested; " + detail
	}
	return delegate.Result{
		Row:        delegate.Row(outcome.Row),
		StopReason: outcome.StopReason,
		SessionID:  outcome.SessionID,
		Candidate:  append([]byte(nil), outcome.Candidate...),
		Violations: outcome.Violations,
		Detail:     detail,
	}, nil
}

// waitSettled makes the accessor select DETERMINISTIC: a settled
// turn always answers, and the accessor context matters only while
// the turn is genuinely unsettled ("a ctx that expires FIRST").
func (t *nativeTurn) waitSettled(ctx context.Context) error {
	select {
	case <-t.done:
		return nil
	default:
	}
	select {
	case <-t.done:
		return nil
	case <-ctx.Done():
		// The race's last word: if settlement became visible while
		// the context was dying, the settled turn still answers —
		// deterministically (CC-R3-001).
		select {
		case <-t.done:
			return nil
		default:
			return ctx.Err()
		}
	}
}

// eventSpool is the bounded, never-blocking event buffer with the
// contiguous-drop overflow contract (design R3-04): once dropping
// starts, every arrival is dropped until the consumer drains to
// empty; the gap event carries the exact count at that transition.
type eventSpool struct {
	mu           sync.Mutex
	events       []delegate.Event
	dropped      uint64
	capacity     int
	byteCapacity int
	bytesHeld    int
	settled      bool
	wake         chan struct{}
}

func (s *eventSpool) append(ev delegate.Event) {
	s.mu.Lock()
	overCount := len(s.events) >= s.capacity
	overBytes := s.byteCapacity > 0 && s.bytesHeld+len(ev.Params) > s.byteCapacity
	if s.dropped > 0 || overCount || overBytes {
		s.dropped++
	} else {
		s.events = append(s.events, ev)
		s.bytesHeld += len(ev.Params)
	}
	s.notifyLocked()
	s.mu.Unlock()
}

func (s *eventSpool) settle() {
	s.mu.Lock()
	s.settled = true
	s.notifyLocked()
	s.mu.Unlock()
}

func (s *eventSpool) notifyLocked() {
	if s.wake != nil {
		close(s.wake)
		s.wake = nil
	}
}

// next yields the head event, a gap event at the drain-to-empty
// transition, or blocks until wakened; (zero, false) on ctx expiry
// or the empty spool of a settled turn (the documented overload).
func (s *eventSpool) next(ctx context.Context) (delegate.Event, bool) {
	for {
		s.mu.Lock()
		if len(s.events) > 0 {
			ev := s.events[0]
			s.events = s.events[1:]
			s.bytesHeld -= len(ev.Params)
			s.mu.Unlock()
			return ev, true
		}
		if s.dropped > 0 {
			count := s.dropped
			s.dropped = 0
			s.mu.Unlock()
			return delegate.Event{Kind: "driver/events-dropped", Params: []byte(fmt.Sprintf("{\"dropped\":%d}", count))}, true
		}
		if s.settled {
			s.mu.Unlock()
			return delegate.Event{}, false
		}
		if s.wake == nil {
			s.wake = make(chan struct{})
		}
		wake := s.wake
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return delegate.Event{}, false
		case <-wake:
		}
	}
}

type spoolStream struct{ spool *eventSpool }

func (st *spoolStream) Next(ctx context.Context) (delegate.Event, bool) { return st.spool.next(ctx) }

type emptyAskStream struct{}

func (emptyAskStream) Next(context.Context) (delegate.Ask, bool) { return delegate.Ask{}, false }

// RegisterNative registers the native driver for one runtime under
// (runtime, "acp"), joining the driver's declaration against the
// runtimes registry's expectation EXACTLY — either direction of
// drift is an init-time panic, the same discipline dialect
// conformance applies. The resolver is the adapter-owned dialect,
// arriving as data (import direction: the composition root imports
// both; this package never imports the adapter).
func RegisterNative(runtime string, resolver func(tools string) (string, error)) *NativeDriver {
	decl, ok := runtimes.Lookup(runtime)
	if !ok {
		panic(fmt.Sprintf("acp: unknown runtime %q", runtime))
	}
	driver, err := newNative(decl, resolver)
	if err != nil {
		panic("acp: " + err.Error())
	}
	delegate.RegisterDriver(delegate.Key{Runtime: runtime, Transport: "acp"}, driver)
	return driver
}

// nativeDeclaration is the surface the native driver earns
// (records/acp/acp-seam-s2-design.md, "The declaration, earned").
var nativeDeclaration = delegate.Declaration{
	Resume:                   true,
	SessionEstablishedSignal: true,
	NativeStructuredOutput:   false,
	NativeEvents:             true,
	NativeUsage:              true,
	GracefulCancel:           true,
	ProtocolServer:           true,
	NativeBudget:             false,
}

// newNative joins the driver's declaration against one runtimes
// entry — the registration core, error-returning so the join's laws
// are table-testable; RegisterNative turns errors into init panics.
func newNative(decl runtimes.Declaration, resolver func(tools string) (string, error)) (*NativeDriver, error) {
	if resolver == nil {
		return nil, fmt.Errorf("RegisterNative needs the adapter-owned mode resolver for runtime %q", decl.Name)
	}
	if decl.ExpectedACP == nil {
		return nil, fmt.Errorf("runtime %q is not expected to speak ACP; a native driver cannot register", decl.Name)
	}
	expected := decl.ExpectedACP.ExpectedCapabilities
	if expected == nil {
		return nil, fmt.Errorf("runtime %q declares no expected native-driver capabilities", decl.Name)
	}
	want := delegate.Declaration{
		Resume:                   expected.Resume,
		SessionEstablishedSignal: expected.SessionEstablishedSignal,
		NativeStructuredOutput:   expected.NativeStructuredOutput,
		NativeEvents:             expected.NativeEvents,
		NativeUsage:              expected.NativeUsage,
		GracefulCancel:           expected.GracefulCancel,
		ProtocolServer:           expected.ProtocolServer,
		NativeBudget:             expected.NativeBudget,
	}
	if nativeDeclaration != want {
		return nil, fmt.Errorf("runtime %q native-driver declaration %+v does not match the registry expectation %+v", decl.Name, nativeDeclaration, want)
	}
	return &NativeDriver{
		ModeResolver:     resolver,
		expectedProtocol: decl.ExpectedACP.ExpectedProtocolVersion,
		declaration:      nativeDeclaration,
	}, nil
}
