# a-correction-lands-from-the-checkout-top

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="path resolution on the commit path every correction uses; a wrong resolution commits the wrong files, caught by the unit's read and the lane proof before main"
- Tier: 2
- Intent: work review --changes --after COMMIT accepts a correction to a reviewed unit from the checkout top as well as from the metasystem/ folder: today the staged paths are made relative to the installation subdirectory and the comparison refuses "the goal branch doesn't hold what was staged" (internal/goal/branch/commit.go:634-652, :753-758), so on 2026-10-24/25 a correction had to be disguised as a new unit (U3b) and on 2026-10-26 one worked only because it ran from the workspace's metasystem/ folder.
- Origin: main
- Next step: One unit U1 (--last, production bound 60) from /Users/wido/agentic-tools-evidence/lane-test-20261009/briefs/a-correction-lands-from-the-checkout-top-U1-brief.md: treeWithoutPaths (internal/goal/branch/commit.go:634-652) runs read-tree/update-index/write-tree with repository-relative paths in the installation directory, so the skipped read of the superseded build stays in wantedTree and :758 refuses every correction after a published read in a nested checkout (findings 587, 11-09 and 11-10). Fix: run the index commands at the repository top. Struggle program item 1 (/Users/wido/agentic-tools-evidence/lane-test-20261009/designs/struggle-program-2026-11-10.md).
- OpenedAt: 2026-10-26T09:12:37Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-26T09:12:37Z 42SFHH9SHT6SR2W8KMB79WJWJ9-m1e-acb921fd open actor=human:Wido targets=a-correction-lands-from-the-checkout-top
- 2026-11-10T09:36:18Z VKGY5H33ZWBP6RBBP66BGA3S0A-m1e-8a741cac edit actor=human:Wido targets=a-correction-lands-from-the-checkout-top
Integrity: sha256=57d4afd8d1b07f39a5bc16829262efc1ca43f816bc0f138aa2c1a2214e557a00
