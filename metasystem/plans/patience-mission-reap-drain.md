# Patience satellite 2: mission reap and bounded drain

Owner: main session (claude). Status: DESIGN — rounds 1 and 2
adjudicated (11/11 and 8/8 accepted; dispositions r1/r2), awaiting
round 3 (final convergence check — round 2 found two genuine safety
defects in round 1's amendments, so the loop has not earned its stop).
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

## R4. Drain-stalled park, cycle-consistent resume — and the heal

The park happens INSIDE a reserved cycle whose turn already ran: the
fence counter is ahead of the ledger, which is exactly the state the
reserve/append heal (task #17) treats as a crashed turn. The two must
not fight, and the resume must be EXECUTABLE against the shipped
sequence:
- THE CLAIM records the continuation POINTER, not a copy:
  `drainStalled: {cycle, turnId, concludePath, survivors}` — written in
  the park state write. `concludePath` names which conclusion the cycle
  owes: `accepted` (the adjudicated verdict and return artifacts are on
  disk under the turn id) or `faulted` (satellite 1's path). The fault
  facts themselves are NOT duplicated into the claim: satellite 1 made
  turn.json carry the outcome, detail, and breaker facts, and turn.json
  is the single source the resumed ConcludeFaultedTurn reads.
  `survivors` records the ids that stalled the drain — the durable
  source for the park ask and for the eventual `- Drain: stalled:<n>`
  annotation, which a successful resume would otherwise have no way to
  count.
- ENTRY, not surgery on resumeState: the run-loop gains an explicit
  entry mode — normal | heal | drain-resume(claim) — decided once at
  startup from the loaded state. A drain-stalled park resumes into
  drain-resume: NO host launch, NO start-gate handshake (the start
  signal reports the runner started in drain-resume mode; the
  launch-handshake contract applies only to turns that launch hosts).
  The mode re-runs R1/R2 reaps, re-drains under a freshly computed R3
  deadline, and on empty calls the recorded continuation for the SAME
  cycle number with the on-disk turn artifacts. The heal fires only in
  entry mode heal — i.e., fences ahead of ledger with NO claim.
- LEDGER EXACTLY ONCE, over the SHIPPED two-write reality: booking is
  the ledger append followed by the state conclude, two writes under
  two locks, and the claim strip lives in the second. A crash between
  them leaves a booked cycle with a live claim; entry detects exactly
  this (the claim's cycle already has its ledger block), strips the
  claim, and defers to the existing ledger-ahead reconciliation that
  already adopts a state lagging its ledger. No new recovery machinery
  — the window lands in a recovery path that shipped before this
  design. An unresumed park leaves an unconcluded cycle behind a parked
  mission, which is exactly what a park means.
- CLAIM LIFECYCLE: created by the park's state write; carried through
  the resumed drain; STRIPPED by the same conclude state write that
  books the cycle (both conclusion proposal builders remove
  `drainStalled` — one write, no window where the cycle is concluded
  but claimed). A claim can therefore never outlive its cycle, and a
  claim for an already-concluded cycle (impossible by the above, but
  checked) is refused loudly at entry rather than obeyed.
- ANSWERS DURING THE PARK: the round-1 premise was wrong for an
  accepted turn whose conclusion is still owed — its verdict was
  adjudicated against turn-start streams, and an answer mutating those
  streams mid-park recreates the mid-turn race. Policy: during a
  drain-stalled park, the ONLY immediately-acting answer is the
  drain-stalled ask's own `resume:`; answers to other asks are recorded
  (answeredAt, answer) but their stream effects apply at the next
  turn's boundary, exactly where answers already take effect relative
  to turns. The resumed conclusion re-validates its stream transitions
  against live state and a now-illegal transition books the conclusion
  as faulted rather than corrupting state (the lawful-transition check
  is already the state writer's refusal).
- THE PARK ASK is raised idempotently by BOTH the park and the
  drain-resume entry (keyed ask id): a crash between the park's state
  write and its ask write is healed by the next resume's entry
  re-raising the missing ask — the recovery has an entry path, not a
  hope.
- Recovery authority: the human clears the named records through the
  existing surfaces (`dispatch.sh cancel --job`, or out-of-band
  terminalization) and answers the drain-stalled ask with the `resume:`
  prefix; a quiet path does not exist here either. If the drain stalls
  again after a resume, it parks again with a fresh ask — bounded each
  round, never wedged.

## R5. Starvation is never booked as stall

A drain-stalled cycle books NOTHING at park time (R4's exactly-once
rule) and, at its eventual conclusion, books whatever its measurement
earned — with the `- Drain: stalled:<n-records>` annotation in the same
block (annotation grammar from satellite 1 — audit trail, never fuse
input). Measurement runs at conclusion time on the resumed path like
any conclusion; only a genuinely unmeasurable tree books
`unmeasurable`. Whether drain-stalled cycles should be excluded from
patience counting entirely is satellite 4's decision; this satellite
guarantees the facts are recorded distinguishably and exactly once.

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
  live custodian, facts-without-death, and a record mid-CAS; reap
  succeeds for a dead custodian with expired budget and for a
  never-launched setup husk past grace.
- R3: deadline arithmetic across the three fallbacks; a drain with a
  reapable husk clears it and exits without parking; a drain with an
  unprovable survivor parks at the deadline with the survivor named.
- R4: heal/park coordination — a drain-stalled park resumes into its
  own cycle's drain and concludes under the same number; a genuine
  crashed turn (no claim) still heals as turn-lost; both directions
  pinned.
- R4 recovery: `resume:` answer re-drains; a second stall parks again
  with a fresh ask; ask idempotence across the park-write crash window.
- R5: the annotation line lands and every parser tolerates it (extend
  the pinned annotation suite); a resumed cycle books its measured
  classification exactly once.

# Migration

One park reason (`drain-stalled`), one park-state field
(`drainStalled`), one ask kind, one annotation line kind — all additive.
The state shape validator admits the new field as optional; legacy
missions never carry it and resume exactly as today. No contract
grammar, no config keys, no changes to dispatch.sh's standing-reap gate
or the supervision reaper.
