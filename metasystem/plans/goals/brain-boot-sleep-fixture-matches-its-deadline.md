# brain-boot-sleep-fixture-matches-its-deadline

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The supervision-hook fixture's fake brain engine sleep failure mode (scripts/agents/supervision-hook-fixtures.sh line 376) sleeps 30 s once inside the failure-mode loop; it only needs to outlast the hook's boot deadline. DONE means the fixture lowers that deadline through its environment and shrinks the sleep to match, saving about 30 s per run.
- Origin: human
- Next step: Slice 4 item 4 of plans/suite-speed-plan.md. Keep the failure mode real: the fake must still exceed the deadline the hook enforces. Code critique only.
- OpenedAt: 2026-09-10T12:02:50Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-10T12:02:50Z EQ3JJ4W7KTC81BACDRSMTC0X3H-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=brain-boot-sleep-fixture-matches-its-deadline
Integrity: sha256=56083f4c91bc584b3cd430730ae14e7765d8555137c286b1c2f6b56c27794ffa
