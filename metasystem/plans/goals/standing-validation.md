# standing-validation

- State: claimed
- Intent: The standing milestone-validation obligation's carrier: this goal owns the governed direct validation recurrence - the full validator run that discharges accumulated landing weight (gate weight-check fires at threshold 60). Opened at the 2026-08-31 takeover per the handoff's weight-discharge sequence and the seat record's R-29 departure clause; the obligation record on this goal is the durable authorization trail.
- Origin: main
- Next step: Standing shared process: when weight-check fires, claim, run the governed validation green under the obligation's budget, discharge at the exact green-run boundary (gate weight-discharge), release. First discharge: weight 123/60 over 8 landings since 2026-08-30, under Wido's tuple 2/24h/120m/1 and his LIMITED word (2026-08-31 in-session decision-ask). FIRST DISCHARGE IN PROGRESS: attempt 1 load-killed (watchdog observation gap, goaled); extended under R-32-m2 with fresh epoch (tuple 2/24h/150m/1, obligation r7); attempt 2 (run -c) completed clean of load kills and went red on two wall-clock-patience test failures - steward-tick-load-flake (handed to m3's shelf) and missionrunner-terminate-flake (opened). ONE attempt remains in the 24h window (to ~2026-09-01 12:48Z): rerun after both fixes land, quiet window coordinated with m3.
- OpenedAt: 2026-08-31T10:47:20Z
- Revision: 8
- Budget: elapsedLimit=3d attemptLimit=2 reservedJobMinutesLimit=150 activeJobLimit=1
- Obligation: revision=7 budgetRevision=6 state=LIMITED owner=Wido authorizedBy=wido authorizedAt=2026-08-31T11:48:45Z authorityOperation=Y8M56Z36EBQVV8V70M0Z8V9ECX-m2-bc1be9cb reviewPolicy=C reviewOutcome=human-approved effects=authorize-governed-launch,authorize-spend,discharge-obligation authorizedEffects=authorize-governed-launch,authorize-spend,discharge-obligation authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06
- ObligationAssumptions: recurrence=standing-shared-process platform=darwin/amd64 toolchainIdentity=go1.26.6 surfaceDigest=0eb6243f9c17400cb05c0891367ab3ebae725dc6e1db608344fcc51e606d384e maxActiveJobs=1 timingEnvelopeSeconds=7200 observationSource=run-terminal-record
- ObligationTriggers: valueJudgment=no reversibility=reversible severeHarm=no unfamiliarApproach=no testDiscrimination=strong correlatedAssumptionRisk=no authorityScopeChange=no destructiveReach=none
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-31T11:48:05Z revision=6
- StopCapability: generation=6 revision=6 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-31T10:47:20Z H03B8PM6P6FCS5DQND5F7R3MKX-m2-bc1be9cb open actor=m2+mac-coordinator targets=standing-validation
- 2026-08-31T10:47:47Z GSYBYYWYSR2PH1YZ6HZ5MGHPHR-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T10:48:08Z WD2SKDV7MB75XTK0CJM6SRW9BX-m2-bc1be9cb claim actor=m2+mac-coordinator targets=standing-validation
- 2026-08-31T10:49:41Z GB17G21NP4V3PNCXGD0M7RK6T7-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T11:46:50Z 31FJMDPCYJV6RHFTVYY44Z3X5Y-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T11:48:05Z 4ZJY74HW86RE1562F6EJJCY3QV-m2-bc1be9cb set-budget actor=human:wido targets=standing-validation
- 2026-08-31T11:48:45Z Y8M56Z36EBQVV8V70M0Z8V9ECX-m2-bc1be9cb set-obligation actor=human:wido targets=standing-validation
- 2026-08-31T12:34:55Z F4TP2H9V7AJFM2XN8A69TGRWMP-m2-bc1be9cb edit actor=m2+mac-coordinator targets=standing-validation
Integrity: sha256=65723d5e8cd132cd624c58ea0a3982d06cb52012b54b17cd245bdcd5a765d691
