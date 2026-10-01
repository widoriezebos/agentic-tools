# steward-continues-a-fenced-goal

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: an unrunnable continuation is minted on every stop refusal, and if one were ever notified it would dispatch a job at a goal no verb can continue; novelty 1: one predicate in the steward's choice of continuation target; exposure 3: every seat's steward; accumulation 2: one orphan intent per refusal with no cancel verb, unbounded"
- Tier: 3
- Intent: What: The steward never keeps working on a goal that was stopped for exceeding its budget, discards a follow-up request made before such a stop, and lets an operator cancel a follow-up request by name. Why: in September the steward created a new follow-up request for a budget-stopped goal on every refusal; none could run or be cancelled, and they piled up. The health check also counts such stopped claims as late. Already done: the steward no longer picks a stopped goal. Left: the discard at creation, a cancel command, and the health check ignoring stopped claims. Pros: no piles of dead requests, and honest delivery health. Cons: a cancel command is one more thing an operator can misuse.
- Origin: main
- Next step: Next: add the stop check where follow-up requests are created and revived, a public command to cancel a named request, and make the delivery health check skip stopped claims. Done when: a test with a budget-stopped goal and three refusals produces zero follow-up requests and a verdict line naming the stop, and a cancelled request leaves the queue with its reason recorded.
- OpenedAt: 2026-09-09T09:31:08Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-09T09:31:08Z TSM0NGHNQHSKGT31NZ27AEMXMH-m1-c6925449 open actor=human:Wido targets=steward-continues-a-fenced-goal
- 2026-09-13T08:18:32Z J19XYYQAHHFJRK6ZESQHFQM8DT-m1-c6925449 edit actor=human:Wido targets=steward-continues-a-fenced-goal
- 2026-09-30T18:53:35Z E2TVVHAN18APKEYK4KPDMKGGP5-ui-bc2fda53 edit actor=human:Wido targets=steward-continues-a-fenced-goal
Integrity: sha256=b3610034690d8ffdb317c33db2750a87cdbd4c070009691e2a1f649636645911
