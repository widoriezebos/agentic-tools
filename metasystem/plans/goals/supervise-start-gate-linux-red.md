# supervise-start-gate-linux-red

- State: queued
- Intent: VM validation sweep finding (Debian guest at e35898c, 2026-08-30): internal/supervise TestLaunchOwnerReportsEarlyExitAndPublicationFailures/start_gate is red on Linux - 'an owner whose start gate was blocked was accepted'. Green on darwin; first Linux run in ~30 landings. Supervise/arming is m1's seam - recorded from the sweep, not diagnosed further from m2.
- Origin: main
- Next step: DONE, fixed by m0 and landed on the fleet line in reconciliation landing b700f44e (originally b5a75617 on machine/m0): the start_gate subtest's blocking directory rides the parent-side Command closure instead of racing from inside the child, so launchOwner's blocked-gate refusal is exercised by construction. Chain start-gate-correct-boundary-decl (MECHANICAL, closed, Sol-built, conformance reviewedTree 663dfe90); verified 50/50 idle and 50/50 under 4x CPU load at build time, 30/30 under load re-verified on the reconciled tree. m2's 'waits for m1' note is superseded — m0 reproduces this natively and fixed it
- OpenedAt: 2026-08-30T09:55:21Z
- Revision: 5
- Budget: elapsedLimit=3d attemptLimit=6 reservedJobMinutesLimit=480 activeJobLimit=2

History:
- 2026-08-30T09:55:21Z S4GWER5Z1HXZXMDK005R62MEKE-m2-bc1be9cb open actor=m2+mac-coordinator targets=supervise-start-gate-linux-red
- 2026-08-30T15:17:08Z 2HB609VDB0YKEH3Y0CG11ZS4Z6-m1-bf243850 set-budget actor=m1+coordinator targets=supervise-start-gate-linux-red
- 2026-08-31T19:09:22Z D85QWW2SV3D8R66DNVCN2X0HB2-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
- 2026-08-31T19:09:25Z WM48H1FV7BDAMQTNY5ESXBZTTJ-m0-c5dbf036 edit actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
- 2026-08-31T19:09:28Z F0X6YA6GB8BMV0P4CQM8VFQZ7R-m0-c5dbf036 release actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
Integrity: sha256=77a566570088cb07858ede8f4797a6c3e1c0b46f461a92232380c3dc38f46b1a
