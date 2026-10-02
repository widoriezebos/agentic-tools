# test-selection-improves-on-the-simple-rule

- State: abandoned
- Risk: severity=1 novelty=2 exposure=2 accumulation=1 basis="severity 1: a selection that misses a red is caught by the full run once per batch; novelty 2: replay over recorded gates has not been done; exposure 2: every fix round and every re-proof after an eject; accumulation 1: one decision and at most three units"
- Tier: 2
- Intent: What: A check with real data of whether choosing which tests to rerun in a fix round can be done better than the current simple rule (rerun the failing tests, the tests that name what changed, and whole packages the fix touches). Why: A deeper test-selection design exists and part of it was built on a side branch, but Wido ruled: no added complexity unless the benefit is real. That benefit has never been measured. Pros: More is built only if the numbers show a gain, which avoids complexity. Cons: It needs the lane to run for a while after switch-on to collect data, and the parked branch keeps ageing.
- Origin: human
- Next step: Next: After switch-on, replay the simple rule over the lane's recorded failing batches (at least ten fix rounds), recording per round the tests chosen, the time taken and whether every failure was caught. Build nothing unless the replay shows a failure the simple rule missed or a fix round made twice as fast. This goal also takes over blast-radius-beside-risk. Done when: a replay report exists, and either its numbers justify a deeper unit or the goal is concluded.
- OpenedAt: 2026-09-19T20:55:29Z
- Revision: 5
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Abandoned: by=human:Wido at=2026-10-02T21:01:33Z revision=5 opid=VEPYMFHJ26MJNGRHDMC3XCESWH-m1e-9c612d71 because=Obsolete (audit 2026-10-02): the plain lane no longer selects tests per fix round

History:
- 2026-09-19T20:55:29Z WMKSS1T0PK4C19YD40KEWPE8T7-m1e-c6925449 open actor=human:Wido targets=test-selection-improves-on-the-simple-rule
- 2026-09-19T21:25:51Z 5TCPEZJHR5X967KZ56J314FSKF-m1e-c19f13ae edit actor=m1e+main-1789561396-11601-6f713b targets=test-selection-improves-on-the-simple-rule
- 2026-09-19T21:27:30Z A207EBSDVH9084EC5112AJ8QZT-m1e-c19f13ae edit actor=m1e+main-1789561396-11601-6f713b targets=test-selection-improves-on-the-simple-rule
- 2026-09-30T18:46:39Z V57HTYTQSEDRFFQHV3HE3ZG48H-m1e-b6a4eb0a edit actor=human:wido targets=test-selection-improves-on-the-simple-rule
- 2026-10-02T21:01:33Z VEPYMFHJ26MJNGRHDMC3XCESWH-m1e-9c612d71 abandon actor=human:Wido targets=test-selection-improves-on-the-simple-rule reason=Obsolete (audit 2026-10-02): the plain lane no longer selects tests per fix round
Integrity: sha256=31a1617c114ac19758ff6265f1ce61ff9ff80e8f8f66df99eb3576026110dda8
