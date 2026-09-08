# fixture-review-by-date-rolls-over

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a red bed and blocked full-width landings, no product defect; novelty 1: a date helper; exposure 3: every seat, every full-width landing, until fixed; accumulation 2: each landing blocked behind it compounds the queue"
- Tier: 3
- Intent: Three fixture legs pass a literal calendar date as the review date of a temporary human word (scripts/agents/dispatch-fixtures.sh near line 1795 and scripts/agents/channel-fixtures.sh, twice: --review-by 2026-09-06). At 2026-09-07 00:00 UTC they all started refusing 'goal approve could not bind its temporary recorded relay: --review-by 2026-09-06 is in the past'; the dispatch scenario is red on main and every full-width landing receipt (which runs the dispatch fixtures) fails from that moment, first casualty chain csc-build1 at 00:11 UTC on m1. DONE means no fixture carries a literal review date: a shared helper computes a date seven days ahead at run time on both BSD and GNU date, and the beds pass again.
- Origin: main
- Next step: SCOPE HALVED 2026-09-07 09:5xZ (m1d): the dispatch leg is FIXED AND LANDED at 423e6ed6, by a different route than this goal's date helper. scripts/agents/dispatch-fixtures.sh near line 1795 now passes --fixture-human-authority and carries no date at all: that bed's scratch repo is tailored to metasystem.runtimes=fake, so FixtureModeRoot holds and the fixture grant is lawful, exactly as the budget fixture beside it already did. GoalHumanAuthorityProbe is deliberately independent of the clock and the horizon, so the scenario proves the same thing without a date. The full battery passed on that tree, exitStatus 0, receipt fefccd8d. So the fleet battery is unblocked and no landing is red any more. WHAT REMAINS IS SMALLER AND STILL WIDO'S: (1) scripts/agents/channel-fixtures.sh lines 105 and 170 still carry --review-by 2026-09-06; they cannot take the same remedy because they run in an export of the real repo whose committed conf names the real runtimes, so ConfValue makes fixture authority unavailable there - that bed is NOT in the landing battery, so it blocks nothing, but it is red. (2) The horizon decision this goal already states: extend TemporaryGoalAuthorityHorizon (internal/governance/types.go, ruling R-32-m1), make it configuration, or confirm relayed authority stays retired in fact. Note the date helper this goal's intent asks for is NOT the answer for either leg: computing a future date still refuses against the horizon, which is what a probe run proved. If Wido retires relayed authority, this goal becomes 'tailor the channel export to fake, or drop those two legs' and the helper is never written.
- OpenedAt: 2026-09-07T00:14:58Z
- Revision: 6
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-07T06:09:36Z revision=4 opid=14BZTM7MV65MZEN17A43QGZJFT-m1d-927ecfdd authority=proven digest=c114202ba2ce24a77495dd8c03405f8aa460800a0505a4887546203be6f68696
- Claimed: machine=m1b lineage=main-1788680071-18713-e76d5d at=2026-09-08T06:06:07Z revision=6 accountingRevision=6
- StopCapability: generation=6 revision=6 machine=m1b claimEpoch=1 fenceEpoch=0

History:
- 2026-09-07T00:14:58Z D18MS06VG2D62TXPQH667TQZ2V-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=fixture-review-by-date-rolls-over
- 2026-09-07T00:16:21Z CHY3THRQM6E8TR8ABVA43FK0GY-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=fixture-review-by-date-rolls-over
- 2026-09-07T00:56:23Z RN93S3KETZAF0AT13W8QWJJCN5-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=fixture-review-by-date-rolls-over
- 2026-09-07T06:09:36Z 14BZTM7MV65MZEN17A43QGZJFT-m1d-927ecfdd approve actor=human:Wido targets=fixture-review-by-date-rolls-over
- 2026-09-07T09:16:06Z 72MK51AVP3YTVD167QSC1BD712-m1d-25755dc0 edit actor=m1d+main-1788764558-63534-a15b0d targets=fixture-review-by-date-rolls-over
- 2026-09-08T06:06:07Z MHHT9VY8244MA30ETD6GBR5KKK-m1b-c6925449 claim actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-rolls-over
Integrity: sha256=33b8f1d6e9a06b6ae341983988343810c0f22803425f7fa6ebf016c1548f601b
