# fixture-review-by-date-rolls-over

- State: claimed
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a red bed and blocked full-width landings, no product defect; novelty 1: a date helper; exposure 3: every seat, every full-width landing, until fixed; accumulation 2: each landing blocked behind it compounds the queue"
- Tier: 3
- Intent: Three fixture legs pass a literal calendar date as the review date of a temporary human word (scripts/agents/dispatch-fixtures.sh near line 1795 and scripts/agents/channel-fixtures.sh, twice: --review-by 2026-09-06). At 2026-09-07 00:00 UTC they all started refusing 'goal approve could not bind its temporary recorded relay: --review-by 2026-09-06 is in the past'; the dispatch scenario is red on main and every full-width landing receipt (which runs the dispatch fixtures) fails from that moment, first casualty chain csc-build1 at 00:11 UTC on m1. DONE means no fixture carries a literal review date: a shared helper computes a date seven days ahead at run time on both BSD and GNU date, and the beds pass again.
- Origin: main
- Next step: CLAIMED BY m1b 2026-09-08 06:2xZ, and the picture has changed twice. TWO LEGS ARE NOW GREEN WITHOUT THIS GOAL: scripts/agents/channel-fixtures.sh carries --fixture-human-authority (landed by another seat and merged forward in a3131fbb) and 'bash scripts/agents/channel-fixtures.sh' exits 0, verified by m1b; cmd/metasystem's TestGoalTemporaryAuthorityRefusesPastAndBeyondHorizon also passes again. ONE LEG IS STILL A LIVE TIME BOMB, and it is the one that matters most: scripts/agents/goal-cli-fixtures.sh lines 521, 524 and 534 still pass literal '--review-by 2026-09-08'. That date is TODAY, so the bed passes today and goes red on 2026-09-09, and goal-cli-fixtures.sh is inside the landing full battery, so every full-width landing on this fleet goes red tomorrow morning. REMEDY, already proven twice in this repository: replace all three with --fixture-human-authority, exactly as scripts/agents/dispatch-fixtures.sh and channel-fixtures.sh now do; that bed's scratch repo is tailored to metasystem.runtimes=fake so the fixture grant is lawful and the probe is clock-independent. The date helper this goal's intent asks for is NOT the answer and should not be written: a computed future date still refuses against TemporaryGoalAuthorityHorizon, which is still 2026-09-06 in internal/governance/types.go. WAITING ON WIDO for one word to dispatch that single implementer round; m1b did not start it unprompted at the end of an overrun night.
- OpenedAt: 2026-09-07T00:14:58Z
- Revision: 7
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
- 2026-09-08T06:07:04Z 7TV2KFQBRKPZHSSAVRV06XG919-m1b-c6925449 edit actor=m1b+main-1788680071-18713-e76d5d targets=fixture-review-by-date-rolls-over
Integrity: sha256=093dbb1e9f007d033be99574af49300f62a390b6c49b8078c8c035ec939e64b0
