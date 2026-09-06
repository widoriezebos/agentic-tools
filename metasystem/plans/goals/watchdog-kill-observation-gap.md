# watchdog-kill-observation-gap

- State: approved
- Intent: A watchdog-killed governed run terminalizes with assumptionState=unavailable (empty platform/toolchain/digest, driftedFields=[observation]), which folds to breaker=ASSUMPTION_FAILED - so ANY load kill nukes the obligation into the Wido-must-choose terminal state even when every real assumption held. Specimen: governed-discharge-20260831 (2026-08-31, the first weight-discharge attempt, killed by the section cap during m3's attested heavy window; record artifacts/agents/governed-obligations/standing-validation.g3.o4.json). Under R-32-m2 load-stall leniency this conflation is exactly wrong: a load kill is coordination debt, not an assumption failure.
- Origin: main
- Next step: Appetite: 2h. The kill path observes assumptions before terminalizing (the observer needs no live process - platform/toolchain/digest are repo facts), or a kill records a distinct LOAD-KILLED/UNOBSERVED terminal class that does NOT trip the assumption breaker; either way a watchdog kill must leave the obligation resumable within its remaining budget. Fixture: a governed run killed mid-section terminalizes without ASSUMPTION_FAILED. Also record: the governed budget block in the run record showed elapsedLimit 3d while the goal budget read 24h at launch time - verify which record lied and why (one specimen, same run).
- OpenedAt: 2026-08-31T11:47:26Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=82f21d837fdaa8854a8ed71caee2faf873e96c9f839859c414fe4977fc753449

History:
- 2026-08-31T11:47:26Z CK8M33HSTVA98ZE88ZENY4GAR7-m2-bc1be9cb open actor=m2+mac-coordinator targets=watchdog-kill-observation-gap
- 2026-09-01T20:27:47Z WPMTTA62N67DZGNM9V6V3QB91A-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=watchdog-kill-observation-gap
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=watchdog-kill-observation-gap reason=sweep
Integrity: sha256=bc477ffe21746f757e1d4bc4cb7b3c85f5458a0706850a8fa93b25144a4c530e
