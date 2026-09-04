# dispatch-fixture-recollection-wallclock-cap

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=2 basis="One fixture leg with a forty-second wall-clock cap that fails under load; nothing outside the suite is affected, but every red in this scenario hides the legs behind it."
- Tier: 2
- Intent: scripts/agents/dispatch-fixtures.sh, scenario dispatch, fails at 'recollection did not conclude the delivered-then-lost critic (elapsed: 40s; scaled cap: 40s)' when the Mac is loaded (m2 2026-09-04 17:35Z, with another suite tracing concurrently); the leg waits on wall-clock instead of counting the recollection's observable passes, against the patience rule (attempts, not wall-clock, with a silence-only failsafe). DONE means the leg counts observed recollection passes with a silence-only failsafe and passes on a loaded Mac; the scenario proceeds to the critic-close legs.
- Origin: main
- Next step: One fixture leg: build, run dispatch-fixtures.sh seat-side, land through a chain. Approved under R-76-m2.
- OpenedAt: 2026-09-04T17:35:51Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-04T17:35:51Z Q8PMTG441VR7FEGM4YC2MWNKPC-m2-5fcf08ab open actor=human:Wido targets=dispatch-fixture-recollection-wallclock-cap
Integrity: sha256=2e8bb47cbda9a90d97ef25850bb6982235f2689bf2623c0274ff9d11ecfbe2e3
