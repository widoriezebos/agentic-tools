# fixture-review-by-date-rolls-over

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=2 basis="severity 2: a red bed and blocked full-width landings, no product defect; novelty 1: a date helper; exposure 3: every seat, every full-width landing, until fixed; accumulation 2: each landing blocked behind it compounds the queue"
- Tier: 3
- Intent: Three fixture legs pass a literal calendar date as the review date of a temporary human word (scripts/agents/dispatch-fixtures.sh near line 1795 and scripts/agents/channel-fixtures.sh, twice: --review-by 2026-09-06). At 2026-09-07 00:00 UTC they all started refusing 'goal approve could not bind its temporary recorded relay: --review-by 2026-09-06 is in the past'; the dispatch scenario is red on main and every full-width landing receipt (which runs the dispatch fixtures) fails from that moment, first casualty chain csc-build1 at 00:11 UTC on m1. DONE means no fixture carries a literal review date: a shared helper computes a date seven days ahead at run time on both BSD and GNU date, and the beds pass again.
- Origin: main
- Next step: MECHANICAL: brief a shared helper fixture_review_by_date (GNU then BSD date) replacing the three literals; build; land as an area chain (accumulation 2 makes it full width: the receipt itself needs this fix, so land it through the tier lane the accumulation allows or ask for the exception); rerun the dispatch bed on m1.
- OpenedAt: 2026-09-07T00:14:58Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-07T00:14:58Z D18MS06VG2D62TXPQH667TQZ2V-m1c-7cd0bd60 open actor=m1c+main-1788680061-17829-64951c targets=fixture-review-by-date-rolls-over
Integrity: sha256=e3741ce1867dcbb1c555df58db51246508173ae45cd65d449d89f52644595b7d
