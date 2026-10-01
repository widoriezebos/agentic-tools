# fleet-card-can-land-now

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="one interface action calling an existing verb; local"
- Tier: 1
- Intent: What: The landing lane card on the Fleet page gets a Land now button. When work is queued in the lane and no landing agent is running, pressing it starts the landing agent at once through the new metasystem landing run verb, and the card shows the verb's two-line result. Why: Wido, 2026-10-01: "I want a verb that does that (from any seat) and from the UI especially" — today queued work waits up to ten minutes for the steward's next tick. Pros: a person sees work land without waiting, from the interface. Cons: the card was read-only by design; this is its first act, so it must only be offered when it can do something (work queued, no agent alive) and say plainly what happened.
- Origin: human
- Next step: Next: build the button and its server wiring against landing run's --json once its commit lands on main, with tests, and land it through the lane. Done when: on the Fleet page, Land now starts the landing agent for queued work and shows the verb's result, and is not offered when nothing is queued or an agent already runs.
- OpenedAt: 2026-10-01T15:04:12Z
- Revision: 2
- Budget: elapsedLimit=6h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=1
- BudgetExceptions: 0
- NormApproval: approvedRef=1PVZ9KT1KSP6RYATCNA96TV2TB-ui-31a738e9 minutes=360 reviewRounds=1 goalRevision=1
- Approved: by=human:Wido at=2026-10-01T15:04:16Z revision=2 opid=1PVZ9KT1KSP6RYATCNA96TV2TB-ui-31a738e9 authority=proven digest=3151600ec9d4136a7e89b8ec7c809526bcdfe576f10c97f1105e9226112b1a6f episode=2

History:
- 2026-10-01T15:04:12Z KT60W3NS3JSGVXHKZWJW64C2Q1-ui-31a738e9 open actor=human:Wido targets=fleet-card-can-land-now
- 2026-10-01T15:04:16Z 1PVZ9KT1KSP6RYATCNA96TV2TB-ui-31a738e9 approve actor=human:Wido targets=fleet-card-can-land-now
Integrity: sha256=589ca8c1c58f2d67966f8c43a41f828588d7d06a322aaba2a9ffb93832ed590a
