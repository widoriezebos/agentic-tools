# design-allocations-leave-ceiling-margin

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: an overflow costs a split and a second build round, not a wrong landing; novelty 1: a guidance number and an early count; exposure 2: every multi-unit design; accumulation 2: repeats per unit"
- Tier: 1
- Intent: Units overflow their 400-line ceiling because design allocations sit at the ceiling: brief-declares-the-round-boundary allocated 390 lines to unit 3a, whose draft reached 474, and coordinator-context unit C1a drafted 401; each overflow cost a split and another build round on 2026-09-15. DONE: design and slicing guidance allocates at most 300 changed lines per unit including tests, every unit brief states its allocation, and the builder counts lines at a checkpoint before the full draft and stops at the first overrun instead of after it.
- Origin: human
- Next step: Build, tier 1: put the 300-line allocation rule in the design guidance and the unit brief template, and add an early line-count checkpoint to the builder brief.
- OpenedAt: 2026-09-15T05:54:45Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-15T05:54:45Z 1WY3ZETND8B1AB7DTW52MHDN0W-m1e-c6925449 open actor=human:Wido targets=design-allocations-leave-ceiling-margin
Integrity: sha256=12d6852057af11705c131df0ff25ae22a1ba064ddd5d45f339371d85c95fa437
