# the-impact-check-selects-only-tests-this-host-can-run

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="test selection tooling only; it can only remove a false red; the lane batch proof is unchanged"
- Tier: 1
- Intent: The unit check's impact selection does not select a test whose build constraint excludes the running host (a //go:build linux test on this Mac) and does not count it as a missing red; the check's report names how many tests were not runnable here. Evidence: lane-steward U6a round 1, 2026-10-26, four Linux-only tests in diskstore, goal and proofrun were selected and counted missing, part of a red that cost a person's act.
- Origin: main
- Next step: Steward cuts one brief (one unit, about 30 production lines in the impact selection): read the test file's build constraint, drop tests the host cannot run from the selection and from the missing count, list them in the check's report; test: a linux-only test file on darwin is neither selected nor counted missing, and a test without a constraint is unchanged. One Astra round, machine route on ui after the queue ahead of it, lane landing.
- OpenedAt: 2026-10-26T07:13:34Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-26T07:14:07Z revision=2 opid=SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd authority=proven digest=b91aeac063448cb8b6177037d212ee9de2771a8892533a24bf0d59cd2a6eb078 episode=2

History:
- 2026-10-26T07:13:34Z 10E9HVQGNXAVED7EQE60PRBA7K-m1e-acb921fd open actor=human:Wido targets=the-impact-check-selects-only-tests-this-host-can-run
- 2026-10-26T07:14:07Z SBX1SDWT81JEWAVEMWM2973Q53-m1e-acb921fd approve actor=human:Wido targets=a-capacity-refusal-is-an-environment-stop,the-impact-check-selects-only-tests-this-host-can-run,the-ui-bundle-builds-again
Integrity: sha256=3d7499e5b93685c6ff3b48140841045f6d3313d2af83d91a8a867e839b482190
