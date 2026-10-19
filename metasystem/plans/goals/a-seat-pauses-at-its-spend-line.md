# a-seat-pauses-at-its-spend-line

- State: parked
- Risk: severity=2 novelty=2 exposure=3 accumulation=2 basis="A wrong hold delays a seat by at most one day window and a person can resume it (severity 2); it reuses the spend reader, alert episodes and the seat ladder (novelty 2); every automatic seat passes the guard (exposure 3); one new record and one episode kind (accumulation 2)"
- Tier: 2
- Intent: A seat that passes its daily spend line finishes its unit, writes its handoff, pauses and tells the person in local time; it resumes at the next day window or, on a person's word, for the rest of the day
- Origin: main
- Next step: U2 dry run (pause refused, would-hold calibration, helm report-only pass, exempt states), U3 the machine list and status surfaces, U4 the hold with machine resume and the clock exclusion (U4a/U4b if the measured inventory passes 250), then spend.mode=pause in the root conf; blocked by a-stop-waits-for-the-unit-boundary; design plans/designs/a-stop-waits-for-the-unit-boundary.md
- OpenedAt: 2026-10-19T09:44:31Z
- Revision: 1
- BlockedBy: a-stop-waits-for-the-unit-boundary
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Parked: by=human:Wido at=2026-10-19T09:44:31Z blocker=a-stop-waits-for-the-unit-boundary because=blocked by a-stop-waits-for-the-unit-boundary; returns when it is done

History:
- 2026-10-19T09:44:31Z SEGKKGMMJDETGRY5VYN4X3C5V6-m1e-c5a1e456 open actor=human:Wido targets=a-seat-pauses-at-its-spend-line,a-stop-waits-for-the-unit-boundary
Integrity: sha256=4b4d447594fd4e2296ff36a0876c9d0a886ae99a4f3c0f2322a3abc73c246ac3
