# missionrunner-suite-speed

- State: claimed
- Intent: The missionrunner suite runs ~10 minutes (161 tests, InternalRun cycles 10-16s each with real waits) and lives at the edge of the default 600s package timeout - it flakes red under machine load, violating the fast-test law; found 2026-08-29 when parallel gauntlets tipped it over
- Origin: main
- Next step: Appetite: 2h — drive the mission cycles from a synthetic clock instead of real sleeps (coordinate with the shared-label timing-tests-synthetic-clock item); target: full suite under 90s; until it lands every gauntlet runs missionrunner with -timeout 900s
- OpenedAt: 2026-08-28T23:16:03Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=8 reservedJobMinutesLimit=120 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-29T11:30:31Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-28T23:16:03Z S8A5SD1ZQFTZDS295VVP3ACTH5-m1-bf243850 open actor=m1+coordinator targets=missionrunner-suite-speed
- 2026-08-29T11:30:17Z YTB71XE1FBJKGD7S7F97C1HNEW-m2-bc1be9cb set-budget actor=human:wido targets=missionrunner-suite-speed
- 2026-08-29T11:30:31Z T58KGN1VA7TPE415794J0TJ5AT-m2-bc1be9cb claim actor=m2+mac-coordinator targets=missionrunner-suite-speed
Integrity: sha256=934c69d69a6e10c3be1ccd0e3d8a78661aa01076645ec1d637a17431c290321d
