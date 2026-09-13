# steward-delivery-counts-a-fenced-claim

- State: done
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a false dead-delivery report, no work lost; novelty 1: one fence check in the delivery health rule; exposure 3: every seat's steward; accumulation 1: one false report per fenced claim"
- Tier: 3
- Intent: The steward's delivery health check (internal/steward/delivery.go) counts a breach-stopped claim among this machine's claimed goals and reports the delivery role dead once that claim's age passes 150 percent of the goal's own elapsed limit, which a stop for ELAPSED_LIMIT guarantees. This predates breach-stop-wedges-seat; that goal makes it durable in a new way, because the seat now works on while the role stays reported dead instead of being wedged beside it. Found by critic bsws-crit1b-20260909 as BSW-07 on 2026-09-09. DONE means a claim under a standing StopFence does not age the delivery role, with a fixture proving a fenced claim older than its limit leaves delivery healthy while an unfenced one past its limit still reports dead.
- Origin: main
- Next step: Appetite: 1h. Read the claim-age rule in internal/steward/delivery.go and apply GoalFile.IsFencedClaim (landed by breach-stop-wedges-seat) before counting a claim's age. Canary: the two fixtures named in DONE.
- Concluded: Absorbed into goal:steward-continues-a-fenced-goal in the 2026-09-13 backlog consolidation on Wido's word; Delivery health does not age a claim under a standing StopFence. Its clause is appended to that goal's next step.
- OpenedAt: 2026-09-10T05:51:30Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T05:51:30Z C8MYY6469M929B1MC14V8GCPSB-m1-c6925449 open actor=human:Wido targets=steward-delivery-counts-a-fenced-claim
- 2026-09-13T08:22:38Z G6H6YYPVESNZHFJW0SEPNNAT5M-m1-c6925449 done actor=human:Wido targets=steward-delivery-counts-a-fenced-claim
Integrity: sha256=06cc9701eb00dc970d7153518347365d0d632f82f04c5034f9d11c623b170308
