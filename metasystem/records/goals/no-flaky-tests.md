# no-flaky-tests

- State: done
- Priority: 1
- Sequence: 1
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Test-only and fixture changes; the suite's trustworthiness decides every landing"
- Tier: 2
- Intent: What: find every flaky test in the suite (passes and fails on the same tree) and fix each one deterministically by its root cause - no retries, no raised limits, artificial clocks instead of wall time, isolated state instead of shared, waiting on process state instead of racing it. Why: Wido 2026-10-02: 'We need to fix these flaky tests. And not only that, figure out if there are more flaky tests we cannot have anymore. So go deep, find them all, and fix them all.' Six known flakes turned green landings red in the VM tonight, each costing a 10-15 min proof. Pros: the lane and every seat get trustworthy reds. Cons: wide; touches many test fixtures.
- Origin: human
- Next step: Discover: mine all VM logs under agentic-tools-evidence, 3 stress runs of the full suite in the VM, static scan for flaky patterns; then fix per root-cause cluster with parallel builders; prove with VM stress runs
- Concluded: Landed b3b34bb4e: 9 flaky clusters plus custodian, attempt-count polling and supervisor timer-exit causes fixed at every pattern match, each with an audit; VM2 full suite green both modes and targeted stress (-count=50) green at the tip
- OpenedAt: 2026-10-02T06:05:29Z
- Revision: 5
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=5
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-02T06:05:36Z revision=2 opid=EQNXHN70ZEJC293A84HKQ1ZPBK-m1e-9c612d71 authority=proven digest=ae5e3ee63b8610aa5894efe1da3b8c25210963777fd7821629fac9d2563c5da4 episode=2

History:
- 2026-10-02T06:05:29Z TD3FPBKZ6P6RE5X8JBF4VGKP65-m1e-9c612d71 open actor=human:Wido targets=no-flaky-tests
- 2026-10-02T06:05:36Z EQNXHN70ZEJC293A84HKQ1ZPBK-m1e-9c612d71 approve actor=human:Wido targets=no-flaky-tests
- 2026-10-02T06:05:41Z SXYVTQXC11PAE2AHBMSDRSBJJX-m1e-9c612d71 set-priority actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,host-setup-from-scratch,landing-deploys-the-engine,landing-lane-runtime-redesign,no-flaky-tests,old-lane-plumbing-is-deleted,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,reviewers-check-the-rulings,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-successor-continues-a-handoff-without-a-human,seat-works-without-a-person,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,steward-acts-on-behaviour-patterns,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,work-review-starts-its-critic reason=priority-order subject=no-flaky-tests from=unranked to=1:1 requested-sequence=1
- 2026-10-02T06:05:48Z PTGKV4AWD4M82G3DXRFDEHMQT9-m1e-9c612d71 set-pin actor=human:Wido targets=no-flaky-tests
- 2026-10-02T10:03:43Z DKBXJ0X5EWAEBPMZZZEY693EA5-m1e-9c612d71 done actor=human:Wido targets=builder-proves-each-rule-by-mutation,busy-seat-shells-are-ended,cross-cutting-change-inventories-its-readers,design-rounds-read-less-and-repeat-less,evidence-and-build-output-have-retention-and-stay-unindexed,fleet-doctor-repairs-what-stops-other-seats,host-setup-from-scratch,landing-deploys-the-engine,landing-lane-runtime-redesign,no-flaky-tests,old-lane-plumbing-is-deleted,one-steward-per-checkout-on-its-own-root,receipt-writer-follows-the-worktree-it-runs-in,reviewers-check-the-rulings,round-proof-feeds-the-next-brief,seat-path-lands-without-help,seat-successor-continues-a-handoff-without-a-human,seat-works-without-a-person,seats-spend-tokens-in-bounded-sessions,spend-fence-reports-tokens-per-model-and-cause,steward-acts-on-behaviour-patterns,stop-hook-never-forces-an-empty-turn,system-start-launches-your-agent,terminal-enrollment-per-computer,testing-surfaces-declare-their-mirror,work-review-starts-its-critic
Integrity: sha256=a90dd92879f895ef38111d13df8f6ee02d768ffb47a6b32840b3f4f2d2623e76
