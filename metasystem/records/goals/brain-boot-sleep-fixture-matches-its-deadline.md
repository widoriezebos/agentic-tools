# brain-boot-sleep-fixture-matches-its-deadline

- State: done
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The supervision-hook fixture's fake brain engine sleep failure mode (scripts/agents/supervision-hook-fixtures.sh line 376) sleeps 30 s once inside the failure-mode loop; it only needs to outlast the hook's boot deadline. DONE means the fixture lowers that deadline through its environment and shrinks the sleep to match, saving about 30 s per run.
- Origin: human
- Next step: Slice 4 item 4 of plans/suite-speed-plan.md. Keep the failure mode real: the fake must still exceed the deadline the hook enforces. Code critique only.
- Concluded: Implemented by the coordinator on the m1e seat and landed by a human commit from the enrolled terminal in Wido's name (2026-09-11). The hook reads METASYSTEM_BRAIN_BOOT_DEADLINE_MS (default 5000, three seconds of grace) and the fixture's sleep failure mode sets 500 ms and sleeps 4 s instead of 30 s. Verified once on the candidate tree: the supervision-hook fixture green standalone in 212 s, every failure mode still reporting its one by-hand notice. Landed 0b6295c1.
- OpenedAt: 2026-09-10T12:02:50Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:19Z revision=2 opid=89QWM1Y8MD36WC1Q1X4FZNW13G-m1-f47a9d40 authority=proven digest=e0927d3d6f076941ee5e5e4e75b74f9ed68eb462ee6547b18af6a13197b5123f

History:
- 2026-09-10T12:02:50Z EQ3JJ4W7KTC81BACDRSMTC0X3H-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=brain-boot-sleep-fixture-matches-its-deadline
- 2026-09-11T14:32:19Z 89QWM1Y8MD36WC1Q1X4FZNW13G-m1-f47a9d40 approve actor=human:Wido targets=brain-boot-sleep-fixture-matches-its-deadline
- 2026-09-11T15:01:47Z 58EHC1WQTMB2B28F7A6XGGG1KY-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=brain-boot-sleep-fixture-matches-its-deadline
- 2026-09-11T15:01:55Z JTCJHVQQR9HPHBRF36JPQ6E76J-m1-c6925449 done actor=human:Wido targets=brain-boot-sleep-fixture-matches-its-deadline displaced=m1e+main-1789030447-51011-5722fc@2026-09-11T15:01:47Z
Integrity: sha256=d82f00fe53e9b0ef25fb159b4812545ece381fa420da82d2fb3fcd5f867f1574
