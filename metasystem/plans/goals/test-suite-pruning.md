# test-suite-pruning

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Removing useful tests could permit regressions; the mechanisms are existing tests and fixtures; the suite runs on every development host and cost accumulates with repeated runs. This item records queued cleanup, without claiming execution or approving a budget."
- Tier: 2
- Intent: At Wido request, prune duplicate and non-beneficial tests as separate work from risk-based execution modes. Reduce recurring validation cost while preserving distinct defect detection.
- Origin: main
- Next step: Inventory the most expensive test groups; identify each candidate test unique behavior and failure signal, equivalent retained coverage and measured runtime. Propose deletions or consolidation with evidence before changing tests. Keep this separate from the common application test interface and coordinator-loop-prevention delivery.
- OpenedAt: 2026-09-08T16:13:31Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-08T16:13:31Z MVQB2N9NB40E5JN0G531CG0793-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=test-suite-pruning
- 2026-09-09T20:13:19Z QK94VV2Y4T80PS4CEPH86Y2RVK-m1c-8d678ae8 edit actor=m1c+main-1788963308-60248-b019cb targets=test-suite-pruning
Integrity: sha256=7fb68fef3216f02a0e2b39d6f7906ae92c3776a4fe8c4ed2ed6ae454e7399438
