# landing-proof-records-are-written

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Landing evidence for every seat; a wrong record misleads the next landing; the gap is dormant today so nothing relies on it yet."
- Tier: 2
- Intent: Every landing writes its proof record: the landing progress recorder has a production implementation, the public landing route wires it, green landings call RecordLandingGreen, and red, canary and fix lines are written as the record format defines, so the next preparation reads a truthful history instead of none.
- Origin: human
- Next step: Confirm the gap on main (runGoalBranchLandPrep passes no Progress at cmd/metasystem/goal_branch.go:564; RecordLandingGreen has no caller; no code writes fix or canary), then design the writer on top of U5 landpath with one Astra round.
- OpenedAt: 2026-09-28T08:23:51Z
- Revision: 2
- Labels: landing
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- NormApproval: approvedRef=16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 minutes=1200 reviewRounds=3 goalRevision=1
- Approved: by=human:Wido at=2026-09-28T08:24:24Z revision=2 opid=16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 authority=proven digest=037b324a7a9a7281954ca2faf451a77a507782c539a46cd1face70076212d23c episode=2

History:
- 2026-09-28T08:23:51Z WMPQJXTF9C48XF27VZDE5QXMW1-m1e-c6925449 open actor=human:Wido targets=landing-proof-records-are-written
- 2026-09-28T08:24:24Z 16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 approve actor=human:Wido targets=landing-proof-records-are-written,steward-sees-stuck-capacity
Integrity: sha256=e52bc483dddc2a81c1f9b79e68e6058643ef4a527ccfe5cb2fdab73bcefb44c6
