# fixture-review-by-date-expired

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: a fixture bed goes red on the calendar, nothing in production; novelty 1: compute a date; exposure 1: every seat landing through the full battery from 2026-09-07 on; accumulation 1: one fix"
- Tier: 1
- Intent: scripts/agents/dispatch-fixtures.sh (line 1795) and scripts/agents/channel-fixtures.sh (lines 105, 170) pass --review-by 2026-09-06 hard-coded; from 2026-09-07 the temporary-human-word scenario refuses the past date and the dispatch bed goes red before its assertions, so every full-battery landing on the fleet fails until it is fixed. DONE means the review-by date is computed at run time (today plus one day, UTC) in both beds and the beds are green again
- Origin: main
- Next step: VERIFIED REMEDY 2026-09-07 05:5xZ (read in internal/humanauthority/authority.go lines 243-252, internal/fixtureauth/fixtureauth.go lines 257-293, internal/config/conf.go line 39). The two rules on --review-by are now mutually unsatisfiable: the date may not be before today (reviewDate.Before(checkedDate)) and may not exceed TemporaryGoalAuthorityHorizon=2026-09-06, so since 2026-09-07 no date passes and computing tomorrow (the stop chain's round-3 change) is refused too. Relayed/temporary human authority is therefore retired in fact, fixtures included. THE BATTERY FIX (unblocks every landing on the fleet): dispatch-fixtures.sh is the only battery bed affected (fullBatteryCommand in internal/landing/tierone.go runs go-gate --fast, dispatch-fixtures.sh, goal-cli-fixtures.sh). Its failing call is the goal approve of fixture-serving near line 1795 in agent_repo, and that repo is tailored to metasystem.runtimes=fake (config tailor --runtimes fake, near line 470), so FixtureModeRoot is true there and --fixture-human-authority is lawful, exactly as the budget fixture already does at line 1273. Switch that call to --fixture-human-authority and drop the review-by computation there; GoalHumanAuthorityProbe is documented as deliberately independent of the clock and the governance horizon. CHANNEL BED, not in the battery and not blocking landings: channel-fixtures.sh lines 105 and 170 run in an export of the real repo whose committed conf says metasystem.runtimes=claude,codex,devin, and ConfValue reads only the committed conf, so fixture authority is unavailable there; leave those two until Wido rules on the horizon, or tailor that export to fake in a later round. STILL WIDO'S: extend TemporaryGoalAuthorityHorizon or confirm relayed authority stays retired.
- OpenedAt: 2026-09-07T00:54:29Z
- Revision: 4
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-07T00:54:29Z AFXY0YFPDFP42YTHS2GJPE0SNE-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
- 2026-09-07T05:27:51Z 4E95BZ3SQ03K8H4XKTT6P5VQ01-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
- 2026-09-07T05:31:53Z W4152JAK88QF13AZ6RPZWSXP9P-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
- 2026-09-07T05:35:43Z 1FH4JQJSG81HB6JCX1ZRYBQAWF-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-expired
Integrity: sha256=46fe3cc891dea99bb4dc33d5fff201a2ccbed59c6eece90560b3ce0d3c923fe4
