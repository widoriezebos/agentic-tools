# setup-refusals-consume-attempts

- State: claimed
- Tier: 1
- Intent: A dispatch refused before any agent ran (refusalClass setup: a census fingerprint that has not caught up with a freshly armed engine, a brief header defect, a script-engine skew) still consumes one of the goal's attempts and its reserved job minutes. On m2 on 2026-09-04 the dispatcher-skew item lost two of its three tier-1 attempts to setup refusals (a census race right after steward arm, and a probe of the same refusal) and its box closed while the real work sat finished on a preserve branch. DONE means a job record that ends in a setup refusal releases its reservation and does not count as an attempt; the goalbudget test pins it; a refusal that happens after the agent started still counts.
- Origin: main
- Next step: TIER 1 per R-54-m1 (one branch in the budget admission and its test): build, go test ./internal/goalbudget/... ./internal/dispatch/..., land through a chain; box 1h/3/60m/1. Waits for human approval for execution.
- OpenedAt: 2026-09-04T10:10:08Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T16:40:49Z revision=2 opid=JDFN5P7295T332AKRXBZCEWZN7-m2-5fcf08ab authority=relayed digest=4f965f7c29b6d5a2453e9c44481c4b9ae2a1054d63b5eda4157470a99a86ce6a reviewBy=2026-09-06
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T16:40:53Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-04T10:10:08Z YXF7VM88PN6VYT2PKPAZJ8VYX8-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=setup-refusals-consume-attempts
- 2026-09-04T16:40:49Z JDFN5P7295T332AKRXBZCEWZN7-m2-5fcf08ab approve actor=human:Wido targets=setup-refusals-consume-attempts authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
- 2026-09-04T16:40:53Z 8YD8JD9N9Z67R57NW1EJ322M0C-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=setup-refusals-consume-attempts
Integrity: sha256=b1a80ec795ad67f1b52ced3c7cfa763e04ae23e77fa21958ba2183061a49ee52
