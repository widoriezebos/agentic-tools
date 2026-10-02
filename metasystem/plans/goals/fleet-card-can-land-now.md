# fleet-card-can-land-now

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="one interface action calling an existing verb; local"
- Tier: 1
- Intent: What: The landing lane card on the Fleet page gets a Land now button. When work is queued in the lane and no landing agent is running, pressing it starts the landing agent at once through the new metasystem landing run verb, and the card shows the verb's two-line result. Why: Wido, 2026-10-01: "I want a verb that does that (from any seat) and from the UI especially" — today queued work waits up to ten minutes for the steward's next tick. Pros: a person sees work land without waiting, from the interface. Cons: the card was read-only by design; this is its first act, so it must only be offered when it can do something (work queued, no agent alive) and say plainly what happened.
- Origin: human
- Next step: Next: build the button and its server wiring against landing run's --json once its commit lands on main, with tests, and land it through the lane. Done when: on the Fleet page, Land now starts the landing agent for queued work and shows the verb's result, and is not offered when nothing is queued or an agent already runs.
- OpenedAt: 2026-10-01T15:04:12Z
- Revision: 6
- Budget: elapsedLimit=6h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 1
- Approved: by=human:Wido at=2026-10-02T15:02:35Z revision=6 opid=15CT9VR6HA0HRAM0DH0QZ66KAS-ui-31a738e9 authority=proven digest=a4006746e9cadb35aa6e39bdb5e7ee330238fabaad088587d725a3a42af3185c episode=6
- Claimed: machine=ui lineage=main-1790664507-87928-61899b at=2026-10-02T15:02:35Z revision=6 accountingRevision=6 episodeAt=2026-10-01T15:04:21Z episodeRevision=3 idleSeconds=71837
- StopCapability: generation=6 revision=6 machine=ui claimEpoch=2 fenceEpoch=0

History:
- 2026-10-01T15:04:12Z KT60W3NS3JSGVXHKZWJW64C2Q1-ui-31a738e9 open actor=human:Wido targets=fleet-card-can-land-now
- 2026-10-01T15:04:16Z 1PVZ9KT1KSP6RYATCNA96TV2TB-ui-31a738e9 approve actor=human:Wido targets=fleet-card-can-land-now
- 2026-10-01T15:04:21Z T3JDMD1C9S170H5074E82DH044-ui-d4023bb7 claim actor=ui+main-1790664507-87928-61899b targets=fleet-card-can-land-now
- 2026-10-01T19:01:11Z QSVPJ3JE6ASDC7W9ERHH9TGW7B-ui-d4023bb7 release actor=ui+main-1790664507-87928-61899b targets=fleet-card-can-land-now reason=built and held; the fleet card lands first through the lane, then this one
- 2026-10-02T14:58:28Z ZFVXBDRDFPK3D7MW2K6XDJ36S7-ui-d4023bb7 claim actor=ui+main-1790664507-87928-61899b targets=fleet-card-can-land-now
- 2026-10-02T15:02:35Z 15CT9VR6HA0HRAM0DH0QZ66KAS-ui-31a738e9 set-budget actor=human:Wido targets=fleet-card-can-land-now displaced=ui+main-1790664507-87928-61899b@2026-10-02T14:58:28Z
Integrity: sha256=b8cd6800c22da8e79ce1708b736f59efcf5d9899693ef459bfe7bb329ba8125d
