# Patience satellite 2: mission reap and bounded drain

Owner: main session (claude). Status: DESIGN — awaiting critique round 1.
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
- Record-side facts from `dispatch reap-facts` — abandoned setup (a
  pending-setup record past its grace), handshake expiry, budget expiry.
  reap-facts carries NO liveness fact and is never treated as one.
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

## R3. The drain is finite

`drainJobs` gains a deadline computed from the mission's active set:
the latest surviving `capDeadline` plus the standing grace; a record
without a parseable capDeadline contributes `createdAt` plus the
mission's job cap plus grace; a record with neither parseable is
already due. The deadline is therefore always finite. Each drain pass
first applies R1/R2 reaps (clearing what is provably dead), then waits;
when the deadline passes with non-terminal, unprovable records
remaining, the runner parks: reason `drain-stalled`, one ask naming
each surviving record (id, status, age, and what proof is missing).

## R4. Drain-stalled park, cycle-consistent resume — and the heal

The park happens INSIDE a reserved cycle whose turn already ran: the
fence counter is ahead of the ledger, which is exactly the state the
reserve/append heal (task #17) treats as a crashed turn. The two must
not fight:
- The drain-stalled park RECORDS ITS CLAIM: the park state carries the
  reserved cycle number and the turn id whose artifacts are complete
  (`drainStalled: {cycle, turnId}`), written in the same park state
  write.
- `resumeState` checks the park reason BEFORE healing: a
  `drain-stalled` park resumes INTO THE DRAIN of its recorded cycle —
  re-running R1/R2 reaps first (the human has usually just cleared
  records), then, if the drain now empties, proceeding to measurement,
  ledger append, and conclusion of the SAME cycle number with the
  already-complete turn artifacts. The heal fires only when NO
  drain-stalled claim covers the reserved cycle — i.e., a genuine
  crashed turn.
- Recovery authority: the human clears the named records through the
  existing surfaces (`dispatch.sh cancel --job`, or out-of-band
  terminalization) and answers the drain-stalled ask with the `resume:`
  prefix; a `reset:`-style quiet path does not exist here either. If
  the drain stalls again after a resume, it parks again with a fresh
  ask — bounded each round, never wedged.

## R5. Starvation is never booked as stall

A cycle that ends in a drain-stalled park books `no-progress,
unmeasurable:drain-stalled` ONLY if measurement genuinely could not run;
when the drain later empties on resume and the cycle concludes, it
books whatever its measurement earned, exactly once, for its own cycle
number (R4). The annotation line `- Drain: stalled:<n-records>` records
the event in the cycle block (annotation grammar from satellite 1 —
audit trail, never fuse input). Whether drain-stalled cycles should be
excluded from patience counting entirely is satellite 4's decision;
this satellite only guarantees the facts are recorded distinguishably.

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
