# Patience satellite 2: mission reap and bounded drain

Owner: main session (claude). Status: DESIGN — rounds 1-3 adjudicated
(11/11, 8/8, 8/8 accepted; dispositions r1/r2/r3). Round 3's findings
all clustered in the same-cycle salvage machinery while the reap half
stood unchallenged — the generating-cause signature — so per the split
rule that machinery is SEVERED from this design: a drain-stalled cycle
is booked by the shipped heal as honestly lost, and same-cycle verdict
salvage is deferred to a possible future satellite if trials prove the
loss matters. Round 4 returned six
material findings, all wiring-level edges of the sever itself with no
machinery challenged — the diminishing-returns stop signal. All six
amended below; the loop stops by recorded judgment (dispositions r4)
and implementation is the next source of truth.
Program: `plans/stop-loss-satellites.md` satellite 2; concepts in
`docs/patience.md`; ground truth in `docs/design/mission-cycle-sequence.md`
(especially the drain narrative and false-stall surfaces). Routed
findings: parent r1[9][10], r2[8][9], r3[10][11] (dispositions r1/r2 and
the r3 return under `plans/stop-loss-*`). Related shipped work: the
reserve/append heal (task #17, `healReservedCycle` in
internal/missionrunner/loop.go) and the standing reaper's abandoned-setup
transition (internal/supervise/reaper.go).

# Intent

A mission that cannot act is not stalling — it is being starved, and
starvation is the harness's fault. Today a dead reservation consumes a
concurrency slot forever unless an armed standing reaper happens to
clear it, the runner's drain is an infinite loop with no deadline, and
the runner's own one-shot reap cannot fail a pending-setup husk at all
(that verdict is gated on the standing reaper's custody). The second
bm-2s repetition died exactly this way. This satellite gives the runner
bounded, provable authority over its own mission's reservations, a
finite drain, and a park-and-resume path that a human can always drive —
so starvation becomes visible and recoverable instead of a silent
stall debited against patience.

# Non-goals

- No change to the standing (supervision) reaper: it keeps machine-wide
  jurisdiction and its existing verdicts; this satellite adds the
  runner's narrower, mission-scoped authority beside it.
- No change to what counts as progress or to patience floors
  (satellite 4).
- No new kill authority: the runner reaps RECORDS whose processes are
  provably dead or that provably never launched; live process-group
  wind-down stays where it is today (the turn cap and the reaper).

# Design

## R1. Authority: the mission's own reservations, nothing else

Holding the mission lease authorizes the runner to act on exactly the
records its mission's fence reservations name (`fences.json`
reservation keys — the same set the drain already watches since the
slot-leak fix). Authority is not proof: it selects the candidates, never
the verdict.

## R2. Proof: record facts plus kernel custodian, the shared discipline

A reservation may be failed by the runner only on the conjunction the
standing reaper already uses:
- Record-side facts from `dispatch reap-facts` AS SHIPPED — abandoned
  setup, budget expiry, and `handshakeWaiting` (which is a state, not an
  expiry: the runner computes handshake expiry itself from the record's
  handshake deadline plus the handshake grace, exactly as the dispatch
  backstop does; a false `handshakeWaiting` is never read as proof of
  anything). reap-facts carries NO liveness fact and is never treated as
  one.
- Process-side death by the kernel custodian discipline: the recorded
  pid alive at its recorded start AND still bearing the job tag, or it
  is dead-to-us; `Unknown` (unreadable) is not death. A record with no
  recorded pid (a setup husk that never launched) needs no custodian
  proof — there is no process to prove; its abandoned-setup fact
  suffices, exactly as the standing reaper judges it.
The verdict is applied through the existing record CAS under the record
lock (lawful transitions only), NOT through dispatch.sh's standing-reap
gate — the runner is not a standing reaper and does not borrow its
custody; it exercises R1's narrower authority with the same proof bar.
Terminal verdict mapping, fixed (the standing reaper's precedent):
budget expiry on a running record is judged FIRST and books
`timeout/budget-cap`; a pending-setup husk past its grace (never left
setup, so provably no process was ever recorded to exist) books
`failed/abandoned-setup`; a proven-dead custodian books
`failed/process-lost`. An expired handshake whose record carries NO
process identity is NOT reapable by the runner — the process may exist
unrecorded, no proof means no verdict (invariant 5) — it survives to
the drain deadline and the park ask names it as "handshake expired, no
recorded process to prove". No other verdicts exist on this path.
Grace ownership, fixed: the handshake grace is the dispatch backstop's
existing constant; the abandoned-setup grace is the standing reaper's
existing ten-minute constant. This design introduces NO new grace and
names which existing one applies where.

## R3. The drain is finite

`drainJobs` gains a deadline computed from the mission's active set:
the latest surviving `capDeadline` plus the handshake grace; a record
without a parseable capDeadline contributes its own clock — `startedAt`
plus the record's immutable `capMin` plus grace for a launched record,
`createdAt` plus the setup grace for a pending-setup husk; a record
with nothing parseable is already due. The deadline is therefore always finite. Each drain pass
first applies R1/R2 reaps (clearing what is provably dead), then waits;
when the deadline passes with non-terminal, unprovable records
remaining, the runner parks: reason `drain-stalled`, one ask naming
the survivors as snapshotted in the claim (id, status, age, missing
proof). The snapshot is best-known-at-park: a reservation racing the
final pass may be absent from the ask, and that is acceptable because
drain-resume re-proves against the LIVE set — the ask advises the
human; the resume trusts only the current reservations.
The deadline RECOMPUTES each pass over the CURRENT active set: a
follow-up reserved mid-drain lawfully extends it (new real work), a
record gaining a capDeadline moves from the fallback to the real one,
and the park condition is evaluated against the deadline computed in
the same pass. Each pass also beats the runner's own heartbeat — a
lawful drain that waits until the last job cap must read as a live
runner to supervision, not as a death (no new artifact; the existing
runner heartbeat is written per pass).

## R4. Drain-stalled park: park plainly, heal honestly, resume normally

Three rounds of critique proved the same-cycle salvage machinery (claims,
entry modes, deferred answers, resumed conclusions) generates crash
windows faster than they can be specified. It is severed. The severed
shape reuses two shipped, tested mechanisms and adds none:

- At deadline expiry the runner parks: reason `drain-stalled`, the
  survivors ask (best-known snapshot; id, status, age, missing proof).
  The park writes NO ledger line and no claim — the reserved cycle
  simply never concluded, which is exactly the state the reserve/append
  heal (task #17) already recovers.
- ANSWERING: the drain-stalled ask becomes answerable with the
  `resume:` prefix through the same shipped pattern as the stop-loss
  reset (answer.go gains the reason; every other answer shape keeps
  today's refusal). Applying the answer unparks AND writes one additive
  state field, `lastDrainStall: {cycle, survivors}` — the durable label
  that survives the cleared park reason.
- HEALING: the runner resumes through the existing entry sequence; the
  heal, finding the fence/ledger gap, CONSUMES `lastDrainStall` when
  its cycle matches — booking `no-progress;
  observed=unmeasurable:drain-stalled` with the survivor-count
  annotation and clearing the field in the same conclude write. A gap
  with no matching field heals as plain turn-lost, exactly as shipped.
  Everything else about the heal is untouched and stays pinned by its
  tests.
- RE-PROOF: nothing special is promised "before the first dispatch" —
  the human's cleanup is re-proved where proving already lives: the
  fence check at the next dispatch and the R1/R2 reaps at the next
  drain. Normal resume means normal rules.
- CRASH ORDER, aligned with the shipped park mechanism (state write,
  then ask write): a crash between the two leaves a parked state
  without its ask, and the public `mission-runner resume` on a
  drain-stalled park re-raises the missing ask idempotently before
  anything else — the recovery path is the command the human was
  already going to run.
- WHAT IS LOST, stated honestly: the stalled cycle's adjudicated verdict
  and its cycle-granular measurement. WHAT IS NOT LOST: the committed
  tree (the next cycle's measurement sees it — the ratchet banks it as
  the new best, so the value registers one cycle late rather than
  never), the delegates' landed returns (satellite 3's orphan capture
  banks them), and the survivors record (the ask and the healed line).
- Answers during the park need no new policy: no conclusion is pending
  (the cycle heals as lost), so the existing answer-at-boundary
  semantics apply unchanged. The mid-TURN answer race remains the map's
  surprise 6, unowned by this design.
- DEFERRED, explicitly: same-cycle verdict salvage — a possible future
  satellite, justified only if trial evidence shows the one-cycle
  attribution delay matters. Its three critique rounds of findings are
  preserved in this plan's dispositions as its starting inheritance.

## R5. Starvation is recorded distinguishably

The healed line for a drain-stalled cycle reads
`unmeasurable:drain-stalled` — distinguishable from every other
no-progress cause — and carries the `- Drain: stalled:<n>` annotation
(satellite 1's grammar; audit trail, never fuse input; the count comes
from the park ask's snapshot). Whether patience counting should exempt
`drain-stalled` cycles is satellite 4's decision; this satellite
guarantees the fact is recorded exactly once and unambiguously.

# Invariants

1. The runner never writes a verdict on a record outside its mission's
   reservation set, and never on any proof weaker than the standing
   reaper's own bar (facts plus custodian death, or a never-launched
   husk's abandoned-setup fact).
2. Every drain terminates: drained empty, or a `drain-stalled` park
   naming every survivor. No unbounded loop, no silent wait.
3. A drain-stalled cycle concludes at most once, under its reserved
   number; the heal never books a turn-lost line for a cycle a
   drain-stalled claim covers.
4. Resuming a drain-stalled park always re-proves before re-waiting:
   reaps run before the deadline restarts.
5. `Unknown` custodian state never reaps anything, ever.

# Failure behavior

- reap-facts unreadable or the CAS refused (record advanced meanwhile):
  the record is left alone this pass; the deadline, not the loop,
  decides the outcome.
- Crash between the park state write and the ask write: the park stands
  and `resumeState` re-raises the missing ask idempotently (asks are
  keyed; a duplicate is not created).
- Crash during a post-resume conclude: the same windows as any conclude;
  the drain-stalled claim still covers the cycle, so the heal still
  stays out.

# Tests

- R1/R2: reap refused for a foreign record, an Unknown custodian, a
  live custodian, facts-without-death, an expired handshake with no
  recorded process, and a record mid-CAS; reap succeeds for a dead
  custodian past budget (timeout/budget-cap), a dead custodian
  otherwise (failed/process-lost), and a setup husk past grace
  (failed/abandoned-setup).
- R3: deadline arithmetic across the per-record clocks; recomputation
  when the active set changes mid-drain; a reapable husk clears without
  parking; an unprovable survivor parks at the deadline; the runner
  heartbeat lands every pass.
- R4: the park writes state then ask; a crash between them is healed by
  resume re-raising the ask; the `resume:` answer unparks and writes
  lastDrainStall; the heal consumes a matching field into the
  drain-stalled line with the survivor count and clears it in the same
  write; a gap without the field heals as plain turn-lost (both
  directions pinned beside the existing heal tests); a second stall
  parks again with a fresh ask.
- R5: the annotation and the drain-stalled observed string parse
  everywhere (extend the pinned annotation suite); the stop-loss replay
  counts the healed line as no-progress exactly once.
- End to end: park → answer → heal → next cycle measures the committed
  tree and the ratchet banks it.

# Migration

One park reason (`drain-stalled`), one additive state field
(`lastDrainStall`, written at unpark, consumed by the heal), one ask
kind, one annotation line kind, one new observed string on healed lines
— all additive.
The state shape validator admits the new field as optional; legacy
missions never carry it and resume exactly as today. No contract
grammar, no config keys, no changes to dispatch.sh's standing-reap gate
or the supervision reaper.
