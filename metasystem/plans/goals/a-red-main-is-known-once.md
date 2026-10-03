# a-red-main-is-known-once

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="Shared signal every seat and the lane read before testing; a false red stalls the fleet, a missed red wastes runs"
- Tier: 2
- Intent: When main is red, the machinery knows it once and every seat acts on it, instead of each seat finding it with its own 45-minute test run (night of 2026-10-02/03: five reds on main; seat m1i lost four runs to reds that were not its own; seats m1g, m1i, m1h and ui each reported the same reds to the supervisor by hand). Wanted: the first proof or gate that finds main red records it with the failing checks and the commit that caused it; seats and the lane see it before they start a full run (they run only what the red cannot affect, or wait); the seat or person who caused it is told at once; a fix to a red main outranks every landing (Wido 2026-09-16) and may land by the fastest safe route; the red clears itself when main is green again. Hand-pushes to main (four reds came from a supervisor's hand-landings) run the cheap gate first.
- Origin: main
- Next step: Tier-2 design over the existing trunk-red register (plans/goals/trunk-red.json), the lane's proofs and devgate static: who records, how seats read it, what a seat may still run, how it clears; Astra critique; build.
- OpenedAt: 2026-10-03T08:52:13Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-03T08:52:13Z FCT9P7JY9B3T65DR8QNRX9FN35-m1e-718ba0eb open actor=human:Wido targets=a-red-main-is-known-once
Integrity: sha256=7ce21081a78735ba406ee16aecc869545fb514704f36e239dc36780e7df60313
