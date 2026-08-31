# standing-validation

- State: claimed
- Intent: The standing milestone-validation obligation's carrier: this goal owns the governed direct validation recurrence - the full validator run that discharges accumulated landing weight (gate weight-check fires at threshold 60). Opened at the 2026-08-31 takeover per the handoff's weight-discharge sequence and the seat record's R-29 departure clause; the obligation record on this goal is the durable authorization trail.
- Origin: main
- Next step: Standing shared process: when weight-check fires, claim, run the governed validation green under the obligation's budget, discharge at the exact green-run boundary (gate weight-discharge), release. First discharge: weight 123/60 over 8 landings since 2026-08-30, under Wido's tuple 2/24h/120m/1 and his LIMITED word (2026-08-31 in-session decision-ask). FIRST DISCHARGE IN PROGRESS: attempt 1 load-killed (watchdog observation gap, goaled); extended under R-32-m2 with fresh epoch (tuple 2/24h/150m/1, obligation r7); attempt 2 (run -c) completed clean of load kills and went red on two wall-clock-patience test failures - steward-tick-load-flake (handed to m3's shelf) and missionrunner-terminate-flake (opened). ONE attempt remains in the 24h window (to ~2026-09-01 12:48Z): rerun after both fixes land, quiet window coordinated with m3.
- OpenedAt: 2026-08-31T10:47:20Z
- Revision: 21
- Budget: elapsedLimit=3d attemptLimit=10 reservedJobMinutesLimit=900 activeJobLimit=1
- Obligation: revision=21 budgetRevision=20 state=LIMITED owner=Wido authorizedBy=wido authorizedAt=2026-08-31T22:08:44Z authorityOperation=WX1G6X0PD88RN80T76JYFDHKX3-m2-bc1be9cb reviewPolicy=C reviewOutcome=human-approved effects=authorize-governed-launch,authorize-spend,discharge-obligation authorizedEffects=authorize-governed-launch,authorize-spend,discharge-obligation authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06
- ObligationAssumptions: recurrence=standing-shared-process platform=darwin/amd64 toolchainIdentity=go1.26.6 surfaceDigest=c2d2302b4aeea66eee429c044676c8f943fd9e31490aada111a203c4e941e748 maxActiveJobs=1 timingEnvelopeSeconds=7200 observationSource=run-terminal-record
- ObligationTriggers: valueJudgment=no reversibility=reversible severeHarm=no unfamiliarApproach=no testDiscrimination=strong correlatedAssumptionRisk=no authorityScopeChange=no destructiveReach=none
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-31T22:08:13Z revision=20
- StopCapability: generation=20 revision=20 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T10:47:20Z H03B8PM6P6FCS5DQND5F7R3MKX-m2-bc1be9cb open actor=m2+mac-coordinator targets=standing-validation
- 2026-08-31T10:47:47Z GSYBYYWYSR2PH1YZ6HZ5MGHPHR-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T10:48:08Z WD2SKDV7MB75XTK0CJM6SRW9BX-m2-bc1be9cb claim actor=m2+mac-coordinator targets=standing-validation
- 2026-08-31T10:49:41Z GB17G21NP4V3PNCXGD0M7RK6T7-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T11:46:50Z 31FJMDPCYJV6RHFTVYY44Z3X5Y-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T11:48:05Z 4ZJY74HW86RE1562F6EJJCY3QV-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T11:48:45Z Y8M56Z36EBQVV8V70M0Z8V9ECX-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T12:34:55Z F4TP2H9V7AJFM2XN8A69TGRWMP-m2-bc1be9cb edit actor=m2+mac-coordinator targets=standing-validation
- 2026-08-31T13:40:40Z 7D9VWANVXM5D4C5NZYGZ3DW2RK-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T13:41:18Z CEQPDNRH476DQEDZD1BQNM5HAY-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T17:43:28Z CR70F83HP822J8TTGVASSQY6GH-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T17:43:51Z Z2FXVG293KRZQBX2PJT5RXFPQD-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T18:06:16Z PST848PCAFWBWPR5MZKWPKFJNW-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T18:06:39Z 1STYXZSQJE97K6MMKG3X6VCDJY-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T19:09:53Z S2HV3NC4YZ57PQ4YX0PW2NFC4Y-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T19:11:43Z ZD95JDT9GSQY65H1PMN36W283Q-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T19:32:54Z GAG6MDBD7VG5ZAKTFNN9JF6ABQ-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T19:34:22Z XDFEMQQ65Z6YYNGQ5NE0BVZCXP-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T20:09:17Z W1N9686GKF8G1QQZ1Y2AE05384-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T22:08:13Z 2FYYJZPZ23N64FMWQYWK6N25TC-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T22:08:44Z WX1G6X0PD88RN80T76JYFDHKX3-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
Integrity: sha256=12df84d53bbdeac3b272c93b919c3725f72418aa829c7b38c21df085d42b372b
