# goal-package-coverage-under-its-floor-since-ce59b313

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the cadence battery is red on goal-full-coverage for every seat until the floor is met; novelty 1: tests for new verbs; exposure 3: every cadence run; accumulation 1: none"
- Tier: 3
- Intent: The race gate's coverage ratchet passes again at cadence. Cadence run 9 (proof-mtxgkya4, 2026-09-12) passed every test and failed only on the ratchet: internal/goal 78.3 percent against its 80.0 floor and internal/counselor 83.6 against 83.7, both from ce59b313 (A verified human can carry one landing past one named refusal, 2026-09-11 22:22, landed by human commit from another seat: about 1300 lines in internal/goal/verbs.go with one test line, 395 in internal/counselor); and internal/supervise 89.0 against 89.5 and internal/validate 63.6 against 63.9 with no commit since their floors were set: timing-dependent branches under a loaded box against floors at the measured maximum. This blocks deep-battery-under-ten-minutes, whose DONE needs three green cadence runs. DONE means tests for the added verbs and counselor paths bring both packages to their floors, the supervise and validate dips are explained (re-measured floors by a recorded human word, or a tolerance the ratchet law grants), and the gate's ratchet is green in a cadence run.
- Origin: main
- Next step: Owned by the seat that landed ce59b313 or the next seat free: read git show ce59b313 -- metasystem/internal/goal/verbs.go, write the tests for the new verb paths, run go test -cover ./internal/goal until it reads 80.0 or more, land by the working rule R-98-m1e.
- Concluded: The race gate's coverage ratchet is green at cadence since run 15 (proof-mty4qhaz-af176dcea378dae3, 2026-09-12): internal/goal lifted to 82.5 percent merged over its shards by the carry lifecycle, guards and declare-free tests (4d91c9ec), internal/counselor to 85.2 by the accepted-risk validator test (4d91c9ec), internal/supervise to 89.8 and internal/validate to 66.4 by tests on Wido's word to add tests rather than move floors again (0edbc3d8); the supervise and validate floors moved once to their loaded reading under R-100-m1e (7a20a58c); three packages that never had a floor (stopfence, stoptransition, testpolicy) were registered half a point under their reading (1dd00469).
- OpenedAt: 2026-09-11T21:31:44Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=600 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-12T05:47:12Z revision=3 opid=EASD9YAB648N9KXW0C3ER3GBPJ-m1-c6925449 authority=proven digest=451435847ffb501871723956bca372aa59026c762e44905826e71fb468b9873f

History:
- 2026-09-11T21:31:44Z V4M9FR9Y5NZA22X0QAYC0AQCPP-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=goal-package-coverage-under-its-floor-since-ce59b313
- 2026-09-11T21:35:27Z 7HVE3KA3N6X8X8RJVTEJBCJ727-m1e-892cdaec edit actor=m1e+main-1789030447-51011-5722fc targets=goal-package-coverage-under-its-floor-since-ce59b313
- 2026-09-12T05:47:12Z EASD9YAB648N9KXW0C3ER3GBPJ-m1-c6925449 approve actor=human:Wido targets=goal-package-coverage-under-its-floor-since-ce59b313
- 2026-09-12T11:22:39Z SXYT1RHFAK3MSCKDCJQ0B1AKZT-m1-c6925449 done actor=human:Wido targets=goal-package-coverage-under-its-floor-since-ce59b313
Integrity: sha256=37634a7fdfc2d695f193f874a4b1f4eb53f7c0d4a7752187d7be2e66452fa44c
