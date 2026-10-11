# lane-speed

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="Changes how much the lane reads and tests before a push; a wrong scope lets a resolution defect reach the full proof, which still gates the push; novelty low (the measures and verbs exist); exposure every landing; accumulation bounded by the proof"
- Tier: 2
- Intent: The landing lane spends its minutes on what a change can affect: a lane merge is read for its resolution only, a red is attributed by replaying the failed tests by name, and resolution and fix checks are cheap or skipped because the batch proof covers the tree
- Origin: main
- Next step: Three units in parallel from plans/designs/lane-speed.md: U1 resolution-only read with a recorded bound, U2 named replay, U3 cheap-or-skip checks; machinery reads; rehearsal; land
- OpenedAt: 2026-10-11T14:54:10Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-11T14:54:10Z YTBBES0SYX3BSXEQA1VW8K2FX9-m1e-f456f182 open actor=human:Wido targets=lane-speed
Integrity: sha256=d7eb301c697a6532b30a6dd8bc832732e3bce667113c32fb07bfa2934dff7ec9
