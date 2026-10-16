# the-lane-deploys-the-engine-it-runs

- State: approved
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="Lane test finding 344 and its addendum (2026-10-16: the lane ran two engine generations twice, landing deploy answered 'nothing to deploy' from the binary on disk, the machine-rebuild re-armed only the steward runner at 07:03:53); memory landing-deploy-nothing-to-deploy-needs-a-restart."
- Tier: 3
- Intent: The engine the lane runs is what its live processes execute: deploy, status and health compare the running processes' pinned engine digests with the engine main builds, never the binary on disk; every arm replaces the whole generation or reports that it could not, the ordinary arm and the detached rebuild included; a swap started during a proof is finished by the keeper before any automatic proof start; a records-only move rebuilds nothing.
- Origin: human
- Next step: U1 brief (briefs/the-lane-deploys-the-engine-it-runs-U1-brief.md, Astra rounds in progress, split U1a/U1b recorded) from the accepted page (designs/the-lane-deploys-the-engine-it-runs.md, Astra 3 → 3, stop rule); Codex build by hand in -deploy after sweep U1, Opus read, gate, hand landing.
- OpenedAt: 2026-10-16T09:24:05Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-16T09:24:13Z revision=2 opid=9WY73PJXDEGZZM3K2SF9TYHBEM-m1e-670f65d0 authority=proven digest=280467978d21c803d22fce699d3e19b018939901f98420636e6e8ea77a5a2c25 episode=2

History:
- 2026-10-16T09:24:05Z BTN2QW0TVD0F41BNSZAR0PBCZ5-m1e-670f65d0 open actor=human:Wido targets=the-lane-deploys-the-engine-it-runs
- 2026-10-16T09:24:13Z 9WY73PJXDEGZZM3K2SF9TYHBEM-m1e-670f65d0 approve actor=human:Wido targets=the-lane-deploys-the-engine-it-runs
Integrity: sha256=3f281ed514414e3fc20ce0adb6b16b419d13efc2cb60d362e5a972d53db70e53
