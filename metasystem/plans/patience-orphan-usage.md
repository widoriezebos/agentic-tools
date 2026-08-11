# Patience satellite 3: orphan and usage capture

Owner: main session (claude). Status: ACCEPTED FOR IMPLEMENTATION —
rounds 1-5 adjudicated (10, 5, 7, 6, 4 material, all accepted;
dispositions r1-r5 beside this plan). round 5's four findings were exact
wiring corrections (no ledger annotation on the unsafe failure ramp; the
unconsumed line is a named new annotation kind; a missing pgid can never
prove the writer dead; the row limit stated once) — the
diminishing-returns stop. Grounded on the verified fact sheet
(`plans/patience-orphan-usage-facts.md`, cited F Qn.m) per the
facts-before-design rule; this revision was regenerated whole after two
piecemeal edits drifted (the skill's generating-cause rule applied to
the document itself).
Program: `plans/stop-loss-satellites.md` satellite 3; concepts in
`docs/patience.md`. Routed findings: parent r1[11][12], r2[10][11][13],
r3[12][13]. Evidence: the bm-2s trials orphaned ~1.26M paid input
tokens of completed critic work, budget-capped jobs recorded
`usage: null`, and a second critic round's tokens never reached fence
aggregation.

# Intent

Nothing paid is silently discarded. A delegate return that landed is
value produced; tokens a killed process already spent are cost
incurred. Both must reach the ledgers that account for them — the
host's next prompt (so landed work is inherited, not lost) and the
fence usage (so spend accounting is complete) — mechanically, with one
writer per artifact, surviving every kill signal and every park.

# Non-goals

- No change to what counts as progress (satellite 4 decides how
  patience treats any of this).
- No second writer for any artifact: the adapter stays the only writer
  of its round's usage.json; reapers never write usage; the runner
  stays the only ledger and state writer.
- No new delegate-side protocol: only artifacts the harness already
  produces are read.
- No recorded surfacing state and no new ledger line kinds: the landed
  list derives from the tree and from records the host already authors.

# Design

## O1. The boundary is the host's own recorded action

The Landed Returns list derives at prompt assembly with no recorded
state of its own. "The host acted on this round" is testified by
artifacts the host already authors: every concluded turn's log carries
the accepted `dispatched` claims and the `certified` entries
({jobId, verdict, evidence}) copied verbatim from the host's return
(F Q1.28-29). A landed round qualifies for the list when:

- its `return.json` exists AND validates against the role schema
  (`validate return-complete`); a return that exists but fails
  validation lists as `invalid` rather than as ready — existence alone
  is deliberately weak (F Q1.22, round-3 POU-R3-002);
- its jobId appears in NO concluded turn's certified entries and no
  concluded turn's accepted dispatch claims name a successor round of
  its chain — the two host-authored "I acted" records;
- the mission owns the chain (records stamped or reserved, F Q1.20).

CHAIN CLOSURE DOES NOT EXCLUDE: the runner's post-loop close fires at
every park (F Q1.11), so a closure exclusion would drop a landed return
at the very park that orphaned it (POU-R3-001 — the recreated bm-2s
loss). A chain the runner closed while its last round was never
certified keeps its row until the host certifies or supersedes it.
Gaming is self-harm only: a false certification filters the host's own
reminder list and never touches a fuse.

Row bound and order (POU-R4-003, POU-R5-004): rows sort by (chain root,
then round), both ascending — deterministic under any tree. At most one
row per chain (the latest qualifying round). The limit is stated once,
unambiguously: when 20 or fewer chains qualify, every qualifying row is
emitted; when MORE than 20 qualify, exactly 19 data rows (the first 19
in sort order) plus one final `overflow  <count-of-remaining>  none`
row are emitted, 20 lines total. There is no other reading.

TERMINAL DELIVERY (POU-R4-001, POU-R5-001, POU-R5-002): a mission that
completes or that the runner FAILS after initialization produces no
next prompt, so the list must land where a terminal mission is read —
the ledger. Delivery is therefore gated on the ledger being SAFELY
WRITABLE, which is exactly and only true after a cycle has concluded:
at the COMPLETION conclude (a ledger block for the final cycle exists),
the derived list is appended to that block as `- Landed unconsumed:`
lines. On the FAILURE ramp there is NO safe total position — the ramp
fires for lease-acquisition failure (no mission owned), pre-cycle
init/resume errors (no cycle block, possibly a foreign or half-written
ledger), and mid-run errors alike — so terminal delivery does NOT run
there; the failure ramp does only what round 4 correctly required of
it (the O4 usage aggregation, which is idempotent and needs no ledger
block) and nothing more. An interrupted mission's landed returns are
recovered on RESUME (its next assembled prompt lists them) or, if never
resumed, remain in the tree for a human — not lost, merely not
annotated into a ledger the runner cannot safely touch. Grammar
(POU-R5-002): `- Landed unconsumed:` is a NEW cycle-block annotation
line kind added alongside the shipped `- Drain: stalled:` and
`- Return:`/`- Outcome:` annotations (satellites 1-2) — the annotation
suite and every cycle-block parser gain it explicitly in the same
change; it is never a classification line and never a fuse input.

## O2. Delivery is a validated seventh prompt section

`## Landed Returns` is added as a records section between
Reconciliation and This Turn (F Q5.3-4: the assembler emits exactly
four records sections and the validator recognizes exactly six headings
in fixed order — both counts move by one, together, with every
six-heading fixture updated in the same change). The shipped records
grammar applies verbatim (F Q5.1-2: exact heading, `<<<DATA>>>`,
tab-joined records, `<<<END>>>`, `(none)` when empty; sanitizing
already defangs fences and CR/LF/tab). Rows are three non-empty tab
fields: `chain-root  round-or-marker  return-path-or-none`; the
`unreadable`, `invalid`, and `overflow` markers use the same form
(F Q5.20: syntax-compatible — `none` is not the `(none)` sentinel; no
per-section row maximum exists, F Q5.18). The prompt is archived per
turn, so surfacing is durable evidence. Parks need nothing else: the
first post-resume assembly carries the list.

## O3. Usage derivation, gated on proven group death

- The adapter owns `rounds/<n>/usage.json` (F Q4.10-11), written
  atomically; reapers never write usage. NO graceful cleanup survives
  a cap or reap (the group TERM includes the adapter, F Q3.2), so a
  capped or killed job's record carries `usage: null` — the bm-2s gap.
  Derivation is therefore the PRIMARY recovery, not a corner case.
- Recovery is DERIVATION by the aggregator, never a second writer, and
  it gates on PROVEN GROUP DEATH, not record status: a record can be
  terminal while its group lingers (satellite 2's reap applies verdicts
  by CAS without wind-down — POU-R3-003), so terminality alone does not
  silence the writer. Derivation runs only when the record is terminal
  AND the WHOLE GROUP is provably gone (POU-R4-002 — named custodians
  do not cover every group member): the recorded `pgid` probes ESRCH
  (the shipped group probe; a permission denial proves existence and
  BLOCKS derivation) AND every recorded custodian is dead by the
  shared kernel custodian proof (internal/identity/custodian.go — the
  one owner both reapers already use). Vintage rule, corrected
  (POU-R5-003): a missing pgid does NOT license derivation from
  custodian death alone — the custodian set may not include the process
  that writes events.jsonl, so proving those custodians dead does not
  prove the writer dead. Derivation requires a recorded pgid that
  probes ESRCH; a record with NO recorded pgid can never satisfy the
  whole-group gate and always aggregates `unavailable` (its usage is
  unrecoverable by proof — honesty over optimism). Then `events.jsonl`
  (F Q3.1)
  has no writer, the two-reads race (F Q3.10) cannot occur, and the
  shipped tolerant JSONL parse (F Q3.6) plus the `CodexUsage`
  last-usage-block rule (F Q3.7) derive the usage in memory. Never
  written back.
- A terminal record whose group is not yet provably dead aggregates
  `pending-death-proof` this pass and derives on a later pass —
  aggregation recomputes at every call site, so the value arrives as
  soon as the proof does. A terminal job with neither usage.json nor a
  parseable stream aggregates `unavailable`, honestly (note F Q3.8: the
  shipped parser's native-with-nulls quirk on unreadable input is
  normalized to `unavailable` by the aggregator's own check).
- Provenance schema, fixed (POU-R3-007): mission `usage.json` gains an
  additive top-level `rounds` array — the one location with zero
  consumers to change (F Q4.19; `ProjectFences` reads only `units`,
  F Q4.4) — sorted by (jobId, round), each entry exactly
  `{jobId, round, provenance, source, detail}` (POU-R4-006), provenance
  one of `reported`, `derived`, `pending-death-proof`, `unavailable`;
  `source` names the event file for derived, else null; `detail`
  carries the parse or proof failure text for unavailable and
  pending-death-proof entries, else null. Nothing enters
  `state.fences.usage` (exact-key strict, F Q4.2-3,20).

## O4. Every terminal round reaches the fence ledger

There is NO aggregation in the runner today — not at conclusions, not
at parks, not on the failure exit (F Q2.9-11,21); it lives only in the
dispatch reap paths and the CLI verb. `ProjectFences` merely copies an
existing usage.json into state (F Q2.11, Q4.4). The fix:

- One `AggregateUsage` call ordered before each existing
  `ProjectFences` call in the conclude and park paths, AND one on the
  runner's failure exit ramp before the lease release (F Q2.21) — a
  mission that dies of a runner error must not also lose its
  accounting.
- Locking: `AggregateUsage` takes its own `mission-fence.lock`
  (F Q2.2), disjoint from `state.json.lock` (F Q2.12) — no ordering
  conflict. The dispatch reap aggregation writes the same usage.json
  from the same inputs under the same lock: double execution is
  last-writer-identical, never double-counting.
- Failure behavior (POU-R3-005, POU-R4-004): an aggregation error at
  any added call emits the NEW registry event kind
  `aggregation-failed` with fields {mission, site, error} — an
  additive entry in the flight-recorder registry, named here because
  no shipped kind fits — and does NOT fail the park, the conclusion,
  or the exit; the projection reads the older usage.json until the
  next successful call catches up.
- Idempotence and `updatedAt` (POU-R4-005): the aggregate write is
  SKIPPED when the computed content (units, unavailableJobs, rounds)
  equals the existing file's content — `updatedAt` then keeps its old
  value and the file is byte-identical. `updatedAt` changes exactly
  when content changes.

# Invariants

1. A landed round appears in the Landed Returns section iff its return
   validates (or lists as invalid/unreadable), no concluded turn's
   certified or successor-dispatch claims name it, and the mission owns
   its chain; it retires only by the host's own recorded action. The
   list is a pure function of the tree and the turn logs, capped at 20
   rows plus one overflow row.
2. usage.json has exactly one writer (the adapter); derived usage is
   never written back; provenance always distinguishes reported,
   derived, pending-death-proof, and unavailable.
3. Derivation reads an event stream only after the record is terminal
   AND its group is provably dead by the shared custodian proof.
4. `state.fences.usage` shape and its consumer are unchanged;
   aggregation is a pure read, idempotent, and its failure never fails
   a park, a conclusion, or an exit.
5. Neither the prompt section nor aggregation timing ever changes a
   stop-loss verdict.

# Failure behavior

- Return validates false → listed as `invalid`; unreadable chain dir →
  `unreadable` row; both are still surfaced — silence is the failure
  mode this satellite closes.
- Event-stream parse failure → `unavailable` with the failure in the
  aggregation provenance; never a hard failure.
- Aggregation failure at any added call → flight-recorder event, older
  projection stands, next successful call catches up.
- Group death unprovable but a pgid IS recorded (still alive/EPERM) →
  `pending-death-proof`, retried every pass; NO recorded pgid →
  `unavailable` permanently (unprovable by design).

# Tests

- O1: qualification matrix (validates / invalid / unreadable; certified
  vs uncertified; successor dispatched vs none; runner-closed chain
  with uncertified round KEEPS its row); two chains with equal round
  numbers; the 19-data-plus-overflow boundary at exactly 21 qualifying
  chains; the list across parks and resumes including drain-stalled;
  terminal delivery: at COMPLETION the `- Landed unconsumed:` lines
  land in the final block; on a runner-FAILURE exit NO ledger write is
  attempted and resume re-lists.
- O2: seven-heading assembler and validator (fixtures moved); `(none)`
  empty case; the three-field rows and all three markers pass strict
  validation.
- O3: adapter usage wins; derivation only after proven death (live
  group → pending-death-proof; killed group → derived matching
  CodexUsage's own parse; truncated final line tolerated); native-
  with-nulls normalizes to unavailable; provenance array sorted and
  exact.
- O4: aggregation before ProjectFences at each conclude/park and on
  the failure ramp; aggregation failure does not fail the park; double
  aggregation byte-identical; the bm-2s-shaped capped-round fixture
  reaches units with derived provenance.
- Invariant 5: stop-loss replay unchanged by the section and by
  aggregation timing.

# Migration

No state fields, no ledger line kinds, no return-schema changes, no
change to `state.fences.usage`. One new prompt records section
(assembler four→five, validator six→seven, fixtures moved together;
the orchestrator preamble documents the section and the retire-by-
certification rule), aggregation calls added at the named runner
points, one additive `rounds` array in mission usage.json. Old
missions need nothing: their landed rounds derive into the next prompt
and their terminal usage aggregates at the next conclude/park.
