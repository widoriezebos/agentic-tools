# channel-fixture-answer-history-drift

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="One fixture assertion on the goal history after a channel answer; the product's answer path changed twice today and the fixture still expects the old history line; nothing outside the suite is affected."
- Tier: 1
- Intent: scripts/agents/channel-fixtures.sh fails silently (bare exit 1, no message) at its assertion grep -q 'answer actor=human:wido' on the goal file's history after the fixture's human answers a budget-above-norm question: since the status-post binding (4bdaa5ec) and the budget-answer landing (3615da7a), a code-verified token answer to a budget question re-approves the goal as a verified channel answer, and the history line the fixture expects is no longer the one written. Seen seat-side on m2 2026-09-04 17:31Z. DONE means the fixture asserts the history the product now writes (the answer event and the verified-channel approval, actor and outcome named), fails loudly with the history printed when it does not match, and the suite is green.
- Origin: main
- Next step: One fixture assertion and a loud failure message: build, run channel-fixtures.sh seat-side, land through a chain. Approved under R-76-m2.
- OpenedAt: 2026-09-04T17:31:30Z
- Revision: 3
- Labels: robustness
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-04T17:35:45Z revision=2 opid=PX3Y8VYNN5B4FBJKA47W2M4M90-m2-5fcf08ab authority=relayed digest=3fb1beebc8aa47552fd6c61e2c08b3097aac5ed7631055fd6f94b19a0d0e0721 reviewBy=2026-09-06
- Claimed: machine=m2 lineage=main-1788441779-14484-82d6ed at=2026-09-04T17:37:46Z revision=3 accountingRevision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=2 fenceEpoch=0

History:
- 2026-09-04T17:31:30Z VVSG8R3HEKTF8RJSE3C7PYK4B3-m2-5fcf08ab open actor=human:Wido targets=channel-fixture-answer-history-drift
- 2026-09-04T17:35:45Z PX3Y8VYNN5B4FBJKA47W2M4M90-m2-5fcf08ab approve actor=human:Wido targets=channel-fixture-answer-history-drift authorityOutcome=TEMPORARY_HUMAN_WORD authorityReviewBy=2026-09-06 authorityRuling=R-32-m1 temporaryHumanWord="You may pick and execute the tier-1 items opened on 2026-09-04 in my name."
- 2026-09-04T17:37:46Z D5K85N2C2E88EMW0V1NJK6YXK4-m2-5fcf08ab claim actor=m2+main-1788441779-14484-82d6ed targets=channel-fixture-answer-history-drift
Integrity: sha256=258dbdde23694e77bdef8a3e8ed21d5b5e89307e707a87b5e150b7efe7d0c00d
