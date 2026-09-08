# dispatch-fixture-refused-by-goal-norm

- State: queued
- Priority: 3
- Sequence: 73
- Intent: scripts/agents/dispatch-fixtures.sh scenario 'dispatch' has failed on main since the goal norm landed (84f847aa): its structured-budget claim asks reservedJobMinutesLimit 10000 and is refused GOAL_NORM_REFUSED (1440m norm), and set -e swallows the refusal so the scenario dies with no message. Found 2026-09-02 by m0b running the breach-clock build gate; reproduced on a clean export of origin/main.
- Origin: main
- Next step: Design first (R-38): decide whether the fixture should claim within the norm (e.g. 1000 minutes, adjusting its over-envelope arithmetic) or carry an --approved-ref, and make the refusal visible (assign the claim before grepping so the echo fires). Human budget needed before READY (R-13).
- OpenedAt: 2026-09-02T19:46:21Z
- Revision: 2
- BudgetExceptions: 0

History:
- 2026-09-02T19:46:21Z T9AM2VDGK5KG6WMKAK5H7T5BXH-m0b-6638932d open actor=m0b+main-1788250419-3170380-8a1fb3 targets=dispatch-fixture-refused-by-goal-norm
- 2026-09-08T16:03:24Z CHVCZR3KZ9QAPG9EAX2AFSSM0D-m1-7cd0bd60 set-priority actor=human:Wido targets=dispatch-fixture-refused-by-goal-norm reason=priority-order subject=dispatch-fixture-refused-by-goal-norm from=unranked to=3:73 requested-sequence=73
Integrity: sha256=94a262480449dc8a6f83abe023afd4e8c31ae7c047f6d912af465aaa743febf1
