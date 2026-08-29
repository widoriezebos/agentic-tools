# missionrunner-suite-speed

- State: done
- Intent: The missionrunner suite runs ~10 minutes (161 tests, InternalRun cycles 10-16s each with real waits) and lives at the edge of the default 600s package timeout - it flakes red under machine load, violating the fast-test law; found 2026-08-29 when parallel gauntlets tipped it over
- Origin: main
- Next step: Appetite: 2h — drive the mission cycles from a synthetic clock instead of real sleeps (coordinate with the shared-label timing-tests-synthetic-clock item); target: full suite under 90s; until it lands every gauntlet runs missionrunner with -timeout 900s
- Concluded: Landed 9e1c291: the wait seam (ScaledWait/ScaledWaitAtLeast, floors for real facts, backoff converted) production-identical at default scale; compression opt-in after it exposed four named defect classes (second-granular taint identity, compounding nested windows, recovery livelocks, cross-test state leak) — each pinned in code with its reason. The <90s target transfers to timing-tests-synthetic-clock WITH its sizing: full 289-test audit shows the giants are real git churn, so the conversion arc's next slices are state-leak fix + parallelism, not more wait compression. Well inside budget.
- OpenedAt: 2026-08-28T23:16:03Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=8 reservedJobMinutesLimit=120 activeJobLimit=1

History:
- 2026-08-28T23:16:03Z S8A5SD1ZQFTZDS295VVP3ACTH5-m1-bf243850 open actor=m1+coordinator targets=missionrunner-suite-speed
- 2026-08-29T11:30:17Z YTB71XE1FBJKGD7S7F97C1HNEW-m2-bc1be9cb set-budget actor=human:wido targets=missionrunner-suite-speed
- 2026-08-29T11:30:31Z T58KGN1VA7TPE415794J0TJ5AT-m2-bc1be9cb claim actor=m2+mac-coordinator targets=missionrunner-suite-speed
- 2026-08-29T14:11:37Z PARQ4V54QB7HRGJD5WW21Z0K3Q-m2-bc1be9cb done actor=m2+mac-coordinator targets=missionrunner-suite-speed
Integrity: sha256=3bdd23a382b419ee2f327ec82f73b2afa54520e7596c80f3949e3539b7a2f4dc
