# stop-hook-budget-is-ours

- State: queued
- Risk: severity=1 novelty=1 exposure=3 accumulation=1 basis="severity 1: a slow hook holds a turn end a few seconds longer, nothing unsafe is permitted and the fail-closed refusal stays; novelty 1: three numbers that already exist move together; exposure 3: every seat on every machine ends every turn through this hook; accumulation 1: nothing compounds, a wrong number is one edit away"
- Tier: 3
- Intent: The Stop hook's five-second budget is our own number, not the provider's: the hook's comment says 'Claude gives the complete Stop hook five seconds', but the five seconds is the timeout we wrote in the repository's Claude settings for the Stop hook, and the runtime's own default is far larger (SessionStart already gets fifteen there). The parent gives its worker four of those seconds and keeps one. Under load - a builder round, a suite run, a Go build, now three seats on one host - a healthy Stop takes longer than four seconds and the parent blocks the turn end with 'deadline expired before a safe turn verdict': eight expiries on m1 on 2026-09-06 before 10:00, every one while something else was building, each costing the seat a turn. The budget moves to fifteen seconds at the provider and thirteen at the worker (the parent keeps two), so a loaded host still ends turns; the fail-closed refusal on a genuine hang stays, it just comes later.
- Origin: main
- Next step: MECHANICAL, tier-1 lane candidate (three files, a few lines): the Stop hook timeout in the repository's Claude settings (5 -> 15), the worker deadline in the deadline parent in supervision-hook.sh (started + 4 -> started + 13) and its comment (state that the number is ours and where it lives), and the deadline scenario in supervision-hook-fixtures.sh (the wrapper's sleep and the 'exceeded the provider's five-second budget' assertion follow the new numbers: sleep just under the worker deadline, assert under the provider timeout; message names fifteen). Receipt: the whole supervision-hook fixture suite. This does not replace stop-hook-health-cost: the hook is still too expensive for its budget on an idle machine and that goal makes it cheap; this one stops a loaded host from refusing healthy turn ends in the meantime. Wido raised the question 2026-09-06 ('I'm not sure that it needs to be this tight'); the number fifteen is a proposal for his approval, one edit if he prefers ten.
- OpenedAt: 2026-09-06T07:39:12Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-06T07:39:12Z 2BXWAPKW3W8TGXDRADDWH9XH8N-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=stop-hook-budget-is-ours
Integrity: sha256=5ea33a0b06426164fc40851c0d1914a6afb42813995f70943481ce0344ffe290
