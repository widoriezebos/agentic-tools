# a-unit-check-runs-the-audits

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="unit check selection; a wrong selection runs extra tests (cost only) or misses one (the lane still catches it); every unit is exposed; recurs on every audit-relevant change"
- Tier: 2
- Intent: The unit check and the independent read run the repository-walking audits the lane runs (the static audits under cmd/metasystem, such as TestAuditFreshProjectionsUseInjectedDeadline): a red the lane will find is found at the seat, where it is cheap (the-steward-sees was returned 2026-10-31 23:28 for an audit neither its build nor its read ran; finding 459)
- Origin: human
- Next step: Build without a design from finding 459: the unit check selects the audit tests with the impacted packages; one unit; brief by the seat
- OpenedAt: 2026-11-01T06:41:16Z
- Revision: 2
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-01T06:41:16Z P00WV97EC6XYPH02Q25ZPD7AKM-m1e-cbd14c5a open actor=human:Wido targets=a-unit-check-runs-the-audits
- 2026-11-01T06:42:26Z DBH8NWNRTMH4QW9TEWKBHTJKC0-m1e-cbd14c5a set-pin actor=human:Wido targets=a-unit-check-runs-the-audits
Integrity: sha256=f4cee7390d160b146c0bde5254792a5fe69f8f4bb1d6a728a5496443dab4f3b5
