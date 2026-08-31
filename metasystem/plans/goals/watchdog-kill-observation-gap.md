# watchdog-kill-observation-gap

- State: queued
- Intent: A watchdog-killed governed run terminalizes with assumptionState=unavailable (empty platform/toolchain/digest, driftedFields=[observation]), which folds to breaker=ASSUMPTION_FAILED - so ANY load kill nukes the obligation into the Wido-must-choose terminal state even when every real assumption held. Specimen: governed-discharge-20260831 (2026-08-31, the first weight-discharge attempt, killed by the section cap during m3's attested heavy window; record artifacts/agents/governed-obligations/standing-validation.g3.o4.json). Under R-32-m2 load-stall leniency this conflation is exactly wrong: a load kill is coordination debt, not an assumption failure.
- Origin: main
- Next step: Appetite: 2h. The kill path observes assumptions before terminalizing (the observer needs no live process - platform/toolchain/digest are repo facts), or a kill records a distinct LOAD-KILLED/UNOBSERVED terminal class that does NOT trip the assumption breaker; either way a watchdog kill must leave the obligation resumable within its remaining budget. Fixture: a governed run killed mid-section terminalizes without ASSUMPTION_FAILED. Also record: the governed budget block in the run record showed elapsedLimit 3d while the goal budget read 24h at launch time - verify which record lied and why (one specimen, same run).
- OpenedAt: 2026-08-31T11:47:26Z
- Revision: 1

History:
- 2026-08-31T11:47:26Z CK8M33HSTVA98ZE88ZENY4GAR7-m2-bc1be9cb open actor=m2+mac-coordinator targets=watchdog-kill-observation-gap
Integrity: sha256=4f967eff51a70d5dd4b7e4cbb9901d44edc6137f2be143c9c8a54a345810c546
