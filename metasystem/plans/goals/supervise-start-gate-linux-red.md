# supervise-start-gate-linux-red

- State: claimed
- Intent: VM validation sweep finding (Debian guest at e35898c, 2026-08-30): internal/supervise TestLaunchOwnerReportsEarlyExitAndPublicationFailures/start_gate is red on Linux - 'an owner whose start gate was blocked was accepted'. Green on darwin; first Linux run in ~30 landings. Supervise/arming is m1's seam - recorded from the sweep, not diagnosed further from m2.
- Origin: main
- Next step: Appetite: m1's call. Reproduce on the guest (lima metasystem-debian-amd64, checkout ~/agentic-tools at transport/main), diagnose the platform divergence in the start-gate acceptance path, fix on the owning seam. Evidence preserved in the guest at artifacts/agents/suite-failures/20260830T095355Z-745160.
- OpenedAt: 2026-08-30T09:55:21Z
- Revision: 3
- Budget: elapsedLimit=3d attemptLimit=6 reservedJobMinutesLimit=480 activeJobLimit=2
- Claimed: machine=m0 lineage=main-1788178136-1684505-4ffe42 at=2026-08-31T19:09:22Z revision=3
- StopCapability: generation=3 revision=3 machine=m0 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-30T09:55:21Z S4GWER5Z1HXZXMDK005R62MEKE-m2-bc1be9cb open actor=m2+mac-coordinator targets=supervise-start-gate-linux-red
- 2026-08-30T15:17:08Z 2HB609VDB0YKEH3Y0CG11ZS4Z6-m1-bf243850 set-budget actor=m1+coordinator targets=supervise-start-gate-linux-red
- 2026-08-31T19:09:22Z D85QWW2SV3D8R66DNVCN2X0HB2-m0-c5dbf036 claim actor=m0+main-1788178136-1684505-4ffe42 targets=supervise-start-gate-linux-red
Integrity: sha256=31f0d50f9e2d445fe5422d84c4926d9d6a7ecdd304cc2477a14143b410a620dc
