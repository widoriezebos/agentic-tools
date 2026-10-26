# a-correction-lands-from-the-checkout-top

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="path resolution on the commit path every correction uses; a wrong resolution commits the wrong files, caught by the unit's read and the lane proof before main"
- Tier: 2
- Intent: work review --changes --after COMMIT accepts a correction to a reviewed unit from the checkout top as well as from the metasystem/ folder: today the staged paths are made relative to the installation subdirectory and the comparison refuses "the goal branch doesn't hold what was staged" (internal/goal/branch/commit.go:634-652, :753-758), so on 2026-10-24/25 a correction had to be disguised as a new unit (U3b) and on 2026-10-26 one worked only because it ran from the workspace's metasystem/ folder.
- Origin: main
- Next step: Steward cuts one brief (one unit, about 30 production lines): resolve staged paths against the repository root the branch records, whatever the caller's directory; tests: a correction submitted from the checkout top after a published review commits as the unit's next version; the same from metasystem/ is unchanged; a correction that touches files outside the unit is still refused. One Astra round, machine route on ui after goal-done-reads-the-ledger, lane landing.
- OpenedAt: 2026-10-26T09:12:37Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-26T09:12:37Z 42SFHH9SHT6SR2W8KMB79WJWJ9-m1e-acb921fd open actor=human:Wido targets=a-correction-lands-from-the-checkout-top
Integrity: sha256=74eb003768b85c81fd14b7396fbcb08e9d8867ac3e1571eca025b150ba2c0d86
