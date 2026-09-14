# failures-show-observed-against-expected

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="severity 2: a one-line failure sends the builder guessing, and a confident wrong guess costs a whole round; novelty 1: printing observed and expected is a known discipline; exposure 3: every failing assertion in every bed and fixture; accumulation 3: every guess-round compounds across seats and days"
- Tier: 2
- Intent: Delivery-efficiency phase D. When an assertion fails, the failure must show what was observed against what was expected, on the surface the builder reads, never a one-line verdict. Why: on 2026-09-14 the hook bed said only 'declared brain start omitted the registry event name'; the builder's first fix was confidently wrong (it restored a field that was not the cause, and its own test had been accepting a malformed object), and the real cause - the arming-failure message replacing the whole context object - only appeared when the seat captured the emitted bytes by hand and ran the same row on trunk beside it. DONE: a bed assertion failure prints the observed value and the expected value or pattern, bounded, with the file and line; a Go fixture failure does the same through one shared helper; a fixture proves both, and the sweep in beds-report-every-failure finds no bare one-line failure left. Companion of row 24, which owns the scenario split and the no-silent-exit guard; this goal owns what a failure says.
- Origin: human
- Next step: Design in the verification-loop program page, then build: the shared observed-versus-expected helper for beds and for Go fixtures, migrate the assertion sites, and add the sweep.
- OpenedAt: 2026-09-14T19:25:47Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T19:25:47Z B9YWHY0CP8CV0QY1J09Y4NZ39D-m1e-c6925449 open actor=human:Wido targets=failures-show-observed-against-expected
Integrity: sha256=4ab7f61785b461379b2a71a17d25f33ff176464c1d79a15b43b2c1f2299b465c
