# lane-records-flakes-and-routes-their-fix

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="a register and a bounded re-proof in the lane plus an auto-opened fix item: it records and routes, it does not decide what lands; a wrong record costs a misrouted item, not a bad landing; every adopter's lane runs it"
- Tier: 2
- Intent: The lane never buys a green by repeating a proof: when a test fails in a proof and the same test passes on the re-proof of the same tree, or passes repeatedly on its own, the lane records a flake (test, package, tree, proof log, load at the time) in a register, opens the fix as a priority-1 item on the goal that owns the test's surface (or a new tier-1 goal when none owns it), and re-proves a red that is not the branch's fault at most once per tree. Language-generic: the test identity and the owning surface come from the testing contract; nothing in the mechanism knows Go. Smallest thing that works: the register, the one bounded re-proof, the auto-opened item; no classification cleverness beyond "failed, then passed on the same tree".
- Origin: main
- Next step: Opened 2026-10-03 22:58 by m1e under Wido's eight-hour watch ("if there's anything not working properly or efficiently, detect it and get the goal defined"; "artificial clocks, never load-fragile tests; fix flaky tests, never retry"). Evidence tonight: (1) internal/ui/httpd TestPartnerEventsRideTheOneStreamWithNoIdOfTheirOwn blocked 36 minutes on a stream read with no deadline in the proof of landing-deploys-the-engine (first time in 53 proofs; passes 20/20 on main and on the unit's tip), aborted by hand; (2) internal/proofrun TestLaunchSuiteWritesBannerProgressAndReapsWatchdog failed once in 53 proofs ('fixture-survivor pid=…', launcher_test.go:145) in the proof of review-findings-read-as-decisions at 22:45, a unit that touches no proofrun code; the lane re-proved the same tree at 22:46 and nothing recorded the flake. Both goals no-flaky-tests and flaky-leftovers are done, so no owner records new flakes. First units: the register and the bounded re-proof (tier 2 design, 1,000 words, one Astra round); the auto-opened fix item; then the two tests above as the first registered flakes with their fixes routed (the httpd one is already the first unit of agents-show-one-line-of-what-they-do). Estimate: design 2 h, build 4 h, one critique round each.
- OpenedAt: 2026-10-03T20:56:55Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-03T20:56:55Z SWNNW886C0EMH8EZP5AFRWDMFK-m1e-718ba0eb open actor=human:Wido targets=lane-records-flakes-and-routes-their-fix
Integrity: sha256=d5e08b51dee2d718303766aefc23257e4312e70fba8a82d2b973efcf1d384a7a
