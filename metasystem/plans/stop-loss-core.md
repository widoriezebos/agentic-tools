# Stop-loss core: the fuse alone

Owner: main session (claude). Status: DESIGN — awaiting critique round 1.
Parent: `plans/stop-loss-last-defense.md` (critique-exhausted; split
approved by the human 2026-08-11). This core inherits its ruling — the
stop-loss is a last defense with high, human-set, mission-level caps and a
vocal-only reset — and ONLY the fuse. Turn identity, mission-scoped
reaping/drain, and orphan/usage capture are satellites
(`plans/stop-loss-satellites.md`), each to be designed against the
runner's actual cycle sequence. The parent's 41 accepted findings route:
core findings are resolved here; the rest carry to the satellites.

# Intent

One stop-loss trigger for missions, sized above any healthy runway, that
cannot be tripped by lawful phase structure, cannot be farmed by
oscillation, cannot be reset quietly, and lives where its state lives — in
the runner, over mission state — leaving `assert-stop-loss.sh` untouched
for every non-mission caller.

# Non-goals

- No closure-credit machinery (`loop-advanced` is deferred to a satellite;
  with last-defense-sized budgets and the decay rule below, a lawful
  design phase survives without credits, and the human reset covers the
  tail).
- No change for non-mission stop-loss users: investigate/improve keep
  `assert-stop-loss.sh` behavior and fixtures bit-for-bit.
- No new sealed contract keys. `ledger.no-gain-budget` keeps its type and
  meaning; only its calibration guidance and its enforcement location
  change.

# Design

## C1. The counter, ratcheted, with bounded decay

Mission state carries `stopLoss: {stagnant, bests, lastResetAskId}`.

- `bests` records, per declared gate metric, the best directed value
  observed so far (direction and noise floor come from the sealed
  contract, as today).
- NEW BEST (the only reset to zero): the measurement improves the
  lexicographic progress tuple — (count of declared thresholds met, then
  each declared metric's directed value in declaration order, each
  comparison gated by that metric's noise floor) — beyond the current
  `bests` tuple. Declaration order makes multi-metric comparison total
  and deterministic; thresholds-met dominates so progress toward the
  completion gate always outranks metric shuffling.
- RECOVERY (bounded decay): a cycle whose measurement improves over the
  PREVIOUS cycle's beyond noise but does not exceed `bests` decrements
  `stagnant` by one (floor zero); classification stays `unresolved`.
  Oscillation cannot farm this: each regress cycle increments before its
  recovery decrements, netting zero or worse while the wall-clock and
  cycle fences burn.
- STAGNATION: any other `no-progress` or `unresolved` cycle increments
  `stagnant`. The fuse fires at `ledger.no-gain-budget`, parking with the
  stop-loss ask, exactly as today.
- `contract-improved` keeps its ledger meaning (improvement over the
  previous measurement); the RESET condition is the new-best tuple, which
  the ledger line also records (`best=yes|no`) so the ledger alone still
  tells the whole story.

## C2. Ownership and scope

- The mission stop-loss decision moves INTO the runner: the counter and
  bests live in mission state, updated inside the same state write that
  concludes the cycle (one generation, hash chain intact — no sidecar).
- `assert-stop-loss.sh` is NOT modified. The runner simply stops
  delegating the mission decision to it; non-mission callers keep the
  script, its lifetime rule, and its fixtures unchanged. No scope switch,
  no shared-script hazard — the round-2/3 carriage problem dissolves by
  relocation instead of extension.
- Calibration: guidance (docs + contract template) says unattended
  missions size `ledger.no-gain-budget` in the order of `fence.cycles`;
  the contract validator warns — never refuses — below half the cycle
  fence, naming this design.

## C3. Vocal reset, by explicit human action only

- The stop-loss ask accepts an answer with the literal prefix `reset:`
  (the remainder is the human's reason): unpark and `stagnant := 0`. The
  sealed budget line is untouched — the human spends more of the
  still-sealed wall-clock and exposure fences, not a new allowance. Any
  other answer keeps today's guidance (amend, price, reseal, sign).
- Transaction: append the ledger line (`- Stop-loss reset: by human
  answer <askId>: <reason> at <time>`) under the ledger lock FIRST, then
  apply the state write (unpark, zero, `lastResetAskId := askId`).
  Recovery is idempotent by construction: on load, a reset line whose
  askId differs from `lastResetAskId` is applied forward; equal means
  already applied; a second line for the same askId is never appended
  because the ask is answered exactly once (answers are one-shot today).
  Repeated crashes at any point replay to the same state.
- No other reset path exists — no flag, no environment variable, no code
  path without a human answer. The flight-recorder event and ask record
  remain best-effort echoes of the authoritative ledger line.

# Invariants

1. For missions, exactly one stop-loss trigger exists: `stagnant`
   reaching the sealed budget, where `stagnant` is incremented by
   stagnation, decremented (floor 0) by above-noise recovery, unchanged
   by nothing else, and zeroed only by a new-best measurement or a
   recorded human reset.
2. The new-best tuple comparison is total, deterministic, and gated by
   the sealed per-metric noise floors.
3. Every reset has a ledger line that landed before its state change, and
   replaying any crash prefix reproduces the same state.
4. Non-mission stop-loss behavior is bit-for-bit unchanged.

# Failure behavior

- Measurement unavailable (`no-progress`/unmeasurable): increments, as
  today — fail toward the fuse.
- Ledger append fails during reset → the unpark does not happen; the
  mission stays parked, loudly.
- A state document without `stopLoss` fields (pre-migration) derives them
  by replaying its own ledger once, inside a normal state write.

# Tests

- Counter arithmetic: increment / decay floor / freeze-free semantics;
  oscillation nets non-negative; fuse at budget; new-best zeroes.
- Multi-metric tuple: thresholds-met dominance; declaration-order
  tie-break; noise-floor gating; single-metric missions unchanged.
- Reset: `reset:` answer unparks + zeroes + ledger-first ordering; append
  failure blocks unpark; crash replay idempotence via lastResetAskId;
  non-reset answer keeps the amendment guidance.
- Non-mission: assert-stop-loss.sh fixtures untouched and green.
- Migration: legacy state derives stopLoss by ledger replay; hash chain
  intact; existing ledgers parse.
- Validator: relative warning below half the cycle fence.

# Migration

Mission state gains the `stopLoss` object (derived on first load by ledger
replay inside a normal generation); the state shape validator admits it as
optional until fixtures migrate. One additive ledger annotation
(`best=yes|no` on measurement lines) and the reset line kind. No contract
grammar changes, no new sealed keys, no script changes.
