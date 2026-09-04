# channel-fixture-answer-history-drift

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="One fixture assertion on the goal history after a channel answer; the product's answer path changed twice today and the fixture still expects the old history line; nothing outside the suite is affected."
- Tier: 1
- Intent: scripts/agents/channel-fixtures.sh fails silently (bare exit 1, no message) at its assertion grep -q 'answer actor=human:wido' on the goal file's history after the fixture's human answers a budget-above-norm question: since the status-post binding (4bdaa5ec) and the budget-answer landing (3615da7a), a code-verified token answer to a budget question re-approves the goal as a verified channel answer, and the history line the fixture expects is no longer the one written. Seen seat-side on m2 2026-09-04 17:31Z. DONE means the fixture asserts the history the product now writes (the answer event and the verified-channel approval, actor and outcome named), fails loudly with the history printed when it does not match, and the suite is green.
- Origin: main
- Next step: One fixture assertion and a loud failure message: build, run channel-fixtures.sh seat-side, land through a chain. Approved under R-76-m2.
- OpenedAt: 2026-09-04T17:31:30Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-04T17:31:30Z VVSG8R3HEKTF8RJSE3C7PYK4B3-m2-5fcf08ab open actor=human:Wido targets=channel-fixture-answer-history-drift
Integrity: sha256=f7e0b8bc0fcc42d71189285ddcf4497a393b598479a7b440cc1f57f3c578a438
