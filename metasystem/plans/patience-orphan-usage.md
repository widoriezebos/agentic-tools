# Patience satellite 3: orphan and usage capture

Owner: main session (claude). Status: DESIGN — round 1 adjudicated
(10/10 accepted, `plans/patience-orphan-usage-dispositions-r1.md`),
awaiting round 2. Round 1 killed the design's spine — implicit
surfacing over-marked, and the ledger tail never carries annotations to
a prompt — so the amendment applies the sever pattern proactively: the
recorded-state machinery (surfacedRounds, orphan ledger lines, scan
points) is REPLACED by derivation at prompt time. Six of the ten
findings are resolved by that removal.
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

## O1. Landed rounds are DERIVED at prompt assembly, never recorded

There is no surfaced-rounds state, no orphan ledger line, and no scan.
At every prompt assembly the runner derives, from the artifacts alone,
the list of landed delegate rounds belonging to the mission's chains
whose chain is not closed — (chain root, round, return path), read from
the shipped round-directory layout — and the prompt gains one section:
`## Landed Returns`, listing them. The host adjudicates what it has not
yet adjudicated (its own dispositions are on disk and it knows them);
a listed round the host already consumed is harmless noise, bounded by
the chain-close lifecycle (closed chains leave the list). Nothing can
be permanently mis-marked because nothing is marked: the list is a pure
function of the tree at assembly time, recomputed every turn, and a
return that lands one second after assembly appears in the next
prompt's list. The prompt is archived per turn (prompt.md), so the
surfacing is durable evidence without a new ledger line kind. The
turn-prompt validator accepts the new section (a named, fenced records
section like the others; the strict parsers gain it explicitly).

## O2. Parks need nothing special

A park orphans nothing anymore: whatever landed is derived into the
next assembled prompt whenever the mission resumes — including after
a drain-stalled park (satellite 2), whose stalled turn's landed
returns appear in the first post-resume prompt. There is no scan, no
crash window, no exactly-once bookkeeping, and no migration: an old
mission's landed rounds simply appear in its next prompt's list, which
is correct behavior, not spam — the list states what exists, not what
was missed.

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

## O4. Every round reaches the fence ledger

Fence usage aggregation runs over ALL of a mission's round directories
— every job, every round, terminal or not — at every point it runs
today PLUS once in each park path, under the same mission-fence lock
and ordering the conclude-time aggregation already uses (one added
call site per park proposal application, not a new locking regime). A
round that lands after the final aggregation of a parked mission is
aggregated by the first aggregation after resume. Aggregation stays
read-only over round artifacts; its output stays the single
mission-state usage projection it is today.

# Invariants

1. Every landed return of an unclosed chain appears in every
   subsequently assembled prompt's Landed Returns section until its
   chain closes; the list is a pure function of the tree at assembly.
2. usage.json has exactly one writer (the adapter); derived usage is
   never written back; the aggregation provenance always distinguishes
   reported, derived, and unavailable per round.
3. The mission-state usage projection's shape and consumers are
   unchanged; aggregation is a pure read, idempotent at every call
   site.
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

- O1: the derived list across turn outcomes, chain closure, follow-up
  chains, two chains with equal round numbers, and a return landing
  between two assemblies; the prompt section parses (extend the
  turn-prompt validator tests); an old mission's first post-upgrade
  prompt lists its landed rounds.
- O2: post-drain-stalled resume lists the stalled turn's returns.
- O3: adapter-written usage wins; codex event-stream derivation matches
  the adapter's own parse; TERM'd-group fixture (no usage.json) derives;
  no-stream runtime aggregates unavailable; provenance detail complete.
- O4: park-path aggregation under the fence lock; double aggregation
  identical; the bm-2s-shaped two-round fixture reaches the projection.
- Invariant 4: replay verdicts unchanged (extend the stop-loss
  annotation test to the new section and aggregation timing).

# Migration

No state fields, no ledger line kinds, no return-schema changes. One
prompt section (validator extended; hosts see a new named section,
documented in the orchestrator preamble), one aggregation call site per
park path, one provenance detail in the aggregation output. Old
missions need nothing: their landed rounds derive into their next
prompt like anyone else's.
