# delegate-sandbox-runs-the-beds

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a wrong temporary directory breaks every delegate build, but the change is one adapter setting with a fixture; novelty 1: moving a temporary directory; exposure 3: every dispatch on every runtime; accumulation 1: nothing downstream reads the directory"
- Tier: 2
- Intent: Delivery-efficiency phase D, delegate loop. The adapter places the delegate's shell and Go temporary directories outside the linked worktree's git administration directory, so the independent-repository fixtures and the three process-owning beds (supervision-fixtures.sh, supervision-hook-fixtures.sh, land-fixtures.sh) run to completion inside the delegate sandbox. The implementer packet then requires, in the return's evidence, every bed the round's diff touches and full-package Go runs for every package it changes, and the return validator refuses a package reported green from a selection that names only the round's own new tests. Why: the stop-decisions build took nineteen rounds because the delegate could not run the beds it changed and the seat relayed one failure per round by hand; three of those rounds repaired the previous round's own blind change. DONE: a fixture proves a delegate under a chain runs all three beds to completion in its sandbox; a return whose evidence lacks a touched bed or a full package run is refused with the missing item named. every mechanism in this goal lives in the Go engine, the ledger verbs and the adapter contract, never in one runtime; it is proven on at least two runtimes.
- Origin: human
- Next step: Design page: the adapter setting, the packet clause and the validator check, each with its fixture; then build.
- OpenedAt: 2026-09-14T15:31:22Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T15:31:22Z NRM60XBC3CTEMVXAVZ14G5514K-m1e-c6925449 open actor=human:Wido targets=delegate-sandbox-runs-the-beds
Integrity: sha256=c387a4453f83f194ce76f1b37073081709a5e80403a5472eaaa241b2e44866e5
