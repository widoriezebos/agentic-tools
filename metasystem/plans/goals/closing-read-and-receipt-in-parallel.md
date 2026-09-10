# closing-read-and-receipt-in-parallel

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=1 basis="severity 1: time, not correctness; novelty 1: the receipt already binds the tree; exposure 2: every chain landing; accumulation 1: nothing built on it"
- Tier: 2
- Intent: A build chain's closing read and its landing receipt are run one after the other by every coordinator today, though nothing binds them: the read judges a frozen round tree, the receipt binds the same tree staged on main, and neither changes the tree. On 2026-09-10 a MECHANICAL chain waited 54 minutes for its receipt after its round, then would have waited for a read; run together they cost the longer of the two. DONE means: docs/orchestration.md states the practice (stage the candidate, dispatch the closing read, take the receipt in the same turn; a material finding voids the receipt and the fold restarts both), land.sh accepts a receipt taken before the read closed as long as the receipt's tree equals the closed round's reviewed tree, and a fixture proves a receipt taken before the read and a clean read land together.
- Origin: main
- Next step: Tier 1, MECHANICAL: one doc paragraph, one check in land.sh, one fixture. Interim practice on m1b from 2026-09-10 12:00: the coordinator runs both in parallel.
- OpenedAt: 2026-09-10T08:45:30Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-10T08:45:30Z 291A0WATNK558JY5QZKZ0FK1GR-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=closing-read-and-receipt-in-parallel
Integrity: sha256=78545c1e2b821b35e6cf5fc87057071df8d117f945586e507f7c145644a82456
