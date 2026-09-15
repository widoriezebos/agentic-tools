# context-handoff-accepts-every-waiter-state

- State: approved
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: blocks a seat's handoff, so an oversized session keeps burning tokens; novelty 1: one state list shared by writer and reader; exposure 3: every seat that ever had a wait reach its deadline; accumulation 2: waiter records persist for days"
- Tier: 2
- Intent: `metasystem context handoff` refuses on seat m1b (2026-09-15) with "handoff waiter record artifacts/agents/waiters/goal-wait-verb-returns-on-recorded-events-30a7e1417b0a.json has unknown state \"deadline\"": the handoff reader rejects the terminal state the waiter itself writes when a wait reaches its deadline (a two-day-old goal wait, the only one of 29 waiter records in that state). A seat with such a record cannot hand off, which blocks the main token remedy of seats-spend-tokens-in-bounded-sessions. DONE: the handoff reader accepts every state the waiter writer can record, with terminal states (deadline included) treated as ended waits that do not block a handoff; a test enumerates the writer's states so a new state cannot be added without the reader; m1b's handoff succeeds against its preserved record. Under seats-spend-tokens-in-bounded-sessions; area of coordinator-context-stays-under-budget.
- Origin: human
- Next step: Build, tier 1: find the waiter state set the writer uses and the handoff reader's check, share one list, add the enumeration test, and prove it against the preserved record on m1b (left untouched as evidence). Suggested owner: m1e's next session or any seat.
- OpenedAt: 2026-09-15T10:00:46Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T10:00:52Z revision=2 opid=RT31VVSB9X81GX42NT3K7BRDW8-m1e-c6925449 authority=proven digest=31e9b143d380631c67960834103a28eb2612f9a6f9f82a8f6179614eb11d2f3f

History:
- 2026-09-15T10:00:46Z 3GX99Q9F2C9SN3NF964SBXD2PR-m1e-c6925449 open actor=human:Wido targets=context-handoff-accepts-every-waiter-state
- 2026-09-15T10:00:52Z RT31VVSB9X81GX42NT3K7BRDW8-m1e-c6925449 approve actor=human:Wido targets=context-handoff-accepts-every-waiter-state
Integrity: sha256=b0bd04e0fe72a940de2f7799276e05a0d6a9b9d20bf8067b89849016a502ae54
