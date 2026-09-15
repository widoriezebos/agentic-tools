# design-allocations-leave-ceiling-margin

- State: approved
- Priority: 1
- Sequence: 63
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: an overflow costs a split and a second build round, not a wrong landing; novelty 1: a guidance number and an early count; exposure 2: every multi-unit design; accumulation 2: repeats per unit"
- Tier: 1
- Intent: Units overflow their 400-line ceiling because design allocations sit at the ceiling: brief-declares-the-round-boundary allocated 390 lines to unit 3a, whose draft reached 474, and coordinator-context unit C1a drafted 401; each overflow cost a split and another build round on 2026-09-15. DONE: design and slicing guidance allocates at most 300 changed lines per unit including tests, every unit brief states its allocation, and the builder counts lines at a checkpoint before the full draft and stops at the first overrun instead of after it.
- Origin: human
- Next step: Build, tier 1: put the 300-line allocation rule in the design guidance and the unit brief template, and add an early line-count checkpoint to the builder brief.
- OpenedAt: 2026-09-15T05:54:45Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T05:54:55Z revision=2 opid=BYG921J04S8NE8X97X4DYDZPV0-m1e-c6925449 authority=proven digest=8d58ee538ac4615b3daa5f816df754798fcd7527f79d6283e2211a803ab88da5

History:
- 2026-09-15T05:54:45Z 1WY3ZETND8B1AB7DTW52MHDN0W-m1e-c6925449 open actor=human:Wido targets=design-allocations-leave-ceiling-margin
- 2026-09-15T05:54:55Z BYG921J04S8NE8X97X4DYDZPV0-m1e-c6925449 approve actor=human:Wido targets=design-allocations-leave-ceiling-margin
- 2026-09-15T05:58:30Z 2FW62FKSWCZKDQWEVSY0496VVA-m1e-c6925449 set-priority actor=human:Wido targets=design-allocations-leave-ceiling-margin reason=priority-order subject=design-allocations-leave-ceiling-margin from=unranked to=1:63 requested-sequence=63
Integrity: sha256=2146de89389bf2a5f3bad0c604a28246429eefa991c6e38ea79cf9ccf07c5dec
