# adoption-filled-delivery-passes-on-trunk

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: a deep section stays red and can hide new adoption regressions; novelty 2: the cause is not yet known; exposure 2: every run of the adoption deep section; accumulation 1: one scenario"
- Tier: 2
- Intent: The adoption bed's filled-delivery scenario fails on unmodified trunk 68682486 (a harness-tracked control by m1b on 2026-09-15) and was already red at a20afc40 on 2026-09-14. Inside the scenario, TestAdoptionComparisonSelectedScenarios in cmd/metasystem fails after about 197 seconds. The cause is not yet diagnosed. DONE: the cause is named with its evidence; filled-delivery passes on trunk under the harness; a regression leg or test pins the cause.
- Origin: human
- Next step: Diagnose first. Run scripts/adopt-fixtures.sh with only filled-delivery, as a harness child outside a delegate sandbox. Read the TestAdoptionComparisonSelectedScenarios failure from the go test JSON output and name the selected scenario that fails. Then fix and prove. Free for the next seat by sequence.
- OpenedAt: 2026-09-14T22:33:47Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T22:33:47Z CYFDAPTQHHHZF7Y88CH77JXAB3-m1e-c6925449 open actor=human:Wido targets=adoption-filled-delivery-passes-on-trunk
Integrity: sha256=a880f21d04520b75b6118c140d1861624e04dc942cb563614a00d7d0a8f9e299
