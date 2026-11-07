# a-unit-reservation-carries-its-deadline

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: Delivery health can read unit launches: a unit reservation carries capDeadline (finding 504; unit_launch.go:78-80 writes capMin only, delivery.go:241-245 requires both)
- Origin: human
- Next step: Build without a design from N2 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: write capDeadline from createdAt and capMin; derive it for older unit-launch reservations in the reader; test through the delivery role on an artificial clock; about 15 lines
- OpenedAt: 2026-11-07T09:50:00Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:50:00Z ABV4P0HD1XK260KM2R2CWSASP0-m1e-1dae276b open actor=human:Wido targets=a-unit-reservation-carries-its-deadline
Integrity: sha256=3b71df7a585db58baee797a0c7e46a1f1601cb7adffd7fa7d8746f3a7c71a675
