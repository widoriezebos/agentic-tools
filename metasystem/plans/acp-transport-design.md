# ACP as the delegate transport (backlog item 18)

- Status: DRAFT r4 — critiques r1–r3 folded (r1: 15 findings; r2: 8;
  r3: 13, of which 11 structural; all folded, none refuted). First
  three-round budget exhausted at r3; the stop criterion was
  checked and findings were still changing what gets built (new
  owners appeared: shared death proof, request normalizer,
  admission surface, GC pinning), so the ratified second budget is
  open.
- Goal: acp-transport
- Next step: Fold the critique verdict when run acp-critique-r4 concludes; implement only after convergence.
- In flight right now: run acp-critique-r4 (codex xhigh critique; watch it with: bin/metasystem run watch --id acp-critique-r4 --root .)

The human's question that raised this (2026-08-15, on the Devin
delivery failures): "Is there no way to use ACP to make this more
robust?" There is: the installed CLI ships `devin acp` (an Agent
Client Protocol server over stdio, re-verified at devin 3000.4.25,
wire capture in plans/acp-wire-probe.md). The ratified direction
(backlog-notes item on streaming, 2026-08-09) fixes the shape: ACP
is a transport WITHIN the per-turn lifecycle, not a persistent
server — process exit stays the turn boundary.

## Protocol pins (r2 F4, r3 F12)

Wire protocolVersion 1 does not freeze the message shapes: ACP's
schema artifact evolves independently of the wire version. The
implementation and every fixture pin:

- ONE schema artifact release, recorded by digest in the repo when
  P2 starts; adopting a newer artifact is a deliberate change with
  a fresh conformance pass.
- The stdio TRANSPORT/FRAMING contract (delimiter, encoding,
  stdout-versus-stderr purity, maximum frame size, partial-frame
  behavior), pinned from P1's raw byte trace — the current probe
  capture is normalized JSON and proves nothing about framing.

## Scope (the honest increment)

- First supported increment: the DEVIN DELEGATE transport only.
  Cross-runtime neutrality is a property of the protocol, not of
  this design (r1 F14).
- Devin MISSION HOSTS are out of scope: hosts keep `devin -p` +
  `host devin-collect`, and B1 host recollection survives
  untouched until a separately scoped host-ACP design (r1 F10).
- Chain persistence (a server crossing job records) is out of
  scope (r1 F12).
- Increment-one WORKLOAD eligibility is narrowed honestly (r3 F4):
  until P1 proves Devin still delivers a useful `end_turn` under
  denied permissions, ACP dispatch is only offered to workloads
  expected to issue no permission calls, and strict refusal (below)
  is classified as a DEFENSIVE FAILURE MODE, not supported
  transport behavior.

## What ACP buys, restated without over-claim

1. **Permission requests become answerable.** Today a Devin
   dispatch runs full auto-approve because prompts dead-stop the
   CLI (D61's ruled waiver). Under ACP, `session/request_permission`
   arrives as a JSON-RPC call answered from the job's envelope.
   This is ADMISSION CONTROL — our side decides what the agent may
   attempt — not containment (r1 F1): nothing here is a sandbox,
   and an allowed shell command can still try to escape its roots.
2. **Delivery gains a typed completion signal.** A `PromptResponse`
   with a stop reason is completion EVIDENCE, not a typed return
   object (r1 F6); content arrives as message-chunk notifications
   and the assembled bytes still enter D62 qualification.
3. **Progress becomes visible.** Streamed updates give supervision
   a per-turn last-event-age heartbeat — ADVISORY accelerators
   under the accelerator ruling (r1 F15); records stay the
   authoritative state.

## Transport identity and selection (r1 F2; r2 F3; r3 F6, F7, F8)

Transport is part of capability and enforcement identity, pinned
BEFORE launch and verified AFTER:

- Pre-launch, selection pins the REQUESTED transport (legacy |
  acp), the EXPECTED protocol version, and the SCHEMA ARTIFACT
  DIGEST, all from trusted configuration. Initialization records
  the ACTUAL negotiated version into job provenance and verifies
  it; a mismatch fails preflight. Job provenance records:
  requested transport, expected version, negotiated version,
  schema digest, and the exact snapshot path.
- The capability snapshot becomes SINGLE-TRANSPORT with identity
  (runtime, CLI version, configuration hash, transport,
  expectedProtocolVersion, schemaArtifactDigest) — the latter two
  not-applicable for legacy. Transport and both pins are explicit
  snapshot fields, filename identity, selection-match fields, and
  the garbage-collection retention key (r3 F6). The enforcement
  content is per-transport by construction.
- Garbage collection must not delete a snapshot referenced by a
  live job (r3 F8): every snapshot referenced from an undeleted
  job record is retained until a durable mirror manifest proves
  that EXACT file was copied; only unreferenced or durably
  mirrored superseded snapshots may be deleted. The mirror's
  current silent omission of a missing snapshot is a defect in
  scope: mirroring records what it copied, and a referenced
  snapshot that cannot be mirrored blocks deletion.
- Existing plural-`transports` snapshots read as legacy-transport
  snapshots under a declared migration rule; they never satisfy an
  ACP selection (fail-closed, confirmed sound by r3).
- Every producer and consumer moves together, now enumerated (r3
  F7): the Devin probe becomes transport-parameterized (one
  invocation produces one single-transport snapshot), the contract
  and fake producers emit single-transport snapshots, dispatch's
  self-heal invokes the probe WITH the transport it needs, the
  selector matches the full identity, evidence GC keys on it, and
  the SELF-TEST READER filters by transport + expected version +
  schema digest instead of newest-snapshot-wins before certifying
  live permission behavior.
- Enforcement declarations split into two surfaces (r3 F7): the
  existing runtime-wide three-field CONTAINMENT declaration
  (writeRoots/readRoots/network, mapped|notEnforced) is about the
  runtime's own sandbox and stays as it is, transport-agnostic. A
  NEW per-(runtime, transport) five-field ADMISSION surface
  declares what permission admission has behaviorally proven. Both
  live in the registry as data; the suite's three-field compare
  stays valid and a new admission compare joins it.
- Each job pins exactly one transport and snapshot before launch;
  never a mid-job switch; evidence never crosses transports (r1
  F2). Devin's containment declaration stays `notEnforced`
  everywhere until the ACP path behaviorally proves allowed AND
  forbidden effects per field (r1 F1); admission proof lands on
  the admission surface, not the containment one.

## The client: one process tree per prompt attempt (r1 F12, F13; r2 F1, F6)

`internal/acp` is a protocol client whose lifetime is exactly one
PROMPT ATTEMPT: spawn `devin acp`, initialize, establish or load a
session, run one prompt, tear down, exit. A job has at most two
attempts — the initial prompt and D62's one repair — each a
separately registered process tree. The repair attempt starts a
fresh client/server, loads the session via `session/load` (only if
P1 proves it), and prompts under the SAME pinned transport. If
load is unproven or fails, ACP jobs run WITHOUT repair. An
ACP-selected job never invokes the legacy repair command
(`devin -p -r` stays legacy-only). No process crosses job records.

## Custody: sealed sets, one proof owner (r2 F1; r3 F1, F2, F3)

Process tree: adapter shell → Go client → `devin acp` server. BOTH
the client and the server's exact identities (pid + start time)
are registered as custody entries before the first protocol byte.
Children are proven by PID/START IDENTITY, never by the
tag-in-argv custodian predicate — nothing guarantees `devin acp`
carries the job tag in argv (r3 F2); P1 captures the server's
actual proc-table appearance anyway.

**Sealed custody generations (r3 F1).** The custody set carries a
generation. Registration advances it; every death-proof
compare-and-swap checks status AND generation atomically, so an
identity appended after a proof was taken invalidates that proof.
Launching the repair attempt atomically opens a new generation;
registration against a sealed record refuses and tears the new
process down. P2 gets the registration-versus-reap and
registration-versus-cancel race fixtures.

**One proof owner (r3 F3).** A single Go-owned death-proof
function (internal/supervise) serves every terminal path —
standing reaper, mission drain, cancellation, handshake and
capability deadlines, protocol failure, normal completion, repair
failure — exposed to the shell paths through a verb. It has two
declared modes:

- `full-set`: the top-level adapter identity PLUS every valid
  custody entry proven dead, and (in kill-capable paths) the
  process group verified empty, before terminal compare-and-swap.
  Used by reap, drain, cancel, deadline, and failure paths. The
  deadline path's current terminalize-before-cleanup order
  inverts: signal, prove, then terminalize.
- `except-live-custodian`: everything except the still-running
  adapter supervisor itself — the explicit weaker proof for NORMAL
  COMPLETION, where the committer is the survivor. This names
  today's semantics instead of silently contradicting the full
  proof (r3 F3): the adapter proves its client and server dead
  before its own terminal CAS.

The proof-set contract (r3 F2): malformed or unknown custody
entries DEFER terminalization (never read as dead); a legacy
record with an absent or empty custody list is proven by the
top-level identity alone — today's contract, stated, so old
records stay reapable. Group death is stamped only when the
applicable mode's whole set is proven. The mission drain's
independent top-level-only reap changes to call the shared owner;
internal/missionrunner is in the blast radius.

**Kill paths.** The sweep signals the process group AND each
registered identity individually, re-verifying pid+start
immediately before every signal (PID-reuse-safe). A registered
identity that survives TERM/KILL escalation leaves the record
NON-TERMINAL with a loud custody failure — never a quiet green. A
daemonized server that left the group is still an exact registered
identity, so the sweep finds it. Descendants spawned outside the
group AND outside registration remain the same residual risk the
legacy path carries; the instance-tag census sweep is the backstop
and this design does not claim to close that hole (r2 F1).

`session/cancel` is a bounded courtesy (it cancels an active
prompt; it is NOT a server-shutdown contract — r3 F11), always
followed by the normal teardown: stdin close, grace deadline, then
TERM/KILL sweep. Kill authority never depends on protocol
cooperation. The lifecycle loop keeps heartbeats and deadlines
running while ACP I/O is blocked in EITHER direction: wedged read
and blocked write (server not draining our frames) are distinct
bounded fixtures (r2 F8).

## Client capabilities: advertise nothing (r1 F3)

The production client advertises NO client filesystem and NO
terminal capabilities. Any unsolicited server→client effect call
fails closed and is recorded as a protocol violation. Enabling
either later is a separate containment and custody design.

## The permission decision (r1 F4, F5; r2 F2; r3 F4, F5)

Two stages with two owners (r3 F5):

**1. The request normalizer** (internal/acp, a named stage)
translates a wire permission request into a NORMALIZED EFFECT
REQUEST, resolving live filesystem facts — canonical paths,
symlink resolution via the existing path canonicalization — so
that the decision itself stays pure. Its Devin-specific input is
the versioned wire-to-effect mapping P1 captures (the dialect);
its output shape is dialect-neutral: one or more effects of class
read(paths) | write(paths) | execute(command) | network(target) |
unknown.

**2. `Decide(normalizedEffects, envelope)`** is pure and total.
The matrix, over the real envelope (readRoots, writeRoots,
network: deny<ask<allow, approvals: deny<ask<allow, tools:
read-only<runtime-default):

| Effect | Verdict |
|---|---|
| read(paths) | allow iff every canonical path is inside readRoots ∪ writeRoots; else deny |
| write(paths) | allow iff tools ≠ read-only AND every canonical path is inside writeRoots; else deny |
| execute(command) | deny when tools = read-only (execution is state-changing); allow when tools = runtime-default, recording the raw command as evidence — admission, not containment: the command string is opaque and roots are NOT verified for it |
| network(target) | allow iff network = allow; deny iff network = deny (ask cannot occur — see preflight) |
| unknown / missing required facts | unclassifiable |
| multiple effects in one request | allow only if EVERY constituent effect is allow; else the whole request denies |

Preflight narrowing makes the matrix total (r3 F5): v1 has no
escalation lifecycle, so an ACP-selected job whose envelope has
approvals ≠ deny or network = ask FAILS PREFLIGHT loudly as
unsupported-on-ACP-v1 — narrowing eligibility honestly instead of
silently answering "ask" fields with denials. Both standard
presets (none, workspace) already carry approvals=deny, so they
are eligible as-is. This restriction lifts only with a future
escalation design.

**Option mapping** (the wire has no abstract allow/deny — r2 F2):
verdict allow → select `allow_once`; if no `allow_once` is offered,
return cancelled — never substitute a persistent grant. Verdict
deny or unclassifiable → select `reject_once`; if absent, return
cancelled. NEITHER `allow_always` NOR `reject_always` is ever
selected (r3 F4: `reject_always` is remembered by the server and
would poison the loaded repair session). Authority is never
inferred from human-readable titles.

**Strict-refusal mode** (pre-dialect): until P1 captures the
versioned Devin dialect, every request takes the deny branch
(`reject_once`, else cancelled). This is a DEFENSIVE FAILURE MODE,
not supported behavior (r3 F4): the shipped seam records that a
denied tool can end a Devin turn with no reply, so P1 must send
actual denials and cancellations and observe whether the turn
still produces a useful `end_turn` — until it proves that,
increment one only dispatches workloads expected to issue no
permission calls.

The envelope is immutable for the job's life; no answer may widen
it. Fixtures must prove the grades bite: `read-only` denies
state-changing effects even inside an allowed root, and
`runtime-default` never overrides roots or network (r1 F5).

## Delivery: watermarked assembly feeding D62 (r1 F6, F9; r2 F5; r3 F13)

`session/load` REPLAYS history through notifications, so:

- The client establishes a WATERMARK after load completes and
  before the prompt is sent. Candidate assembly consumes only
  `agent_message_chunk` updates for the matching session ID inside
  the current prompt window (after the watermark, before the
  matched PromptResponse).
- Assembly is deterministic: chunks are preserved in ARRIVAL ORDER
  within the window; there is NO content-based deduplication —
  repeated text is legitimate — and no stable chunk identity is
  assumed unless P1 proves one (r3 F13). Chunks group by message
  ID when present, one message when absent; multiple assistant
  messages in a window → the final complete message is the
  candidate, earlier ones stay journaled evidence; non-text blocks
  are journaled, never candidate bytes; truncation and
  size-ceiling breaches disqualify (evidence, not delivery).
- Channel rule (r3 F13): for ACP-selected jobs, `acp` is the ONLY
  candidate channel — an invalid or disqualified ACP candidate
  FAILS HONESTLY and never falls through to stdout/named-file/
  transcript scraping (those are legacy-transport channels; falling
  through would cross transports). For legacy jobs the existing
  precedence is untouched. The collector gains the `acp` channel
  with protocol provenance (session ID, prompt window, stop
  reason, schema pin) additively; everything downstream of
  candidate selection — qualification, normalization, validation,
  immutable snapshot, adjudication — is reused as-is.
- A matched successful PromptResponse is COMPLETION evidence;
  partial streams are evidence, never delivery. D62's ladder is
  not retired by this design (r1 F9); simplification of legacy
  legs waits until ACP delivery independently earns trust.
- P1 captures load-replay-followed-by-prompt; P2 proves a stale
  prior JSON answer cannot win candidate selection.

## Repair: what is reused and what changes (r2 F6; r3 F9)

Reused unchanged: validation, adjudication RULES, the durable
repair claim, and precedence. Changed for ACP: repair EXECUTION
(second registered ACP tree + session/load, never `devin -p -r`),
COLLECTION (the acp channel on the repair window), USAGE (below),
SETTLEMENT, CUSTODY (new sealed generation), and TERMINAL
SEQUENCING (both trees in the proof set).

Setup failures split (r3 F9): an INITIAL-attempt
initialize/auth/new/load failure is preflight — no prompt was
sent, the job fails cleanly. A REPAIR-attempt setup or load
failure is NOT preflight: it consumes the claimed repair, accounts
both attempts' usage, proves second-tree custody, skips
settlement, and follows the existing repair-failure precedence.

## Usage: unresolved until P1, then exactly one algorithm (r1 F11; r2 F7; r3 F10)

ACP v1 defines `usage_update`; whether Devin emits it, with what
semantics, and whether ACP mode can produce a bounded ATIF export
at all (today ATIF exists because the legacy command passes an
explicit export flag) are P1 questions — across initial, loaded,
and repair prompts. After P1, select EXACTLY ONE complete
authoritative source; if none exists, the ACP transport publishes
usage UNAVAILABLE, loudly. Wire and transcript figures are
alternatives, never summed.

Both selectable branches are prescribed now (r3 F10):

- **Cumulative-session totals** (today's shipped algorithm): the
  repair attempt's metrics REPLACE the provisional initial
  calculation and are differenced against the exact predecessor's
  totals.
- **Genuine per-attempt deltas**: attempts combine EXACTLY ONCE,
  including the load spend of the repair attempt.

Either way: a launched repair lacking complete usage makes the
whole round's usage unavailable, and FAILED repairs still account
their spend. Wire `usage_update` frames are journaled evidence; if
selected as the source, the usage owner consumes the JOURNAL,
keeping the accelerator ruling intact. Dead-round recovery stays a
distinct operation, declared unsupported for devin unless complete
wire evidence proves an honest delta.

## Failure outcomes (r1 F7; r2 F4; r3 F9)

Phase-by-phase, against the pinned schema artifact and framing:

| Phase / event | Outcome |
|---|---|
| initialize: negotiated version ≠ expected | refuse before session; preflight failure; no transport switch |
| INITIAL attempt — initialize/auth/new/load JSON-RPC error (standard or arbitrary code) | phase-named preflight failure with the error as evidence (no prompt sent) |
| REPAIR attempt — any setup or load failure | repair failure, NOT preflight: consumes the claim, accounts both attempts, proves second-tree custody, skips settlement (r3 F9) |
| authMethods non-empty and session methods reject unauthenticated | distinguishable `auth-required` failure; never interactive auth inside a job |
| unknown REQUIRED capability/variant | refuse the session |
| malformed frame, oversized frame, mismatched response ID | protocol error; record; teardown |
| prompt: JSON-RPC error | turn fails with the error as evidence |
| stop reason end_turn | assemble window + qualify |
| stop reason cancelled | cancelled turn; chunks are evidence |
| stop reason refusal | refused turn with evidence |
| stop reason max_tokens / max_turn_requests | INCOMPLETE turn; evidence only; never enters delivery qualification |
| unknown stop reason | protocol error; never silent success |
| update: agent_message_chunk | journal + assemble (inside the watermarked window) |
| update: tool_call / plan / mode / config / session-info | journal; advisory heartbeat only |
| update: usage_update | journal; source pending the P1 ruling |
| unknown / extension notifications | journal and ignore; never fail the turn |
| unsolicited server→client REQUEST | fail closed; protocol violation recorded |
| cancellation race | a complete PromptResponse prior to teardown wins; otherwise cancelled; both journaled |
| EOF before PromptResponse | turn fails; journaled chunks are evidence |
| teardown timeout | TERM/KILL sweep; full custody proof before terminal CAS |

Once a prompt MAY have executed, server death never causes
automatic restart or replay. An ACP failure never switches the
same job to the legacy transport or `dangerous` mode (r1 F8);
rollback is a pre-launch selection for the NEXT job (r1 F9).

## D61 and D62 retirement, stated honestly (r1 F8, F9)

- D61: ACP-selected jobs never invoke it. The legacy path keeps
  the waiver while callable; D61 retires only when the legacy
  devin delegate path is REMOVED — a later decision on ACP's
  record.
- D62: retirement is not coupled to permission proof; the owners
  survive with one additive channel.
- Session bridges: P1 proves ACP→ACP load, legacy→ACP load, and
  ACP→legacy resume separately; a failed direction is CLOSED as a
  fallback before ACP jobs enter chains.

## Registry: data only (r1 F14)

`internal/runtimes` gains only expected shapes: the expected-ACP
declaration (transport + expected protocol version) and the
per-(runtime, transport) admission surface — both data.
Launch argv and protocol translation live in adapter-owned tables,
usage decoding in internal/usage, and conformance joins
declarations to registrations both ways. A runtime without the
declaration keeps its current path; adoption is per-runtime and
reversible.

## Events are advisory (r1 F15)

ACP notifications feed progress display and wakeups. They COMMIT
nothing: outcomes are committed by their existing owners to
records, and recovery reads records — correct under missing,
duplicated, reordered, or truncated notifications. The bounded raw
wire journal stays separate from the typed flight-recorder
catalog: the journal is evidence, the catalog is contract.

## Prototype plan (design gate before build stands)

P1 (extend the existing probe; throwaway, never shipped) must
answer, beyond the captured initialize:

1. Unauthenticated `session/new`: works or fails, and how
   `devin-browser` auth manifests on the wire.
2. `session/load` in all three bridge directions, including a
   capture of load REPLAY followed by a fresh prompt (watermark
   evidence).
3. A real prompt turn's stream: stop reasons, update kinds,
   message-ID usage.
4. Permission dialect: provoke real requests (out-of-workspace
   write, shell command, network fetch), capture offered options
   verbatim, AND send denial (`reject_once`) and cancellation
   responses, observing whether the turn still ends usefully
   (r3 F4).
5. Usage: `usage_update` emission and semantics; whether ACP mode
   can produce a bounded ATIF export — across initial, loaded, and
   repair-shaped prompts.
6. `session/cancel`: its actual effect on the prompt AND the
   server process (it is not a shutdown contract).
7. Launch identity (r3 F11): the production-shaped argv and how
   cwd/workspace, requested model, and job-derived configuration
   map into `devin acp` + session/new/load; effective-model
   certification evidence.
8. Wind-down (r3 F11): normal-success and setup-failure shutdown —
   stdin/client close, late frames, exit code, grace deadline,
   session durability before a repair load, TERM/KILL escalation;
   pid/start/pgid/descendant proc-table state throughout both
   attempts (including whether the server carries the job tag in
   argv).
9. Raw byte framing trace (r3 F12): delimiter, encoding,
   stdout/stderr purity, maximum frame size, partial-frame
   behavior.

The probe advertises no client capabilities; captures land beside
the first one in plans/acp-wire-probe.md.

P2: `internal/acp` with the fake runtime speaking a stub ACP
server — fixtures drive every matrix row, the normalizer + Decide
table (strict mode and post-dialect), watermarked assembly
including the stale-prior-JSON attack, the custody set (sealed
generations, registration races, survivor, daemonizer, blocked
read, blocked write before/after prompt, TERM/KILL, client death,
repair second tree), the shared proof owner in both modes across
all terminal paths, snapshot identity + GC pinning + self-test
filtering, and both usage branches.

P3: devin's declaration + adapter integration behind a conf flag,
selection pinning transport + versions into the snapshot;
bm-style live smoke. Acceptance for any containment `mapped` flip:
behavioral proof of allowed AND forbidden effects per field on the
ACP path; admission proof lands on the admission surface (r1 F1,
r3 F7).

## Blast radius

internal/acp (NEW: client, normalizer, Decide), internal/runtimes
(expected-ACP declaration + admission surface), internal/adapter
(devin ACP launch table, devincollect `acp` channel, snapshot
single-transport schema, selftest reader filtering),
internal/capability (selection on full identity),
internal/evidence (GC retention key + referenced-snapshot
pinning), internal/dispatch (custody generations, mirror manifest
honesty, provenance fields), internal/census (custody identity
recognition), internal/supervise (shared death-proof owner; reaper
uses it), internal/missionrunner (drain uses it — r3 F3),
internal/usage (per the P1 ruling), scripts/agents/dispatch.sh
(cancellation sweep: group + exact identities; deadline path
signal-prove-terminalize; self-heal transport parameter),
scripts/agents/adapters/runtime-common.sh (registration seam,
completion proof via the verb, repair seam guard),
scripts/agents/adapters/devin.sh (transport-parameterized probe,
dispatch path behind the flag), scripts/validate-metasystem.sh
(admission compare beside the containment compare), dispatch/job
records (protocol provenance), fixtures (stub server suite), docs
(orchestration transport section). NOT touched:
scripts/agents/hosts/devin.sh and internal/host recollection; the
D62 owners downstream of candidate selection.

## Loop discipline

Critique rounds with codex at xhigh; two-budget allowance; stop on
zero unrefuted material findings or the ratified exits. History:
r1 15 findings, r2 8, r3 13 (11 structural — including that r2's
custody fold was itself unimplementable against the shipped reaper
and that a second reaper lives in mission drain). First budget
exhausted; the findings were still naming new owners, so the
second budget opened. The r4 critique should attack: whether
sealed custody generations are implementable against the shipped
record locking and CAS; whether the two-mode proof owner is
coherent when the committer is the adapter itself (normal
completion self-proof); whether the decision matrix is actually
TOTAL — name a request shape with no row; whether preflight
narrowing (approvals ≠ deny, network = ask unsupported) is
over-tight against envelopes actually used in this repo; whether
the containment/admission surface split keeps every existing suite
compare and conformance join valid; and whether the P1 list is
complete — anything the implementation needs that no probe
question captures is a standing defect.
