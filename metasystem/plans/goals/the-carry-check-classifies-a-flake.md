# the-carry-check-classifies-a-flake

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="carry check classification; a wrong one passes a real red to the read (the lane proof still catches it) or keeps refusing a flake (cost only); every hand-committed unit is exposed; recurs on every load flake"
- Tier: 2
- Intent: The carry check of a hand-committed unit classifies a red the way the unit run does: a test that fails and then passes on the check's own diagnostic rerun is a flake, recorded as the unit run records one, and the retained check passes; a red that stays red refuses as today. Finding 542: U1c2b's check ended red on 2026-11-05 22:52 on TestProofRunLegacyPublicLaunchUsesAndClearsHostPhase alone, green on its rerun, with 92 groups and the audits green, and the hand route stopped for a person
- Origin: human
- DesignGate: off
- Next step: Build without a design from finding 542: cmd/metasystem/intent_carry_check.go applies the unit run's red classification (internal/launch/unit_attribution.go, loopstop.ClassifyRed) to the retained check's result before it refuses; one unit; brief by the seat
- OpenedAt: 2026-11-05T23:53:26Z
- Revision: 4
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-05T23:58:16Z revision=3 opid=F9XH43A3JPNNB4CJRWCHFDA7ZC-m1e-721b8823 authority=proven digest=044b20e6a6ea34e1a8b74cbf12267fc0e587edf5908520e9c39369d9f7b8883a episode=3

History:
- 2026-11-05T23:53:26Z AFBXA72ACKH65MS88B83DX2DPE-m1e-721b8823 open actor=human:Wido targets=the-carry-check-classifies-a-flake
- 2026-11-05T23:55:43Z JDGH8M5VDYNP5X7CYSY2PRR6FS-m1e-721b8823 edit actor=human:Wido targets=the-carry-check-classifies-a-flake reason=Allowed: build-without-design why=Allowed: build-without-design. A one-unit fix from a recorded finding (542) that reuses the unit run's classifier; the brief names the finding, the owner files and the tests; the lane proof and the read gate it (decision 111c) impact=This lets goal the-carry-check-classifies-a-flake be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of the-carry-check-classifies-a-flake until withdrawn. To undo: metasystem goal disallow the-carry-check-classifies-a-flake build-without-design
- 2026-11-05T23:58:16Z F9XH43A3JPNNB4CJRWCHFDA7ZC-m1e-721b8823 approve actor=human:Wido targets=the-carry-check-classifies-a-flake
- 2026-11-05T23:59:53Z 1NFE48JX81S9KXA32V7VYKBX0S-m1e-721b8823 set-pin actor=human:Wido targets=the-carry-check-classifies-a-flake
Integrity: sha256=50bf69800c791e3a1b8389e968f36ccba10a703b0461bfea571c4329697092e7
