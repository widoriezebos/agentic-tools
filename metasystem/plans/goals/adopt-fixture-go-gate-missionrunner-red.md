# adopt-fixture-go-gate-missionrunner-red

- State: approved
- Tier: 1
- Intent: scripts/adopt-fixtures.sh runs the Go unit gate and on m2 2026-09-04 that gate failed at internal/missionrunner after 1443 seconds (output kept under artifacts/agents/gate-failures of the run) while three other fixture suites were running on the same Mac, so the adoption fixture's vendored receipt leg (fixed in the fixture-suite drift landing) could not be confirmed green. DONE means the missionrunner package's tests pass in the adopt fixture's gate on a quiet Mac and, if they are load-sensitive, they count attempts rather than wall-clock; and the adopt fixture is confirmed green.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one test package, one seat-side run): run adopt-fixtures.sh on a quiet Mac first; if red, build, land through a chain; box 1h/3/60m/1. Waits for human approval for execution; Wido 2026-09-04: 'land what you can, leave the rest on the backlog'.
- OpenedAt: 2026-09-04T13:14:15Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T18:51:02Z revision=2 opid=8EHQY1SJFFJ7K63Z2J1HNCJYMJ-m2-5fcf08ab authority=relayed digest=48c9785d91fd0afd4da24a1fa7d1f5f46011fdd2277fc5247db8dac3f722c420 reviewBy=2026-09-06

History:
- 2026-09-04T13:14:15Z CJ1RAFN79D6A04C959KEG8ZH6Q-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=adopt-fixture-go-gate-missionrunner-red
- 2026-09-04T18:51:02Z 8EHQY1SJFFJ7K63Z2J1HNCJYMJ-m2-5fcf08ab approve actor=human:Wido targets=adopt-fixture-go-gate-missionrunner-red authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
Integrity: sha256=4b1a1739787572d130e7e47d012160fbff3c4b08dc1da3433356bf45c15d9db4
