# Stop-loss as last defense

Owner: main session (claude). Status: DESIGN — awaiting critique round 1.
Ruling: human, 2026-08-11 — "the mechanism that killed the loop should be a
last-defense kind of thing … really high caps set at the mission level …
resetting should never be quiet." Evidence: the bm-2s trial cohorts
(`benchmark/results/cohorts/bm-2s-20260810t*.json` and their targets'
ledgers), where a lawful, converging design-critique loop was killed by the
no-gain budget after 3 cycles with 82% of wall clock unused.

# Intent

The mission ledger's no-gain stop-loss exists to catch pathological
non-convergence that no inner mechanism anticipated — an endless loop that
materialized unexpectedly. It is NOT a pacing device. Today it fires during
provably correct operation because its only progress sensor is the product
gate, which is blind to every phase of work before code lands. The governed
inner loops (design critique rounds with mechanical closure and an
exhaustion budget) already own "is this phase stuck"; the stop-loss must own
only "is the mission stuck in a way nothing else can see."

Three changes, plus two contract-touching harness fixes the same evidence
demands:

# Non-goals

- No change to the design-critique loop itself (battle-tested; its closure
  rule and round budget stay exactly as shipped).
- No change to the money fences: wall clock, EUR exposure, cycles, jobs
  remain the hard spend guards and are untouched.
- No ambient "attended mode": a human's presence never silently alters
  mechanism behavior; only explicit, recorded human actions do.
- The benchmark kit's gate redesign (count metric, requirement-5 ruling,
  role-specific caps) is companion work in the kit, decided by the human
  separately; this note covers the metasystem only.

# Design

## D1. Jurisdiction-aware gain: `loop-advanced` classification

A cycle whose turn mechanically advances a governed inner loop classifies as
`loop-advanced` in the ledger — distinct from `contract-improved` (gate
metric moved) and from `no-progress`/`unresolved`. Round 1:

- The only recognized advancement is: a design-critique round reached
  mechanical closure this cycle — `assert-critique-closed.sh` exit 0 over
  the round's findings and dispositions, the same proof the loop itself
  uses. Nothing prose-based qualifies.
- `loop-advanced` cycles do NOT increment the trailing no-gain counter.
  They also do not reset it: only `contract-improved` resets. Rationale: a
  mission alternating closure and stagnation forever must still trend
  toward the fuse; not counting is enough to stop punishing lawful phases,
  while resetting would let governance loops launder stagnation.
- Unbounded-loop safety: critique rounds are numbered and capped by the
  existing exhaustion machinery, so `loop-advanced` can occur at most
  (round budget) times per stream. The fuse cannot be starved indefinitely.

## D2. The no-gain budget becomes a last-defense cap

- Semantics unchanged mechanically (trailing non-improving cycles reach the
  budget → park with the stop-loss ask), but the counted set shrinks per D1
  and the calibration guidance inverts: the budget is sized ABOVE any
  healthy runway, not at the expected pace. Contract guidance (docs +
  template): unattended missions set `ledger.no-gain-budget` in the same
  order as `fence.cycles` (e.g. cycles-2), never below the sum of the
  mission's mandated pre-build stages plus margin.
- The budget stays a sealed, human-signed contract key — "set at mission
  level by the human" is already the instrument; no new mechanism.
- validate: the contract validator warns (not refuses) when
  no-gain-budget < 5, naming this design; refusing would break existing
  fixtures and the human may knowingly run tight experiments.

## D3. Vocal reset, by explicit human action only

- Answering the stop-loss ask (the existing unpark path) now also RESETS
  the trailing no-gain counter, and the reset is triple-recorded: a ledger
  line (`- Stop-loss reset: by human answer <askId> at <time>`), a
  flight-recorder event (`stop-loss-reset`), and the ask/answer record
  itself. A reset that leaves no durable trace must be impossible; there is
  no flag, environment variable, or config knob that resets quietly.
- No other reset paths. Attended operation is expressed by the human
  actually answering; presence without action changes nothing.

## D4. Turn identity: honesty must not be punishable

Evidence: bm-2s rep 3 cycle 1 — the prompt announced `Host-Session: none`
(hostSession null), the host truthfully reported its real session id, and
`adjudicate.go` discarded a fully productive turn (design committed, critic
dispatched) as "unmeasurable", debiting the no-gain counter.

- The runner stamps `turn.hostSession` from the launch handshake's
  session-established signal when one exists, BEFORE adjudication, so the
  expected value is the real session whenever the harness knows it.
- The return schema and the turn prompt document the rule: `identity.
  sessionId` must echo the prompt's `Host-Session` header, `null` when the
  header says `none`.
- A session-identity mismatch on an otherwise schema-valid return
  classifies the turn `invalid-run`: the turn's return is not applied, BUT
  the cycle still runs measurement over the committed tree, the ledger
  entry names the mismatch, and the cycle does not increment the no-gain
  counter (the harness's own confusion is never the mission's stagnation).

## D5. The runner reaps its own dead reservations

Evidence: bm-2s rep 2 — five reservation records stuck non-terminal froze
dispatch (concurrency 2), and the janitor's sweep refused for lack of
supervision custody while the mission starved.

- The mission runner, as the live holder of the mission lease, gains
  standing to reap records that (a) its own mission's fence reservations
  name, and (b) are provably dead by the existing verdict facts (abandoned
  setup, proven process loss). Implementation reuses `dispatch reap-facts`
  + the record CAS; no new verdict logic. Custody rule: lease-holdership of
  the mission IS custody over the mission's own reservations — narrower
  than the janitor's machine-wide authority, so no new global power.

## D6. Nothing paid is silently discarded

Evidence: two completed critic returns (~1.26M paid input tokens) landed
seconds before parks and were never read; budget-capped jobs record
`usage: null`; a delegate round's usage never rolled into fence usage.

- At park, the runner drains landed-but-unadjudicated returns into the
  ledger as `orphaned-return` entries naming the artifact paths, so the
  resurrection turn starts from them instead of losing them.
- Budget-capped and failed jobs record whatever usage their runtime
  reported before death; the fence aggregator includes every round.

# Invariants

1. The stop-loss can only fire after `no-gain-budget` trailing cycles in
   which no gate improvement AND no mechanical inner-loop closure occurred.
2. Every stop-loss reset is attributable to a named human action and leaves
   ledger, event, and ask records; there is no quiet path.
3. `loop-advanced` is grantable only by mechanical proof (the loop's own
   closure checker), never by orchestrator assertion.
4. A turn discarded for harness-side identity confusion never debits the
   mission's no-gain counter and never skips measurement.
5. The runner's reap authority never exceeds its own mission's reservation
   set; machine-wide reaping remains the janitor's.

# Failure behavior

- If the closure proof errors (checker crash, unreadable dispositions), the
  cycle classifies as it would today (no `loop-advanced`) — fail toward the
  fuse, never away from it.
- If the reset's ledger append fails, the unpark itself fails loudly; the
  mission stays parked rather than resetting quietly.
- If reap-facts cannot prove death, the runner leaves the record alone and
  the fence stays consumed — same conservative posture as every reaper.

# Tests

- missionrunner: `loop-advanced` classified only with a proven closure this
  cycle; counter arithmetic (not-counted vs not-reset); fuse still fires
  through interleaved closures once rounds exhaust.
- missionrunner: stop-loss answer resets counter + writes all three records;
  append failure blocks the unpark.
- missionrunner: identity mismatch → `invalid-run`, measurement still runs,
  counter not debited; hostSession stamped from the handshake when present.
- missionrunner/dispatch: runner reaps its own mission's provably dead
  reservation, refuses a foreign mission's and an unprovable one.
- ledger fixtures: orphaned returns drained at park with paths named.
- contract validator: warning below the guidance floor.

# Migration

On-disk: one new ledger classification string and one new ledger line kind —
both additive; existing ledgers parse unchanged. The turn.json field set is
unchanged (hostSession merely gets a real value earlier). No config or
contract grammar changes; `ledger.no-gain-budget` keeps its type and
meaning. The bm-2s spec's own fences change separately in the kit under the
human's fence-approval rule.
