# budget-origin-landing-leaves-main-red

- State: done
- Risk: severity=3 novelty=1 exposure=3 accumulation=2 basis="severity 3: every deep landing on the fleet fails on two groups no candidate can affect, and ledgers the new engine writes are unreadable to any pinned older engine; novelty 1: a validation check contradicts an existing fixture and a claim record grew a key the closed grammar of older engines refuses; exposure 3: every seat and every landing; accumulation 2: each claim written since 12:03Z adds a record older engines cannot parse"
- Tier: 3
- Intent: Main is green again after 5b51fe4c (2026-09-11 12:03Z, Preserve the elapsed budget origin across approved raises). Since that landing a clean tip worktree fails TestSeverityTieredRigorUtilityWrappers in internal/goal in under a second: SetBudgetApproved on the utility-goal fixture is rejected with 'claimed episodeAt=2026-08-30T08:05:00Z is later than claimed at=2026-08-20T22:00:00Z' (file.go line 724), and the land-fixtures receipt-cutover scenario fails because its pinned older engine cannot parse the ledger the candidate writes: 'plans/goals/fx.md: line 14: Claimed: unknown key episodeAt, the record grammar is closed'. Receipt attempt proof-mtwuyrew (tree 77903fc3, 42 groups, 67 minutes, reproduced by m1e) failed on exactly those two groups. DONE means goal-full-coverage and section/land-fixtures pass on a clean tip, by forward fix or by exact revert of 5b51fe4c, and the claim grammar change is either compatible with the pinned engine the cutover fixture builds or that fixture pins the engine that knows it.
- Origin: main
- Next step: Owner of the elapsed-origin design fixes forward (or m1e lands an exact revert on Wido's word): reconcile the episodeAt-before-claim check with the fixture, and make the cutover fixture's pinned engine tolerate or pin past the new claim key.
- Concluded: Landed f1954e86 (2026-09-11 13:02Z) by a human commit from the enrolled terminal in Wido's name: the test's clock follows its fixture's claim, and the cutover leg's seed goal is claimed by the pinned pre-cutover engine. Verified by hand on a clean tip worktree: go test ./internal/goal green in 738 s, land-fixtures bed green in 283 s (13 legs, receipt-cutover passed). The agent lanes could not carry the fix: behavior paths sit on the never-direct-fix floor and commit.sh demands a deep proof that cannot pass on a red main.
- OpenedAt: 2026-09-11T12:24:15Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T12:24:15Z CKBGDG8Y7Z4PC6ZYXGBGHJ2XKC-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=budget-origin-landing-leaves-main-red
- 2026-09-11T13:01:25Z XZ6RGMHDBFJXBD9J8GGZ53RKX1-m1e-892cdaec done actor=m1e+main-1789030447-51011-5722fc targets=budget-origin-landing-leaves-main-red
Integrity: sha256=0412cecafe7fe38ad13072188d641f47deff8bd48c4bd1febcaa7871688598c1
