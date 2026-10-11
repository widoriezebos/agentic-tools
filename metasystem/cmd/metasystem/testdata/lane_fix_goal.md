# lane-recovers-and-pauses

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=1 basis="Changes when the lane may change a goal (its own fix round) and when a goal is breach-stopped; a wrong allowance lets the lane rewrite a goal without bound or stops a goal that waits; novelty moderate (admission and budget records exist); exposure every landing; accumulation bounded by one round per batch proof and a recorded grant per extra round"
- Tier: 3
- Intent: The landing lane recovers from every scenario by a human verb (resume the goal in the lane or retract it), a goal's elapsed clock stops while it waits in the lane, the lane owns its fix rounds, and the merge read covers only the resolution
- Origin: main
- Next step: Design plans/designs/lane-recovers-and-pauses.md (Astra critique to material zero), then U2 lane fix rounds + landing fix, U1 clock pause, U3 resolution-only read, U4 landing retract + verbs in every status line; each unit rehearsed before landing
- OpenedAt: 2026-10-11T06:10:01Z
- Revision: 2
- Budget: elapsedLimit=3d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-11T06:10:09Z revision=2 opid=G3P7CMQJ8ZTHNBA1860NCBEHRR-m1e-f456f182 authority=proven digest=a953ab48c1239c0e8ac229bd5891e3111be35db357a2e57affce80a921edf3b2 episode=2

History:
- 2026-10-11T06:10:01Z M998RMD4CHPGAYQM8XCPV5NVDY-m1e-f456f182 open actor=human:Wido targets=lane-recovers-and-pauses
- 2026-10-11T06:10:09Z G3P7CMQJ8ZTHNBA1860NCBEHRR-m1e-f456f182 approve actor=human:Wido targets=lane-recovers-and-pauses
Integrity: sha256=f2aa756d0eb15bd8009d28396abb965f811e450b1f0ba57ea38017b3245232a0
