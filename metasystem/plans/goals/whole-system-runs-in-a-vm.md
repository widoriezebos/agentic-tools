# whole-system-runs-in-a-vm

- State: queued
- Risk: severity=2 novelty=3 exposure=2 accumulation=2 basis="Changes where everything runs; design needed first"
- Tier: 3
- Intent: What: run the whole setup (seats, landing lane, proofs, full suites) inside a VM instead of on the host. Why: Wido 2026-10-01, after asking whether the lane uses the VM for suite runs: 'I'm thinking of running all inside a VM anyways; that would do away with most of this anyways' - one environment for everything removes the host/VM split (lane proofs on host with selected tests, full suites by hand in Lima), host load and environment drift. Pros: one place to prove, full suites where the work runs, host stays clean. Cons: VM setup, performance and tooling (Claude Code, Codex, UI access) to be worked out. To be designed separately.
- Origin: human
- Next step: Design separately with Wido; no work before that
- OpenedAt: 2026-10-01T15:12:13Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-10-01T15:12:13Z 92NGH2XVZN50RA6CS22MSW1PFS-m1e-528c72bf open actor=human:Wido targets=whole-system-runs-in-a-vm
Integrity: sha256=0cc680b81a0f3849f7a6bbbd4626221cf25e59ae99f4957fd42307f4c564c0bd
