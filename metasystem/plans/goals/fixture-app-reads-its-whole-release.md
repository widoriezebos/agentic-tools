# fixture-app-reads-its-whole-release

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Lane test finding 334 and its 17:58 addendum (analysis/memory-hogs-and-root-causes-2026-10-15.md U6); the gate of 15:54-17:25 was red only in TestAppRunThatEndsByItself."
- Tier: 1
- Intent: The fixture application consumes one whole release line of its exit gate, one release per instance, so a second start of the same contract stays alive until released or stopped; the test asserts it (lane test finding 334: the one-byte read left an exit token for the next instance and its start raced the exit).
- Origin: human
- Next step: U1 built by hand (worktree -stewardtick, commit 459babd2a, Opus read LAND), landed by the hand route with the steward-tick gate.
- OpenedAt: 2026-10-15T17:25:44Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-15T17:25:44Z MYSK78CAB8RZ3VYECKMFVCN0CH-m1e-670f65d0 open actor=human:Wido targets=fixture-app-reads-its-whole-release
Integrity: sha256=7a7fd28e34e11c2c7cdbe717bd8e4a668c24833774216ffe327f80d9ae1caa29
