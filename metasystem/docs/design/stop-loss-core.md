# The stop-loss core: the fuse contract

The durable rules of the mission stop-loss fuse (C1..C3, its
invariants, and its failure behavior), promoted from
`plans/stop-loss-core.md` (which keeps intent, tests, and migration
history). `internal/mission`'s ledger and `internal/missionrunner`'s
stop-loss step implement it. A change here is a contract change, not a
cleanup.

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
  lexicographically greater AND its first differing component clears its
  gate: the thresholds-met count is an integer and needs no noise gate (a
  strict increase qualifies outright); a metric-value component must
  exceed the best by more than that metric's sealed noise floor. Qualification is only ever evaluated
  candidate-versus-current-best, so the fold is monotone and
  deterministic; no total order over arbitrary pairs is needed. The
  measurement line records `best=yes|no` so replay never re-derives
  qualification from arithmetic alone; when a marker exists it WINS over
  re-derivation (older binaries replay marker-less lines by derivation).
  Grammar: ledger measurement lines are the existing `key=value;` token
  form; `best` is one more token, and the turn-prompt Ledger Tail
  validator accepts records with and without it (named migration).
- STAGNATION: `stagnant` = the count of cycles since the last `best=yes`
  line or `Stop-loss reset:` line, counting every `no-progress` and
  `unresolved` cycle. There is NO decay rule: round 1 showed any recovery
  decrement can be funded by a single regression and farmed; reverts are
  instead covered by last-defense budget sizing and the human reset.
- The fuse fires when `stagnant` reaches `ledger.no-gain-budget`, parking
  with the stop-loss ask, exactly as today. The runner also enforces
  `ledger.cycle-budget` in the same derived verdict — relocation carries
  BOTH duties the shell check performed for missions. The two parks are
  distinct: a cycle-budget park is an exhausted sealed allowance and its
  ask accepts ONLY the amendment path (`reset:` is refused for it, with
  the refusal naming why); the vocal reset applies to stagnation parks
  alone.
- Semantics versioning: mission state records `ledgerSemantics: 2` at
  init by a binary that carries this design. A state without the field
  replays under the legacy rules (trailing-without-improvement plus the
  lifetime no-progress pair) — the runner implements both, keyed by the
  field, so a mission sealed and started under the old meaning finishes
  under it. The sealed budget's meaning never changes mid-mission.

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
- Transaction, in binding order: (1) append the reset line under the
  ledger lock; (2) mark the ask answered; (3) apply the unpark state
  write. A crash after (1) leaves the ask open — a re-answer appends a
  second line, which is lawful, vocal, and harmless (the counter is
  already zero); a crash after (2) leaves a parked mission whose derived
  counter is zero and an answered ask — the next resume applies the
  unpark. The reason is sanitized before (1): newlines are refused (not
  mangled), length-capped, so a line can never be split or structurally
  injected; the append itself is a single atomic write as today.
- Reconciliation tolerance, as a precise predicate: reconcile forgives
  exactly the state where (a) mission state is parked with the
  stagnation park reason, and (b) the ledger's unanchored suffix consists
  solely of `Stop-loss reset:` lines each naming an ask that exists on
  disk as a stop-loss ask. Any other divergence parks on disagreement
  exactly as today — the corruption guarantee is narrowed by one named,
  checkable exception, not weakened.
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
