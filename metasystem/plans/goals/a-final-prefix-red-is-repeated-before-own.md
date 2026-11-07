# a-final-prefix-red-is-repeated-before-own

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: The lane repeats each failing test alone on the merge tree before it returns a goal as own (final-prefix shortcut, internal/landing/plain/replay.go:398-405; journal 2026-11-07 06:42)
- Origin: human
- Next step: Build without a design from N4 and section 7 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: per-identity repeat selection, a retained repeat identity beyond the one-repeat-per-tree guard, mixed-result publication; test one reproducing and one passing failure; about 120 lines, split seam selection then publication; after a-return-names-the-failing-test U1a
- OpenedAt: 2026-11-07T09:51:15Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:51:15Z QSC4WX8X6BKFCMA1YRRD530EJN-m1e-1dae276b open actor=human:Wido targets=a-final-prefix-red-is-repeated-before-own
Integrity: sha256=7c45ddc1ceeb90f4343d8c51be1c41ed1d57410f062e6e394dba8abad06ec265
