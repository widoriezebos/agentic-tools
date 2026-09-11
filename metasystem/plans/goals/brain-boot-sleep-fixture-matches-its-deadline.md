# brain-boot-sleep-fixture-matches-its-deadline

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The supervision-hook fixture's fake brain engine sleep failure mode (scripts/agents/supervision-hook-fixtures.sh line 376) sleeps 30 s once inside the failure-mode loop; it only needs to outlast the hook's boot deadline. DONE means the fixture lowers that deadline through its environment and shrinks the sleep to match, saving about 30 s per run.
- Origin: human
- Next step: Slice 4 item 4 of plans/suite-speed-plan.md. Keep the failure mode real: the fake must still exceed the deadline the hook enforces. Code critique only.
- OpenedAt: 2026-09-10T12:02:50Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:19Z revision=2 opid=89QWM1Y8MD36WC1Q1X4FZNW13G-m1-f47a9d40 authority=proven digest=e0927d3d6f076941ee5e5e4e75b74f9ed68eb462ee6547b18af6a13197b5123f
- Claimed: machine=m1e lineage=main-1789030447-51011-5722fc at=2026-09-11T15:01:47Z revision=3 accountingRevision=3 episodeAt=2026-09-11T15:01:47Z episodeRevision=3
- StopCapability: generation=3 revision=3 machine=m1e claimEpoch=1 fenceEpoch=0

History:
- 2026-09-10T12:02:50Z EQ3JJ4W7KTC81BACDRSMTC0X3H-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=brain-boot-sleep-fixture-matches-its-deadline
- 2026-09-11T14:32:19Z 89QWM1Y8MD36WC1Q1X4FZNW13G-m1-f47a9d40 approve actor=human:Wido targets=brain-boot-sleep-fixture-matches-its-deadline
- 2026-09-11T15:01:47Z 58EHC1WQTMB2B28F7A6XGGG1KY-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=brain-boot-sleep-fixture-matches-its-deadline
Integrity: sha256=15cb826da31a34fd24fce8c39758ec6b50b8c136807a37ffc6aa0cc4559cd3bb
