# finish-test-repairs-and-integrate

- State: claimed
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="Finish the existing test-runner repair and validation work; runtime correctness protects application landings. No new capability or broader redesign."
- Tier: 3
- Intent: Finish these repairs, verify them with targeted tests, complete final validation, then push/integrate.
- Origin: human
- Next step: Wido redirected this same goal on 23 September: stop the Git-maintenance workaround and replace real Git in behavior tests with per-test mocks/stubs, retaining only explained narrow Git adapter integration tests. Implement the bounded design in plans/test-suite-redesign.md through Sol units: shared strict stub, goal repository fake, then remaining consumers; prove migrated tests with Git denied, preserve parallel/race/coverage behavior, complete required validation, commit/push/install and verify. No new goal. Earlier final-validation ETA is superseded.
- OpenedAt: 2026-09-22T16:37:11Z
- Revision: 12
- Budget: elapsedLimit=2d4h attemptLimit=19 reservedJobMinutesLimit=800 activeJobLimit=8 reviewRoundLimit=3
- BudgetExceptions: 5
- Approved: by=human:Wido at=2026-09-23T11:45:40Z revision=11 opid=43WN4GGSZX9SW5864M1D4Z9SMP-m1e-226b0953 authority=proven digest=b652d610a6691203898843e022be84f48c4503ad3e437694c342f78ebc738471 episode=11
- Claimed: machine=m1e lineage=main-1789893000-16595-1a7a6a at=2026-09-23T11:45:40Z revision=11 accountingRevision=11 episodeAt=2026-09-22T17:29:09Z episodeRevision=3
- StopCapability: generation=11 revision=11 machine=m1e claimEpoch=3 fenceEpoch=0

History:
- 2026-09-22T16:37:11Z BPWZD876FT92J8N278RKRXMM1S-m1e-32fbf0db open actor=human:Wido targets=finish-test-repairs-and-integrate
- 2026-09-22T17:29:05Z 9AV3C3S4XWF8NPP3AAP66NR0WG-m1e-226b0953 approve actor=human:Wido targets=finish-test-repairs-and-integrate
- 2026-09-22T17:29:09Z T23RM9Y0KE81KEVX8DXSM46N5T-m1e-226b0953 claim actor=m1e+main-1789893000-16595-1a7a6a targets=finish-test-repairs-and-integrate
- 2026-09-22T21:49:18Z DQPZR6EYWCZDKTSE6C2CY55DFR-m1e-226b0953 edit actor=m1e+main-1789893000-16595-1a7a6a targets=finish-test-repairs-and-integrate
- 2026-09-22T23:45:05Z G35WQVDKDMNKYVXPWRFT1JY61C-m1e-43182c96 breach-stop actor=m1e+goal-stop-custodian targets=finish-test-repairs-and-integrate
- 2026-09-23T05:59:38Z YMR0W4BPR61GZ080R2B0YGHY30-m1e-226b0953 set-budget actor=human:Wido targets=finish-test-repairs-and-integrate resumed=stop-finish-test-repairs-and-integrate-r3-f1
- 2026-09-23T06:00:22Z R06B0S8ECQTMP4M7BHG71VW4QB-m1e-226b0953 edit actor=m1e+main-1789893000-16595-1a7a6a targets=finish-test-repairs-and-integrate
- 2026-09-23T06:01:53Z HXVR04XQ2BVXJF6RMB357RTWD8-m1e-226b0953 set-budget actor=human:Wido targets=finish-test-repairs-and-integrate
- 2026-09-23T08:35:55Z 6KHZWK1YSNZ9XYVJZR7YMJGKRJ-m1e-226b0953 set-budget actor=human:Wido targets=finish-test-repairs-and-integrate
- 2026-09-23T10:02:41Z SDWABF5TYJQRDRE7QBQSAWMEN3-m1e-226b0953 set-budget actor=human:Wido targets=finish-test-repairs-and-integrate
- 2026-09-23T11:45:40Z 43WN4GGSZX9SW5864M1D4Z9SMP-m1e-226b0953 set-budget actor=human:Wido targets=finish-test-repairs-and-integrate
- 2026-09-23T12:27:30Z F0P32927BA623M0J01FQ77E9TB-m1e-226b0953 edit actor=m1e+main-1789893000-16595-1a7a6a targets=finish-test-repairs-and-integrate
Integrity: sha256=99d8bdcc3051ca2a8237dbc9af79cd555c0e50b0f4c361dbc99f343c2c39c85e
