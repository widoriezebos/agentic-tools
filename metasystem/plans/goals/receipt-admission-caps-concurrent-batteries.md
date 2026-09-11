# receipt-admission-caps-concurrent-batteries

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a wrong cap starves landings or lets batteries collide as today; novelty 2: attempt reservations exist per checkout but nothing is host-wide; exposure 3: every receipt on every seat; accumulation 1: the battery content is unchanged"
- Tier: 3
- Intent: Delivery receipts and cadence batteries on one host are admitted under a host-wide cap so batteries stop timing each other out. Evidence 6 to 11 September (proof-attempts.md section 4 of the delivery deep dive): 97 attempts across five seats; goal-full-coverage failed 8 percent with no overlapping attempt and 53, 67 and 100 percent with 1, 2 and 3 or more; attempt failure 32 percent alone against 57 percent at 3 or more overlaps; heavy groups up to two times slower under overlap. DONE means: test run admits a delivery attempt only when fewer than N attempts (default 2, conf key testing.host-attempts) are active on the host across every checkout, otherwise waits in a visible queue naming the holders; a cadence run never overlaps another cadence run; the admission is host-wide (a reservation keyed by host under the durable evidence root or the host temp root), a crashed launcher's reservation expires with its attempt deadline, and metasystem status names who holds the slots. Proof: two seats launching together, one waits with the reason; a killed launcher frees its slot within the deadline. Docs: docs/project-rules.md records the cap and the standing ruling to run three seats until deep-battery-under-ten-minutes lands. Goal 1 of plans/delivery-efficiency-plan.md. Wido's word 2026-09-11: execute in order on m1e.
- Origin: human
- Next step: Design first: read internal/proofrun/attempt.go admission and reservation (the 'reserved before process publication' status lines), choose the host-wide primitive, write the design page, critique, build, land with its own battery and three green cadence runs.
- OpenedAt: 2026-09-11T15:44:50Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:44:50Z ATQJYESKCTSPAS92AEEAW0TKS3-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=receipt-admission-caps-concurrent-batteries
Integrity: sha256=fc915165b9b87b16b789c3732ba861d54ad0147cb98960b7d0f78bd2fa3fddb3
