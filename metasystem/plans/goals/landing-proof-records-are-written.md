# landing-proof-records-are-written

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="Landing evidence for every seat; a wrong record misleads the next landing; the gap is dormant today so nothing relies on it yet."
- Tier: 2
- Intent: Every landing writes its proof record: the landing progress recorder has a production implementation, the public landing route wires it, green landings call RecordLandingGreen, and red, canary and fix lines are written as the record format defines, so the next preparation reads a truthful history instead of none.
- Origin: human
- Next step: Confirm the gap on main (runGoalBranchLandPrep passes no Progress at cmd/metasystem/goal_branch.go:564; RecordLandingGreen has no caller; no code writes fix or canary), then design the writer on top of U5 landpath with one Astra round.
- OpenedAt: 2026-09-28T08:23:51Z
- Revision: 3
- Labels: landing
- BudgetExceptions: 0

History:
- 2026-09-28T08:23:51Z WMPQJXTF9C48XF27VZDE5QXMW1-m1e-c6925449 open actor=human:Wido targets=landing-proof-records-are-written
- 2026-09-28T08:24:24Z 16GEZK3EHFYPX730B5TWWE3X4G-m1e-c6925449 approve actor=human:Wido targets=landing-proof-records-are-written,steward-sees-stuck-capacity
- 2026-09-30T19:04:33Z 4RM8KYH1BE9A3P4Q2W4G5HV6BM-m1e-b6a4eb0a unapprove actor=human:wido targets=landing-proof-records-are-written reason=Held: it would change the landing lane while the lane is being redesigned; revisit under landing-lane-runtime-redesign (backlog sync 2026-09-30)
Integrity: sha256=c41374b4cb3e44ee110f168b8a8ab34ef651bef57638b42bb7b56f4f657b20b4
