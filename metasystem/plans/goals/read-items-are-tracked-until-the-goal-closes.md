# read-items-are-tracked-until-the-goal-closes

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="A ledger check at conclude; a defect blocks a close, never lands code."
- Tier: 2
- Intent: Fix-forward loses nothing. DONE: (1) every read's non-breaking items become one tracked fix unit on the goal's Next at the read's return; (2) goal conclude refuses while a read item is open on the ledger; (3) the retro lists open read items per goal, zero on concluded goals.
- Origin: human
- Next step: Evidence 2026-09-17: Wido's rule that a read gates a merge only on breaking items (14:25 local) leaves Build B's 10 minors and C+D's items queued behind the merge with no machine record. No design first: brief the Next-row shape and the conclude check, build, read.
- OpenedAt: 2026-09-17T13:39:38Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-17T13:39:57Z revision=2 opid=MEXHB60SVQSK10C0EQF4637P5D-m1e-c6925449 authority=proven digest=ed1000d79bcfa246da632243514028564930f372879043737aca3ea3c5a75ee7

History:
- 2026-09-17T13:39:38Z 670G3NHRW8CK93V56V53DK1V51-m1e-c6925449 open actor=human:Wido targets=read-items-are-tracked-until-the-goal-closes
- 2026-09-17T13:39:57Z MEXHB60SVQSK10C0EQF4637P5D-m1e-c6925449 approve actor=human:Wido targets=read-items-are-tracked-until-the-goal-closes
Integrity: sha256=33a3eedb50791881f87b6884f832ff4c10eeaa753c54b45e9c9c2a6a417c6498
