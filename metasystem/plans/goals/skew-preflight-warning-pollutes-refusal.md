# skew-preflight-warning-pollutes-refusal

- State: queued
- Tier: 1
- Intent: The engine-skew preflight in scripts/agents/dispatch.sh (landed 269e4cdb) prints a warning to stderr on every dispatch whose engine stamp cannot be compared with the checkout (unknown commit, shallow clone, dev stamp), and the delegate verb folds the dispatcher's stderr into the refusal detail. In every fixture repository the stamp is a checkout commit unknown to the fixture's own repository, so the warning appears on every fixture dispatch and breaks the dispatch scenario's permission-envelope leg (dispatch-fixtures.sh line 2083 expects the detail 'permission roots must be arrays' exactly). Found seat-side on m2 2026-09-04 13:00Z while landing the fixture-suite drift fix. DONE means the preflight is silent unless it refuses (the allow-reasons go to the job's events log if one exists at that point), the refusal keeps its full message, and the dispatch scenario passes its permission-envelope leg.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one shell function, a fixture leg): build, run dispatch-fixtures.sh seat-side, land through a chain; box 1h/3/60m/1. Waits for human approval for execution; Wido 2026-09-04: 'land what you can, leave the rest on the backlog'.
- OpenedAt: 2026-09-04T13:13:56Z
- Revision: 1
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0

History:
- 2026-09-04T13:13:56Z SNFF55WETAWGYN4Z8BMRA6X7G4-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=skew-preflight-warning-pollutes-refusal
Integrity: sha256=a0152cb12de7329287ebe6dfba24b1c8217a66bceb49027eda8c6c25c01cc9b7
