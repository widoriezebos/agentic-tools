# The native ACP driver behind the seam (acp-adapter-seam slice two)

Working Mode: design

Owner: m2 session under the acp-adapter-seam claim (slice one landed
358f970; slice two gated on the bm-2dc verdict, concluded
2026-08-24). Critique loop failsafe, declared at open: round 3 — at
round 3 this design lands with residue recorded, never chasing AGREE
past it. Round 1 (design-critic-20260824t114605z-624c): 10 material,
all folded in v2. Round 2 (design-critic-20260824t115618z-6af3): 10
material against v2, all folded below; dispositions at the end.

## The step being implemented

`acp.RunTurn behind delegate.Driver, declared capabilities checked
against internal/runtimes expectations` (the goal's own words). Slice
two registers the first complete Driver: the native protocol
implementation in internal/acp, conformed to the seam's vocabulary.

## Shape

New file `internal/acp/driver.go` (package acp imports delegate and
runtimes only; the adapter-owned dialect arrives as a FUNCTION at
registration — no adapter import, R2-04):

- `type NativeDriver struct { HandshakeTimeout time.Duration;
  ModeResolver func(tools string) (string, error); expected int64 }`
  HandshakeTimeout defaults to the verb's 120s when zero, via ONE
  exported constant `DefaultHandshakeTimeout` that acp_verbs.go's
  flag default also references — shared symbol, cannot drift
  (R1-05).
- `Open(ctx, OpenRequest)` builds `acp.NewConn` over the Endpoint's
  pre-opened pipes; no protocol traffic at Open. Returns a one-shot
  Session (R1-06) holding conn, resume id, and its claim state.
- `Session.PromptTurn(ctx, PromptRequest)` in this exact order:
  1. CLAIM FIRST (R2-08): an atomic compare-and-swap on the
     session's once-flag is the entry gate. The FIRST caller —
     concurrent or sequential — wins the session; every later call
     returns `delegate.ErrSessionExhausted` immediately, even while
     the first turn is still in flight. The claim is consumed by
     entry, not by success: a preflight-refused first call also
     exhausts the session (the adapter owns process custody; a
     refused envelope is a dead process, not a retry surface) — the
     refusal error says so.
  2. `acp.PreflightACP(envelope)` — refuse ineligible envelopes with
     the named reason before any wire traffic (R1-04).
  3. Mode: PromptRequest.Mode is authoritative when non-empty;
     otherwise `ModeResolver(envelope.Tools)`; a resolver error or
     empty resolution refuses, typed. Precedence proof-pinned
     (R1-05).
  4. Start the pump goroutine: RunTurn(turnCtx, conn, cfg) with the
     event tap below; on return, write the cached Outcome, close the
     turn's done channel, end the goroutine (R1-08).

## The event tap (R1-01, R2-01, R2-02, R2-06)

`TurnConfig` gains `OnEvent func(ev TapEvent)` where `TapEvent{
WireSeq uint64, Synthetic bool, Kind string, Params []byte}`:

- Called synchronously by the one pump for (a) every session/update
  notification it services — WireSeq is the notification frame's own
  Seq, the one truthful arrival order conn.go documents (R2-02);
  (b) two synthetic beats, marked Synthetic with WireSeq of the
  frame that triggered them: `session-established` fires the moment
  the pump learns the session id — after session/new's or
  session/load's successful classification, BEFORE set_mode and
  before the prompt is sent — carrying `{"sessionId": ...}` as
  Params (encoding pinned, R2-06); `permission-request-refused`
  fires when an in-window server ask is answered by the strict
  refusal, carrying the ask params verbatim.
- ORDER CONTRACT, stated exactly: the spool yields events in
  tap-call order, which is the pump's service order; events derived
  from inbound frames carry their wire Seq; the JOURNAL remains the
  only order authority across frame classes. No claim of global
  protocol order is made (R2-02).
- OVERFLOW CONTRACT (R2-01, algorithm pinned by R3-04): the spool
  holds at most 4096 events (exported constant). The tap appends
  under a mutex and never blocks. On a full spool the tap enters
  DROP MODE: it increments the drop counter and appends nothing —
  and once dropping has started, EVERY subsequent event is dropped
  (the counter grows) until the consumer drains the spool to empty;
  at that transition the spool yields one `driver/events-dropped`
  event carrying the exact count, then normal appending resumes.
  Drops are therefore contiguous, the gap marker's position is
  unambiguous under concurrent draining, and interleaved
  drop-append races are unrepresentable. Drain-on-close: after the
  turn settles the remaining events stay readable; end-of-stream is
  the empty spool of a settled turn. Loss is impossible to miss and
  the pump is impossible to block.
- Nil OnEvent is the zero value: RunTurn's behavior with a nil tap
  is bit-identical to today, and the SHELL path keeps passing nil.
- PROJECTION into delegate.Event (R3-03): Seq = TapEvent.WireSeq;
  Kind = `update/<sessionUpdate discriminator>` for wire-derived
  events (the nested discriminator, not the outer method name) and
  `driver/<beat>` for synthetic beats (`driver/session-established`,
  `driver/permission-request-refused`, `driver/events-dropped`);
  Params = the update object verbatim for wire events, the pinned
  encodings for beats. The internal `settlement-started` beat is
  consumed by the driver and NEVER projected.

Asks: PreflightACP admits only approvals=deny envelopes; the
envelope's approvals value never crosses the wire — only the
dialect-resolved mode does. An in-window session/request_permission
is answered by the shipped strict-refusal backstop (turn.go's
StrictAnswer), journaled, and NOT counted a violation — the v2 text
claiming "counted" was wrong and is withdrawn (R2-05). The refused
ask is visible on the event stream as the synthetic beat; AskStream
yields nothing by contract truth; `Answer()` returns a typed
refusal naming the policy. The day an approvals grade permits
asking, AskStream grows a tap — slice three at the earliest.

EventStream.Next's false is overloaded by the SEAM's own signature
(no error return, R3-05): this driver returns (zero, false) both on
accessor-ctx expiry and at end-of-stream, documented on the method;
a consumer distinguishes by checking whether the turn has settled.
The signature wart is seam-level, recorded as residue for the
contract's next amendment window.

## The declaration, earned

resume TRUE (session/load per negotiated loadSession; unsupported
resume is RunTurn's existing typed refusal), sessionEstablishedSignal
TRUE (the synthetic beat, observable through the seam BEFORE the
prompt settles — proof 4 pins the ordering mechanically: the
scripted server delays the prompt response until the harness has
observed session-established on the stream, R2-06), 
nativeStructuredOutput FALSE, nativeEvents TRUE (the tap),
nativeUsage TRUE, gracefulCancel TRUE (scoped below), protocolServer
TRUE, nativeBudget FALSE.

Typed-refusal identities (R3-06), all exported from internal/acp:
`ErrPreflightRefused` (wraps PreflightACP's reason string),
`ErrModeUnresolved` (resolver error or empty resolution),
`ErrTimeoutUnset` (R3-07 below), `ErrAskPolicyRefused` (Answer's
refusal); plus `delegate.ErrSessionExhausted` in the seam. Each is
errors.Is-able; the proofs assert identities, not message strings.

Zero-duration law (R3-07): the driver owns a default ONLY for
HandshakeTimeout (the seam's PromptRequest has no such field). For
the fields the seam DOES expose — PromptTimeout, LateWindow,
CancelGrace — zero is refused with ErrTimeoutUnset before any wire
traffic: explicit beats implicit, and an immediate-deadline accident
is unrepresentable.

## Cancel, honestly (R1-07, R2-03)

The v2 setup-phase remap is WITHDRAWN: a wrapper flag cannot
distinguish cancellation-caused setup failure from a genuine
protocol error racing the Cancel, and a lying RowCancelled is worse
than an ugly RowProtocolError (R2-03). Shipped semantics instead:

- `Cancel(ctx)` cancels the turn context and records
  cancelRequested.
- Result returns the pump's row VERBATIM, always. When
  cancelRequested preceded settle, the driver prefixes Detail with
  `cancel-requested; ` — both facts visible, no row rewritten.
- The three lifecycle arms and their true rows: prompt-in-flight →
  RowCancelled (the pump's own cancelAndSettle); setup-phase →
  whatever the pump classifies (typically RowProtocolError), with
  the prefix; post-settlement-start → Cancel returns nil, the turn
  settles as it was going to (drain is bounded by LateFrameWindow).
- HOW the wrapper knows settlement started (R3-02): the pump emits
  an INTERNAL synthetic beat `settlement-started` through the tap
  the moment it moves from the prompt response into settle; the
  driver consumes it (sets an atomic, never spools it) and gates the
  Detail prefix on cancel-before-that-beat. One beat, one atomic,
  no guessing.
- gracefulCancel TRUE means exactly: cancel during the prompt yields
  the native session/cancel courtesy and the typed RowCancelled.
  The declaration table's meaning is this paragraph.

## Result/Usage synchronization (R1-08, R2-10)

The turn holds `done chan struct{}` + the cached Outcome. Result and
Usage select on done and their own ctx; ctx expiry returns ctx.Err()
without consuming anything; both are repeatable and concurrent-safe.
ACCESSOR COPIES (R2-10): Result.Candidate and Usage.Raw are COPIED
at every accessor return — the cache's slices are never aliased to
callers, so no caller can mutate what another observes. Usage:
Available = present AND not JSON null (whitespace-trimmed byte
comparison); Raw verbatim (copied) either way (R1-10).

## Endpoint and journal lifetime (R1-09)

The ADAPTER (or test harness) owns process, pipes, and journal:
open order, close, Conn.Done wait, JournalErr sampling, sync, close
— acp_verbs.go's exact sequence. The Driver only speaks. Result
returning means the outcome is settled, not that the wire is closed;
evidence finality is the owner's quiesce. Endpoint keeps io.Reader/
io.Writer; no Close migrates into the seam in slice two.

## The one-shot Session and the seam's language (R2-07)

delegate.go gains, in slice two, the contract-side note and the
sentinel: `ErrSessionExhausted` (exported) and one sentence on
Session: "A Session MAY be one turn wide; PromptTurn reports
exhaustion with ErrSessionExhausted, and reopening is the caller's
move." The seam admits one-shot sessions EXPLICITLY, so the native
driver's cardinality is contract, not surprise. (Slice one's
"one open delegate session" language stays; the note qualifies
cardinality of TURNS, not of sessions.)

## The expectations check (R1-02)

`internal/runtimes.ACPExpectation` grows `ExpectedCapabilities` —
the eight booleans as a plain struct in runtimes (pure data, field
names mirror delegate.Declaration one-for-one; a parity test in the
delegate package pins the mirror both ways — the duplication is the
POINT: registry expectation vs driver claim, joined at registration,
same discipline as dialect conformance; drift in either direction is
an init-time panic, R2 pressure question answered). devin's entry
declares the table above.

TWO SURFACES, ONE CONVERGENCE OBLIGATION (R3-01): the adapter
probe's capability snapshot (devin.sh) describes what the SHELL
adapter delivers to dispatch today — over the acp transport it
surfaces no events and no session signal to its consumer, so its
booleans are honestly false. The driver's Declaration describes what
the NATIVE DRIVER delivers to seam consumers — the tap makes events
and the session beat real THERE. Both are true of their own surface;
the registry's ExpectedCapabilities pins the DRIVER surface (the
seam's registry, the seam's truth). The two truths converge the day
the shell path routes through the driver, at which point the
snapshot's booleans flip and the probe reads them from the driver —
that convergence is recorded residue on the goal, owed to slice
three or the adapter-routing slice, whichever lands first.

`RegisterNative(runtime string, resolver func(string) (string,
error))` (R2-04): panics without ExpectedACP, with a nil resolver,
on declaration/expectation mismatch (exact equality), or on a
duplicate key. Protocol version comes from the registry, never
duplicated. Key: `delegate.Key{runtime, "acp"}`. Wiring in
cmd/metasystem: `acp.RegisterNative("devin",
devinACPModeResolver)` where the resolver closes over
`adapter.ACPDialectFor("devin").ModeForTools` — the dialect stays
adapter-owned and arrives as data; import direction is cmd → both,
never acp → adapter.

## Proof obligations

1. Scripted-server differential: same scripted bytes through direct
   RunTurn (nil tap) and through the Driver; Outcome and mapped
   Result agree field-for-field; after the harness quiesce, journal
   bytes identical.
2. Shell-path pin (R2-09): the EXISTING acp turn fixtures (the verb
   end to end: stdout, stderr, exit code, SessionFile, flag
   defaults) run unmodified and green at the slice's landing — the
   verb passes a nil tap and the fixtures pin its bytes; plus a unit
   test that a zero TurnConfig tap field changes nothing on a
   scripted delivered turn (journal byte-compare).
3. Row vocabulary mapping over every row; Detail prefix behavior
   under cancelRequested (R2-03's shape: no remap to verify, a
   prefix to verify).
4. Stream truth: session-established observed on the stream BEFORE
   the scripted server releases the prompt response (server gated on
   harness observation, R2-06); update events in tap-call order
   carrying their wire Seq; the refused-ask beat under a scripted
   in-window ask, violations total UNCHANGED by the beat (R2-05);
   EventStream drains after settle then ends; AskStream yields
   nothing; Answer returns the typed refusal.
5. Overflow law (R2-01): a scripted flood past the spool cap yields
   the events-dropped gap event with the exact count, in order, and
   the pump is provably unblocked (the flood completes without
   consumer reads).
6. Envelope law: approvals=ask through PromptTurn refused with
   PreflightACP's reason before any wire traffic; the session is
   exhausted by the attempt (R2-08's consumed-by-entry rule).
7. Mapping law: DefaultHandshakeTimeout shared symbol; Mode
   precedence (set Mode wins, resolver otherwise, resolver error
   refuses).
8. One-shot law (R2-08): concurrent double PromptTurn — exactly one
   wins, the loser gets ErrSessionExhausted immediately; sequential
   second call ditto; preflight-refused first call exhausts.
9. Synchronization (R1-08, R2-10): concurrent Result+Usage; repeated
   Result; accessor-ctx expiry; settle-without-accessor then late
   read; mutation of a returned Candidate/Raw does not affect a
   second read.
10. Cancel arms (R1-07, R2-03): all three lifecycle arms; the
    prompt-in-flight arm asserts RowCancelled; the setup arm asserts
    the verbatim row plus the cancel-requested prefix.
11. Usage boundary (R1-10): absent / present-null / present-object →
    Available false/false/true, Raw verbatim.
12. Registry law (R1-02, R2-04): no ExpectedACP → panic; nil
    resolver → panic; each declaration boolean flipped once →
    panic, table-driven; duplicate → panic; DriverFor finds
    devin/acp; runtimes/delegate mirror parity both ways.

## Non-goals

- No adapter-script change: devin.sh keeps custody and the `acp
  turn` verb; the verb keeps a nil tap.
- No interactive ask surface (slice three at the earliest).
- No multi-turn Session (one-shot, contract-noted).
- No host-turn transport work (host-turn-transport-scope owns it).
- No new capability booleans in the seam's Declaration struct.

## Round-1 disposition (design-critic-20260824t114605z-624c)

R1-01 folded (tap; honest ask story) · R1-02 folded
(ExpectedCapabilities + two-way join) · R1-03 folded (synthetic beat
through the seam) · R1-04 folded (PreflightACP in PromptTurn;
StrictAnswer named) · R1-05 folded (shared default constant; Mode
precedence) · R1-06 folded (one-shot) · R1-07 folded (three-arm
scope; v2's remap later withdrawn by R2-03) · R1-08 folded (done
channel + cache) · R1-09 folded (owner law; harness quiesce) ·
R1-10 folded (null-aware Available).

## Round-2 disposition (design-critic-20260824t115618z-6af3)

- R2-01 folded: overflow contract — 4096 cap, never-blocking tap,
  in-order events-dropped gap event, drain-on-close rule.
- R2-02 folded: order contract restated — wire Seq on frame-derived
  events, tap-call order for the spool, journal the only cross-class
  authority; no global-order claim survives.
- R2-03 folded: remap withdrawn; verbatim rows + cancel-requested
  Detail prefix.
- R2-04 folded: RegisterNative takes the resolver; wiring closes
  over adapter.ACPDialectFor; import direction stated.
- R2-05 folded: "counted as violation" withdrawn; beat proven not to
  change the violations total.
- R2-06 folded: TapEvent encoding pinned; firing point pinned
  (after session classification, before set_mode/prompt); proof
  gates the server on stream observation.
- R2-07 folded: ErrSessionExhausted + contract-side sentence land in
  delegate.go with the slice.
- R2-08 folded: CAS claim-first entry gate; consumed-by-entry rule
  including preflight refusals; concurrent proof.
- R2-09 folded: shell-path proof via the existing verb fixtures plus
  the nil-tap byte-compare unit.
- R2-10 folded: accessor copies, with the mutation proof.

## Round-3 disposition (design-critic-20260824t120526z-a31d, failsafe round)

- R3-01 (critical) folded in text + RESIDUE: two-surface reconciliation
  stated; convergence obligation recorded on the goal (snapshot flips
  when the shell routes through the driver).
- R3-02 folded: internal settlement-started beat; atomic gate for the
  Detail prefix.
- R3-03 folded: projection law (Seq, Kind namespaces, Params,
  never-projected internal beat).
- R3-04 folded: contiguous-drop overflow algorithm — drop mode until
  empty, gap event at the empty transition.
- R3-05 folded as documented behavior + RESIDUE: the seam signature's
  overloaded false is a contract wart for the next amendment window.
- R3-06 folded: named error identities, errors.Is-pinned proofs.
- R3-07 folded: zero-duration refusal ErrTimeoutUnset for seam-exposed
  timeouts; driver default only where the seam has no field.

LANDED AT THE FAILSAFE: rounds ran 10 → 10 → 7 material findings;
per the declared loop covenant this design lands here with the two
residues above recorded, and implementation begins.
