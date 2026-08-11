# Patience satellite 3: orphan and usage capture

Owner: main session (claude). Status: DESIGN — rounds 1-2 adjudicated;
re-grounded on the verified fact sheet
(`plans/patience-orphan-usage-facts.md`, cited below as F Qn.m) per the
facts-before-design rule. Round 3 pending. The recorded-state spine
(surfacedRounds, orphan ledger lines, scans) is gone; landed returns
DERIVE from the tree, and the derivation's retention boundary is fixed
to what the code actually provides.
Program: `plans/stop-loss-satellites.md` satellite 3; concepts in
`docs/patience.md`; ground truth in `docs/design/mission-cycle-sequence.md`
(NOTE its superseded-sections header — cross-check the shipped satellite 1
and 2 designs for the turn conclusion and park paths as they are NOW).
Routed findings: parent r1[11][12], r2[10][11][13], r3[12][13]. Evidence:
the bm-2s trials orphaned ~1.26M paid input tokens of completed critic
work (one return landed three seconds before a park and was never read),
budget-capped jobs recorded `usage: null`, and a second critic round's
tokens never reached fence aggregation.

# Intent

Nothing paid is silently discarded. A delegate return that landed is
value produced; tokens a killed process already spent are cost incurred.
Both must reach the ledgers that account for them — the mission's story
(so the next turn inherits landed work instead of losing it) and the
fence usage (so spend accounting is complete) — mechanically, with one
writer per artifact, surviving every kill signal the runtime can receive.

# Non-goals

- No change to what counts as progress (satellite 4 decides how patience
  treats any of this).
- No second writer for any artifact: the adapter stays the only writer
  of its round's usage.json; reapers never write usage; the runner stays
  the only ledger writer.
- No new delegate-side protocol: only artifacts the harness already
  produces (return.json, the runtime's on-disk event stream) are read.

# Design

## O1. Landed rounds derive at prompt assembly; the boundary is succession

No surfaced-rounds state, no orphan ledger line, no scan. At every
prompt assembly the runner derives the LANDED-BUT-UNACTED list from the
tree alone and adds one prompt section, `## Landed Returns`.

Retention boundary — NOT chain closure (F Q1.2-4,11-13: chains stay
unclosed at every park and a mission can run all its cycles closing
nothing, so closure would strand returns at parks and accumulate them
forever). The boundary is SUCCESSION, purely derivable:
- A landed round qualifies when its `return.json` exists (F Q1.22:
  written atomically before validation, so it can sit beside a
  non-terminal or failed record), its chain is not `chainClosed`
  (F Q1.24), AND no successor round was dispatched — i.e. no
  `<root>/rounds/<n+1>/` record exists for that chain (F Q1.21: rounds
  are `<root>/rounds/<round>`, follow-ups carry the predecessor).
- The list therefore contributes AT MOST the single latest unacted
  round per open chain, and a round DROPS OFF the instant the host acts
  on it — dispatching a follow-up (a successor round appears) or
  closing the chain. No accumulation, no park loss: a return that
  landed one second before a park appears in the first post-resume
  prompt and stays until the host acts.
No consumption ledger is invented (F Q1.10: none exists); succession is
the honest, existing trace of "the host acted on this round".

## O2. Delivery is a validated seventh prompt section

`## Landed Returns` is added as a records section between Reconciliation
and This Turn (F Q5.3-4: the assembler emits exactly four records
sections and the validator recognizes exactly six headings in fixed
order — both counts move by one, together). It uses the shipped records
grammar verbatim (F Q5.1-2: heading, `<<<DATA>>>`, tab-joined records,
`<<<END>>>`, `(none)` when empty; field sanitizing already defangs
fences and CR/LF/tab). Fields: `chain-root`, `round`, `return-path`
(three non-empty tab fields; F Q5.10-11 declares per-section field
counts — this section declares three). An unreadable chain lists as
`chain-root  unreadable  none` (F Q5.20: syntax-compatible — all fields
non-empty, `none` is not the `(none)` sentinel). No per-record maximum
exists (F Q5.18), so every qualifying row is emitted. Both the assembler
(prompt.go) and the strict validator (turnprompt.go) gain the section;
the six-heading fixtures move to seven in the same change. The prompt is
archived per turn, so the surfacing is durable evidence. Parks need
nothing else: the next assembled prompt after any resume carries the
list.

## O3. Usage: one writer, and a derivation path that survives SIGKILL

- The adapter owns its round directory's usage.json, written
  atomically, and reapers never write usage. NO graceful-termination
  cleanup is assumed: a cap or reap TERMs the whole process group,
  adapter included, and today's adapters write nothing on that path —
  so derivation (below) is the PRIMARY recovery for every kill, not a
  SIGKILL corner case.
- Under an uncatchable kill the adapter writes nothing — recovery is
  DERIVATION, not a second writer: where the runtime leaves an on-disk
  event stream in the round directory (the codex JSONL event file
  survives its process), the fence aggregator derives the usage IN
  MEMORY at aggregation time, marks the result's provenance
  (`derived: event-stream`), and never writes it back — the derivation
  is a pure function of the surviving file, recomputed by any
  aggregation that needs it. A runtime with no surviving stream
  aggregates as `availability: unavailable`, honestly, exactly as the
  shipped unavailable shape.

## O4. Every terminal round reaches the fence ledger, including at parks

There is NO conclude-time or park-time aggregation in the runner today
(F Q2.9-10,14-19: aggregation lives only in `dispatch.sh reap_one_locked`
and the CLI verb; the runner's conclude and park paths call
`ProjectFences`, which merely COPIES an existing `usage.json` into
state — F Q2.11, Q4.4). So the gap is real: a terminal job whose reap
did not aggregate (the runner's own satellite-2 reap applies verdicts
by CAS without the dispatch aggregation) never reaches the mission
usage until a later dispatch reap happens to run.

The fix is one added aggregation call, ordered before the existing
`ProjectFences` in each park and conclude path: run `AggregateUsage`
(its own `mission-fence.lock`, F Q2.2 — disjoint from the park's
`state.json.lock`, F Q2.12, so no lock-ordering conflict) so the
usage.json ProjectFences then reads is current. It stays a pure read
over terminal round records plus the O3 derivation; running it twice is
identical. A round terminal only after a park aggregates at the first
park/conclude after it terminalizes.

# Invariants

1. A landed round appears in the Landed Returns section iff its chain
   is open, its return.json exists, and no successor round was
   dispatched; it drops off the instant a successor appears or the
   chain closes. The list is a pure function of the tree at assembly,
   bounded to at most one row per open chain.
2. usage.json has exactly one writer (the adapter); derived usage is
   never written back; the aggregation provenance always distinguishes
   reported, derived, and unavailable per round.
3. The mission-state usage projection (`state.fences.usage`) shape and
   its only consumer (`ProjectFences`) are unchanged; provenance lives
   only in the mission usage.json aggregate detail. Aggregation is a
   pure read over terminal round records, idempotent at every call site.
4. Neither the Landed Returns section nor aggregation timing ever
   changes a stop-loss verdict.

# Failure behavior

- Derivation of the landed list fails for one chain (unreadable dir):
  the section names the chain as unreadable rather than omitting it —
  losing it silently is the failure mode this satellite closes.
- Event-stream derivation fails (unparseable, truncated): that round
  aggregates as unavailable with the failure in the aggregation
  provenance; never a hard failure of the cycle.
- An unreadable return.json still lists (it landed); the host decides
  what to do with a corrupt artifact.

# Tests

- O1 succession boundary: latest unacted round listed; a dispatched
  successor drops it; a closed chain drops it; an unreadable chain
  lists as the sentinel row; two chains with equal round numbers do not
  collide; a return landing between two assemblies appears in the
  second.
- O2 grammar: the seven-heading assembler and validator accept the new
  section (move every six-heading fixture to seven); `(none)` when the
  list is empty; the three-field rows and the unreadable sentinel pass
  strict validation; a non-conforming row fails exactly as other
  sections do.
- O3: adapter usage.json wins when present; codex events.jsonl
  derivation for a terminal job matches CodexUsage's own parse
  (truncated-final-line tolerated); a terminal job with neither is
  unavailable; the `rounds` provenance names reported/derived/
  unavailable; ProjectFences still reads only `units`.
- O4: aggregation runs before ProjectFences at each park and conclude
  under mission-fence.lock; a terminal-after-park round aggregates at
  the next park/conclude; double aggregation identical; the
  bm-2s-shaped capped-round fixture reaches the projection with derived
  provenance.
- Invariant 4: stop-loss replay verdicts unchanged by the new prompt
  section and by aggregation timing.

# Migration

No state fields, no ledger line kinds, no return-schema changes, no
change to `state.fences.usage`. One new prompt records section (the
assembler's four→five and the validator's six→seven heading counts move
together, fixtures updated in the same change; the orchestrator
preamble documents the section), one added `AggregateUsage` call before
ProjectFences in the park/conclude paths, one additive `rounds`
collection in the mission `usage.json` aggregate. Old missions need
nothing — their landed rounds derive into the next prompt and their
terminal usage aggregates on the next park/conclude.
