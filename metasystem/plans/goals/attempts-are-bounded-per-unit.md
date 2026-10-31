# attempts-are-bounded-per-unit

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="It changes the admission path every goal passes through (exposure), and a wrong bound either blocks a healthy goal or lets a unit loop (severity); it reshapes counters and caps that exist today rather than adding a mechanism (novelty); it leaves no record that grows (accumulation)."
- Tier: 2
- Intent: The goal budget box bounds what the machine spends, not how big a goal is: the attempts member bounds the reservations of one unit (its rounds, retries and corrections) and closes admission for that unit alone; the per-goal total stays visible as spend; a goal never stops for having many units (the-steward-sees spent 12 of 12 attempts on eight normal reads and stood 12 h for a raise, 2026-10-31; decision 102(e))
- Origin: human
- Next step: Design through the engine: metasystem design write attempts-are-bounded-per-unit --brief agentic-tools-evidence/lane-test-20261009/designs/attempts-are-bounded-per-unit-request.md, then design review rounds (Astra is the design critic) to zero material findings, acceptance by hand until an-accepted-design-is-a-record lands; then one or two units on m1e behind the-steward-sees and the-steward-decides
- OpenedAt: 2026-10-31T20:49:44Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-31T20:49:44Z E778V71KD29KJWRZ5265EHR8BX-m1e-ad960d25 open actor=human:Wido targets=attempts-are-bounded-per-unit
Integrity: sha256=6a0b465672b710ea82a7060c32b500ad33fd7aeb73046f919858d7f0647b32d1
