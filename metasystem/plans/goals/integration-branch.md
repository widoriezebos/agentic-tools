# integration-branch

- State: queued
- Intent: The branch all work integrates into is designated, not assumed: default is the repo's default branch, but a development branch can be named instead — so protected-branch rules (PR-only merges to master) never block the metasystem's own landing flow (Wido's request 2026-08-24, low priority)
- Origin: human
- Next step: NEEDS PROPER DESIGN (Wido 2026-08-24) — a design loop with critique per the covenant, not a config flag. The design must carry these recorded considerations: (1) TWO GOVERNANCE SYSTEMS — GitHub branch protection is an organizational gate for human teams; the metasystem's own machinery (review chains, the wall, warden custody, captured-rc landing gates) is a stronger machine-paced gate for the same purpose. The designated integration branch is the BOUNDARY between them: the metasystem governs everything flowing into the development branch at machine cadence; the organization's PR rules govern promotion from development to master. (2) PROMOTION STAYS A HUMAN DECISION POINT — dev-to-master is a deliberate act (a human merge, or a PR the metasystem opens and a human approves), never eroded by automation pressure. (3) THE GOAL LEDGER FOLLOWS THE DESIGNATION — backlog publication is compare-and-swap against a canonical branch; if landings move to a development branch the ledger's home must move with it or the two drift. (4) THE TWO-PART LAW — the designation is repo-side configuration, owned by the app, never overwritten by a metasystem update. (5) INCEPTION AFFINITY — this matters the day the metasystem builds apps in org repos with protected branches; pull forward if inception's design wants it. Default remains the repo's default branch; naming a development branch is the override. Design slice 4-6h (inventory every default-branch assumption: landing pushes, mission branch targeting, goal-verb publication, adoption baselines), then critique, then a tokened implementation slice. LOW PRIORITY, queued behind the app-guardrail program.
- OpenedAt: 2026-08-23T19:32:07Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-23T19:32:07Z DXYC8H8AP7QBP0ZVBAZ294W0ZJ-m1-bf243850 open actor=human:wido targets=integration-branch
- 2026-08-23T19:34:35Z EG4BSC1H49XCZEP3ET8KH85ZH0-m1-bf243850 edit actor=human:wido targets=integration-branch
- 2026-08-23T19:45:10Z 1Q961KF373XSSJZX0CSDW5ASWV-m1-bf243850 edit actor=human:wido targets=integration-branch
- 2026-09-01T20:26:56Z GD1NGJDVMPM0SF88K4APACD1AQ-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=integration-branch
Integrity: sha256=a7a1b930121b1577e2ad553ae9a4974cd97f431baf4d4156be88164b1fc13fad
