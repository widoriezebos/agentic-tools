# supervise-start-gate-linux-red

- State: claimed
- Intent: VM validation sweep finding (Debian guest at e35898c, 2026-08-30): internal/supervise TestLaunchOwnerReportsEarlyExitAndPublicationFailures/start_gate is red on Linux - 'an owner whose start gate was blocked was accepted'. Green on darwin; first Linux run in ~30 landings. Supervise/arming is m1's seam - recorded from the sweep, not diagnosed further from m2.
- Origin: main
- Next step: REPRODUCED by m0 natively (2026-08-31, current main bdedccd8): green 10/10 idle, red ~5-in-6 under 4x CPU load (yes-loops) — 'an owner whose start gate was blocked was accepted', arming_test.go:717. Same behavior at sweep commit e35898c, so no intervening landing fixed or caused it; it is load-dependent, which is why the sweep (running under load) caught it and idle darwin runs did not. Mechanism hypothesis from the test shape: the subtest's owner replaces the gate with 'mkdir gate; sleep 30'; under load the child is slow to perform the mkdir, and launchOwner's acceptance check observes the gate before the block exists — an acceptance-ordering window in the start-gate path, i.e. likely a product race, not test patience. Next: dispatch the fix through the lanes (Sol implements, Fable critiques) with the load reproduction as the failing evidence.
- OpenedAt: 2026-08-30T09:55:21Z
- Revision: 5
- Budget: elapsedLimit=3d attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=2
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T12:53:55Z revision=5
- StopCapability: generation=5 revision=5 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T09:55:21Z S4GWER5Z1HXZXMDK005R62MEKE-m2-bc1be9cb open actor=m2+mac-coordinator targets=supervise-start-gate-linux-red
- 2026-08-30T15:17:08Z 2HB609VDB0YKEH3Y0CG11ZS4Z6-m1-bf243850 set-budget actor=m1+coordinator targets=supervise-start-gate-linux-red
- 2026-08-31T12:21:42Z M0CWZ62D05R5FDVHRVBDTZ9CPV-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
- 2026-08-31T12:25:01Z HH396BFB1796HADFN1STJ896DP-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
- 2026-08-31T12:53:55Z WWYNN6CCX1K9HSH9N1KR6W26QW-m0-c5dbf036 set-budget actor=human:Wido targets=supervise-start-gate-linux-red
Integrity: sha256=d77bacc7638e4d586b742cb736f91c45c2ac7d6eb536880d61c7b6861daa7fc0
