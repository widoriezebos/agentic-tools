# the-lane-deploys-the-engine-it-runs

- State: queued
- Risk: severity=3 novelty=2 exposure=2 accumulation=2 basis="Lane test finding 344 and its addendum (2026-10-16: the lane ran two engine generations twice, landing deploy answered 'nothing to deploy' from the binary on disk, the machine-rebuild re-armed only the steward runner at 07:03:53); memory landing-deploy-nothing-to-deploy-needs-a-restart."
- Tier: 3
- Intent: The engine the lane runs is what its live processes execute: deploy, status and health compare the running processes' pinned engine digests with the engine main builds, never the binary on disk; every arm replaces the whole generation or reports that it could not, the ordinary arm and the detached rebuild included; a swap started during a proof is finished by the keeper before any automatic proof start; a records-only move rebuilds nothing.
- Origin: human
- Next step: U1 brief (briefs/the-lane-deploys-the-engine-it-runs-U1-brief.md, Astra rounds in progress, split U1a/U1b recorded) from the accepted page (designs/the-lane-deploys-the-engine-it-runs.md, Astra 3 → 3, stop rule); Codex build by hand in -deploy after sweep U1, Opus read, gate, hand landing.
- OpenedAt: 2026-10-16T09:24:05Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-16T09:24:05Z BTN2QW0TVD0F41BNSZAR0PBCZ5-m1e-670f65d0 open actor=human:Wido targets=the-lane-deploys-the-engine-it-runs
Integrity: sha256=537fc7493fb212b7e1722fde215b1ae6fb5f094a25558cfa9208b76b333b8074
