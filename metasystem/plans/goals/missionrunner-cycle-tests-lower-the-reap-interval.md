# missionrunner-cycle-tests-lower-the-reap-interval

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The TestInternalRun tests in internal/missionrunner take 15 to 24 s each; METASYSTEM_DRAIN_REAP_INTERVAL_MS defaults to 5000 in drain.go and only one test lowers it. DONE means one of those tests is measured with the interval at 300 ms; if the cycle waits on the reap, the package's TestMain sets it and the tests drop accordingly; if not, the finding is recorded and the goal concludes without a change.
- Origin: human
- Next step: Slice 4 item 6 of plans/suite-speed-plan.md: an investigation first, no number promised. Code critique only if code changes.
- OpenedAt: 2026-09-10T12:02:58Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:34Z revision=2 opid=5PWNFGGJPHQ6D2C88YKQQRG2NT-m1-f47a9d40 authority=proven digest=150fe12c222cfb17d28ef8f01e7df5cccefab868c5ef2f4d5ff80ed1c844d486

History:
- 2026-09-10T12:02:58Z PMW3CCRE4AGCB9NECMP7MF6J8W-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=missionrunner-cycle-tests-lower-the-reap-interval
- 2026-09-11T14:32:34Z 5PWNFGGJPHQ6D2C88YKQQRG2NT-m1-f47a9d40 approve actor=human:Wido targets=missionrunner-cycle-tests-lower-the-reap-interval
Integrity: sha256=8b4eb96227644e0861bcf0c4bfcc22f26401e78f846c48b084e76aabaa651156
