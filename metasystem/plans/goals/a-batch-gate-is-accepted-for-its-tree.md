# a-batch-gate-is-accepted-for-its-tree

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, root cause read in a recorded finding (556, 557); the unit check and the lane proof gate it"
- Tier: 2
- Intent: A recorded batch gate is accepted again for the same tree (Skipped result, Green static check, same BaseCommit and PlanHash) instead of only for the same commit, so a re-merge after an abandoned resolution does not run the gate twice (finding 556; internal/landing/plain/impact.go:103-105, prove.go:467, scope.go:498)
- Origin: human
- Next step: Build without a design from finding 556: key the gate acceptance on the tree; one test with two merge commits of one tree; about 20 lines
- OpenedAt: 2026-11-07T15:02:25Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T15:02:25Z FYXYRR9049G82FDFAZ9HG6HHCX-m1e-5a3e856b open actor=human:Wido targets=a-batch-gate-is-accepted-for-its-tree
Integrity: sha256=cae0cb171c85fd034d39be4491aa088ec2774faa97f35f7b87c34132a0aee409
