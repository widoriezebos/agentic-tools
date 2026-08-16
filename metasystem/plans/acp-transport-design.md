# ACP as the delegate transport (backlog item 18)

- Status: DRAFT r2 — critique r1 folded (plans/acp-critique-r1.md: 15
  findings, 7 critical-structural; all folded, none refuted)
- Goal: acp-transport
- Next step: Fold the critique verdict when run acp-critique-r2 concludes; implement only after convergence.
- In flight right now: run acp-critique-r2 (codex xhigh critique; watch it with: bin/metasystem run watch --id acp-critique-r2 --root .)

The human's question that raised this (2026-08-15, on the Devin
delivery failures): "Is there no way to use ACP to make this more
robust?" There is: the installed CLI ships `devin acp` (an Agent
Client Protocol server over stdio, re-verified at devin 3000.4.25,
wire capture in plans/acp-wire-probe.md). The ratified direction
(backlog-notes item on streaming, 2026-08-09) already fixes the
shape: ACP is a transport WITHIN the per-turn lifecycle, not a
persistent server — one process per turn stays, because the lease,
census, and reaper are built on process exit as the turn boundary.

## Scope (r1 claimed too much; this is the honest increment)

- First supported increment: the DEVIN DELEGATE transport only.
  Cross-runtime neutrality is a property of the protocol, not of
  this design — every additional runtime requires its own wire,
  lifecycle, and conformance evidence before its declaration turns
  on (critique r1 F14).
- Devin MISSION HOSTS are out of scope: "Devin delegate transport =
  ACP" does not change "Devin host delivery = legacy collector."
  The hosts keep `devin -p` + `host devin-collect`, and the B1
  host-recollection capability survives untouched until a
  separately scoped host-ACP design proves its turn-contract and
  retry behavior (F10).
- Chain persistence (a server crossing job records) is out of
  scope; it would require a custody-transfer design that does not
  exist (F12).

## What ACP buys, restated without over-claim

1. **Permission requests become answerable.** Today a Devin
   dispatch runs full auto-approve because prompts dead-stop the
   CLI (D61's ruled waiver). Under ACP, `session/request_permission`
   arrives as a JSON-RPC call the adapter answers from the job's
   permission envelope. This is ADMISSION CONTROL — our side decides
   what the agent may attempt — not containment: nothing here is a
   filesystem or network sandbox, and an allowed shell command can
   still try to escape its roots (F1).
2. **Delivery gains a typed completion signal.** ACP ends a prompt
   turn with a `PromptResponse` carrying a stop reason; response
   content arrives as message-chunk notifications along the way.
   That is completion EVIDENCE, not a typed return object (F6) —
   the assembled bytes still enter the existing D62 qualification.
3. **Progress becomes visible while it happens.** Streamed
   tool-call updates give supervision a per-turn last-event-age
   heartbeat. These events are ADVISORY accelerators under the
   accelerator ruling (docs/architecture.md): records stay the
   authoritative state, and every recovery path must remain correct
   with missing, duplicated, reordered, or truncated notifications
   (F15).

## Transport identity and selection (F2)

Transport is part of capability and enforcement identity. The
capability snapshot records `transport` (legacy | acp) and, for
ACP, the negotiated protocol version; selection matches on them the
same way it matches runtime, CLI version, and configuration hash.
Consequences:

- Each job pins exactly one selected transport and its snapshot
  BEFORE launch. Changing transport means fresh selection and a
  fresh handshake — never a mid-job switch.
- Evidence never crosses transports: legacy `dangerous`-mode
  behavioral evidence cannot inherit ACP enforcement claims, and an
  ACP probe proves nothing about the legacy path.
- Devin's `ExpectedEnvelopeEnforcement` stays `notEnforced` on
  every field until the real ACP launch path BEHAVIORALLY proves
  both the allowed and the forbidden effect per field — including
  an allowed shell attempting to escape its roots (F1). Only then
  does a field flip to `mapped`, and only in the ACP-transport
  snapshot.

## The client: one turn, one process tree (F12, F13)

`internal/acp` is a protocol client whose lifetime is exactly one
job turn: spawn `devin acp`, initialize, establish or load a
session, run ONE prompt turn, tear down, exit before the job record
becomes terminal. Follow-ups launch fresh custody and use
`session/load` only after P1 proves it (see the probe plan). No
process crosses job records.

Custody is specified before any protocol I/O:

- Process tree: adapter shell → Go client → `devin acp` server. The
  server is a GRANDCHILD, and today's custody registration records
  one exact direct child, the supervisor waits only on that child,
  and the reaper proves only the recorded custodian dead. The
  design therefore registers the Go client as the custodian, the
  server inherits the job's process group and instance tag, and
  terminal CAS requires the WHOLE group proven gone — client,
  server, and any descendants — exactly like today's cancellation
  sweep.
- `session/cancel` is a bounded courtesy (deadline, then the
  existing TERM/KILL group sweep). Kill authority never depends on
  protocol cooperation.
- The lifecycle loop keeps heartbeats flowing and enforces
  handshake and capability deadlines even while ACP I/O is blocked;
  a wedged read never wedges custody.
- P2 fixtures must include: a survivor left after server exit, a
  daemonizing server, a blocked half-frame, TERM-then-KILL
  escalation, and client death with the server alive.

## Client capabilities: advertise nothing (F3)

ACP lets clients advertise filesystem and terminal methods the
server can call back into — a second side-effect path that would
bypass the envelope entirely. The production client advertises NO
client filesystem and NO terminal capabilities (the wire probe
already ran this way). Any unsolicited server→client effect call
fails closed and is recorded as a protocol violation in the round
events. Enabling either capability later is a separate containment
and custody design, not a flag.

## The permission decision (F4, F5)

`internal/acp.Decide(envelope, request)` is a pure function over
ALL FIVE envelope fields — writeRoots, readRoots, network, tools,
approvals — with a total decision table, not a slogan:

- Effect admissibility and human interruption are separate axes.
  The envelope is immutable for the life of the job; no answer may
  widen it.
- v1 ships WITHOUT an escalation lifecycle: a request the table
  cannot classify as in-envelope is denied. Both standard presets
  (none, workspace) already deny approvals, so there is no
  same-job approval surface to preserve; building a durable
  escalation path (ownership, timeout, cancellation, restart,
  authorization) is future work and gets its own design if ever
  wanted.
- Fixtures must prove the `tools` grades bite: `read-only` denies
  state-changing tool categories even inside an allowed root, and
  `runtime-default` never overrides roots or network.
- Unknown permission-request dialects fail closed (deny + record),
  never fall through to allow.

## Delivery: assembly feeds the existing owners (F6, F9)

Message-chunk notifications are journaled durably as they arrive
and deterministically assembled at turn end. A matched successful
`PromptResponse` is COMPLETION evidence; the assembled bytes are
the return candidate WITH protocol provenance, and they enter the
unchanged D62 owners: qualification, normalization, validation,
immutable snapshot, adjudication, one-repair. Partial streams
remain evidence, never delivery. D62's ladder is not retired by
this design: it remains the qualification machinery for ALL
transports and the delivery path for legacy ones; ACP merely gives
it a better-provenanced candidate. ACP delivery earns any future
ladder simplification on its own record, independent of permission
enforcement (the D62 gate is independent of the D61 gate — three
dangerous-mode rounds once produced no result, so delivery and
permissions demonstrably fail independently).

## Usage: one live source per transport (F11)

The mandate records that ACP has no standard usage reporting.
Therefore:

- The ACP transport's live usage source stays what it is today:
  ATIF final_metrics at exit, cumulative counters differenced
  against the predecessor by the existing internal/usage devin
  owner. If the wire turns out to carry usable metrics (probe
  question), they become the declared live source for the ACP
  transport ONLY after their counter semantics (cumulative vs
  delta, predecessor identity, resumed-turn delta, repair
  replacement) are specified and fixtured. Transcript-derived and
  wire-derived figures are ALTERNATIVES — never summed.
- Dead-round recovery is a distinct operation and stays declared
  unsupported for devin unless complete wire evidence proves an
  honest delta can be reconstructed.

## Failure outcomes (F7)

The client ships with an outcome table, not fixture names:

| Event | Outcome |
|---|---|
| protocolVersion mismatch | refuse before session; job fails preflight; no retry on the other transport (F8) |
| authMethods non-empty and session methods reject unauthenticated | fail closed with a distinguishable `auth-required` error surfaced to the operator; never attempt interactive auth inside a job (wire probe: devin advertises `devin-browser`) |
| unknown REQUIRED capability/variant | refuse the session |
| malformed or oversized frame | protocol error; record; teardown |
| JSON-RPC error on prompt | turn fails with the error as evidence |
| every stop reason | explicit mapping: end_turn→assemble+qualify; cancelled→cancelled turn; refusal→refused turn with evidence; unknown stop reason→protocol error, never silent success |
| EOF before PromptResponse | turn fails; journaled chunks are evidence, not delivery |
| teardown timeout | TERM/KILL group sweep; custody proof before terminal CAS |

Once a prompt MAY have executed, server death never causes
automatic restart or replay — side effects and spend are uncertain,
so the turn fails and a human-visible record says why (F7). An ACP
failure never switches the same job to the legacy transport or to
`dangerous` mode (F8): rollback is a pre-launch selection, made for
the NEXT job, never a mid-job fallback (F9).

## D61 and D62 retirement, stated honestly (F8, F9)

- D61 (dangerous-mode waiver): ACP-selected jobs never invoke it
  after acceptance. The legacy path keeps the waiver as long as it
  remains callable; D61 retires only when the legacy devin delegate
  path is REMOVED, which is a later decision on ACP's record.
- D62 (delivery ladder): retirement is not coupled to permission
  proof. The ladder's qualification owners survive regardless; any
  simplification of the legacy scraping legs waits until ACP
  delivery has independently earned trust.
- Session-bridge honesty: follow-up and repair currently resume via
  `devin -p -r`. P1 must separately prove ACP→ACP `session/load`,
  legacy→ACP load, and ACP→legacy resume; a direction that fails is
  CLOSED as a fallback, and repair planning must know which
  directions exist before ACP jobs enter chains.

## Registry: data only (F14)

`internal/runtimes` gains only the EXPECTED shape: a declaration
that a runtime is expected to support ACP transport, with the
pinned protocol version. Behavior lives where behavior lives:
launch argv and protocol translation in the adapter-owned tables,
usage decoding in internal/usage, and the conformance test joins
declarations to registrations both ways, exactly like the B1
capability tables. A runtime without the declaration keeps its
current path unchanged; adoption is per-runtime and reversible.

## Events are advisory (F15)

ACP notifications feed progress display and wakeups (the per-turn
last-event-age heartbeat the streaming mandate asked for). They
COMMIT nothing: job, run, usage, delivery, and turn outcomes are
committed by their existing owners to records, and recovery reads
records. The bounded raw wire journal (every JSON-RPC exchange,
size-capped) is kept separate from the typed flight-recorder
catalog; the journal is evidence, the catalog is contract.

## Prototype plan (design gate before build stands)

P1 (extend the existing probe; throwaway, never shipped): beyond
the captured initialize (plans/acp-wire-probe.md), the probe must
answer: does an unauthenticated `session/new` work or fail (and
how does `devin-browser` auth manifest); does `session/load`
actually load in all three bridge directions; what does a real
prompt turn's notification stream and PromptResponse look like
(stop reasons observed); does the wire carry usage metrics; what
does `session/cancel` do to the server process. The probe
deliberately advertises no client capabilities, and its captures
land beside the first one.

P2: `internal/acp` with the fake runtime speaking a stub ACP server
— fixtures drive every row of the outcome table, the full decision
table over all five envelope fields, chunk assembly and torn
streams, the custody probes (survivor, daemonizer, blocked frame,
TERM/KILL, client death), and usage counter semantics.

P3: devin's declaration + adapter integration behind a conf flag,
selection pinning the transport into the snapshot; bm-style live
smoke. Acceptance for any `mapped` flip: behavioral proof of
allowed AND forbidden effects per field on the ACP path (F1).

## Blast radius

internal/acp (NEW), internal/runtimes (expected-ACP declaration),
internal/adapter (devin ACP launch table + decision function wiring
+ snapshot transport field), internal/capability (selection on
transport), internal/usage (only if wire metrics prove out),
scripts/agents/adapters/devin.sh (dispatch path behind the flag),
dispatch/job records (protocol provenance fields), fixtures (stub
server suite), docs (orchestration transport section). NOT touched:
scripts/agents/hosts/devin.sh, internal/host recollection, the D62
qualification owners.

## Loop discipline

Critique rounds with codex at xhigh; two-budget allowance; stop on
zero unrefuted material findings or the ratified exits. r1 found 15
(7 critical-structural); all folded above, none refuted. The r2
critique should attack: whether the outcome table is complete
against the ACP v1 schema, whether transport-in-identity breaks any
existing snapshot consumer, whether the custody registration change
(Go client as custodian) holds against the reaper's proofs, whether
the decision table can actually be total over today's five fields,
and whether anything in this draft still needs a fact only P1 can
supply.
