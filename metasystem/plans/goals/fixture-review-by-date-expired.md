# fixture-review-by-date-expired

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: a fixture bed goes red on the calendar, nothing in production; novelty 1: compute a date; exposure 1: every seat landing through the full battery from 2026-09-07 on; accumulation 1: one fix"
- Tier: 1
- Intent: scripts/agents/dispatch-fixtures.sh (line 1795) and scripts/agents/channel-fixtures.sh (lines 105, 170) pass --review-by 2026-09-06 hard-coded; from 2026-09-07 the temporary-human-word scenario refuses the past date and the dispatch bed goes red before its assertions, so every full-battery landing on the fleet fails until it is fixed. DONE means the review-by date is computed at run time (today plus one day, UTC) in both beds and the beds are green again
- Origin: main
- Next step: ROOT CAUSE found 2026-09-07 by the stop-verb build (round 4): the blocker is not only the hard-coded literal but the constant TemporaryGoalAuthorityHorizon = 2026-09-06 in internal/governance/types.go (ruling R-32-m1), enforced in internal/humanauthority/authority.go (--review-by may not exceed the horizon) and internal/goal/file.go (a goal carrying temporary authority is expired once the horizon passes). Since 2026-09-07 the relayed/temporary human word is dead fleet-wide and the dispatch fixture bed refuses before its assertions, so every full-battery landing fails. The stop-verb chain landed the run-time date computation in both beds; extending or retiring the horizon is Wido's policy act and this goal carries it. Ask Wido: extend the horizon (a new date in internal/governance/types.go with the ruling row) or accept that relayed authority stays retired and change the fixtures to stop exercising it.
- OpenedAt: 2026-09-07T00:54:29Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-07T00:54:29Z AFXY0YFPDFP42YTHS2GJPE0SNE-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
- 2026-09-07T05:27:51Z 4E95BZ3SQ03K8H4XKTT6P5VQ01-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
Integrity: sha256=0c48403d170d863173ea44bbe5ff7e393ec9bf5c770b758bf91862a6fd79754b
