# ACP as the delegate transport (backlog item 18)

- Status: DRAFT r3 — critiques r1 and r2 folded (plans/acp-critique-r1.md:
  15 findings; plans/acp-critique-r2.md: 8 findings, 7 structural;
  all folded, none refuted)
- Goal: acp-transport
- Next step: Fold the critique verdict when run acp-critique-r3 concludes; implement only after convergence.
- In flight right now: run acp-critique-r3 (codex xhigh critique; watch it with: bin/metasystem run watch --id acp-critique-r3 --root .)

The human's question that raised this (2026-08-15, on the Devin
delivery failures): "Is there no way to use ACP to make this more
robust?" There is: the installed CLI ships `devin acp` (an Agent
Client Protocol server over stdio, re-verified at devin 3000.4.25,
wire capture in plans/acp-wire-probe.md). The ratified direction
(backlog-notes item on streaming, 2026-08-09) already fixes the
shape: ACP is a transport WITHIN the per-turn lifecycle, not a
persistent server — process exit stays the turn boundary, because
the lease, census, and reaper are built on it.

## Schema pin (r2 F4)

Wire protocolVersion 1 does not freeze the message shapes: ACP's
schema artifact evolves independently of the wire version. The
implementation and every fixture pin ONE schema artifact release
(recorded by digest in the repo when P2 starts); adopting a newer
artifact is a deliberate change with a fresh conformance pass, not
a dependency bump.

## Scope (the honest increment)

- First supported increment: the DEVIN DELEGATE transport only.
  Cross-runtime neutrality is a property of the protocol, not of
  this design — every additional runtime requires its own wire,
  lifecycle, and conformance evidence before its declaration turns
  on (r1 F14).
- Devin MISSION HOSTS are out of scope: "Devin delegate transport =
  ACP" does not change "Devin host delivery = legacy collector."
  The hosts keep `devin -p` + `host devin-collect`, and the B1
  host-recollection capability survives untouched until a
  separately scoped host-ACP design proves its turn-contract and
  retry behavior (r1 F10).
- Chain persistence (a server crossing job records) is out of
  scope; it would require a custody-transfer design that does not
  exist (r1 F12).

## What ACP buys, restated without over-claim

1. **Permission requests become answerable.** Today a Devin
   dispatch runs full auto-approve because prompts dead-stop the
   CLI (D61's ruled waiver). Under ACP, `session/request_permission`
   arrives as a JSON-RPC call the adapter answers from the job's
   permission envelope. This is ADMISSION CONTROL — our side decides
   what the agent may attempt — not containment: nothing here is a
   filesystem or network sandbox, and an allowed shell command can
   still try to escape its roots (r1 F1).
2. **Delivery gains a typed completion signal.** ACP ends a prompt
   turn with a `PromptResponse` carrying a stop reason; response
   content arrives as message-chunk notifications along the way.
   That is completion EVIDENCE, not a typed return object (r1 F6) —
   the assembled bytes still enter D62 qualification.
3. **Progress becomes visible while it happens.** Streamed updates
   give supervision a per-turn last-event-age heartbeat. These
   events are ADVISORY accelerators under the accelerator ruling
   (docs/architecture.md): records stay the authoritative state,
   and every recovery path must remain correct with missing,
   duplicated, reordered, or truncated notifications (r1 F15).

## Transport identity and selection (r1 F2, r2 F3)

Transport is part of capability and enforcement identity, pinned
BEFORE launch and verified AFTER:

- Pre-launch, selection pins the REQUESTED transport (legacy | acp)
  and the EXPECTED protocol version, both from trusted
  configuration — not from negotiation, which cannot happen until
  the server is up. Initialization then records the ACTUAL
  negotiated version into the job's provenance and verifies it
  against the expectation; a mismatch fails preflight before any
  session is established.
- The capability snapshot becomes SINGLE-TRANSPORT: one snapshot
  per (runtime, CLI version, configuration hash, transport), with
  transport in the filename identity and in the garbage-collection
  retention key — today's GC retains one snapshot per
  (runtime, CLI version, configuration hash) and would otherwise
  let legacy and ACP snapshots supersede each other. The
  enforcement map is per-transport by construction because each
  snapshot carries exactly one transport. Existing snapshots (the
  plural-`transports` shape) read as legacy-transport snapshots
  under a declared migration rule; they never satisfy an ACP
  selection.
- Every producer and consumer moves together: the shared snapshot
  writer (runtime-common's emit path), the fake and contract
  producers, the selector, job provenance, and evidence GC. This is
  in the blast radius, not a footnote.
- Each job pins exactly one selected transport and snapshot before
  launch. Changing transport means fresh selection — never a
  mid-job switch, and evidence never crosses transports: legacy
  `dangerous`-mode behavioral evidence cannot inherit ACP
  enforcement claims.
- Devin's `ExpectedEnvelopeEnforcement` stays `notEnforced` on
  every field until the real ACP launch path BEHAVIORALLY proves
  both the allowed and the forbidden effect per field — including
  an allowed shell attempting to escape its roots (r1 F1). Only
  then does a field flip to `mapped`, and only in the ACP-transport
  snapshot.

## The client: one process tree per prompt attempt (r1 F12, F13; r2 F1, F6)

`internal/acp` is a protocol client whose lifetime is exactly one
PROMPT ATTEMPT: spawn `devin acp`, initialize, establish or load a
session, run one prompt, tear down, exit. A job has at most two
attempts — the initial prompt and D62's one repair — and each
attempt is its own separately registered process tree. The repair
attempt starts a fresh client/server, loads the session via
`session/load` (only if P1 proves it), and issues the repair
prompt under the SAME pinned transport. If load is unproven or
fails, ACP jobs run WITHOUT repair — adjudication proceeds and the
job fails qualification honestly. An ACP-selected job never
invokes the legacy repair command (`devin -p -r` stays
legacy-only); this resolves the contradiction r2 F6 found between
the one-prompt lifetime and the shipped same-session repair. No
process crosses job records.

Custody is specified before any protocol I/O, and it changes code
r2 showed does not currently do what r2's predecessor claimed:

- Process tree: adapter shell → Go client → `devin acp` server.
  BOTH the client and the server's exact identities (pid + start
  time) are registered as custody processes before the first
  protocol byte; the server pid is known at spawn, so registration
  precedes initialize.
- The reaper today writes `groupDeathProvenAt` after proving only
  the job record's top-level adapter pid dead, ignoring
  `custodyProcesses`. That changes: group death requires EVERY
  registered custody identity proven dead — one dead pid never
  establishes group death. This is a reaper change with its own
  fixtures, listed in the blast radius alongside the census and
  dispatch custody code.
- The kill-capable owner (today's cancellation sweep) verifies
  every registered identity AND the process group dead before
  terminal compare-and-swap, for both attempts' trees.
- Escaped descendants: a daemonized server that left the process
  group is still a registered exact identity, so the sweep finds
  it by identity; a registered identity that survives TERM/KILL
  escalation leaves the record NON-TERMINAL with a loud custody
  failure — never a quiet green. Descendants the server spawned
  outside the group and outside registration are acknowledged as
  the same residual risk the legacy path carries today (the
  instance-tag census sweep remains the backstop); the design does
  not claim to close that hole, only not to widen it.
- `session/cancel` is a bounded courtesy (deadline, then the
  existing TERM/KILL group sweep). Kill authority never depends on
  protocol cooperation.
- The lifecycle loop keeps heartbeats flowing and enforces
  handshake and capability deadlines while ACP I/O is blocked in
  EITHER direction: a wedged read and a blocked WRITE (server not
  draining our frames during initialize or prompt) are distinct
  fixtures, both bounded by deadlines, both ending in teardown
  with custody proof (r2 F8).
- P2 custody fixtures: survivor after server exit, daemonizing
  server, blocked half-frame read, blocked-writer backpressure
  before and after possible prompt execution, TERM-then-KILL
  escalation, client death with server alive, repair-attempt
  second tree.

## Client capabilities: advertise nothing (r1 F3)

ACP lets clients advertise filesystem and terminal methods the
server can call back into — a second side-effect path that would
bypass the envelope entirely. The production client advertises NO
client filesystem and NO terminal capabilities (the wire probe
already ran this way). Any unsolicited server→client effect call
fails closed and is recorded as a protocol violation in the round
events. Enabling either capability later is a separate containment
and custody design, not a flag.

## The permission decision (r1 F4, F5; r2 F2)

The wire shape rules the design: an ACP permission request carries
a tool-call update plus OFFERED OPTIONS, and the response must
select an offered option ID or return cancelled — there is no
abstract allow/deny on the wire. Paths, locations, kind, and raw
input can be absent or agent-specific, and `ToolKind` is a coarse
category, not an authoritative effect description. Therefore:

- **Until P1 captures a versioned, machine-readable Devin request
  dialect, every permission request is refused**: Decide selects
  the most restrictive offered option that denies the action, or
  returns cancelled when no denying option exists. That makes the
  first ACP increment safe-but-strict; it cannot make it
  permissive, because permissiveness requires facts the wire has
  not yet shown us.
- After the dialect is captured, `Decide(envelope, request)` gets
  the real table: a pure function over ALL FIVE envelope fields —
  writeRoots, readRoots, network, tools, approvals — specified
  over: every observed tool kind; missing, multiple,
  canonicalized, and symlinked paths; mixed effects in one
  request; network targets; every ordinal value of `tools` and
  `approvals`; and DETERMINISTIC selection among offered options.
  Authority is never inferred from human-readable titles.
  `allow_always` (persistent grants) is never selected. If no
  offered option matches the table's verdict, cancel.
- The envelope is immutable for the life of the job; no answer may
  widen it. v1 ships WITHOUT an escalation lifecycle: a request
  the table cannot classify is denied (both standard presets
  already deny approvals, so no same-job approval surface is
  lost). A durable escalation path is future work with its own
  design.
- Fixtures must prove the `tools` grades bite: `read-only` denies
  state-changing tool categories even inside an allowed root, and
  `runtime-default` never overrides roots or network.
- Unknown dialects and unclassifiable requests fail closed (deny
  or cancel + record), never fall through to allow.

## Delivery: watermarked assembly feeding D62 (r1 F6, F9; r2 F5)

`session/load` REPLAYS conversation history through notifications.
Without a boundary, a previously delivered answer could be
assembled into the new attempt's candidate. So:

- The client establishes a WATERMARK after load completes and
  before the prompt is sent. Candidate assembly consumes only
  `agent_message_chunk` updates for the matching session ID inside
  the current prompt window (after the watermark, before the
  matched PromptResponse).
- Assembly is deterministic and specified: arrival order within
  the window; duplicate frames dropped by content-and-position;
  chunks grouped by message ID when present, treated as one
  message when absent; multiple assistant messages in one window
  → the final complete message is the candidate, earlier ones stay
  journaled evidence; non-text content blocks are journaled but
  never enter the candidate bytes; truncated streams and
  size-ceiling breaches disqualify the candidate (evidence, not
  delivery).
- Honesty about ownership (r2 F5 caught r2 contradicting itself):
  the qualification owner IS touched, additively. The devin
  collector gains one new candidate channel — `acp` — carrying the
  assembled bytes with protocol provenance (session ID, prompt
  window, stop reason, schema pin). Its existing channels (stdout,
  named file, transcript, none) are unchanged, and everything
  downstream of candidate selection — qualification,
  normalization, validation, immutable snapshot, adjudication,
  one-repair — is reused as-is.
- A matched successful PromptResponse is COMPLETION evidence.
  Partial streams remain evidence, never delivery. D62's ladder is
  not retired by this design: it remains the qualification
  machinery for all transports and the delivery path for legacy
  ones. Any future simplification of the legacy scraping legs
  waits until ACP delivery has independently earned trust (the D62
  gate is independent of the D61 gate — three dangerous-mode
  rounds once produced no result, so delivery and permissions
  demonstrably fail independently).
- P1 must capture a load-replay-followed-by-prompt session; P2
  must prove a stale prior JSON answer cannot win candidate
  selection.

## Usage: unresolved until P1, then exactly one source (r1 F11; r2 F7)

r2 corrected two stale claims at once: current ACP v1 DOES define a
`usage_update` notification (context use, optional cumulative
session cost), and ATIF `final_metrics` is not free under ACP —
today it exists because the legacy command passes an explicit
export flag. So the honest position:

- The ACP transport's usage source is UNRESOLVED until P1 answers:
  does devin emit `usage_update` (and with what counter semantics),
  and can ACP mode produce a bounded ATIF export at all — across
  initial, resumed (session/load), and repair prompts?
- After P1: select EXACTLY ONE complete authoritative live source
  for the ACP transport, with specified cumulative-vs-delta
  semantics, predecessor identity, resumed-turn delta, and repair
  replacement rule (the repair attempt's usage replaces or extends
  the initial attempt's per the existing owner's rules). If no
  single complete source exists, the ACP transport publishes
  usage UNAVAILABLE — loudly, never a fabricated number. Wire and
  transcript figures are ALTERNATIVES, never summed.
- Wire `usage_update` frames are journaled evidence; if selected
  as the source, the usage owner consumes the JOURNAL (a record),
  keeping the accelerator ruling intact.
- Dead-round recovery is a distinct operation and stays declared
  unsupported for devin unless complete wire evidence proves an
  honest delta can be reconstructed.
- internal/usage and its adapter integration stay in the blast
  radius under every P1 outcome.

## Failure outcomes (r1 F7; r2 F4)

A phase-by-phase matrix, against the pinned schema artifact:

| Phase / event | Outcome |
|---|---|
| initialize: negotiated version ≠ expected | refuse before session; preflight failure; no transport switch |
| initialize/auth/new/load: JSON-RPC error (standard or arbitrary code) | phase-named failure with the error as evidence; job fails preflight (no prompt was sent) |
| authMethods non-empty and session methods reject unauthenticated | distinguishable `auth-required` failure surfaced to the operator; never interactive auth inside a job (wire probe: devin advertises `devin-browser`) |
| unknown REQUIRED capability/variant | refuse the session |
| malformed frame, oversized frame, mismatched response ID | protocol error; record; teardown |
| prompt: JSON-RPC error | turn fails with the error as evidence |
| stop reason end_turn | assemble window + qualify (delivery candidate) |
| stop reason cancelled | cancelled turn; chunks are evidence |
| stop reason refusal | refused turn with evidence |
| stop reason max_tokens / max_turn_requests | INCOMPLETE turn; chunks retained as evidence only; never enters delivery qualification |
| unknown stop reason | protocol error; never silent success |
| update kinds: agent_message_chunk | journal + assemble (within the watermarked window) |
| update kinds: tool_call / tool_call_update / plan / mode / config / session-info | journal; advisory heartbeat only |
| update kind: usage_update | journal; usage source pending the P1 ruling |
| unknown / extension notifications | journal and ignore; never fail the turn |
| unsolicited server→client REQUEST (fs/terminal/other) | fail closed; protocol violation recorded |
| cancellation race (cancel sent, PromptResponse arrives) | PromptResponse wins if complete and prior to teardown; otherwise cancelled turn; both journaled |
| EOF before PromptResponse | turn fails; journaled chunks are evidence, not delivery |
| teardown timeout | TERM/KILL group sweep; full custody proof before terminal CAS |

Once a prompt MAY have executed, server death never causes
automatic restart or replay — side effects and spend are uncertain,
so the turn fails and a human-visible record says why. An ACP
failure never switches the same job to the legacy transport or to
`dangerous` mode (r1 F8): rollback is a pre-launch selection, made
for the NEXT job, never a mid-job fallback (r1 F9).

## D61 and D62 retirement, stated honestly (r1 F8, F9)

- D61 (dangerous-mode waiver): ACP-selected jobs never invoke it
  after acceptance. The legacy path keeps the waiver as long as it
  remains callable; D61 retires only when the legacy devin
  delegate path is REMOVED, which is a later decision on ACP's
  record.
- D62 (delivery ladder): retirement is not coupled to permission
  proof; the qualification owners survive with one additive
  channel (see Delivery).
- Session-bridge honesty: P1 must separately prove ACP→ACP
  `session/load`, legacy→ACP load, and ACP→legacy resume; a
  direction that fails is CLOSED as a fallback, and repair
  planning must know which directions exist before ACP jobs enter
  chains.

## Registry: data only (r1 F14)

`internal/runtimes` gains only the EXPECTED shape: a declaration
that a runtime is expected to support ACP transport, with the
expected protocol version. Behavior lives where behavior lives:
launch argv and protocol translation in the adapter-owned tables,
usage decoding in internal/usage, and the conformance test joins
declarations to registrations both ways, exactly like the B1
capability tables. A runtime without the declaration keeps its
current path unchanged; adoption is per-runtime and reversible.

## Events are advisory (r1 F15)

ACP notifications feed progress display and wakeups (the per-turn
last-event-age heartbeat the streaming mandate asked for). They
COMMIT nothing: job, run, usage, delivery, and turn outcomes are
committed by their existing owners to records, and recovery reads
records. The bounded raw wire journal (every JSON-RPC exchange,
size-capped) is kept separate from the typed flight-recorder
catalog; the journal is evidence, the catalog is contract.

## Prototype plan (design gate before build stands)

P1 (extend the existing probe; throwaway, never shipped) must now
answer, beyond the captured initialize
(plans/acp-wire-probe.md):

1. Does an unauthenticated `session/new` work or fail, and how
   does `devin-browser` auth manifest on the wire?
2. `session/load` in all three bridge directions (ACP→ACP,
   legacy→ACP, ACP→legacy resume), including a capture of load
   REPLAY followed by a fresh prompt (the watermark evidence).
3. A real prompt turn's notification stream and PromptResponse:
   observed stop reasons, update kinds, message-ID usage.
4. Permission dialect: provoke real `session/request_permission`
   requests (e.g. a write outside the workspace, a shell command,
   a network fetch) and capture the offered options verbatim —
   the machine-readable dialect the Decide table needs.
5. Usage: does devin emit `usage_update` (semantics?), and can ACP
   mode produce a bounded ATIF export — across initial, loaded,
   and repair-shaped prompts?
6. What does `session/cancel` actually do to the server process?

The probe advertises no client capabilities, and its captures land
beside the first one in plans/acp-wire-probe.md.

P2: `internal/acp` with the fake runtime speaking a stub ACP
server — fixtures drive every row of the outcome matrix, the
Decide table (strict-refusal mode AND the post-dialect table over
all five envelope fields), watermarked assembly including the
stale-prior-JSON attack, the custody fixtures (survivor,
daemonizer, blocked read, blocked write before/after prompt,
TERM/KILL, client death, repair second tree), the reaper
all-identities change, snapshot-per-transport selection and GC,
and usage counter semantics.

P3: devin's declaration + adapter integration behind a conf flag,
selection pinning the transport into the snapshot; bm-style live
smoke. Acceptance for any `mapped` flip: behavioral proof of
allowed AND forbidden effects per field on the ACP path (r1 F1).

## Blast radius

internal/acp (NEW), internal/runtimes (expected-ACP declaration),
internal/adapter (devin ACP launch table, Decide wiring,
devincollect additive `acp` channel with protocol provenance,
snapshot single-transport schema), internal/capability (selection
on transport + expected version), internal/evidence (GC retention
key gains transport), internal/dispatch (custody registration of
client + server identities), internal/census (custody identity
recognition), internal/supervise (reaper: group death requires
every registered identity), internal/usage (per the P1 ruling),
scripts/agents/dispatch.sh (custody + cancellation sweep for two
trees), scripts/agents/adapters/runtime-common.sh (registration
seam, repair seam guard: ACP jobs never call the legacy repair),
scripts/agents/adapters/devin.sh (dispatch path behind the flag),
dispatch/job records (protocol provenance fields), fixtures (stub
server suite), docs (orchestration transport section). NOT
touched: scripts/agents/hosts/devin.sh and internal/host
recollection; the D62 owners downstream of candidate selection are
reused unchanged (the collector itself gains the one additive
channel noted above).

## Loop discipline

Critique rounds with codex at xhigh; two-budget allowance; stop on
zero unrefuted material findings or the ratified exits. r1 found
15 (7 critical-structural); r2 found 8 (7 structural) — including
r2's catch that r1's custody fold was itself wrong about what the
reaper proves. All folded above, none refuted. The r3 critique
should attack: whether the reaper/custody change is specified
tightly enough to implement without a second design pass; whether
the strict-refusal permission mode is livable (does a devin that
gets every permission denied still complete useful work, or does
this increment reduce to "ACP dispatch always fails closed" until
the dialect lands — and is that acceptable as increment one);
whether snapshot-per-transport migration covers every existing
producer/consumer (fake, contract, GC, selector, provenance);
whether the two-attempt custody story (repair as second tree)
holds usage and terminal-timing together; and whether the P1
question list is now COMPLETE — anything the implementation needs
that no probe question captures is a standing defect.
