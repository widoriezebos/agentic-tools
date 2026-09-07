# fixture-review-by-date-expired

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: a fixture bed goes red on the calendar, nothing in production; novelty 1: compute a date; exposure 1: every seat landing through the full battery from 2026-09-07 on; accumulation 1: one fix"
- Tier: 1
- Intent: scripts/agents/dispatch-fixtures.sh (line 1795) and scripts/agents/channel-fixtures.sh (lines 105, 170) pass --review-by 2026-09-06 hard-coded; from 2026-09-07 the temporary-human-word scenario refuses the past date and the dispatch bed goes red before its assertions, so every full-battery landing on the fleet fails until it is fixed. DONE means the review-by date is computed at run time (today plus one day, UTC) in both beds and the beds are green again
- Origin: main
- Next step: m1b is folding this fix into the stop-verb build chain (stopverb-build1 round 3, 2026-09-07 01:0xZ) because that chain already owns dispatch-fixtures.sh; if that chain lands first, mark this goal done against its landing. Otherwise: approve, claim, one implementer round replacing the literal with a date computed at run time, land.
- OpenedAt: 2026-09-07T00:54:29Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-07T00:54:29Z AFXY0YFPDFP42YTHS2GJPE0SNE-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
Integrity: sha256=2e452dc306d718804d11da1ef82df134d2bb011616b02bef366e291314853786
