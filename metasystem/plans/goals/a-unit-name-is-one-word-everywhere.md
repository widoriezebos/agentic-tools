# a-unit-name-is-one-word-everywhere

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, root cause read (findings 554, 555; the week review of 2026-11-07); the unit check and the lane proof gate it"
- Tier: 2
- Intent: A unit name is one word that work build, the next-step hint and the branch commit owner all accept (finding 555: the hint and build take the page row label "U1a2 the command routing" while the commit owner refuses it; a stopped run and a rebuild)
- Origin: human
- DesignGate: off
- Next step: Build without a design from finding 555: the hint and build take the row first word, or build refuses a multi-word name up front with the one-word remedy; one test; about 20 production lines
- OpenedAt: 2026-11-07T19:47:44Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-07T19:49:25Z revision=3 opid=24PMP09FFTPZYZ3Q2QDJT9SKNC-m1e-170920ec authority=proven digest=39a671466bc42f2e69d06de83a4cb286ecc73ea114d7f050dcab19463dd2feef episode=3

History:
- 2026-11-07T19:47:44Z AMSXCWJ3GNYG1BSYR9PHC75PCX-m1e-170920ec open actor=human:Wido targets=a-unit-name-is-one-word-everywhere
- 2026-11-07T19:48:52Z AWPJ6XX1QXCP703MR186W932FE-m1e-170920ec edit actor=human:Wido targets=a-unit-name-is-one-word-everywhere reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a read finding impact=This lets goal a-unit-name-is-one-word-everywhere be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-unit-name-is-one-word-everywhere until withdrawn. To undo: metasystem goal disallow a-unit-name-is-one-word-everywhere build-without-design
- 2026-11-07T19:49:25Z 24PMP09FFTPZYZ3Q2QDJT9SKNC-m1e-170920ec approve actor=human:Wido targets=a-unit-name-is-one-word-everywhere
Integrity: sha256=12508e3b8cb999da617af5c4942c7e9cd9fa05d6c90d969673cd92765d88373e
