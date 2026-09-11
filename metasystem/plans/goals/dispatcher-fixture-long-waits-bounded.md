# dispatcher-fixture-long-waits-bounded

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: The dispatcher fixture section has three long gaps that are waits, not work (46 s after the cannot-mirror cancel-husk step, 19 s and 16 s elsewhere). DONE means what each gap waits for in scripts/agents/dispatch-fixtures.sh and dispatch.sh is named and bounded, saving up to 80 s per run of section/dispatcher-adapter-and-mission-runner-fixtures.
- Origin: human
- Next step: Slice 4 item 5 of plans/suite-speed-plan.md. Measure first (CENSUS-WAIT-MEASUREMENT lines in the retained log), name the wait, then bound it without weakening the fixture's proof. Code critique only.
- OpenedAt: 2026-09-10T12:02:53Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:27Z revision=2 opid=YFGZYZ0XT9CX3P1EY5D3VH2Z3R-m1-f47a9d40 authority=proven digest=ca54f5af32de2d5f2d1ca20b2452912f609b3a40f4cadd3dae1abd29c67fc486
- Claimed: machine=m1e lineage=main-1789030447-51011-5722fc at=2026-09-11T15:15:53Z revision=3 accountingRevision=3 episodeAt=2026-09-11T15:15:53Z episodeRevision=3
- StopCapability: generation=3 revision=3 machine=m1e claimEpoch=1 fenceEpoch=0

History:
- 2026-09-10T12:02:53Z PNVBM3MW0J0PTVC03C145AF6XC-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=dispatcher-fixture-long-waits-bounded
- 2026-09-11T14:32:27Z YFGZYZ0XT9CX3P1EY5D3VH2Z3R-m1-f47a9d40 approve actor=human:Wido targets=dispatcher-fixture-long-waits-bounded
- 2026-09-11T15:15:53Z VDTHR8CEY32MD4FPYXZ91NKFDG-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=dispatcher-fixture-long-waits-bounded
Integrity: sha256=7e711962dce16615eab2ccbd1267e353bb7cf059bbef305f6a7c5140bc333658
