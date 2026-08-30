# supervise-start-gate-linux-red

- State: queued
- Intent: VM validation sweep finding (Debian guest at e35898c, 2026-08-30): internal/supervise TestLaunchOwnerReportsEarlyExitAndPublicationFailures/start_gate is red on Linux - 'an owner whose start gate was blocked was accepted'. Green on darwin; first Linux run in ~30 landings. Supervise/arming is m1's seam - recorded from the sweep, not diagnosed further from m2.
- Origin: main
- Next step: Appetite: m1's call. Reproduce on the guest (lima metasystem-debian-amd64, checkout ~/agentic-tools at transport/main), diagnose the platform divergence in the start-gate acceptance path, fix on the owning seam. Evidence preserved in the guest at artifacts/agents/suite-failures/20260830T095355Z-745160.
- OpenedAt: 2026-08-30T09:55:21Z
- Revision: 1

History:
- 2026-08-30T09:55:21Z S4GWER5Z1HXZXMDK005R62MEKE-m2-bc1be9cb open actor=m2+mac-coordinator targets=supervise-start-gate-linux-red
Integrity: sha256=95ad2037f13ee830c2e764f93754890baf7c01b54d6550b9be512ad973f06d80
