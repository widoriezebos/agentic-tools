# adopt-fixture-go-gate-missionrunner-red

- State: queued
- Tier: 1
- Intent: scripts/adopt-fixtures.sh runs the Go unit gate and on m2 2026-09-04 that gate failed at internal/missionrunner after 1443 seconds (output kept under artifacts/agents/gate-failures of the run) while three other fixture suites were running on the same Mac, so the adoption fixture's vendored receipt leg (fixed in the fixture-suite drift landing) could not be confirmed green. DONE means the missionrunner package's tests pass in the adopt fixture's gate on a quiet Mac and, if they are load-sensitive, they count attempts rather than wall-clock; and the adopt fixture is confirmed green.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one test package, one seat-side run): run adopt-fixtures.sh on a quiet Mac first; if red, build, land through a chain; box 1h/3/60m/1. Waits for human approval for execution; Wido 2026-09-04: 'land what you can, leave the rest on the backlog'.
- OpenedAt: 2026-09-04T13:14:15Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T13:14:15Z CJ1RAFN79D6A04C959KEG8ZH6Q-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=adopt-fixture-go-gate-missionrunner-red
Integrity: sha256=f717522594c4590c3d1b7175f1a55ad60ce63f33622b0443ad47edab44f98181
