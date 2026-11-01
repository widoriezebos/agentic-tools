# a-push-folds-a-records-only-move

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="lane push path; a wrong fold pushes an unproven code change (bounded by the trunk check); every landing is exposed; recurs on every records move during a proof"
- Tier: 2
- Intent: The lane push folds a records-only move of main into a proven batch without a second proof: when every commit main gained since the batch base is records-only by the classifier of a-records-only-move-keeps-mains-green, the merged tree is pushed on the batch proof (or after the cheap check that classifier already prescribes); the-steward-sees stood an hour green at 03:55-06:32 on 2026-11-01 for one goal-edit commit (finding 460)
- Origin: human
- Next step: Build without a design from finding 460: the push path reuses the records-only classifier; one unit; brief by the seat
- OpenedAt: 2026-11-01T06:40:58Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-01T06:40:58Z T05J239XGRNW745CQ4PGG88T1B-m1e-cbd14c5a open actor=human:Wido targets=a-push-folds-a-records-only-move
Integrity: sha256=0001ae7098e2b79c4db18ab983f172e4971ae49db93701e9b1268cfa58ea54c1
