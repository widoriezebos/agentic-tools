# the-carry-check-classifies-a-flake

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="carry check classification; a wrong one passes a real red to the read (the lane proof still catches it) or keeps refusing a flake (cost only); every hand-committed unit is exposed; recurs on every load flake"
- Tier: 2
- Intent: The carry check of a hand-committed unit classifies a red the way the unit run does: a test that fails and then passes on the check's own diagnostic rerun is a flake, recorded as the unit run records one, and the retained check passes; a red that stays red refuses as today. Finding 542: U1c2b's check ended red on 2026-11-05 22:52 on TestProofRunLegacyPublicLaunchUsesAndClearsHostPhase alone, green on its rerun, with 92 groups and the audits green, and the hand route stopped for a person
- Origin: human
- Next step: Build without a design from finding 542: cmd/metasystem/intent_carry_check.go applies the unit run's red classification (internal/launch/unit_attribution.go, loopstop.ClassifyRed) to the retained check's result before it refuses; one unit; brief by the seat
- OpenedAt: 2026-11-05T23:53:26Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-05T23:53:26Z AFBXA72ACKH65MS88B83DX2DPE-m1e-721b8823 open actor=human:Wido targets=the-carry-check-classifies-a-flake
Integrity: sha256=9f2eaa7608cbeeda171cec62b9b0bc601da3e569b4450440314d7d50e80ea141
