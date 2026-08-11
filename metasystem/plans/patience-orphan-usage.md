# Patience satellite 3: orphan and usage capture

Owner: main session (claude). Status: DESIGN — awaiting critique round 1.
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

## O1. Orphanhood is mechanical: the surfaced-rounds set

Mission state gains `surfacedRounds`: a set of (chain root, round) pairs
whose landing the mission's story has accounted for. A pair enters the
set exactly two ways:
- IMPLICITLY: its return.json landed before a turn's prompt assembly —
  that turn's host had the artifact available (reading it is the host's
  duty and the prompt names the chain); the conclusion of that turn (any
  outcome) marks it surfaced.
- EXPLICITLY: the orphan scan (O2) wrote its ledger line.
A landed return.json whose pair is not in the set is ORPHANED — a set
membership test, no prose judgment, idempotent by construction (the set
makes every scan exactly-once). Rounds are chain-local; the scalar
high-water marks the parent design once proposed are wrong and are not
used (parent r2[10]).

## O2. One scan, at every exit of a cycle

A single scan function runs at every point a cycle's story is written:
turn conclusion (accepted and faulted — satellite 1's path included) and
every park (stop-loss, drain-stalled, host-failure, all-streams-parked).
For each orphaned pair it appends one ledger line —
`- Orphaned return: chain=<root> round=<n> path=<return.json path>` —
inside the concluding cycle's block when one is being written, or as a
standalone annotation before the park otherwise (grammar per the shipped
annotation rules: never a classification line, tolerated by every
parser, pinned by the annotation suite). The pair enters `surfacedRounds`
in the same state write that carries the scan's cycle. The next turn's
prior context already carries the ledger tail, so the resurrection or
successor turn starts from the orphan lines without new prompt
machinery. The drain-stalled park (satellite 2) is one of the park
scan points: a stalled turn's landed returns are surfaced before the
mission sleeps.

## O3. Usage: one writer, and a derivation path that survives SIGKILL

- The adapter owns its round directory's usage.json, written atomically;
  on a graceful termination (TERM from a cap or reap) its existing
  cleanup writes what the runtime reported before death. Reapers never
  write usage. Unchanged, restated as the invariant it is.
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
today PLUS at every O2 scan point, so a round that lands late (the
bm-2s second critic round) is aggregated by the cycle that surfaces it
rather than never. Aggregation stays read-only over round artifacts;
its output stays the single mission-state usage projection it is today.

# Invariants

1. Every landed return.json is eventually either implicitly surfaced or
   carries exactly one orphan ledger line; no third fate exists.
2. `surfacedRounds` only grows, and each pair's ledger line is written
   at most once (set membership guards the append).
3. usage.json has exactly one writer (the adapter); derived usage is
   never written back; provenance always distinguishes reported,
   derived, and unavailable.
4. Fence aggregation is a pure read over round artifacts; running it
   twice at any point yields the same projection.
5. Orphan lines and usage derivation never change a stop-loss verdict
   (annotations and aggregation are not fuse inputs — the shipped
   replay invariant extends over the new line kind).

# Failure behavior

- Scan crash after the ledger append, before the state write: the pair
  is not yet in the set; the next scan re-tests and — finding the line
  already present for that pair (the append is guarded by a read of the
  cycle block, not blind) — adds the pair to the set without a second
  line. Stated as the one crash window and pinned by a test.
- Event-stream derivation fails (unparseable, truncated): that round
  aggregates as unavailable with the parse failure recorded in the
  aggregation provenance; never a hard failure of the cycle.
- A return.json that is unreadable JSON is still a LANDED artifact: it
  is surfaced with its orphan line noting unreadability — losing it
  silently is the failure mode this satellite exists to close.

# Tests

- O1: implicit surfacing at each turn outcome; explicit surfacing at
  each park kind including drain-stalled; chain-local rounds (two
  chains, same round numbers) never collide; set-guarded exactly-once
  lines across repeated scans and the crash window.
- O2: the orphan line parses everywhere (extend the pinned annotation
  suite); the ledger-tail prompt carries it to the next turn.
- O3: adapter-written usage wins when present; codex event-stream
  derivation matches the adapter's own parse of the same file;
  provenance marks derived and unavailable; SIGKILL fixture (kill -9
  a stub runtime, derive from its stream).
- O4: a late-landing round aggregates at the surfacing scan; running
  aggregation twice is identical; state usage covers every round of
  the bm-2s-shaped two-round fixture.
- Invariant 5: replay verdicts unchanged by orphan lines and by
  aggregation timing (extend the stop-loss annotation test).

# Migration

One state field (`surfacedRounds`, optional in the shape validator until
fixtures migrate; absent means empty), one annotation line kind, one
provenance field on the aggregated usage projection. No schema changes
to returns, no adapter protocol changes, no new writers.
