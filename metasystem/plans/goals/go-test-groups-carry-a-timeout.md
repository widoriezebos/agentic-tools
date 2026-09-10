# go-test-groups-carry-a-timeout

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a false red on the biggest package under ordinary load; novelty 1: one flag and one reason string; exposure 2: every receipt touching internal/goal or missionrunner; accumulation 1: nothing built on it"
- Tier: 2
- Intent: DUPLICATE, opened in error by m1b on 2026-09-10 13:55Z: the same defect is goal-suite-sits-on-the-default-test-timeout, which now carries this record's evidence (the receipt group goal-full-coverage cut at 10m0s under load, proof run proof-mtvb7pc9-8ce278d20b305ce2). This record holds no work and must not be claimed; it is the specimen for goal abandon (goal-abandoned-with-a-reason) once that verb lands, and stays queued and unranked until then.
- Origin: main
- Next step: Do not claim. Abandon with --because 'duplicate of goal-suite-sits-on-the-default-test-timeout' when goal abandon exists.
- OpenedAt: 2026-09-10T10:05:24Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T10:05:24Z N9MGEVG7F9553P8AJ4XPC5XY5N-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=go-test-groups-carry-a-timeout
- 2026-09-10T10:05:56Z TETZMDY2C7CVEH93J829P95WVS-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=go-test-groups-carry-a-timeout
Integrity: sha256=4adc1871cf8f2bf2bc1a05ca1b821cd78bdf8cebcedd2af1d0eee5a89704f653
