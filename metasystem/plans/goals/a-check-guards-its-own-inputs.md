# a-check-guards-its-own-inputs

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="finding 474 (2026-11-01, a1): two checks of the-steward-decides U2b1a were stopped by the steward own records commit and goal opens; step.moved was refs; the stopped retry differed only in refs/heads/main."
- Tier: 2
- Intent: A unit check guards only what it reads: the goal branch head, the worktree index and its bytes; a move of main, a ledger act or another goal refs during the check is a recorded fact on the round, never a stop; a changed input in the worktree still fails as environment naming the file.
- Origin: human
- Next step: Accept the design page (drafted, Astra read it once) at the next boundary; then build U1 from it.
- OpenedAt: 2026-11-01T16:24:47Z
- Revision: 2
- Budget: elapsedLimit=5d attemptLimit=24 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=EK6WKE5RM5FVMY1SQY7BSQ3QFP-m1e-2dcf8e78 minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-01T16:24:58Z revision=2 opid=EK6WKE5RM5FVMY1SQY7BSQ3QFP-m1e-2dcf8e78 authority=proven digest=4704e2ee768242f282998c17e3e3b4da657ec32bfc112faaf6de92feb26b0988 episode=2

History:
- 2026-11-01T16:24:47Z WQBMKCXY4AGVK6SBVF71VHCF8Z-m1e-2dcf8e78 open actor=human:Wido targets=a-check-guards-its-own-inputs reason=TierOverride: derived=3 set=2 why=every unit round on every seat is affected; tier 2 by the steward reading of severity 3 exposure 3
- 2026-11-01T16:24:58Z EK6WKE5RM5FVMY1SQY7BSQ3QFP-m1e-2dcf8e78 approve actor=human:Wido targets=a-check-guards-its-own-inputs
Integrity: sha256=455580870fbc12b72445899bc1859e5d90ce8ffe5f800835986b6a1da0f15dc6
