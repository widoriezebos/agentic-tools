# coordinator-wakes-on-events-not-polls

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="severity 2: a wait that never returns idles a seat; novelty 2: a wait verb over existing records is new; exposure 3: every seat; accumulation 2: touches the hook and the verbs"
- Tier: 3
- Intent: Coordinator seats spent 302 of 360 session hours waiting: 25 percent in 2,139 blocking polls (90 hours, issued by turns carrying 982 million prompt tokens), 28 percent idle awaiting a background notification, 30 percent idle awaiting a human (claude-sessions.md sections 2d and 2f of the delivery deep dive). DONE means: a seat waiting on a delegate job, a proof attempt, a landing or a human act issues one wait verb (metasystem wait --job, --attempt or --goal) that returns on the event or a bounded deadline with no polling from the model; the harness's own background-task notification is the wake for Claude seats; the Stop hook does not force a turn while a registered wait is pending; proven by one coordinator session carrying a whole goal with fewer than 20 polling calls and a waiting share under 40 percent. Goal 13 of plans/delivery-efficiency-plan.md.
- Origin: human
- Next step: Design first: inventory the wait sites in the transcripts (watch, TaskOutput, tmux capture loops, Monitor), define the wait verb over the existing job, attempt and ledger records, critique, build, land.
- OpenedAt: 2026-09-11T15:45:15Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-11T15:45:15Z HPJKBN0ZC1BZBK2M5KWKW0JP71-m1e-5083721b open actor=m1e+main-1789141490-27414-b9c0d2 targets=coordinator-wakes-on-events-not-polls
Integrity: sha256=44845df537612f2e7bac3fbfd07cebbf58b0a487c414f9d1a7142a5ee0d0b5b7
