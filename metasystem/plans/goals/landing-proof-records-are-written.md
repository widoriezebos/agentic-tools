# landing-proof-records-are-written

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Landing evidence for every seat; a wrong record misleads the next landing; the gap is dormant today so nothing relies on it yet."
- Tier: 2
- Intent: Every landing writes its proof record: the landing progress recorder has a production implementation, the public landing route wires it, green landings call RecordLandingGreen, and red, canary and fix lines are written as the record format defines, so the next preparation reads a truthful history instead of none.
- Origin: human
- Next step: Confirm the gap on main (runGoalBranchLandPrep passes no Progress at cmd/metasystem/goal_branch.go:564; RecordLandingGreen has no caller; no code writes fix or canary), then design the writer on top of U5 landpath with one Astra round.
- OpenedAt: 2026-09-28T08:23:51Z
- Revision: 1
- Labels: landing
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-28T08:23:51Z WMPQJXTF9C48XF27VZDE5QXMW1-m1e-c6925449 open actor=human:Wido targets=landing-proof-records-are-written
Integrity: sha256=3bc7d326c83352167b0de8bee5f3c0eaf29d0093cb8a539022b4987e89d13024
