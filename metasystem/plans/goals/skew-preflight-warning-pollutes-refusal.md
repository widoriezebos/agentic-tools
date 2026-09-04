# skew-preflight-warning-pollutes-refusal

- State: claimed
- Tier: 1
- Intent: The engine-skew preflight in scripts/agents/dispatch.sh (landed 269e4cdb) prints a warning to stderr on every dispatch whose engine stamp cannot be compared with the checkout (unknown commit, shallow clone, dev stamp), and the delegate verb folds the dispatcher's stderr into the refusal detail. In every fixture repository the stamp is a checkout commit unknown to the fixture's own repository, so the warning appears on every fixture dispatch and breaks the dispatch scenario's permission-envelope leg (dispatch-fixtures.sh line 2083 expects the detail 'permission roots must be arrays' exactly). Found seat-side on m2 2026-09-04 13:00Z while landing the fixture-suite drift fix. DONE means the preflight is silent unless it refuses (the allow-reasons go to the job's events log if one exists at that point), the refusal keeps its full message, and the dispatch scenario passes its permission-envelope leg.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one shell function, a fixture leg): build, run dispatch-fixtures.sh seat-side, land through a chain; box 1h/3/60m/1. Waits for human approval for execution; Wido 2026-09-04: 'land what you can, leave the rest on the backlog'.
- OpenedAt: 2026-09-04T13:13:56Z
- Revision: 4
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T13:33:54Z revision=2 opid=S39GRJJC63EWMPC8TT8AT6S2C6-m2-5fcf08ab authority=relayed digest=adc0e711e57faa92f4be1d30913febfe88d4a0dc5d53b6a95d3aec6749c4191e reviewBy=2026-09-06
- Sliced: machine=m2 lineage=main-1788441779-14484-82d6ed revision=3 at=2026-09-04T13:34:10Z
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T13:34:00Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-04T13:13:56Z SNFF55WETAWGYN4Z8BMRA6X7G4-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=skew-preflight-warning-pollutes-refusal
- 2026-09-04T13:33:54Z S39GRJJC63EWMPC8TT8AT6S2C6-m2-5fcf08ab approve actor=human:Wido targets=skew-preflight-warning-pollutes-refusal authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
- 2026-09-04T13:34:00Z TM936QD33K9J9WFA3DEHCTG0ME-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=skew-preflight-warning-pollutes-refusal
- 2026-09-04T13:34:10Z P87SXSH4FT251X9RMQ3RA9GBEM-m2-5fcf08ab slice-start actor=m2+main-1788441779-14484-82d6ed targets=skew-preflight-warning-pollutes-refusal
Integrity: sha256=3e2a436dc4afbcd48b0de7357dff1b7114f29be3c9ad22b69b05eadd591594f3
