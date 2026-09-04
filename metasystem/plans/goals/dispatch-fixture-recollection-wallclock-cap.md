# dispatch-fixture-recollection-wallclock-cap

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=2 basis="One fixture leg with a forty-second wall-clock cap that fails under load; nothing outside the suite is affected, but every red in this scenario hides the legs behind it."
- Tier: 2
- Intent: scripts/agents/dispatch-fixtures.sh, scenario dispatch, fails at 'recollection did not conclude the delivered-then-lost critic (elapsed: 40s; scaled cap: 40s)' when the Mac is loaded (m2 2026-09-04 17:35Z, with another suite tracing concurrently); the leg waits on wall-clock instead of counting the recollection's observable passes, against the patience rule (attempts, not wall-clock, with a silence-only failsafe). DONE means the leg counts observed recollection passes with a silence-only failsafe and passes on a loaded Mac; the scenario proceeds to the critic-close legs.
- Origin: main
- Next step: One fixture leg: build, run dispatch-fixtures.sh seat-side, land through a chain. Approved under R-76-m2.
- OpenedAt: 2026-09-04T17:35:51Z
- Revision: 2
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T17:35:57Z revision=2 opid=B5NG7Y149H50BW784Q2QY09NXX-m2-5fcf08ab authority=relayed digest=df7f74a96e966bb9eb73fe549d81c45f8ff0c87e2c02ed3dd95233cb61fb2730 reviewBy=2026-09-06

History:
- 2026-09-04T17:35:51Z Q8PMTG441VR7FEGM4YC2MWNKPC-m2-5fcf08ab open actor=human:Wido targets=dispatch-fixture-recollection-wallclock-cap
- 2026-09-04T17:35:57Z B5NG7Y149H50BW784Q2QY09NXX-m2-5fcf08ab approve actor=human:Wido targets=dispatch-fixture-recollection-wallclock-cap authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
Integrity: sha256=3bf2444f31a8eb077dcd9c2ebade355cb48a2a113325c0cbb68493bf6284947d
