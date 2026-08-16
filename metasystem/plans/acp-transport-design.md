# ACP as the delegate transport (backlog item 18)

- Status: DRAFT r5 — critiques r1–r4 folded (r1: 15, r2: 8, r3: 13,
  r4: 13 findings; all folded, none refuted). Second budget in
  progress. r4 validated: preflight narrowing is sound against the
  envelopes real dispatches use, the per-effect matrix is total
  over validated ordinals, and the shipped record lock can support
  the sealed-generation protocol.
- Goal: acp-transport
- Next step: Fold the critique verdict when run acp-critique-r5 concludes; implement only after convergence.
- In flight right now: run acp-critique-r5 (codex xhigh critique; watch it with: bin/metasystem run watch --id acp-critique-r5 --root .)

The human's question that raised this (2026-08-15, on the Devin
delivery failures): "Is there no way to use ACP to make this more
robust?" There is: the installed CLI ships `devin acp` (an Agent
Client Protocol server over stdio, wire capture in
plans/acp-wire-probe.md). The ratified direction (backlog-notes
streaming item, 2026-08-09) fixes the shape: ACP is a transport
WITHIN the per-turn lifecycle — process exit stays the turn
boundary.

## Protocol pins (r2 F4, r3 F12)

The implementation and every fixture pin ONE schema artifact
release (recorded by digest when P2 starts; upgrades are
deliberate changes with fresh conformance) and the stdio
TRANSPORT/FRAMING contract (delimiter, encoding, stdout/stderr
purity, max frame size, partial-frame behavior) from P1's raw byte
trace.

## Scope (the honest increment)

- DEVIN DELEGATE transport only (r1 F14). Devin MISSION HOSTS out
  of scope (r1 F10). Chain persistence out of scope (r1 F12).
- Until P1 proves Devin still delivers a useful `end_turn` under
  denied permissions, ACP dispatch is only offered to workloads
  expected to issue no permission calls; strict refusal is a
  DEFENSIVE FAILURE MODE, not supported behavior (r3 F4).

## What ACP buys, restated without over-claim

1. **Permission requests become answerable** — ADMISSION CONTROL,
   not containment (r1 F1): nothing here is a sandbox.
2. **Delivery gains a typed completion signal** — completion
   EVIDENCE, not a typed return object (r1 F6); assembled bytes
   still enter D62 qualification.
3. **Progress becomes visible** — ADVISORY accelerators under the
   accelerator ruling (r1 F15); records stay authoritative.

## Transport identity and selection (r1 F2; r2 F3; r3 F6, F7, F8; r4 F7)

- Pre-launch, selection pins the REQUESTED transport, the EXPECTED
  protocol version, and the SCHEMA ARTIFACT DIGEST from trusted
  configuration; initialization records and verifies the
  negotiated version. Job provenance records requested transport,
  expected version, negotiated version, digest, and the exact
  snapshot path.
- Snapshots are SINGLE-TRANSPORT with identity (runtime, CLI
  version, configuration hash, transport, expectedProtocolVersion,
  schemaArtifactDigest — the pins not-applicable for legacy);
  identity appears in the snapshot fields, filename, selection
  match, and GC retention key (r3 F6).
- GC never deletes a snapshot referenced by an undeleted job
  record until a durable mirror manifest proves that EXACT file
  was copied; the mirror's silent omission of missing snapshots is
  a defect in scope (r3 F8).
- Existing plural-`transports` snapshots read as legacy under a
  declared migration rule and never satisfy ACP selection.
- Transport-keyed interfaces are explicit, not implied (r4 F7):
  the registry enumerates each runtime's declared transports; the
  probe, contract producer, fake producer, and self-test all take
  an explicit transport parameter and emit/consume
  single-transport snapshots (the common emitter's hard-coded
  plural list, devin's and fake's zero-argument contract verbs,
  and the self-test parameter set all change); dispatch self-heal
  invokes the probe WITH the transport it needs; the self-test
  reader filters by transport + version + digest, never
  newest-wins. The suite iterates every declared (runtime,
  transport) pair: containment certifies once per runtime,
  admission runs per pair.
- Each job pins one transport and snapshot before launch; never a
  mid-job switch; evidence never crosses transports (r1 F2).

## Two enforcement surfaces (r3 F7; r4 F5)

The existing runtime-wide three-field CONTAINMENT declaration
(writeRoots/readRoots/network, mapped|notEnforced) is untouched —
the suite's exactly-three-key compare stays valid, and Devin stays
`notEnforced` everywhere until behavioral proof of allowed AND
forbidden effects per field (r1 F1).

The NEW ADMISSION surface is an exact type, defined now:

- Key: (runtime, transport). Fields: exactly the five envelope
  fields — readRoots, writeRoots, network, approvals, tools.
- Values: a closed two-state enum per field — `unproven` |
  `proven` — where `proven` means admission behavior was
  demonstrated in BOTH directions (an in-envelope request
  admitted, an out-of-envelope request refused) on that transport.
- Absence semantics: a pair with no declaration is all-`unproven`;
  the legacy transport is all-`unproven` BY DEFINITION (it answers
  no permission requests); non-ACP runtimes have no ACP pair.
  Duplicate or unknown field keys fail Validate.
- Actual side: the adapter registers admission results in a
  seam-local table (the B1 pattern); each `proven` field binds to
  its evidence — the certifying snapshot identified by runtime,
  transport, protocol version, schema digest, and snapshot path.
- Conformance joins declarations to registrations both ways; the
  suite gains an admission compare beside the containment compare.

## The client: protocol only — launch stays with scripts (r4 F8)

Doctrine assigns launch, wait, signaling, and environment wiring
to scripts; this design does not request a third Go-launch
exception. The topology:

- The ADAPTER SCRIPT creates the stdio plumbing (FIFOs), launches
  `devin acp` with its stdio attached, registers custody, and
  launches the Go client (`bin/metasystem acp turn …`) with the
  already-open pipe ends. The script owns fifo cleanup on every
  exit path.
- The GO CLIENT (`internal/acp`, exposed as a verb) contains no
  Devin-specific launch knowledge. It consumes: the pipe ends, the
  envelope, the pins, the session directive (new | load
  <sessionId>), and the prompt. It produces: the raw wire journal,
  the watermarked assembled candidate with protocol provenance,
  and a TYPED OUTCOME (one row of the failure matrix) on stdout
  for the script to act on. cmd/metasystem gains the verb; the
  shell-callable contracts for `acp turn` and the death-proof verb
  are part of this design's deliverables.

Client lifetime is one PROMPT ATTEMPT (r1 F12): a job has at most
two attempts — initial and D62's one repair — each its own
registered process tree. Repair semantics are disambiguated below
(r4 F9). No process crosses job records. An ACP-selected job never
invokes the legacy repair command.

## Custody: the sealed-generation state machine (r2 F1; r3 F1–F3; r4 F1–F4)

Process tree per attempt: adapter shell → `devin acp` server AND
Go client, both direct children of the script (the launch
topology above), both registered by exact identity (pid + start
time). Children are proven by PID/START, never the tag-in-argv
predicate (r3 F2).

**The state machine (r4 F1), all transitions under the record
lock the custody file already shares with the record CAS:**

1. OPEN a generation before spawning any process of an attempt.
2. REGISTER identities against the open generation; every
   registration advances a mutation revision.
3. SEAL the generation when the script has finished spawning (no
   more processes can be created for the attempt: the spawner
   moves past its spawn section).
4. Death proof is REFUSED while a generation is open.
5. Terminal compare-and-swap atomically compares status AND
   generation AND revision AND sealed state; any mismatch
   invalidates the proof. (The shipped CAS compares only status;
   extending it is in scope — r4 confirmed the lock supports
   this.)
6. Opening the repair attempt opens a NEW generation atomically;
   registration against a sealed generation refuses and the new
   process is torn down.
7. ABANDONED-OPEN recovery: a generation left open by a spawner
   proven dead (pid+start) may be sealed by the sweeper — the
   spawner being dead, no further registration can occur; the
   identities registered so far are the set.
8. Records WITHOUT generations follow the stated legacy contract:
   empty custody list → top-level identity alone; populated list →
   top-level plus every valid entry. Malformed or unknown entries
   DEFER terminalization (r3 F2).
9. Generic record-patch verbs REFUSE to modify custody-control
   fields (list, generation, seal, revision).

**One proof owner, mode selected by COMMITTER LIVENESS (r3 F3;
r4 F2):** a single Go-owned death-proof function
(internal/supervise), exposed as a verb, serves every terminal
path. The mode is chosen by who commits, not by outcome:

- `except-live-custodian` — any terminal write performed by the
  STILL-LIVE adapter itself: setup failures, running failures,
  protocol errors, deadline handling, handshake rejection, normal
  completion. The committer authenticates itself (its pid+start
  must match the registered custodian) and proves everything but
  itself dead.
- `full-set` — any EXTERNAL committer: the standing reaper,
  mission drain, dispatch cancellation, and the lease sweep. The
  whole set including the adapter is proven dead.

**Both modes always observe the process group (r4 F4):**
`full-set` requires the group EMPTY; `except-live-custodian`
permits only the exact authenticated adapter in it. Kill authority
controls SIGNALING only, never proof strength. Non-kill paths
DEFER terminalization on survivors or indeterminate enumeration.

**Every shipped terminal writer routes through the owner (r4
F3):** the standing reaper and mission drain (both currently prove
only the top-level pid); the LEASE SWEEP (currently one group
SIGTERM then a direct `failed` rewrite — it becomes signal, prove
full-set, defer on survivors; internal/lease joins the blast
radius); dispatch's post-spawn launch failures (once launch has
returned a PID, that identity is retained and proven dead before
`pending → failed`; only a positively proven never-launched
reservation husk — no PID ever recorded — may use the zero-process
exception); and the deadline path (signal, prove, then
terminalize — never terminalize first).

**Kill paths:** the sweep signals the process group AND each
registered identity individually, re-verifying pid+start
immediately before every signal (PID-reuse-safe). A registered
identity surviving TERM/KILL escalation leaves the record
NON-TERMINAL with a loud custody failure. A daemonized server that
left the group is still an exact registered identity. Descendants
outside both the group and registration remain the legacy residual
(tag census backstop); this design does not claim to close that
hole, and with r4 F4 folded the no-kill paths no longer widen it.

`session/cancel` is a bounded courtesy (it cancels a prompt; it is
NOT a shutdown contract — r3 F11), always followed by stdin close,
grace deadline, TERM/KILL sweep. The lifecycle loop keeps
heartbeats and deadlines running while ACP I/O is blocked in
either direction: wedged read and blocked write are distinct
bounded fixtures (r2 F8).

## Client capabilities: advertise nothing (r1 F3)

No client filesystem, no terminal capabilities. Unsolicited
server→client effect calls fail closed and are recorded. Enabling
either later is a separate design.

## The permission decision (r1 F4, F5; r2 F2; r3 F4, F5; r4 F6, F12, F13)

**Correlation gate BEFORE normalization (r4 F6):** a permission
request must carry the ACTIVE session ID and arrive inside an OPEN
prompt window. Wrong-session, pre-prompt, and post-response
requests are named protocol violations — recorded, answered
`cancelled`, never normalized, never reaching Decide.

**Stage 1 — the request normalizer** (internal/acp, a named
stage): translates a correlated wire request into a NORMALIZED
EFFECT REQUEST, resolving live filesystem facts (canonical paths,
symlinks) so the decision stays pure. Its Devin-specific input is
the versioned wire-to-effect mapping P1 captures. Output classes:
read(paths) | write(paths) | execute(command) | network(target) |
unknown. Effect shapes P1 has not captured (reads, deletes, moves,
searches — r4 F12) normalize to `unknown` until captured.

**Stage 2 — `Decide(normalizedEffects, envelope)`**, pure and
total over the real ordinals (network/approvals: deny<ask<allow;
tools: read-only<runtime-default):

| Effect | Verdict |
|---|---|
| read(paths) | allow iff every canonical path ⊆ readRoots ∪ writeRoots; else deny |
| write(paths) | allow iff tools ≠ read-only AND every canonical path ⊆ writeRoots; else deny |
| execute(command) | deny when tools = read-only; allow when tools = runtime-default, raw command recorded as evidence — admission, not containment |
| network(target) | allow iff network = allow; deny iff network = deny (ask cannot occur — preflight) |
| unknown / missing required facts | unclassifiable |
| multiple effects | allow only if EVERY constituent allows; else deny |

**Preflight narrowing** keeps the matrix total: an ACP job whose
envelope has approvals ≠ deny or network = ask fails preflight
loudly as unsupported-on-ACP-v1 (r3 F5; r4 confirmed this is not
over-tight: shipped presets use approvals=deny and real dispatches
use network=allow|deny).

**Option mapping (r2 F2; r3 F4; r4 F13):** the response must
return one exact offered option ID. Verdict allow → requires
EXACTLY ONE `allow_once` option: zero, multiple, or duplicate-ID
matches return `cancelled`. Verdict deny/unclassifiable → exactly
one `reject_once`, else `cancelled`. NEITHER `allow_always` NOR
`reject_always` is ever selected (`reject_always` is remembered
server-side and would poison a loaded repair session). Authority
is never inferred from titles. Fixtures cover each cardinality.

**Strict-refusal mode** (pre-dialect): every request takes the
deny branch. A defensive failure mode, not supported behavior
(r3 F4); P1 must send real denials and cancellations and observe
whether turns still end usefully.

The envelope is immutable; no answer may widen it. Fixtures prove
the grades bite (r1 F5).

## Delivery: watermarked assembly feeding D62 (r1 F6, F9; r2 F5; r3 F13)

- WATERMARK after `session/load` replay completes, before the
  prompt; candidate assembly consumes only `agent_message_chunk`
  updates for the matching session inside the current prompt
  window.
- Deterministic assembly: arrival order; NO content deduplication
  (repeated text is legitimate; no stable chunk identity is
  assumed unless P1 proves one); message-ID grouping when present;
  final complete message wins, earlier ones stay evidence;
  non-text blocks journaled, never candidate bytes; truncation and
  size ceilings disqualify.
- Channel rule: for ACP jobs, `acp` is the ONLY candidate channel —
  an invalid candidate FAILS HONESTLY, never falls through to the
  legacy scraping channels. Legacy jobs keep the existing
  precedence. The collector gains the channel additively;
  everything downstream of candidate selection is reused as-is.
- A matched successful PromptResponse is COMPLETION evidence;
  partial streams are never delivery. D62 is not retired (r1 F9).
- P1 captures load-replay-then-prompt; P2 proves stale prior JSON
  cannot win.

## Repair (r2 F6; r3 F9; r4 F9, F10)

Reused unchanged: validation, adjudication RULES, the durable
repair claim, precedence. Changed for ACP: execution (second
registered tree + session/load), collection (acp channel on the
repair window), usage, settlement, custody (new generation),
terminal sequencing (both trees in the proof set).

Two DISTINCT no-repair/failed-repair cases (r4 F9):

- **Repair disabled pre-claim**: P1 never proved `session/load`
  (or the direction was closed). NO claim is written; the job
  simply has no repair. This is the "runs without repair" case.
- **Claimed repair fails**: the durable claim was written, then
  the live load or setup of the repair attempt failed. The claim
  is consumed; both attempts' usage is accounted (per the
  completeness boundary below); second-tree custody is proven;
  settlement is skipped; repair-failure precedence applies. Never
  preflight.

**ACP settlement (r4 F10):** the shipped settlement requires the
legacy repair transcript, so `DevinSettle` is declared
LEGACY-ONLY. ACP repair settlement is defined fresh: authoritative
session identity is the ACP session ID with negotiated provenance;
effective-model evidence is the session-certification evidence P1
question 7 captures; artifacts are the wire journal and the
immutable candidate snapshot; the owner is a new adapter-registered
ACP settlement hook (B1 seam pattern); a settlement failure on a
successful repair fails the round, exactly today's contract. If P1
shows the wire cannot certify the effective model, ACP repair
STAYS DISABLED (the pre-claim case) until it can — settlement
without evidence is not an option.

## Usage (r1 F11; r2 F7; r3 F10; r4 F11)

Unresolved until P1: does Devin emit `usage_update` (semantics?),
and can ACP mode produce a bounded ATIF export — across initial,
loaded, and repair prompts, AND through the failure paths: induced
repair-load error, prompt error, cancellation, early EOF, observed
through teardown (r4 F11).

**Completeness boundary (r4 F11):** `usage_update` has no
final-marker, so an attempt's wire usage is COMPLETE only when a
matched PromptResponse and clean wind-down bound it — or when the
selected source proves completeness another way (a post-exit ATIF
export, if ACP mode has one). An attempt without a complete source
publishes usage UNAVAILABLE for the round — including initial
setup/load failures — never a fabricated partial number.

After P1: EXACTLY ONE authoritative source, with both selectable
branches prescribed (r3 F10): cumulative-session totals (repair
REPLACES provisional, differenced against exact predecessor) or
genuine per-attempt deltas (combined EXACTLY ONCE, including load
spend). A launched repair lacking complete usage makes the round
unavailable; failed repairs still account spend WHEN their source
is complete. Wire frames are journaled evidence; the owner
consumes the journal. Dead-round recovery stays unsupported.

## Failure outcomes (r1 F7; r2 F4; r3 F9; r4 F2)

Phase-by-phase against the pinned schema and framing. The
COMMITTER column fixes the proof mode (r4 F2): `adapter` =
still-live adapter, `except-live-custodian`; `external` =
reaper/drain/cancel/sweep, `full-set`.

| Phase / event | Committer | Outcome |
|---|---|---|
| initialize: negotiated ≠ expected | adapter | preflight failure; no transport switch |
| INITIAL attempt setup error (initialize/auth/new/load JSON-RPC error) | adapter | phase-named preflight failure, error as evidence |
| CLAIMED REPAIR setup/load failure | adapter | repair failure: claim consumed, both attempts accounted, second-tree custody proven, settlement skipped (r4 F9) |
| auth required (authMethods non-empty, unauthenticated rejected) | adapter | distinguishable `auth-required` failure; never interactive auth |
| unknown REQUIRED capability/variant | adapter | refuse the session |
| malformed/oversized frame, mismatched ID | adapter | protocol error; teardown |
| prompt JSON-RPC error | adapter | turn fails, error as evidence |
| stop end_turn | adapter | assemble window + qualify |
| stop cancelled | adapter | cancelled turn; chunks are evidence |
| stop refusal | adapter | refused turn with evidence |
| stop max_tokens / max_turn_requests | adapter | INCOMPLETE; evidence only |
| unknown stop reason | adapter | protocol error; never silent success |
| update: agent_message_chunk | — | journal + assemble (in window) |
| update: tool_call / plan / mode / config / session-info | — | journal; advisory heartbeat |
| update: usage_update | — | journal; source pending P1 |
| unknown/extension notifications | — | journal and ignore |
| unsolicited server→client request | adapter | fail closed; violation recorded |
| permission request failing the correlation gate | adapter | protocol violation; answered cancelled; never normalized (r4 F6) |
| cancellation race | adapter | complete PromptResponse before teardown wins; else cancelled |
| EOF before PromptResponse | adapter | turn fails; chunks are evidence |
| teardown timeout | adapter → external | TERM/KILL sweep; proof per the surviving committer's mode |
| stale record swept (reaper/drain/lease) | external | full-set proof; defer on survivors |
| post-spawn launch failure | adapter or external | recorded PID retained and proven dead before failed (r4 F3) |

Once a prompt MAY have executed, server death never causes
automatic restart or replay. An ACP failure never switches the
same job to legacy or `dangerous` (r1 F8); rollback is a
pre-launch selection for the NEXT job (r1 F9).

## D61 and D62 retirement, stated honestly (r1 F8, F9)

- D61: ACP jobs never invoke it; the legacy path keeps the waiver
  while callable; D61 retires only when the legacy path is
  REMOVED.
- D62: retirement is not coupled to permission proof; owners
  survive with one additive channel.
- Session bridges: P1 proves all three directions separately; a
  failed direction is CLOSED before ACP jobs enter chains.

## Registry: data only (r1 F14)

`internal/runtimes` gains only expected shapes: the expected-ACP
declaration (transport + expected protocol version), the declared
transport enumeration, and the admission surface — all data.
Launch argv and protocol translation live in adapter-owned tables
and scripts; usage decoding in internal/usage; conformance joins
both ways. Runtimes without declarations keep their current paths.

## Events are advisory (r1 F15)

Notifications feed progress and wakeups; they COMMIT nothing.
Outcomes are committed by owners to records; recovery reads
records, correct under missing, duplicated, reordered, or
truncated notifications. The bounded raw wire journal stays
separate from the typed flight-recorder catalog.

## Prototype plan (design gate before build stands)

P1 (extend the probe; throwaway) must answer:

1. Unauthenticated `session/new`; how `devin-browser` auth
   manifests.
2. `session/load` in all three bridge directions, including load
   REPLAY followed by a fresh prompt.
3. A real prompt turn's stream: stop reasons, update kinds,
   message-ID usage.
4. Permission dialect — provoked WRITE, shell, network requests
   AND inside-root/outside-root READS, deletes, moves, searches
   (r4 F12), options captured verbatim; then send `reject_once`
   and `cancelled` responses and observe whether turns still end
   usefully (r3 F4). If reads provoke no request, read admission
   stays unproven.
5. Usage: `usage_update` emission and semantics; ATIF export
   ability in ACP mode; across initial, loaded, repair prompts AND
   induced failures (repair-load error, prompt error,
   cancellation, early EOF) through teardown (r4 F11).
6. `session/cancel`: effect on prompt AND server process.
7. Launch identity: production argv mapping (cwd/workspace, model,
   job config) into session/new/load; effective-model
   certification evidence (also the ACP settlement prerequisite —
   r4 F10).
8. Wind-down: normal and failure shutdown — stdin close, late
   frames, exit code, grace deadline, session durability before
   repair load, TERM/KILL; pid/start/pgid/descendants through both
   attempts; whether the server carries the job tag in argv.
9. Raw byte framing trace: delimiter, encoding, stdout/stderr
   purity, frame size, partial frames.

P2: `internal/acp` + the fake runtime's stub ACP server — fixtures
drive every matrix row with its committer/proof mode, the
correlation gate, the normalizer + Decide (strict and
post-dialect, option cardinalities), watermarked assembly with the
stale-JSON attack, the custody state machine (open/register/seal/
CAS, abandoned-open recovery, registration races, patch refusal,
survivor, daemonizer, blocked read, blocked write before/after
prompt, TERM/KILL, client death, repair second tree), the proof
owner in both modes across ALL terminal writers (reaper, drain,
cancel, lease sweep, launch failure), snapshot identity + GC
pinning + transport-keyed producers and self-test, and both usage
branches with the completeness boundary.

P3: devin's declarations + adapter integration behind a conf
flag; bm-style live smoke. Containment `mapped` flips need
behavioral proof per field; admission proof lands on the admission
surface per (runtime, transport).

## Blast radius

internal/acp (NEW: client verb, normalizer, Decide),
internal/runtimes (expected-ACP declaration, transport
enumeration, admission surface), internal/adapter (devin ACP
integration, devincollect `acp` channel, snapshot single-transport
schema, selftest reader + parameters, ACP settlement hook;
DevinSettle declared legacy-only), internal/capability (selection
on full identity), internal/evidence (GC retention + pinning),
internal/dispatch (custody state machine, CAS extension, mirror
manifest honesty, provenance fields, patch refusal),
internal/census (custody recognition), internal/supervise (shared
death-proof owner + verb; reaper), internal/missionrunner (drain),
internal/lease (sweep — r4 F3), internal/usage (per P1),
cmd/metasystem (acp turn + death-proof verbs — r4 F8),
scripts/agents/dispatch.sh (launch topology, cancellation sweep,
deadline ordering, self-heal transport, post-spawn failure
handling), scripts/agents/adapters/runtime-common.sh (fifo
plumbing, registration, completion proof via verb, repair guard,
single-transport emitter), scripts/agents/adapters/devin.sh
(transport-parameterized probe/contract, flagged dispatch path),
scripts/agents/adapters/fake.sh + internal fake producer
(transport parameter), scripts/validate-metasystem.sh (per-pair
admission compare; containment compare unchanged), dispatch/job
records (protocol provenance), fixtures, docs (orchestration
transport section). NOT touched: scripts/agents/hosts/devin.sh,
internal/host recollection, D62 owners downstream of candidate
selection, docs/architecture.md's launch doctrine (the fifo
topology conforms — r4 F8).

## Loop discipline

Codex at xhigh; two-budget allowance; stop on zero unrefuted
material findings or the ratified exits. History: r1 15, r2 8, r3
13, r4 13 — r4 confirmed the lock supports sealed generations and
that preflight narrowing and the matrix hold, while showing the
proof owner missed lease sweep and launch-failure writers, the
admission surface lacked a type, and repair had contradictory
load-failure outcomes. All folded. The r5 critique should attack:
the custody state machine's edge cases (abandoned-open recovery
against a REUSED pid; whether any shipped verb patches
custody-control fields today; seal timing when the script spawns
client and server in sequence); whether except-live-custodian
self-authentication is sound against pid reuse of the adapter
itself; the fifo topology (who owns cleanup on every exit path;
does the blocked-write fixture still hold when the script, not the
client, launched the server; SIGPIPE behavior); whether the
admission type's two-state enum loses information the selftest
evidence model already distinguishes; the ACP settlement evidence
chain against the shipped settle refusals; the usage completeness
boundary against both branches; and P1 completeness once more —
anything the implementation needs that no probe question captures
is a standing defect.
