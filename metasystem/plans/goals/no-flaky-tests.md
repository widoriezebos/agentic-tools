# no-flaky-tests

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Test-only and fixture changes; the suite's trustworthiness decides every landing"
- Tier: 2
- Intent: What: find every flaky test in the suite (passes and fails on the same tree) and fix each one deterministically by its root cause - no retries, no raised limits, artificial clocks instead of wall time, isolated state instead of shared, waiting on process state instead of racing it. Why: Wido 2026-10-02: 'We need to fix these flaky tests. And not only that, figure out if there are more flaky tests we cannot have anymore. So go deep, find them all, and fix them all.' Six known flakes turned green landings red in the VM tonight, each costing a 10-15 min proof. Pros: the lane and every seat get trustworthy reds. Cons: wide; touches many test fixtures.
- Origin: human
- Next step: Discover: mine all VM logs under agentic-tools-evidence, 3 stress runs of the full suite in the VM, static scan for flaky patterns; then fix per root-cause cluster with parallel builders; prove with VM stress runs
- OpenedAt: 2026-10-02T06:05:29Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=5
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T06:05:36Z revision=2 opid=EQNXHN70ZEJC293A84HKQ1ZPBK-m1e-9c612d71 authority=proven digest=ae5e3ee63b8610aa5894efe1da3b8c25210963777fd7821629fac9d2563c5da4 episode=2

History:
- 2026-10-02T06:05:29Z TD3FPBKZ6P6RE5X8JBF4VGKP65-m1e-9c612d71 open actor=human:Wido targets=no-flaky-tests
- 2026-10-02T06:05:36Z EQNXHN70ZEJC293A84HKQ1ZPBK-m1e-9c612d71 approve actor=human:Wido targets=no-flaky-tests
Integrity: sha256=e397413865c03f52640cf8290caed5048090cf5d8f8175371cfdb214b4e29b53
