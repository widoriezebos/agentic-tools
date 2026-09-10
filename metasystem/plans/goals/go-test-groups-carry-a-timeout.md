# go-test-groups-carry-a-timeout

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="severity 2: a false red on the biggest package under ordinary load; novelty 1: one flag and one reason string; exposure 2: every receipt touching internal/goal or missionrunner; accumulation 1: nothing built on it"
- Tier: 2
- Intent: The testing contract's go adapter (internal/proofrun/test_build.go) runs a group's packages with go test and passes no -timeout, so Go's default of ten minutes applies. The goal-full-coverage group (internal/goal, coverage on) takes about 9.5 minutes on a quiet machine and more under load: on 2026-09-10 11:53 a schema-2 receipt for goal receipt-beds-run-the-candidate-engine (proof run proof-mtvb7pc9-8ce278d20b305ce2) failed only that group with 'panic: test timed out after 10m0s' at 600.265 s while a build round and a critic read shared the machine (load average 11.9); the same group passed on a lighter receipt two hours earlier. A receipt that fails by wall clock rather than by a test is a false red that costs an hour and a relaunch. DONE means: every go group runs with an explicit -timeout derived from its declared cap (the group's cap in the contract, or the section cap the receipt already enforces), never Go's default; a group that exceeds its cap is reported as capped, not as a package failure, with the cap and the elapsed time in the reason; a fixture proves a group whose cap is below its runtime is reported capped and a group with a cap above it passes.
- Origin: main
- Next step: Tier 2, MECHANICAL: the go adapter's argv gains -timeout from the group's cap, the result classifier gains the capped reason, one fixture. Opened by m1b 2026-09-10 13:55Z. Related: machine-concurrency-governor (load), closing-read-and-receipt-in-parallel (the practice that raised the load).
- OpenedAt: 2026-09-10T10:05:24Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T10:05:24Z N9MGEVG7F9553P8AJ4XPC5XY5N-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=go-test-groups-carry-a-timeout
Integrity: sha256=c0d1c4fa06bb17fb728e107d6340167708c84e695cde2033b65f2c2c5aad6ec6
