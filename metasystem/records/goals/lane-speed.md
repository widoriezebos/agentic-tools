# lane-speed

- State: done
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="Changes how much the lane reads and tests before a push; a wrong scope lets a resolution defect reach the full proof, which still gates the push; novelty low (the measures and verbs exist); exposure every landing; accumulation bounded by the proof"
- Tier: 2
- Intent: The landing lane spends its minutes on what a change can affect: a lane merge is read for its resolution only, a red is attributed by replaying the failed tests by name, and resolution and fix checks are cheap or skipped because the batch proof covers the tree
- Origin: main
- Next step: Three units in parallel from plans/designs/lane-speed.md: U1 resolution-only read with a recorded bound, U2 named replay, U3 cheap-or-skip checks; machinery reads; rehearsal; land
- Concluded: S1-S3 landed in hot-fix 1 (2026-10-11 22:05): resolution-only read, named replay with whole-unit confirmation, cheap-or-skip checks
- OpenedAt: 2026-10-11T14:54:10Z
- Revision: 3
- Budget: elapsedLimit=3d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=GZ4E6J4TGHZEVXEKEFKBKPF9V6-m1e-f456f182 minutes=1200 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-10-11T14:54:18Z revision=2 opid=GZ4E6J4TGHZEVXEKEFKBKPF9V6-m1e-f456f182 authority=proven digest=05c1babd5ecb6246cd12962a6e3341f92a2a4b726d032c471f0c084801f2825a episode=2

History:
- 2026-10-11T14:54:10Z YTBBES0SYX3BSXEQA1VW8K2FX9-m1e-f456f182 open actor=human:Wido targets=lane-speed
- 2026-10-11T14:54:18Z GZ4E6J4TGHZEVXEKEFKBKPF9V6-m1e-f456f182 approve actor=human:Wido targets=lane-speed
- 2026-10-12T08:40:02Z BZ4PJGVSH8950HV2N8W8TPFFYY-m1e-c6925449 done actor=m1e+main-1788680071-18713-e76d5d targets=lane-speed
Integrity: sha256=75c09001fe599fa5b278206b1de67b67ea59d53839137a185a630d1c4187d1b8
