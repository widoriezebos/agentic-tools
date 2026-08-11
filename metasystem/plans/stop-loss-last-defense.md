# Stop-loss as last defense

Owner: main session (claude). Status: CRITIQUE EXHAUSTED at round 3
(13, 14, 14 material findings; all accepted; dispositions r1/r2 committed,
r3 return at artifacts/agents/design-critic-20260811t071055z-4fa5). Not
converged: round 3's findings show the runner-integration half of this
design (credit minting inside the conclude sequence, chain-to-stream
binding, orphan delivery, drain resume points) was specified against a
runner that does not work the way the design assumed — the host, not the
runner, dispatches critique roots; the ledger tail does not reach the
prompt as claimed; conclude-time credit minting misses same-cycle
classification. Per the loop's own rule, exhaustion is not agreement:
escalated to the human with a proposed split — a small CORE design (the
fuse: ratcheted counter with a debt-decay decision, multi-metric best
definition, mission-scoped retirement, vocal reset) to converge and ship
first, and separate satellite designs for turn identity, mission-scoped
reaping/drain, and orphan/usage capture, each written against the runner's
actual cycle sequence. AWAITING HUMAN DIRECTION.
Ruling: human, 2026-08-11 — "the mechanism that killed the loop should be a
last-defense kind of thing … really high caps set at the mission level …
resetting should never be quiet." Evidence: the bm-2s trial cohorts
(`benchmark/results/cohorts/bm-2s-20260810t*.json` and their targets'
ledgers), where a lawful, converging design-critique loop was killed by the
no-gain budget after 3 cycles with 82% of wall clock unused.

# Intent

The mission ledger's stop-loss exists to catch pathological non-convergence
that no inner mechanism anticipated. It is NOT a pacing device. Today it
fires during provably correct operation because its only progress sensor is
the product gate, which is blind to every phase before code lands — and a
second, hidden lifetime fuse (two no-progress classifications ever) parks
lawful missions regardless of any budget. The governed inner loops own "is
this phase stuck"; the stop-loss owns only "is the mission stuck in a way
nothing else can see," and it must be impossible to trip it silently or
reset it quietly.

# Non-goals

- No change to the design-critique loop itself (battle-tested; its closure
  rule and per-chain round budget stay exactly as shipped).
- No change to the money fences: wall clock, EUR exposure, cycles, jobs
  remain the hard spend guards and are untouched.
- No ambient "attended mode": only explicit, recorded human actions alter
  mechanism behavior.
- The benchmark kit's gate redesign (count metric, requirement-5 ruling,
  role-specific caps) is companion kit work, decided by the human
  separately.

# Design

## D1. Jurisdiction-aware gain: `loop-advanced` with one-use credits

A cycle classifies `loop-advanced` only when the RUNNER itself mints a
closure credit during conclude. Orchestrator assertions never qualify.

- Credit identity: (critique chain root job, round number). The runner
  recognizes a chain only if the mission's own fence reservations name its
  root job. For a candidate closure it mechanically re-runs the closure
  check (`assert-critique-closed.sh` semantics) over that round's findings
  and dispositions artifacts.
- One-use: each (chain, round) credits at most once, recorded as a ledger
  line (`- Loop credit: chain=<root> round=<n>`); rounds must be strictly
  increasing per chain. Replaying an old round or re-joining the same
  artifacts cannot credit again.
- Farming bound: credits per stream are capped by the sealed contract key
  `ledger.loop-credit-budget` (default: 2× the critique skill's per-chain
  round budget). Beyond the cap, closures still happen but no longer
  classify cycles as `loop-advanced` — the fuse resumes counting.
- Counter algorithm (the single, binding rule — freeze semantics with a
  ratchet): one counter, `stagnant`, incremented by every cycle classified
  `no-progress` or `unresolved` without a credit; UNCHANGED by a
  `loop-advanced` cycle; RESET to zero only by a NEW BEST — a measurement
  that beats the best value recorded so far by more than the sealed noise
  floor. Merely recovering ground lost to a regression is not a reset, so
  oscillation cannot farm resets. The fuse fires when `stagnant` reaches
  `ledger.no-gain-budget`.
- Precedence: a cycle that both improves (new best) and closes a round
  classifies `contract-improved`; its credits are still minted, recorded,
  and budget-consumed. A cycle that closes several rounds mints every
  credit (each consuming budget) and classifies once.
- Stream identity for the credit budget is the runner's own: the stream a
  chain belongs to is recorded in mission state when the runner dispatches
  the chain root, never taken from a return's assertion.

## D2. One fuse, sized as a last defense

- The lifetime "two no-progress classifications ever" rule is RETIRED
  FOR MISSION LEDGERS ONLY. `assert-stop-loss.sh` serves non-mission
  workflows too (investigate/improve); those keep today's behavior
  unchanged — the script gains an explicit mission scope switch rather
  than a silent behavioral change for every caller. Within missions the
  ratcheted no-gain counter is the only stop-loss trigger. (The
  2-consecutive host-failure park is NOT touched: host-health breaker,
  different jurisdiction, own ask.)
- Calibration guidance inverts: `ledger.no-gain-budget` is sized ABOVE any
  healthy runway — unattended missions in the order of `fence.cycles`,
  never below the mission's mandated pre-build stages plus margin. It
  stays a sealed, human-signed contract key.
- The contract validator warns (never refuses) when the budget is below
  half of `fence.cycles`, naming this design.

## D3. Vocal reset, by explicit human action only

- The stop-loss ask becomes answerable in one specific shape: an answer
  beginning with the literal prefix `reset:` (the rest is the human's
  reason). That answer unparks the mission and resets `stagnant` to zero.
  The sealed budget LINE is untouched — the human is spending more of the
  still-sealed wall-clock and exposure fences, not rewriting the
  allowance. Any other answer keeps today's guidance: amend, price,
  reseal, sign.
- Authoritative record: the LEDGER append (`- Stop-loss reset: by human
  answer <askId>: <reason> at <time>`) is written FIRST under the ledger
  lock; the unpark state write happens only after it lands. The
  flight-recorder event and the ask/answer file are best-effort echoes and
  the docs say so. A reset that leaves no ledger line is impossible
  because the unpark never happens without it. Crash consistency: the
  ledger line carries the askId; on startup the runner reconciles — a
  reset line whose unpark never applied is replayed forward, and a second
  line for the same askId is never written. The transaction is
  append-then-apply with idempotent recovery, not two-phase.
- No other reset path: no flag, no environment variable, no quiet code
  path. Attended operation is expressed by the human actually answering.

## D4. Turn identity: announced and observed

The prompt header is assembled before launch, so the harness may learn the
real session only mid-launch. Two identities, both recorded in turn.json:

- `announcedSession`: what the prompt's `Host-Session` header said (null
  when it said `none`). Written at assembly, never changed.
- `observedSession`: stamped harness-side from the earliest trusted
  source that names the session — the launch handshake's
  session-established signal or, failing that, the adapter's terminal
  result envelope. Both are the harness's own artifacts; absent only when
  neither carries a session.
- Adjudication accepts a return whose `identity.sessionId` equals EITHER —
  the conservative echo of the header and the honest report of the real
  session are both correct. When neither matches, that is a host protocol
  violation: the turn fails normally, debits normally, and feeds the
  existing consecutive-host-failure breaker. No blanket exemption exists.
- Measurement is runner-run and trusted regardless of return validity: the
  cycle classifies from the measured tree (an improvement counts and
  resets the counter) even when the return itself is rejected; the ledger
  entry names the identity fault beside the classification.
- The return schema and turn prompt document the echo rule explicitly.

## D5. The runner reaps its own dead reservations — with real proof

- Authority: holding the mission lease authorizes the runner to act on
  records that its own mission's fence reservations name — and nothing
  else. Authority is not proof.
- Proof: record-side facts come from `dispatch reap-facts` (abandoned
  setup, handshake expiry, budget expiry — it carries NO liveness fact);
  process-side death is proven exactly as the supervision reaper proves
  it: the kernel custodian discipline (pid alive at its recorded start AND
  command still bearing the job tag; anything less than provable death is
  not death). The existing record CAS applies the verdict.
- Bounded drain: drainJobs gains a deadline — the latest surviving
  `capDeadline` among the mission's active records plus the standing
  grace; a record without a parseable capDeadline contributes
  `createdAt + the mission's job cap + grace`, and one with neither
  parseable is treated as already past deadline. The deadline is
  therefore always finite. When it passes with non-terminal, unprovable
  records remaining, the runner parks with reason `drain-stalled` and an
  ask naming each stuck record. Recovery is defined: the human clears the
  named records through the existing surfaces (`dispatch cancel`, or
  out-of-band terminalization) and answers the ask with the `resume:`
  prefix; the runner re-drains from the top on resume. The conservative
  posture (never reap the unprovable) is kept; neither the wait nor the
  park is unbounded ownership.

## D6. Nothing paid is silently discarded

- Applied rounds are recorded in mission state as a SET of
  (chain root, round) pairs per stream — rounds are chain-local, so a
  scalar high-water mark proves nothing across chains. A landed
  return.json for a pair outside the set is orphaned by definition.
- The orphan scan runs at every turn conclusion AND at park — not only at
  park, or a return landing during a failed-turn park path would still be
  silently lost. Each orphan gets one ledger line naming the artifact
  paths (`- Orphaned return: chain=<root> round=<n> path=<...>`), written
  once per pair (the applied/orphaned sets make the scan idempotent). The
  next turn's prior-context already carries the ledger tail.
- Usage single-writer rule: the adapter owns its round directory's
  usage.json (written atomically). Reapers never write usage. Under an
  uncatchable kill the adapter writes nothing — recovery then comes from
  the runtime's own on-disk event stream where one exists (the codex
  JSONL event file survives the process): the next aggregation derives
  usage from it and records the derivation source. Runtimes with no
  surviving stream record `availability: unavailable`, stated honestly,
  as today. The fence aggregator reads round directories independently of
  record status.

# Invariants

1. The stop-loss fires only when the count of stagnant cycles since the
   last `contract-improved` reaches `ledger.no-gain-budget`, where
   credited `loop-advanced` cycles never increment that count. No other
   stop-loss trigger exists.
2. Every reset is attributable to a named human answer and has a ledger
   line that preceded the unpark; there is no quiet path.
3. A closure credit exists only for a (chain, round) the runner itself
   proved, at most once, rounds monotone per chain, chains owned by the
   mission, total credits bounded by the sealed credit budget.
4. A trusted measured improvement always counts, even on a turn whose
   return was rejected.
5. The runner's reap authority never exceeds its own mission's reservation
   set; process death is only ever established by the kernel custodian
   proof; machine-wide reaping remains the janitor's.
6. drainJobs terminates: every drain ends in drained, reaped, or a
   `drain-stalled` park naming the survivors.

# Failure behavior

- Closure checker errors → no credit, cycle classifies as today (fail
  toward the fuse, never away).
- Ledger append failure during reset → the unpark fails loudly; the
  mission stays parked.
- Handshake signal absent → `observedSession` stays absent; adjudication
  falls back to `announcedSession` alone.
- Unprovable custodian → record left alone; the drain deadline, not the
  loop, decides when that becomes a park.

# Tests

- missionrunner: freeze arithmetic (increment/freeze/reset); fuse fires at
  budget through interleaved credits; credit replay refused; per-chain
  monotone rounds; credit budget exhaustion resumes counting; foreign
  chain refused.
- missionrunner: retired lifetime rule — a ledger with no-progress,
  contract-improved, no-progress does NOT park.
- missionrunner: `reset:` answer unparks + resets + ledger-line-first
  ordering (append failure blocks unpark); non-reset answer still refused
  toward amendment.
- missionrunner: announced/observed matrix — echo-null accepted, honest
  real id accepted, neither → failed turn feeding the host-failure
  breaker; improvement on a rejected-return turn still resets the counter.
- missionrunner/dispatch: reap needs both facts and custodian death;
  unprovable → survives to drain deadline → `drain-stalled` park naming
  the record.
- ledger fixtures: loop-credit and orphaned-return lines parse; orphan
  paths land in the next prompt's ledger tail.
- validator: relative warning below half the cycle fence.

# Migration

Additive ledger line kinds (`Loop credit`, `Stop-loss reset`, `Orphaned
return`) and one classification string; existing ledgers parse unchanged.
turn.json gains `announcedSession`/`observedSession` (hostSession retained,
equal to announcedSession, until the fixtures migrate).

Mission STATE gains three fields — the stagnant counter with its best-value
ratchet, the per-stream credit accounting, and the applied-(chain,round)
sets. All are derivable by replaying the existing ledger and job records,
and the runner performs exactly that derivation once when it loads a state
document that lacks them; the state writer's shape validation admits the
new fields as optional until the fixtures migrate. Hash-chain integrity is
preserved because the derivation happens inside a normal state write (a new
generation), never by mutating history.

Sealed-contract compatibility: a contract sealed WITHOUT
`ledger.loop-credit-budget` grants NO credit budget by default — the fuse
behaves exactly as the signers saw it. The default (2× the per-chain round
budget) applies only to contracts sealed after the key exists in the
template. A seal means what was signed; the design never derives a new
allowance under an old signature.

The mission-scoped retirement changes `assert-stop-loss.sh` only behind
its new mission switch; non-mission callers and their fixtures are
untouched. bm-2s fence values change separately in the kit under the
human's fence-approval rule.
