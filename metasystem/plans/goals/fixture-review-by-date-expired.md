# fixture-review-by-date-expired

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: a fixture bed goes red on the calendar, nothing in production; novelty 1: compute a date; exposure 1: every seat landing through the full battery from 2026-09-07 on; accumulation 1: one fix"
- Tier: 1
- Intent: scripts/agents/dispatch-fixtures.sh (line 1795) and scripts/agents/channel-fixtures.sh (lines 105, 170) pass --review-by 2026-09-06 hard-coded; from 2026-09-07 the temporary-human-word scenario refuses the past date and the dispatch bed goes red before its assertions, so every full-battery landing on the fleet fails until it is fixed. DONE means the review-by date is computed at run time (today plus one day, UTC) in both beds and the beds are green again
- Origin: main
- Next step: REMEDY FOUND 2026-09-07 05:4xZ, no policy change needed. Both offending calls are SETUP, not the thing under test: dispatch-fixtures.sh (approve of goal fixture-serving, near line 1795) and channel-fixtures.sh (lines 105 and 170) approve a scratch goal's budget with --temporary-human-word --review-by 2026-09-06. The same beds already use the purpose-built --fixture-human-authority elsewhere (dispatch-fixtures.sh line 1273; goal-cli-fixtures.sh lines 254, 270, 295, 497, 536, 597), which needs no date and no policy clock. DONE means those setup calls use --fixture-human-authority, so no bed depends on TemporaryGoalAuthorityHorizon; if relayed authority itself deserves coverage it gets its own scenario asserting the refusal once the horizon has passed. This unblocks every full-battery landing on the fleet (m1's brain chain included). m1b is folding this into the stop-verb chain (round 6) because that chain already owns both bed files; if it lands there, mark this goal done against that landing. Separately and still Wido's: whether to extend TemporaryGoalAuthorityHorizon past 2026-09-06 or leave relayed authority retired.
- OpenedAt: 2026-09-07T00:54:29Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-07T00:54:29Z AFXY0YFPDFP42YTHS2GJPE0SNE-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
- 2026-09-07T05:27:51Z 4E95BZ3SQ03K8H4XKTT6P5VQ01-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
- 2026-09-07T05:31:53Z W4152JAK88QF13AZ6RPZWSXP9P-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
Integrity: sha256=019d4837f069c0ec0bfd49a27fe72e07ee043b97a5819149f783ed36202dc1d3
