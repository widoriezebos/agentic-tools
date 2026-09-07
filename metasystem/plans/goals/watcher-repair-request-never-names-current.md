# watcher-repair-request-never-names-current

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="severity 3: the seat's watcher repair silently does not happen, and health reads healthy while it does not; novelty 2: the failure is in the owner's own repair branch, not a known seam; exposure 3: every seat runs this owner; accumulation 3: it spun for twenty hours here and nothing bounded it"
- Tier: 3
- Intent: The supervision owner can get stuck in a repair loop it can never complete. On m1d, artifacts/agents/supervision/owner.ndjson carries 1177 consecutive ticks of "watcher-restart-failed: watcher restart request does not name the owner's current watcher", once a minute from 2026-09-06 13:24 to 2026-09-07 09:12 - roughly twenty hours of a repair the owner keeps attempting and keeps refusing to itself, because the pending restart request names a watcher that is no longer the owner's current one. Nothing escalated: the tick kept reporting verdict=continue and observation=healthy while the repair never happened. DONE means the owner either rewrites a stale restart request against its current watcher or abandons it and mints a fresh one, so a repair cannot spin; and a repair that fails more than a few consecutive ticks stops reporting healthy and surfaces as an alert with the request and the current watcher both named.
- Origin: main
- Next step: Read the failed-repair branch in the supervision owner against owner.ndjson on m1d (1177 lines, first at 2026-09-06T13:24:29+02:00, last at 2026-09-07T09:11:18+02:00, cleared when the restart finally completed at 09:12:18). Decide between rewriting the stale request and re-minting it, then add the consecutive-failure escalation so twenty silent hours cannot recur.
- OpenedAt: 2026-09-07T07:21:49Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T07:21:49Z XQNY093C5MJWAYSYAEQ3H79X48-m1d-25755dc0 open actor=m1d+main-1788764558-63534-a15b0d targets=watcher-repair-request-never-names-current
Integrity: sha256=2d649169531da1e7aafde5576bc8a53aed776636d43df2836ad0303a7cb80e9b
