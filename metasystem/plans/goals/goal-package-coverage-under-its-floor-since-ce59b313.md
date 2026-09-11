# goal-package-coverage-under-its-floor-since-ce59b313

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: the cadence battery is red on goal-full-coverage for every seat until the floor is met; novelty 1: tests for new verbs; exposure 3: every cadence run; accumulation 1: none"
- Tier: 3
- Intent: The internal/goal package's coverage is at or above its ratchet floor of 80.0 percent again. ce59b313 (A verified human can carry one landing past one named refusal, 2026-09-11 22:22, landed by human commit from another seat) added about 1300 lines to internal/goal/verbs.go, 18 to validate.go and 41 to txn.go with one test line in txn_test.go; since then the merged package coverage reads 78.3 percent in cadence runs 8 (six shards) and 9 (four shards), the identical figure both times, where every earlier run passed the floor. This blocks deep-battery-under-ten-minutes, whose DONE needs three green cadence runs. DONE means tests for the added verbs bring the package to the floor, or the floor is lowered by a recorded human word; proven by goal-full-coverage green in a cadence run.
- Origin: main
- Next step: Owned by the seat that landed ce59b313 or the next seat free: read git show ce59b313 -- metasystem/internal/goal/verbs.go, write the tests for the new verb paths, run go test -cover ./internal/goal until it reads 80.0 or more, land by the working rule R-98-m1e.
- OpenedAt: 2026-09-11T21:31:44Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T21:31:44Z V4M9FR9Y5NZA22X0QAYC0AQCPP-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=goal-package-coverage-under-its-floor-since-ce59b313
Integrity: sha256=68f6d2fe553434d1af1bc127744e198474fcd79c24a0817c3d5781af1e03c441
