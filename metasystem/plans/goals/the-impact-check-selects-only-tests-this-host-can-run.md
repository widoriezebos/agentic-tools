# the-impact-check-selects-only-tests-this-host-can-run

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="test selection tooling only; it can only remove a false red; the lane batch proof is unchanged"
- Tier: 1
- Intent: The unit check's impact selection does not select a test whose build constraint excludes the running host (a //go:build linux test on this Mac) and does not count it as a missing red; the check's report names how many tests were not runnable here. Evidence: lane-steward U6a round 1, 2026-10-26, four Linux-only tests in diskstore, goal and proofrun were selected and counted missing, part of a red that cost a person's act.
- Origin: main
- Next step: Steward cuts one brief (one unit, about 30 production lines in the impact selection): read the test file's build constraint, drop tests the host cannot run from the selection and from the missing count, list them in the check's report; test: a linux-only test file on darwin is neither selected nor counted missing, and a test without a constraint is unchanged. One Astra round, machine route on ui after the queue ahead of it, lane landing.
- OpenedAt: 2026-10-26T07:13:34Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-10-26T07:13:34Z 10E9HVQGNXAVED7EQE60PRBA7K-m1e-acb921fd open actor=human:Wido targets=the-impact-check-selects-only-tests-this-host-can-run
Integrity: sha256=a4d1b8b5eb59b0c4b8f23b1ed7577da465a03ab9d814b39c9fd26693317f8213
