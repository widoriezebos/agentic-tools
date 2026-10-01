# goal-intake-detects-overlapping-work

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="Severity 2: duplicate live work wastes budget and can create competing patches; novelty 2: overlap resolution across different goal IDs is not currently enforced; exposure 3: every seat sharing a backlog; accumulation 2: duplicated design, critique and implementation compound across the fleet."
- Tier: 3
- Intent: What: Before costly design or build work starts on a goal, the system checks for approved or claimed goals that aim at the same outcome, shows both goals and their owners, and requires a recorded choice: reuse the existing goal, hand it over, or declare the work separate. Why: On 2026-09-10 two seats built the same repair under two different goal names at the same time; the one-goal-per-seat rule did not notice, and one seat's work was thrown away. Pros: No two seats pay for the same repair, and useful work moves to whoever continues. Cons: A check that is too broad would block unrelated work that only touches the same files, and it depends on seats being able to see each other's claims.
- Origin: human
- Next step: Next: When seat-mutual-awareness has landed, design the smallest check at claim time that compares a new goal's outcome with live claimed goals and asks for a recorded choice when they overlap. Done when: A test with two seats claiming differently named goals for the same repair shows the second claim stopped with both goals named, while two unrelated goals are claimed in parallel.
- OpenedAt: 2026-09-10T08:18:39Z
- Revision: 2
- Labels: headless-communication, headless-fleet
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T08:18:39Z XD3ZHSERMV7CP61K6NYMJCPVWT-m1c-66a02980 open actor=m1c+main-1789023008-75159-b69143 targets=goal-intake-detects-overlapping-work
- 2026-09-30T18:49:20Z H54MAQ7TJ9Q9CAASNVTN3JE7TT-ui-bc2fda53 edit actor=human:Wido targets=goal-intake-detects-overlapping-work
Integrity: sha256=9ba5f7aeee8fe0797443c06bb80190e008d7906715c9ef01a32f0203bd31927c
