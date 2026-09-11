# goal-package-coverage-under-its-floor-since-ce59b313

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the cadence battery is red on goal-full-coverage for every seat until the floor is met; novelty 1: tests for new verbs; exposure 3: every cadence run; accumulation 1: none"
- Tier: 3
- Intent: The race gate's coverage ratchet passes again at cadence. Cadence run 9 (proof-mtxgkya4, 2026-09-12) passed every test and failed only on the ratchet: internal/goal 78.3 percent against its 80.0 floor and internal/counselor 83.6 against 83.7, both from ce59b313 (A verified human can carry one landing past one named refusal, 2026-09-11 22:22, landed by human commit from another seat: about 1300 lines in internal/goal/verbs.go with one test line, 395 in internal/counselor); and internal/supervise 89.0 against 89.5 and internal/validate 63.6 against 63.9 with no commit since their floors were set: timing-dependent branches under a loaded box against floors at the measured maximum. This blocks deep-battery-under-ten-minutes, whose DONE needs three green cadence runs. DONE means tests for the added verbs and counselor paths bring both packages to their floors, the supervise and validate dips are explained (re-measured floors by a recorded human word, or a tolerance the ratchet law grants), and the gate's ratchet is green in a cadence run.
- Origin: main
- Next step: Owned by the seat that landed ce59b313 or the next seat free: read git show ce59b313 -- metasystem/internal/goal/verbs.go, write the tests for the new verb paths, run go test -cover ./internal/goal until it reads 80.0 or more, land by the working rule R-98-m1e.
- OpenedAt: 2026-09-11T21:31:44Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T21:31:44Z V4M9FR9Y5NZA22X0QAYC0AQCPP-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=goal-package-coverage-under-its-floor-since-ce59b313
- 2026-09-11T21:35:27Z 7HVE3KA3N6X8X8RJVTEJBCJ727-m1e-892cdaec edit actor=m1e+main-1789030447-51011-5722fc targets=goal-package-coverage-under-its-floor-since-ce59b313
Integrity: sha256=ab1368cb5d5ff0e9b4e55e89ad59aca5c71ab3e28268ab90db06bfb0c75eebdc
