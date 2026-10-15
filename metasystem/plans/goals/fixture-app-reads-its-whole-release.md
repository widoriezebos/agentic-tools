# fixture-app-reads-its-whole-release

- State: approved
- Priority: 1
- Sequence: 56
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="Lane test finding 334 and its 17:58 addendum (analysis/memory-hogs-and-root-causes-2026-10-15.md U6); the gate of 15:54-17:25 was red only in TestAppRunThatEndsByItself."
- Tier: 1
- Intent: The fixture application consumes one whole release line of its exit gate, one release per instance, so a second start of the same contract stays alive until released or stopped; the test asserts it (lane test finding 334: the one-byte read left an exit token for the next instance and its start raced the exit).
- Origin: human
- Next step: U1 built by hand (worktree -stewardtick, commit 459babd2a, Opus read LAND), landed by the hand route with the steward-tick gate.
- OpenedAt: 2026-10-15T17:25:44Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-15T17:26:06Z revision=2 opid=7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 authority=proven digest=36b548d68bf1f3fa39ffcf215b82e581ea0b85f91d638a4789a5816ae3c46be2 episode=2

History:
- 2026-10-15T17:25:44Z MYSK78CAB8RZ3VYECKMFVCN0CH-m1e-670f65d0 open actor=human:Wido targets=fixture-app-reads-its-whole-release
- 2026-10-15T17:26:06Z 7AE86DPTJ585CHVZG57NAXZ9A2-m1e-670f65d0 approve actor=human:Wido targets=a-proof-compiles-a-package-once,fixture-app-reads-its-whole-release,the-reconciler-reads-attempt-headers
- 2026-10-15T17:26:20Z 757ZPJDT9W7HJMVN1EM3TGKQG7-m1e-670f65d0 set-priority actor=human:Wido targets=fixture-app-reads-its-whole-release reason=priority-order subject=fixture-app-reads-its-whole-release from=unranked to=1:56 requested-sequence=append
Integrity: sha256=6cf2c5670aa49fd479e2f3d1fcd5082decb47650599541162f07002d9e8b9bd2
