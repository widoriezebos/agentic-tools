# Stop-loss core: the fuse alone

Owner: main session (claude). Status: DESIGN — round 1 adjudicated (9/9
accepted, `plans/stop-loss-core-dispositions-r1.md`), awaiting round 2.
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
oscillation, cannot be reset quietly, and is derived rather than stored —
a pure replay of the ledger inside the runner — leaving
`assert-stop-loss.sh` untouched for every non-mission caller.

# Non-goals

- No closure-credit machinery and no decay rule (`loop-advanced` is
  deferred to a satellite; round 1 refuted decay as farmable): with
  last-defense-sized budgets, a lawful design phase survives on budget
  alone, and the human reset covers reverts and tails.
- No change for non-mission stop-loss users: investigate/improve keep
  `assert-stop-loss.sh` behavior and fixtures bit-for-bit.
- No new sealed contract keys. `ledger.no-gain-budget` keeps its type and
  meaning; only its calibration guidance and its enforcement location
  change.

# Design

## C1. The fuse is a pure function of the ledger

No cached counter, no new state fields. The stop-loss verdict is computed
by replaying the mission ledger against the sealed contract every time the
runner needs it. Ledgers are small (a line per cycle, fences cap cycles),
the ledger is already flock-guarded and append-only, and the append that
concludes a cycle IS the transaction — there is no second write to keep
coherent and no crash window between "recorded" and "counted".

- BESTS: initialized from the sealed baseline measurement (every sealed
  contract carries one), then folded forward over each measurement line's
  recorded metric values. Measurement lines gain a structured
  `observed=` value already; the fold uses raw directed values.
- NEW BEST (the only automatic reset): the candidate tuple — (count of
  declared thresholds met, then each metric's raw directed value in
  declaration order) — qualifies against the stored-best tuple when it is
  lexicographically greater AND its first differing component exceeds
  that metric's sealed noise floor. Qualification is only ever evaluated
  candidate-versus-current-best, so the fold is monotone and
  deterministic; no total order over arbitrary pairs is needed. The
  measurement line records `best=yes|no` so replay never re-derives
  qualification from arithmetic alone.
- STAGNATION: `stagnant` = the count of cycles since the last `best=yes`
  line or `Stop-loss reset:` line, counting every `no-progress` and
  `unresolved` cycle. There is NO decay rule: round 1 showed any recovery
  decrement can be funded by a single regression and farmed; reverts are
  instead covered by last-defense budget sizing and the human reset.
- The fuse fires when `stagnant` reaches `ledger.no-gain-budget`, parking
  with the stop-loss ask, exactly as today. The runner also enforces
  `ledger.cycle-budget` in the same derived verdict — relocation carries
  BOTH duties the shell check performed for missions, not one.

## C2. Ownership and scope

- The mission stop-loss decision moves INTO the runner as a pure
  replay over (sealed contract, ledger) — nothing lives in mission state.
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
  the unpark state write. Idempotence is structural, not keyed: the
  counter is derived by replay, and a reset line zeroes it at its
  position in the ledger — appending is the whole reset, duplicate
  answers merely append lines that each (harmlessly, vocally) zero an
  already-low counter, and a crash between append and unpark leaves a
  parked mission whose derived counter is already zero; the next resume
  applies the unpark. The ledger-anchor reconciliation treats a trailing
  reset line without its unpark as exactly that replayable state, not as
  divergence — this tolerance is part of the design, specified against
  `mission-state reconcile`'s existing park-on-disagreement rule.
- No other reset path exists — no flag, no environment variable, no code
  path without a human answer. The flight-recorder event and ask record
  remain best-effort echoes of the authoritative ledger line.

# Invariants

1. For missions, exactly one stop-loss trigger exists: the replay-derived
   count of stagnant cycles since the last new-best or reset line
   reaching the sealed budget. The replay is a pure function of the
   sealed contract and the ledger; identical inputs give identical
   verdicts on every load, after any crash.
2. The new-best tuple comparison is total, deterministic, and gated by
   the sealed per-metric noise floors.
3. Every reset has a ledger line that landed before its state effect, and
   any crash prefix replays to a verdict already reflecting it.
4. Non-mission stop-loss behavior is bit-for-bit unchanged.

# Failure behavior

- Measurement unavailable (`no-progress`/unmeasurable): increments, as
  today — fail toward the fuse.
- Ledger append fails during reset → the unpark does not happen; the
  mission stays parked, loudly.
- Replay handles every ledger vintage: lines without `best=yes|no`
  markers (legacy) contribute conservatively — bests fold from the sealed
  baseline plus any parseable `observed=` values, and classification
  words alone drive the count. Unparseable measurement values fold as
  baseline. Derivation never writes anything.

# Tests

- Replay arithmetic: stagnant counts from last best/reset; no-progress
  and unresolved both count; fuse at budget; cycle-budget enforced in the
  same verdict; a regression followed by recovery never lowers the count
  (no decay).
- New-best qualification: baseline-initialized bests; thresholds-met
  dominance; declaration-order comparison; first-differing-component
  noise gate; single-metric missions behave exactly as a scalar ratchet;
  best=yes|no recorded and honored by replay over re-derivation.
- Reset: `reset:` answer appends-then-unparks; append failure blocks
  unpark; duplicate answers are harmless and vocal; crash between append
  and unpark leaves a resumable parked mission and reconcile does not
  park on the trailing line; non-reset answer keeps amendment guidance.
- Non-mission: assert-stop-loss.sh fixtures untouched and green.
- Legacy replay: marker-less ledgers derive conservatively; unparseable
  observed values fold as baseline; derivation is read-only.
- Validator: relative warning below half the cycle fence.

# Migration

No state shape change at all. One additive ledger annotation (`best=yes|no`
on measurement lines) and the reset line kind; legacy lines replay
conservatively as specified. No contract grammar changes, no new sealed
keys, no script changes. The reconciliation tolerance for a trailing reset
line is the only behavioral change outside the runner.
