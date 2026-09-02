# dispatch-fixture-refused-by-goal-norm

- State: queued
- Intent: scripts/agents/dispatch-fixtures.sh scenario 'dispatch' has failed on main since the goal norm landed (84f847aa): its structured-budget claim asks reservedJobMinutesLimit 10000 and is refused GOAL_NORM_REFUSED (1440m norm), and set -e swallows the refusal so the scenario dies with no message. Found 2026-09-02 by m0b running the breach-clock build gate; reproduced on a clean export of origin/main.
- Origin: main
- Next step: Design first (R-38): decide whether the fixture should claim within the norm (e.g. 1000 minutes, adjusting its over-envelope arithmetic) or carry an --approved-ref, and make the refusal visible (assign the claim before grepping so the echo fires). Human budget needed before READY (R-13).
- OpenedAt: 2026-09-02T19:46:21Z
- Revision: 1

History:
- 2026-09-02T19:46:21Z T9AM2VDGK5KG6WMKAK5H7T5BXH-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=dispatch-fixture-refused-by-goal-norm
Integrity: sha256=6b133da9538f376958a8209e2cbda60499559853f1ffb82f40c19019eae3c19f
