# goal-suite-sits-on-the-default-test-timeout

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: a false red, no product effect, but it costs a verification cycle and invites the wrong diagnosis; novelty 1: a flag, a split, or a faster fixture path; exposure 2: every seat that runs go test on this package without the flag, which is the ordinary way to verify a change; accumulation 2: the suite has 363 test functions and grows, so the margin only shrinks"
- Tier: 2
- Intent: internal/goal's test suite runs within a minute of go test's default ten-minute timeout, so any load tips it into a panic that reads as a hang in whichever test was running. Measured on this Mac 2026-09-07 on one tree: 494s and 526s and 550s in quiet runs, 602s then a timeout panic in TestParkCascadePinsOneAcknowledgment while a delegate job ran alongside; the same package on the same tree passed with -timeout 30m at 494s. The suite is slow because 363 test functions each drive real git transactions through goalGit and PublishCAS. The landing gate is unaffected because scripts/agents/go-gate.sh passes -timeout 60m, so this bites humans and orchestrators running the package directly, and the failure names an innocent test. DONE means the package's own suite cannot be tipped by ambient load: either the slow git-driving tests share one prepared ledger instead of each building their own, or the package declares its own timeout so the ceiling does not depend on the caller, and a fixture or a recorded measurement proves the margin
- Origin: main
- Next step: measure first: go test ./internal/goal/ -count=1 -timeout 30m and look at the slowest tests; the shape of the fix follows from whether the cost is per-test repository construction (share a prepared ledger) or genuinely per-assertion (declare the timeout and document it). Do not simply raise the flag in every caller: the point is that a caller should not need to know. Adjacent: the ordinary orchestrator verification for any chain touching this package is the eleven-package matrix, which inherits the default
- OpenedAt: 2026-09-07T18:19:56Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T18:19:56Z BSXXBF2E00PR101Q0HMAPQZRDW-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=goal-suite-sits-on-the-default-test-timeout
Integrity: sha256=aee9b832065d26a250332003ca2806a93f4a2c62a44447433dc9042075488d78
