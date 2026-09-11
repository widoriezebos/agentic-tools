# new-elapsed-budgets-use-explicit-hours

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=1 basis="severity 3: budget enforcement can stop lawful work or permit unbounded time; novelty 2: a bounded change to existing ledger and elapsed projection; exposure 3: shared goal execution across seats; accumulation 1: each episode is evaluated independently, with no new aggregate accounting"
- Tier: 3
- Intent: New elapsed budget inputs in hours and minutes are stored verbatim, so 24h stays 24h. Reject newly supplied d tokens with an explanation of the eight-hour legacy versus calendar-day ambiguity and an explicit-hour alternative. Preserve historical d records at their original eight-hour interpretation, including journal recovery; do not silently reinterpret or migrate a live budget.
- Origin: main
- Next step: Second independent successor of goal:breach-clock-and-budget-honesty. Reuse accepted Fix 2 and its review decisions in plans/breach-clock-and-budget-honesty-design.md. Retarget all actual input/replay writers and their tests to current main. Wait for goal:budget-raises-preserve-elapsed-origin before operational budget corrections, which otherwise reset the clock. Keep rollout deliberate with old and new binaries; do not absorb human-wait or quota work.
- OpenedAt: 2026-09-11T05:37:45Z
- Revision: 1
- Labels: breach-clock-successor
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T05:37:45Z 67J50K768RCDA25GHBZ4J69J7M-m1c-66a02980 open actor=m1c+main-1789023008-75159-b69143 targets=new-elapsed-budgets-use-explicit-hours
Integrity: sha256=05da5f9ab0e98803f5418d8579760e971904ddd4eb5c6a0daaa86606dd8db69f
