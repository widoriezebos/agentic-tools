# a-register-append-is-published-by-its-writer

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="Seat m1e has not followed a deployed engine by itself since 2026-10-31 because the accept-risk act leaves the tracked register modified (finding 620; design a-unit-costs-what-its-change-costs G2, agreed with Astra round 3)."
- Tier: 2
- Intent: A role that appends to a tracked coordination register publishes the append through the ledger in the same act or does not write the tracked file; the machinery never leaves a checkout dirty, so a seat's engine follow is not refused for the counselor's accepted-risk register (finding 620; design a-unit-costs-what-its-change-costs G2).
- Origin: main
- Next step: Read first who wrote the seventeen unpublished lines of records/counselor/accepted-risk-register.jsonl in the m1e checkout and by which verb; publish or discard them as that owner's rule says; then build one unit (60, --last): the writer publishes its append in the same act; test that the checkout is clean after the write.
- OpenedAt: 2026-11-13T09:17:55Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-13T09:17:55Z K80QRSCVZYT8P41861GP21R79S-m1e-16ea023a open actor=human:Wido targets=a-register-append-is-published-by-its-writer
Integrity: sha256=4d98c9cc34975603471a717d55835b7aba56972e76b9f1a5606dfaabc057c55d
