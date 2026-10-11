# lane-rehearsal-before-landing

- State: queued
- Risk: severity=2 novelty=3 exposure=2 accumulation=1 basis="A wrong rehearsal passes a broken lane change or blocks a good one; it touches no production path except the new verb; novelty high (first real-shape gate); exposure limited to lane changes; accumulation bounded by scratch cleanup"
- Tier: 3
- Intent: Every change to the landing lane is rehearsed before it lands: metasystem landing rehearse runs the lane end to end against the candidate engine in an isolated scratch world with fixture goals, and a change lands only after its rehearsal passed
- Origin: main
- Next step: U1 green scenario (built, read, real run 48.7 s on the committed candidate; branch goal/lane-rehearsal-before-landing); then U2 red scenario, U3 roster carried, U4 the lane gate runs the rehearsal, U5 records outside the certified tree; design plans/designs/lane-rehearsal-before-landing.md
- OpenedAt: 2026-10-11T06:08:04Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-11T06:08:04Z P8RP1235HM7D4YC1RZE1121RSW-m1e-f456f182 open actor=human:Wido targets=lane-rehearsal-before-landing
Integrity: sha256=edbe3d69e764aa89bcd09ac1b4e4ede33c5146e72ea5a19af7954f2434e8434b
