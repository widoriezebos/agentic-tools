# go-test-groups-carry-a-timeout

- State: done
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a false red on the biggest package under ordinary load; novelty 1: one flag and one reason string; exposure 2: every receipt touching internal/goal or missionrunner; accumulation 1: nothing built on it"
- Tier: 2
- Intent: DUPLICATE, opened in error by m1b on 2026-09-10 13:55Z: the same defect is goal-suite-sits-on-the-default-test-timeout, which now carries this record's evidence (the receipt group goal-full-coverage cut at 10m0s under load, proof run proof-mtvb7pc9-8ce278d20b305ce2). This record holds no work and must not be claimed; it is the specimen for goal abandon (goal-abandoned-with-a-reason) once that verb lands, and stays queued and unranked until then.
- Origin: main
- Next step: Do not claim. Abandon with --because 'duplicate of goal-suite-sits-on-the-default-test-timeout' when goal abandon exists.
- Concluded: Landed: The record's only evidence (receipt group goal-full-coverage cut at 10m0s under load, proof-mtvb7pc9) is fixed by c08fe2cb (a go test group is bounded by what it consumes, -timeout 0); the record calls itself a duplicate of goal-suite-sits-on-the-default-test-timeout and holds no work. Concluded 2026-09-11 in the backlog consolidation on Wido's word, no further work.
- OpenedAt: 2026-09-10T10:05:24Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T10:05:24Z N9MGEVG7F9553P8AJ4XPC5XY5N-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=go-test-groups-carry-a-timeout
- 2026-09-10T10:05:56Z TETZMDY2C7CVEH93J829P95WVS-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=go-test-groups-carry-a-timeout
- 2026-09-11T22:03:05Z D5ZPA78E39F6DBMCJ1ZHXRN6Z0-m1-c6925449 done actor=human:Wido targets=go-test-groups-carry-a-timeout
Integrity: sha256=b44b5dbe971c7e5f95f8567a88fe7a299e5ea3d9cfb218ca0b9808091d10de12
