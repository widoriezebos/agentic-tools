# fixture-review-by-date-rolls-over

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a red bed and blocked full-width landings, no product defect; novelty 1: a date helper; exposure 3: every seat, every full-width landing, until fixed; accumulation 2: each landing blocked behind it compounds the queue"
- Tier: 3
- Intent: Three fixture legs pass a literal calendar date as the review date of a temporary human word (scripts/agents/dispatch-fixtures.sh near line 1795 and scripts/agents/channel-fixtures.sh, twice: --review-by 2026-09-06). At 2026-09-07 00:00 UTC they all started refusing 'goal approve could not bind its temporary recorded relay: --review-by 2026-09-06 is in the past'; the dispatch scenario is red on main and every full-width landing receipt (which runs the dispatch fixtures) fails from that moment, first casualty chain csc-build1 at 00:11 UTC on m1. DONE means no fixture carries a literal review date: a shared helper computes a date seven days ahead at run time on both BSD and GNU date, and the beds pass again.
- Origin: main
- Next step: NEEDS WIDO'S DECISION, WIDER THAN FIRST SEEN (m1c 2026-09-07 01:00Z): the fixture literal is only the surface. internal/governance/types.go sets TemporaryGoalAuthorityHorizon = "2026-09-06" (ruling R-32-m1, landed dfb72ef2); since 2026-09-07 00:00 UTC every temporary-word act refuses ('the temporary authority horizon 2026-09-06 has passed' / '--review-by <date> exceeds temporary goal authority horizon 2026-09-06'), fleet-wide, not only in fixtures. A probe run of the dispatch bed with the fixture literal moved to 2026-09-14 failed on exactly that horizon. So the fixture cannot be fixed by a date helper alone. Two lawful shapes, both Wido's: (a) move the horizon (his relayed word was extended to 2026-09-12 on 2026-09-05; the constant was not) or make it configuration; (b) retire the temporary-word class now that the enrolled terminal exists, and make the fixtures use fixture authority. Until his word: full-width landings are blocked (their receipt runs the dispatch bed); m1c lands area-width chains and records the bed as owed on each. Approving this goal alone is not enough; his decision on (a) or (b) is.
- OpenedAt: 2026-09-07T00:14:58Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T00:14:58Z D18MS06VG2D62TXPQH667TQZ2V-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=fixture-review-by-date-rolls-over
- 2026-09-07T00:16:21Z CHY3THRQM6E8TR8ABVA43FK0GY-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=fixture-review-by-date-rolls-over
- 2026-09-07T00:56:23Z RN93S3KETZAF0AT13W8QWJJCN5-m1c-7cd0bd60 edit actor=m1c+main-1788680061-17829-64951c targets=fixture-review-by-date-rolls-over
Integrity: sha256=ee2eeb1d17a59c308675896830a179668424f9e4f7316a6b39e58e3276141d37
