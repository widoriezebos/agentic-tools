# supervision-fixture-census-lifecycle-red

- State: claimed
- Tier: 1
- Intent: The supervision fixture suite's census-lifecycle scenario is red on plain main (m2, 2026-09-04 12:5xZ, evidence under artifacts/agents/suite-failures of the run), after its enumerate-filter-resolve leg passes; the idle-hook scenario was red on the same baseline run and green with the fixture-suite drift fix, so it may be timing-sensitive. DONE means both scenarios pass on a Mac, with the cause fixed where it lives (fixture expectation or product) and named in the landing.
- Origin: main
- Next step: TIER 1 per R-54-m1 (two scenarios): build, run supervision-fixtures.sh seat-side twice, land through a chain; box 1h/3/60m/1. Waits for human approval for execution; Wido 2026-09-04: 'land what you can, leave the rest on the backlog'.
- OpenedAt: 2026-09-04T13:14:11Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T18:50:56Z revision=2 opid=0WER692HT5CWS2AFDHT9A2ZC7M-m2-5fcf08ab authority=relayed digest=568b8e8b38b7efaa10a00dc900ae5e8768080347fbf4297440c4ba876d4991f1 reviewBy=2026-09-06
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T19:56:13Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-04T13:14:11Z 6GG49RSQTCKD71JVKJY5BAQMR8-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=supervision-fixture-census-lifecycle-red
- 2026-09-04T18:50:56Z 0WER692HT5CWS2AFDHT9A2ZC7M-m2-5fcf08ab approve actor=human:Wido targets=supervision-fixture-census-lifecycle-red authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
- 2026-09-04T19:56:13Z 5AJBHET3PAT4D0B9MZDRP87WQH-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=supervision-fixture-census-lifecycle-red
Integrity: sha256=335602a5f2a103b254ef5a79bca21317ff61b04bb813c5c7654633fe955e1f2d
