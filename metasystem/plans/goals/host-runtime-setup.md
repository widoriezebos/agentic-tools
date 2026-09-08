# host-runtime-setup

- State: approved
- Priority: 1
- Sequence: 2
- Risk: severity=3 novelty=2 exposure=2 accumulation=1 basis="Severity 3: incorrect lifecycle hooks can misidentify a coordinator or lose stop enforcement. Novelty 2: extend the existing runtime registration and hook owners. Exposure 2: repository host setup and lifecycle entry. Accumulation 1: one shared installation path."
- Tier: 3
- Intent: Make this MetaSystem checkout and adopted installations discoverable and correctly supervised under Claude Code, Devin, and Codex, with repeatable automatic setup and explicit runtime selection, preserving existing Claude behavior and unrelated settings.
- Origin: human
- Next step: Source delivered on origin/main at 8ed97738; full suite and normal landing passed, canonical engine rearmed at generation 27, and all three runtime setup/authentication probes passed. Human administrative closure remains. User has made coordinator-loop-prevention the immediate priority.
- OpenedAt: 2026-09-07T05:41:13Z
- Revision: 9
- Budget: elapsedLimit=9d attemptLimit=24 reservedJobMinutesLimit=2880 activeJobLimit=2 reviewRoundLimit=3
- BudgetExceptions: 2
- NormApproval: approvedRef=R-86-m1c minutes=2880 reviewRounds=3 goalRevision=5
- Approved: by=human:Wido at=2026-09-07T19:03:27Z revision=6 opid=9ZZ7SHCGZPWJMZ3JGHSZA381Z4-m1c-7cd0bd60 authority=proven digest=ded6f46db604b60a1a0988b39282ab1cf914b52edadcc3139971a9e85c76ac62
- Sliced: machine=m1c lineage=main-1788759014-39092-24fa5c revision=3 at=2026-09-07T05:51:33Z

History:
- 2026-09-07T05:41:13Z 627Q493CTPEA321NRM4RG0X5HH-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-07T05:42:17Z 40CKH10008EJAV4AR6C7NGC1QX-m1-1274caf4 approve actor=human:Wido targets=host-runtime-setup
- 2026-09-07T05:46:29Z C5QDX4WYB00BW4Q44EQ9V127FQ-m1c-1274caf4 claim actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-07T05:51:33Z 9MPT30W94WE6JNW8FGYBXSNZ46-m1c-1274caf4 slice-start actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-07T10:34:41Z AS1JDFQSA6T2NJW0BWWHC5ZYVC-m1c-7cd0bd60 set-budget actor=human:Wido targets=host-runtime-setup displaced=m1c+main-1788759014-39092-24fa5c@2026-09-07T05:46:29Z
- 2026-09-07T19:03:27Z 9ZZ7SHCGZPWJMZ3JGHSZA381Z4-m1c-7cd0bd60 set-budget actor=human:Wido targets=host-runtime-setup displaced=m1c+main-1788759014-39092-24fa5c@2026-09-07T10:34:41Z
- 2026-09-08T05:30:30Z 1AJ8YH12YD5FB205WTAC2RKP7N-m1c-1274caf4 edit actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-08T05:30:33Z 1SM22NR0S4GA251X9V0GPVX88E-m1c-1274caf4 release actor=m1c+main-1788759014-39092-24fa5c targets=host-runtime-setup
- 2026-09-08T15:55:25Z ZFGJK03RHKCAK03SYJF7ZETS0F-m1-7cd0bd60 set-priority actor=human:Wido targets=host-runtime-setup reason=priority-order subject=host-runtime-setup from=unranked to=1:2 requested-sequence=2
Integrity: sha256=c6cff2a12d7581af3e81a58f62c71e74a5a698ab73de79ca94f98c864e977b16
