# test-suite-pruning

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Removing useful tests could permit regressions; the mechanisms are existing tests and fixtures; the suite runs on every development host and cost accumulates with repeated runs. This item records queued cleanup, without claiming execution or approving a budget."
- Tier: 2
- Intent: At Wido request, prune duplicate and non-beneficial tests as separate work from risk-based execution modes. Reduce recurring validation cost while preserving distinct defect detection.
- Origin: main
- Next step: Inventory the most expensive test groups; identify each candidate test unique behavior and failure signal, equivalent retained coverage and measured runtime. Propose deletions or consolidation with evidence before changing tests. Keep this separate from the common application test interface and coordinator-loop-prevention delivery.
- OpenedAt: 2026-09-08T16:13:31Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-08T16:13:31Z MVQB2N9NB40E5JN0G531CG0793-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=test-suite-pruning
Integrity: sha256=b6da6d1d4c7fb362ba822f7a73117efe804d6a1fedd8cd9224266c0dab85c13d
