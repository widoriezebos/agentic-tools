# supervise-start-gate-linux-red

- State: claimed
- Intent: VM validation sweep finding (Debian guest at e35898c, 2026-08-30): internal/supervise TestLaunchOwnerReportsEarlyExitAndPublicationFailures/start_gate is red on Linux - 'an owner whose start gate was blocked was accepted'. Green on darwin; first Linux run in ~30 landings. Supervise/arming is m1's seam - recorded from the sweep, not diagnosed further from m2.
- Origin: main
- Next step: DONE, landed b5a75617 by m0 (account Wido@M0): the start_gate subtest's blocking directory now rides the parent-side Command closure instead of racing from inside the child, so launchOwner's blocked-gate refusal is exercised by construction. Chain start-gate-correct-boundary-decl closed (MECHANICAL, Sol-built, conformance reviewedTree 663dfe90); verified 50/50 idle and 50/50 under 4x CPU load. Side yields: goal codex-148-adapter-drift (the code-mode-host defect that ate two launches); receipts.log (immutable round-1 diffBoundary kills a chain — briefs must state the repository-relative path convention). Remaining act: whoever holds the Current slot may mark it done.
- OpenedAt: 2026-08-30T09:55:21Z
- Revision: 7
- Budget: elapsedLimit=3d attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=2
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T12:53:55Z revision=5
- StopCapability: generation=5 revision=5 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T09:55:21Z S4GWER5Z1HXZXMDK005R62MEKE-m2-bc1be9cb open actor=m2+mac-coordinator targets=supervise-start-gate-linux-red
- 2026-08-30T15:17:08Z 2HB609VDB0YKEH3Y0CG11ZS4Z6-m1-bf243850 set-budget actor=m1+coordinator targets=supervise-start-gate-linux-red
- 2026-08-31T12:21:42Z M0CWZ62D05R5FDVHRVBDTZ9CPV-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
- 2026-08-31T12:25:01Z HH396BFB1796HADFN1STJ896DP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
- 2026-08-31T12:53:55Z WWYNN6CCX1K9HSH9N1KR6W26QW-m0-c5dbf036 set-budget actor=human:Wido targets=supervise-start-gate-linux-red
- 2026-08-31T13:08:09Z J9Z2Z01JF2V9BQHW4VCEPBAXYN-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
- 2026-08-31T13:09:25Z A1HVEQZFZSW8Z7FAJ5VVKBTTV8-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
Integrity: sha256=163dba54e1e091877717da45a259e1befb760d06ae4827424952b9b341129765
