# Stop-loss as last defense

Owner: main session (claude). Status: DESIGN — round 1 adjudicated (13/13
accepted, see `plans/stop-loss-dispositions-r1.md`), awaiting round 2.
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
- Counter algorithm (the single, binding rule — freeze semantics): one
  counter, `stagnant`, incremented by every cycle classified `no-progress`
  or `unresolved` without a credit; UNCHANGED by a `loop-advanced` cycle;
  RESET to zero only by `contract-improved`. The fuse fires when
  `stagnant` reaches `ledger.no-gain-budget`.

## D2. One fuse, sized as a last defense

- The lifetime "two no-progress classifications ever" rule in
  `assert-stop-loss.sh` is RETIRED. The no-gain counter above becomes the
  only stop-loss trigger; `no-progress` cycles feed it like any stagnant
  cycle instead of a separate lifetime tripwire. (The 2-consecutive
  host-failure park is NOT touched: that is a host-health breaker, a
  different jurisdiction, and it keeps its own ask.)
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
  because the unpark never happens without it.
- No other reset path: no flag, no environment variable, no quiet code
  path. Attended operation is expressed by the human actually answering.

## D4. Turn identity: announced and observed

The prompt header is assembled before launch, so the harness may learn the
real session only mid-launch. Two identities, both recorded in turn.json:

- `announcedSession`: what the prompt's `Host-Session` header said (null
  when it said `none`). Written at assembly, never changed.
- `observedSession`: stamped from the launch handshake's
  session-established signal when one arrives; absent otherwise.
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
  grace. When the deadline passes and non-terminal, unprovable records
  remain, the runner parks with reason `drain-stalled` and an ask naming
  each stuck record. The conservative posture (never reap the unprovable)
  is kept, but it can no longer wedge the runner inside one cycle forever.

## D6. Nothing paid is silently discarded

- Applied rounds are recorded per stream in mission state; a landed
  return.json for any round beyond its stream's applied mark is orphaned
  by definition — no prose judgment involved.
- At park, the runner writes one ledger line per orphan naming the
  artifact paths (`- Orphaned return: job=<id> round=<n> path=<...>`).
  The next turn's prior-context already carries the ledger tail, so the
  resurrection reads its inheritance without new prompt machinery.
- Usage single-writer rule: the adapter owns its round directory's
  usage.json (written atomically; on a capped job, best effort from the
  events already streamed before termination). Reapers never write usage.
  The fence aggregator reads round directories independently of record
  status, so late usage is picked up by the next aggregation regardless of
  who terminalized the record.

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
equal to announcedSession, until the fixtures migrate). One new sealed key
`ledger.loop-credit-budget` with a default — absent keys behave as today
plus the default, so existing sealed contracts stay valid. The retired
lifetime rule changes `assert-stop-loss.sh` behavior; its fixtures are
updated in the same change. bm-2s fence values change separately in the
kit under the human's fence-approval rule.
