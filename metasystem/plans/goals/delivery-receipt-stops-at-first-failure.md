# delivery-receipt-stops-at-first-failure

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the receipt is the landing judge; novelty 1: the runner loop exists and gains a purpose-dependent stop; exposure 3: every delivery attempt; accumulation 1: cadence unchanged"
- Tier: 3
- Intent: 31 failed attempts consumed 26 hours, 20.7 of them running groups that had already passed, because the runner continues after the first failure under ruling R-16 (proof-attempts.md finding 3 of the delivery deep dive). DONE means: a delivery-purpose attempt stops launching new groups at the first failed group, records the remaining groups as not-run with the reason and preserves the failure evidence; cadence-purpose attempts keep continue-and-collect; the retry launches only the failed and not-run groups and reuses the passed groups of the failed attempt; proven by a fixture with a failing second group and, in the field, by failed-attempt mean duration under 15 minutes over a week. The amendment of R-16 for the delivery purpose is Wido's to confirm on the design page. Goal 8 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Read RunTestPlan in internal/proofrun/test_build.go and R-16 in memory/rulings.md, design the purpose-dependent stop and the retry projection, critique, build, land with its own battery.
- OpenedAt: 2026-09-11T15:45:11Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:45:11Z RKPY1HJBS8V1X9RAP3GX84EXYT-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=delivery-receipt-stops-at-first-failure
Integrity: sha256=f55fa409d0b1f54a28c921b26839804a6c9c65dd4b4fbb43bd100198f78e19a3
