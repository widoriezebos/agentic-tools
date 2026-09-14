# failures-show-observed-against-expected

- State: approved
- Priority: 1
- Sequence: 52
- Risk: severity=2 novelty=1 exposure=3 accumulation=3 basis="severity 2: a one-line failure sends the builder guessing, and a confident wrong guess costs a whole round; novelty 1: printing observed and expected is a known discipline; exposure 3: every failing assertion in every bed and fixture; accumulation 3: every guess-round compounds across seats and days"
- Tier: 2
- Intent: Delivery-efficiency phase D. When an assertion fails, the failure must show what was observed against what was expected, on the surface the builder reads, never a one-line verdict. Why: on 2026-09-14 the hook bed said only 'declared brain start omitted the registry event name'; the builder's first fix was confidently wrong (it restored a field that was not the cause, and its own test had been accepting a malformed object), and the real cause - the arming-failure message replacing the whole context object - only appeared when the seat captured the emitted bytes by hand and ran the same row on trunk beside it. DONE: a bed assertion failure prints the observed value and the expected value or pattern, bounded, with the file and line; a Go fixture failure does the same through one shared helper; a fixture proves both, and the sweep in beds-report-every-failure finds no bare one-line failure left. Companion of row 24, which owns the scenario split and the no-silent-exit guard; this goal owns what a failure says.
- Origin: human
- Next step: Design in the verification-loop program page, then build: the shared observed-versus-expected helper for beds and for Go fixtures, migrate the assertion sites, and add the sweep.
- OpenedAt: 2026-09-14T19:25:47Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T19:25:53Z revision=2 opid=QD7ZE871TP71FAY0C3EAK3MBBV-m1e-c6925449 authority=proven digest=a52c17a71a816b5ba694bf4a1fdd7eb1a1ca5fe71b9525112a8f68eccb70c4a8

History:
- 2026-09-14T19:25:47Z B9YWHY0CP8CV0QY1J09Y4NZ39D-m1e-c6925449 open actor=human:Wido targets=failures-show-observed-against-expected
- 2026-09-14T19:25:53Z QD7ZE871TP71FAY0C3EAK3MBBV-m1e-c6925449 approve actor=human:Wido targets=failures-show-observed-against-expected
- 2026-09-14T19:26:01Z SJ38D0H8EA527MWY6KC43K99WM-m1e-c6925449 set-priority actor=human:Wido targets=failures-show-observed-against-expected reason=priority-order subject=failures-show-observed-against-expected from=unranked to=1:52 requested-sequence=append
Integrity: sha256=40dc0b7ecad89e8884ee2e97ea029a275c3a69d3baeca9b82d0fa1d06f129969
