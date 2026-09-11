# brain-boot-sleep-fixture-matches-its-deadline

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The supervision-hook fixture's fake brain engine sleep failure mode (scripts/agents/supervision-hook-fixtures.sh line 376) sleeps 30 s once inside the failure-mode loop; it only needs to outlast the hook's boot deadline. DONE means the fixture lowers that deadline through its environment and shrinks the sleep to match, saving about 30 s per run.
- Origin: human
- Next step: Slice 4 item 4 of plans/suite-speed-plan.md. Keep the failure mode real: the fake must still exceed the deadline the hook enforces. Code critique only.
- OpenedAt: 2026-09-10T12:02:50Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:19Z revision=2 opid=89QWM1Y8MD36WC1Q1X4FZNW13G-m1-f47a9d40 authority=proven digest=e0927d3d6f076941ee5e5e4e75b74f9ed68eb462ee6547b18af6a13197b5123f

History:
- 2026-09-10T12:02:50Z EQ3JJ4W7KTC81BACDRSMTC0X3H-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=brain-boot-sleep-fixture-matches-its-deadline
- 2026-09-11T14:32:19Z 89QWM1Y8MD36WC1Q1X4FZNW13G-m1-f47a9d40 approve actor=human:Wido targets=brain-boot-sleep-fixture-matches-its-deadline
Integrity: sha256=a11e331e21a847bdea1b62939c7c291d82f5075fbf75b1814f35f2f14f11a9d4
