# fixture-suite-drift-carry

- State: claimed
- Tier: 1
- Intent: Finish and carry the fixture-suite drift fix (goal fixture-suite-drift-after-approval-gate; preserve/fsd-build1-r3, seven files over three rounds) through one chain to main: the one remaining defect is the dispatch scenario's serving-goal leg, which must approve and claim the goal it opens because a converted checkout serves the machine's claimed goal. The parent's tier-1 box closed at three rounds; Wido answered 'allow two more rounds' on the channel (question CF77YSK1TTFRE26C0D9WNN8537, 2026-09-04 12:01Z) and the ledger binds no raise from a channel answer nor a second relayed approval on one goal, so the rounds are spent here as the engine's suggested arc split. DONE means the five suites (channel, dispatch, supervision, adopt, static-reproof) run green seat-side and the change lands as a chain landing; the parent concludes.
- Origin: main
- Next step: TIER 1: one chain (cherry-pick the preserve branch, fix the serving-goal leg, return with repository-relative paths), seat-side suite runs, land; box 1h/3/60m/1.
- OpenedAt: 2026-09-04T12:03:05Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- Approved: by=human:Wido at=2026-09-04T12:03:13Z revision=2 opid=2KZPNAQF9K93C1D1C1P9DQDJEJ-m2-5fcf08ab authority=relayed digest=2253667438587ac4da11e8904b2ec6807c918a73df6d85e6ced742ad44800aa6 reviewBy=2026-09-06
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T12:08:11Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-04T12:03:05Z 4FNJAHAR8EXXRFZ9H4WCKY3DJ2-m2-5fcf08ab open actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-carry
- 2026-09-04T12:03:13Z 2KZPNAQF9K93C1D1C1P9DQDJEJ-m2-5fcf08ab approve actor=human:Wido targets=fixture-suite-drift-carry authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="allow two more rounds"
- 2026-09-04T12:08:11Z V59GTKPTY8Q05DM0ANS66NTXG0-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=fixture-suite-drift-carry
Integrity: sha256=cd34f332bd8772370e6a61c28d67738ac88a8a434d1217f2e1d0a17afe0ef7d2
