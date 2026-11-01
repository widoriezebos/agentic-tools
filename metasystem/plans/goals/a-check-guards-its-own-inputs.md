# a-check-guards-its-own-inputs

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="finding 474 (2026-11-01, a1): two checks of the-steward-decides U2b1a were stopped by the steward own records commit and goal opens; step.moved was refs; the stopped retry differed only in refs/heads/main."
- Tier: 2
- Intent: A unit check guards only what it reads: the goal branch head, the worktree index and its bytes; a move of main, a ledger act or another goal refs during the check is a recorded fact on the round, never a stop; a changed input in the worktree still fails as environment naming the file.
- Origin: human
- Next step: Accept the design page (drafted, Astra read it once) at the next boundary; then build U1 from it.
- OpenedAt: 2026-11-01T16:24:47Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-01T16:24:47Z WQBMKCXY4AGVK6SBVF71VHCF8Z-m1e-2dcf8e78 open actor=human:Wido targets=a-check-guards-its-own-inputs reason=TierOverride: derived=3 set=2 why=every unit round on every seat is affected; tier 2 by the steward reading of severity 3 exposure 3
Integrity: sha256=80f11efb4eb4ec7171a976b3c557a436e8077b3a023cd2820e0904f809300f1f
