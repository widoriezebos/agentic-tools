# ACP as the delegate transport (backlog item 18)

- Status: IMPLEMENTATION-FIRST (D81, human-ratified 2026-08-16:
  "agreed and approved") — this document is the SPEC. Six prose
  rounds (trajectory 15, 8, 13, 13, 10, 8) are folded; the five
  standing r6 structural findings (plans/acp-critique-r6.md:
  marker immutability under the terminal lock, the lease-sweep
  lock split, the cancellation committer split, the settlement
  refusal table, PromptResponse.usage) are resolved BY FIXTURES
  during P2, not by more prose. r6 accepted the D79 scope pivot
  and confirmed the ancestry mechanism implementable.
- Goal: acp-transport (current)
- Next step: P1 — the throwaway wire probe, per the nine-question
  list below; its captures land in plans/acp-wire-probe.md. Then
  P2 (internal/acp + stub-server fixtures, each r6 residue pinned
  by a named fixture), then P3 behind the conf flag.

The human's question that raised this (2026-08-15, on the Devin
delivery failures): "Is there no way to use ACP to make this more
robust?" There is: `devin acp` (stdio ACP server, wire capture in
plans/acp-wire-probe.md). The ratified direction (backlog-notes
streaming item): ACP is a transport WITHIN the per-turn lifecycle —
process exit stays the turn boundary.

## Protocol pins (r2 F4, r3 F12)

One schema artifact release pinned by digest when P2 starts;
upgrades are deliberate with fresh conformance. The stdio framing
contract (delimiter, encoding, stream purity, max frame size,
partial frames) pinned from P1's raw byte trace.

## Scope: gated protocol, split hardening (r5 F10; D79)

- DEVIN DELEGATE transport only (r1 F14); hosts out of scope (r1
  F10); chain persistence out of scope (r1 F12); permission-free
  workloads only until P1 proves denied turns end usefully (r3
  F4).
- **The sealed-custody protocol (below) applies ONLY to records
  that carry it.** ACP-selected jobs are built with a custody
  protocol marker (`custodyProtocol: sealed-v1`); the shared
  proof owner and every external terminal writer (reaper, drain,
  lease sweep, cancellation) check the marker and apply sealed
  rules to marked records ONLY. Records without the marker keep
  today's terminal behavior byte-for-byte. The marker is data on
  the record — no owner learns a runtime name (agnosticism
  preserved).
- **The pre-existing system-wide holes are NOT this design's to
  fix** (r5 F10): the standing reaper's top-level-only proof for
  legacy records, the lease sweep's signal-once-then-rewrite, the
  `RecordProtocolError` CAS bypass for legacy records, and
  second-resolution process identity across the fleet exist today
  and predate ACP. They are split to the queued goal
  `custody-death-proof` (D79) with their evidence — this document's
  critique history. This design inherits those holes for legacy
  records and fixes them for sealed-v1 records; the scope
  statement and the blast radius are now both true.

## What ACP buys (unchanged claims)

Admission control, not containment (r1 F1); a typed completion
signal, not a typed return (r1 F6); advisory progress under the
accelerator ruling (r1 F15).

## Transport identity and selection (r1 F2; r2 F3; r3 F6–F8; r4 F7)

Unchanged from r5: requested transport + expected version + schema
digest pinned pre-launch from configuration and verified at
initialize; single-transport snapshots with the six-part identity
in fields, filename, selection, and GC key; GC never deletes a
snapshot a live job references until a mirror manifest proves the
exact copy; plural legacy snapshots migrate as legacy and never
satisfy ACP selection; transport-keyed probe/contract/fake/
self-test interfaces with registry-derived transport enumeration;
the suite certifies containment once per runtime and admission per
(runtime, transport) pair; no mid-job transport switch, evidence
never crosses transports.

## Two enforcement surfaces (r3 F7; r4 F5; r5 F6)

Containment (three fields, mapped|notEnforced, runtime-wide) is
untouched; Devin stays notEnforced until behavioral proof per
field (r1 F1).

The ADMISSION surface keyed by (runtime, transport) now carries a
FOUR-STATE closed enum per envelope field, preserving the
distinctions the shipped self-test evidence already makes (r5 F6):

- `absent` — no registration for the field.
- `constructed` — evidence assembled but not behaviorally
  exercised.
- `partial` — exactly one direction observed (admitted OR
  refused).
- `certified` — both directions observed: an in-envelope request
  admitted AND the corresponding out-of-envelope request refused,
  under separately pinned envelopes.

Pair presence is separate from field state. The per-field
certification matrix over supported values: readRoots (in-root
read admitted / out-of-root read refused), writeRoots (in/out
write), network (allow admits / deny refuses — two pinned
envelopes), tools (runtime-default admits execute / read-only
refuses execute). **`approvals` is UNCERTIFIABLE in v1** and stays
`absent` with a recorded reason: preflight fixes approvals=deny
and Decide never consumes the field; certification waits for a
concrete governed behavior with a two-sided test (r5 F6). Each
certified field binds to its evidence (runtime, transport,
protocol version, schema digest, snapshot path). Conformance joins
both ways.

## The client: protocol only — launch stays with scripts (r4 F8)

The adapter script owns launch; the Go client (`bin/metasystem acp
turn`) speaks pure protocol over pre-opened pipes and emits the
wire journal, the watermarked candidate with provenance, and a
typed outcome row. No Devin launch knowledge in Go; no doctrine
exception needed (r5 confirmed conformance).

**The descriptor-level launch sequence (r5 F3):**

1. Traps installed BEFORE any resource creation; FIFO paths are
   per-attempt (jobId + generation), never reused.
2. Both children start behind SPAWN GATES: each child blocks on a
   gate before touching protocol I/O, so the script captures and
   registers both birth identities without the short-lived-child
   race, and FIFO opens cannot block against a missing peer
   (the script holds bootstrap descriptors open until both
   children are up).
3. Registration order: open generation → spawn server behind gate
   → register server → spawn client behind gate → register client
   → SEAL → release both gates. Protocol execution cannot begin
   before seal.
4. A partial launch (either spawn or registration fails)
   ABORT-SEALS the generation: seal what registered, tear down,
   typed failure.
5. After releasing the gates the script CLOSES every bootstrap
   descriptor it holds — a retained write end would suppress EOF
   forever. The script waits on and reaps BOTH children.
6. EOF, EPIPE, and SIGPIPE map to typed outcomes (peer-close is a
   named protocol death, not a silent hang); P2 gains
   peer-close-before-write, peer-close-during-write,
   partial-spawn, registration-failure, and crash-cleanup
   fixtures.
7. Cleanup on trap-reachable exits is the script's; stale FIFOs
   after an untrappable death (SIGKILL) are recovered by the
   EXTERNAL sweep that already handles the dead generation — the
   janitor path for marked records, keyed by the per-attempt
   naming.

Client lifetime is one PROMPT ATTEMPT (r1 F12); at most two
attempts per job; no process crosses job records; ACP jobs never
invoke the legacy repair command.

## Custody: sealed-v1 (r2 F1; r3 F1–F3; r4 F1–F4; r5 F1, F2, F4, F5)

Applies to marked records only (see Scope).

**Identity (r5 F1):** sealed-v1 custody entries and the top-level
adapter identity carry a KERNEL-RESOLUTION BIRTH TOKEN (Darwin:
pid + microsecond start; Linux: pid + starttime ticks) — the
second-resolution identity the fleet uses today cannot exclude
same-second pid reuse. internal/identity gains the token type with
cross-platform fixtures; sealed-v1 proof, signaling, custodian
authentication, and abandoned-open recovery all compare tokens.
Where a sealed-v1 path meets a second-resolution identity it
DEFERS rather than claims safety. (Fleet-wide migration of legacy
identities: custody-death-proof goal.)

**The state machine (r4 F1):** open generation before spawn;
register against the open generation advancing a mutation
revision; seal when spawning is complete; proof REFUSED while
open; abandoned-open recovery seals a generation whose spawner is
proven dead BY TOKEN (r5 F1); repair opens a new generation;
registration after seal refuses and tears down.

**Proof and terminal commit are ONE operation (r5 F2):** the
proof owner is a single verb that proves AND commits under the
record lock in the same invocation — never "prove, return, then
the caller commits." Its two modes, selected by committer
liveness (r4 F2):

- `except-live-custodian` — invoked from inside the still-live
  adapter's ancestry. The verb AUTHENTICATES the invoking
  ancestry: it walks its own parent chain, finds the recorded
  adapter identity, and verifies it BY TOKEN — untrusted pid/start
  flags are not authentication. It excludes exactly the
  authenticated verifier chain as instrumentation (the shipped
  group-members verb already excludes its own invocation ancestry
  — the same discipline), proves the remaining custody set dead
  and the group otherwise empty, and commits against the same
  status + generation + revision + seal snapshot atomically.
- `full-set` — external committers (reaper, drain, cancellation,
  lease sweep, the handshake-timeout backstop). The whole set
  including the adapter proven dead by token, the group empty,
  then commit — one operation.

Both modes always observe the group (r4 F4); kill authority
controls signaling only; non-kill paths defer on survivors or
indeterminate enumeration.

**Immutability (r5 F4):** every proof input becomes immutable
through generic CAS after its named trusted transition — top-level
pid, birth token, pgid, instance tag, ownership proof, custody
entries, generation, seal, revision, and the generation-owner
identity. Only the dedicated launch/open/register/seal/recovery
verbs mutate them under the record lock. This tightening applies
to ALL records (it changes nothing for well-behaved writers);
sealed-v1 proof REQUIRES it.

**Terminal-writer totality (r4 F3; r5 F5):** every terminal write
for a marked record goes through the one proof-commit verb.
`RecordProtocolError` becomes a wrapper over it for marked records
(legacy records keep the shipped direct write until
custody-death-proof). The dispatcher's handshake-timeout backstop
is an explicit external/full-set writer that signals, proves, then
terminalizes. Launch failure splits: TOP-LEVEL launch failure
(dispatch never got or lost the adapter: external committer,
full-set over whatever identities were recorded; a positively
proven never-launched husk — no pid ever recorded — may use the
zero-process exception) versus ADAPTER CHILD-LAUNCH failure (the
adapter is alive: abort-seal, except-live-custodian). Teardown
handoff when the adapter dies mid-teardown: the adapter makes NO
terminal write; it exits; the named external finalizer (the
standing reaper for marked records) proves full-set and commits.

**Kill paths:** group signal + each registered identity
individually, token re-verified immediately before every signal.
A survivor after TERM/KILL leaves the record NON-TERMINAL with a
loud custody failure. Out-of-group unregistered descendants remain
the legacy residual (tag census backstop). `session/cancel` is a
bounded courtesy, never a shutdown contract (r3 F11); wedged read
and blocked write are distinct bounded fixtures (r2 F8).

## Client capabilities: advertise nothing (r1 F3)

Unchanged: no client fs/terminal capabilities; unsolicited
server→client calls fail closed and are recorded.

## The permission decision (r1 F4, F5; r2 F2; r3 F4, F5; r4 F6, F12, F13)

Unchanged from r5: the correlation gate (active session + open
prompt window) before normalization; the normalizer resolving
filesystem facts into read/write/execute/network/unknown effects;
the pure total Decide matrix over the real ordinals; preflight
narrowing (approvals ≠ deny, network = ask → unsupported-on-ACP-v1,
confirmed not over-tight); option mapping requiring EXACTLY ONE
matching one-shot option (zero/multiple/duplicate → cancelled;
never `allow_always`, never `reject_always`); strict-refusal as a
defensive failure mode; the envelope immutable.

## Delivery: watermarked assembly feeding D62 (r1 F6, F9; r2 F5; r3 F13)

Unchanged from r5: watermark after load replay; arrival-order
assembly, no content dedup, message-ID grouping, final complete
message wins; size ceilings and truncation disqualify; `acp` is
the ONLY channel for ACP jobs (fail honest, no fallthrough);
legacy precedence untouched; D62 not retired; P2 proves stale
prior JSON cannot win.

## Repair (r2 F6; r3 F9; r4 F9, F10; r5 F7)

Reused unchanged: validation, adjudication rules, the durable
claim, precedence. Changed: execution (second tree + load),
collection, usage, settlement, custody (new generation), terminal
sequencing. The two no-repair cases stay split (r4 F9): disabled
pre-claim (no claim written) versus claimed-and-failed (claim
consumed, both attempts accounted per the usage boundary,
second-tree custody proven, settlement skipped, repair-failure
precedence).

**ACP settlement is an EVIDENCE JOIN (r5 F7),** aligned with the
shipped refusal semantics rather than a single decisive
prerequisite:

1. The repair `session/load` REQUEST names the initial session ID
   (LoadSessionResponse echoes nothing, so the request side plus
   replay correlation carry the proof).
2. Replay and prompt frames correlate to that session in the
   journal.
3. The candidate derives from the repair window of that journal.
4. Model evidence follows the shipped policy: observed-and-match
   certifies; ABSENT is recorded `unobserved` and does NOT refuse
   (today's semantics); observed-and-MISMATCH refuses.
5. Journal and candidate artifacts are present, bounded, and
   immutable.

Missing or contradictory SESSION evidence (1–3) disables repair or
fails settlement as decisively as a model mismatch. Each refusal
lists its terminal outcome; settlement failure on a successful
repair fails the round, exactly today's contract. (r5 corrected
r4 here: model evidence is refusal-on-mismatch, not
refusal-on-absence — the P1 question stays, but its absence
gates nothing beyond the `unobserved` marker.)

## Usage (r1 F11; r2 F7; r3 F10; r4 F11; r5 F8)

Unresolved until P1, then EXACTLY ONE source. The completeness
boundary is now branch-specific (r5 F8), because `usage_update`
carries current-context tokens and only optional cumulative cost
with no final marker:

- The wire source is ELIGIBLE only with a version-pinned Devin
  emission guarantee that the final update covers all attempt
  spend. Without that guarantee, wire usage is unavailable EVEN ON
  SUCCESS.
- **Cumulative branch**: a failed attempt (no matched
  PromptResponse) has NO complete wire total — usage is ALWAYS
  unavailable for it unless a post-exit authoritative export or
  query supplies the final total (the ATIF-export P1 question).
- **Per-attempt branch**: failed attempts are unavailable on the
  same terms; successful attempts combine exactly once including
  load spend.

A launched repair lacking complete usage makes the round
unavailable; failed repairs account spend only WHEN their source
is complete — otherwise the round says unavailable, never a
partial number. Frames are journaled; the owner consumes the
journal. Dead-round recovery stays unsupported.

## Failure outcomes (r1 F7; r2 F4; r3 F9; r4 F2; r5 F5)

The matrix from r5 with the committer column made UNIQUE per row:

| Phase / event | Committer (mode) | Outcome |
|---|---|---|
| initialize: negotiated ≠ expected | adapter (elc) | preflight failure |
| INITIAL setup error (initialize/auth/new/load) | adapter (elc) | phase-named preflight failure |
| CLAIMED REPAIR setup/load failure | adapter (elc) | repair failure: claim consumed, abort-seal second generation |
| auth required | adapter (elc) | `auth-required` failure; never interactive |
| unknown REQUIRED capability | adapter (elc) | refuse session |
| malformed/oversized frame, mismatched ID | adapter (elc) | protocol error; teardown |
| peer close / EPIPE / SIGPIPE on the wire | adapter (elc) | typed protocol death (r5 F3) |
| prompt JSON-RPC error | adapter (elc) | turn fails |
| stop end_turn | adapter (elc) | assemble + qualify |
| stop cancelled / refusal | adapter (elc) | cancelled / refused turn |
| stop max_tokens / max_turn_requests | adapter (elc) | INCOMPLETE; evidence only |
| unknown stop reason | adapter (elc) | protocol error |
| updates (chunks, tool_call, plan, usage_update, unknown) | — (no terminal write) | journal per r5 rules |
| unsolicited server→client request | adapter (elc) | fail closed; recorded |
| permission request failing correlation gate | adapter (elc) | violation; answered cancelled |
| cancellation race | adapter (elc) | complete PromptResponse wins; else cancelled |
| EOF before PromptResponse | adapter (elc) | turn fails |
| teardown timeout, adapter alive | adapter (elc) | TERM/KILL sweep then proof-commit |
| adapter dies mid-teardown | external finalizer (full-set) | no adapter write; reaper proves and commits |
| handshake timeout backstop | external (full-set) | signal, prove, terminalize (r5 F5) |
| stale record swept (reaper/drain/lease) | external (full-set) | defer on survivors |
| TOP-LEVEL launch failure | external (full-set) | recorded identities proven; husk exception only when no pid ever recorded |
| ADAPTER child-launch failure | adapter (elc) | abort-seal, typed failure |

(elc = except-live-custodian.) Once a prompt MAY have executed,
never restart or replay; an ACP failure never switches the job to
legacy or dangerous (r1 F8); rollback is pre-launch for the NEXT
job (r1 F9).

## D61 and D62 (r1 F8, F9) — unchanged

ACP jobs never invoke D61; it retires only when the legacy path is
removed. D62 owners survive with one additive channel. Session
bridges proven per direction or closed.

## Registry: data only (r1 F14) — unchanged

Expected-ACP declaration, transport enumeration, admission surface
— all data; behavior in adapter tables, scripts, and usage owners;
conformance joins both ways.

## Events are advisory (r1 F15) — unchanged

Records commit, notifications accelerate; the raw journal is
evidence, the typed catalog is contract.

## Prototype plan

P1 must answer (r5 F9 added the allow side):

1. Unauthenticated `session/new`; `devin-browser` manifestation.
2. `session/load` all three directions; load REPLAY then prompt.
3. A real turn's stream: stop reasons, update kinds, message IDs.
4. Permission dialect, BOTH directions: provoke writes, shell,
   network, in-root and out-of-root reads, deletes, moves,
   searches; capture options verbatim; send `reject_once` and
   `cancelled` AND `allow_once` — verifying an allowed effect
   actually executes and the turn completes usefully; then the
   out-of-envelope refusals under separately pinned envelopes,
   covering readRoots, writeRoots, network allow/deny, tools
   runtime-default/read-only (the admission evidence pipeline).
5. Usage: `usage_update` emission, semantics, and whether any
   version-pinned guarantee covers final spend; ATIF export in
   ACP mode; through induced failures (repair-load error, prompt
   error, cancellation, early EOF) to teardown.
6. `session/cancel` effect on prompt AND process.
7. Launch identity: argv mapping, session certification evidence
   (the settlement `unobserved`/mismatch inputs).
8. Wind-down: closes, late frames, exit codes, grace, session
   durability before repair load, TERM/KILL; token-resolution
   proc-table observations through both attempts; tag-in-argv.
9. Raw byte framing trace.

P2: the stub-server fixture suite as r5 listed, PLUS the r5 F3
launch fixtures (peer-close ×2, partial-spawn,
registration-failure, crash-cleanup), the one-operation
proof-commit verb fixtures (ancestry authentication, verifier
exclusion, token comparison, generation snapshot CAS), and the
marker gating fixtures (a legacy record beside a sealed-v1 record:
external writers apply the right rules to each).

P3: devin's declarations + adapter integration behind a conf flag;
bm-style live smoke; containment flips need behavioral proof;
admission states advance only on P1-shaped evidence.

## Blast radius

internal/acp (NEW), internal/runtimes (declarations + admission
surface), internal/adapter (integration, collect channel, snapshot
schema, selftest reader/params, settlement hook; DevinSettle
legacy-only), internal/capability (selection),
internal/evidence (GC pinning), internal/dispatch (custody state
machine + marker, CAS extension, immutable set, RecordProtocolError
wrapper for marked records, mirror manifest, provenance),
internal/identity (birth token + fixtures — r5 F1),
internal/census (marked-record recognition), internal/supervise
(the one proof-commit verb; reaper marker check),
internal/missionrunner (drain marker check), internal/lease
(sweep marker check), internal/usage (per P1), cmd/metasystem
(acp turn + proof verbs), scripts/agents/dispatch.sh (launch
topology, sweep, deadline ordering, self-heal transport,
launch-failure split), scripts/agents/adapters/runtime-common.sh
(gate/register/seal sequence, descriptor hygiene, completion via
the verb, repair guard, single-transport emitter),
scripts/agents/adapters/devin.sh + fake (transport parameters),
scripts/validate-metasystem.sh (admission compare), records,
fixtures, docs. NOT touched: hosts, host recollection, D62
downstream owners, docs/architecture.md launch doctrine, and —
per D79 — every legacy record's terminal behavior.

## Loop discipline

Codex at xhigh; budget boundary reached at this round. History:
15, 8, 13, 13, 10 — r5 confirmed the launch placement conforms to
doctrine and forced the D79 scope pivot (gate to marked records;
split the fleet-wide holes to custody-death-proof). The r6
critique should attack: whether the marker gating is coherent end
to end (can every external writer distinguish records reliably;
does any path apply sealed rules to legacy records or vice versa);
whether the one-operation proof-commit verb's ancestry
authentication is implementable on both platforms; whether the
spawn-gate sequence closes the registration race the shipped
helper has; whether the four-state admission enum and its
certification matrix are consistent with the P1 allow-side
pipeline; the settlement evidence join against the shipped
refusals one more time; and whether anything in this document
still needs a fact only the wire can supply beyond the P1 list. At
this boundary the exits are: converged; falling-and-mechanical →
fixtures-as-arbiter; otherwise escalate to the human with the
critique history and D79 as the evidence.
