# the-size-hold-stops-at-the-production-ceiling

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, root cause read (findings 554, 555; the week review of 2026-11-07); the unit check and the lane proof gate it"
- Tier: 2
- Intent: The size hold stops a unit only above the production ceiling of 250 production lines (decision 56); the production estimate of the units row is a signal that is reported, and the total allocation with tests is reported, never a stop (finding 554: today the hold stops above twice the declared changed lines with tests, internal/launch/unit_build_outcome.go:59-75, so an underestimate is a person act)
- Origin: human
- Next step: Build without a design from finding 554 and the week review F7: measure production lines (tests excluded) against 250; report the estimate and the allocation; a unit at 1.8x its estimate below 250 continues, a unit at 260 stops; about 30 production lines
- OpenedAt: 2026-11-07T16:14:07Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T16:14:07Z CCEPB2R5ZHS9SSEY90B0A8QTRM-m1e-7d79a016 open actor=human:Wido targets=the-size-hold-stops-at-the-production-ceiling
Integrity: sha256=d20d9af7c41de8961ce4801fc7e4e8f4bcd48a6f99093733bb89365c9a142aa8
