# a-unit-check-ends-on-progress

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="changes when every unit check is ended and how a stopped unit is classified and asked about; a wrong progress clock lets a hung check run on, a wrong cause retries a real failure"
- Tier: 2
- Intent: A unit's frozen check ends on its result, not on a wall clock that load or selection size can exhaust: proof.deadline means minutes without a finished unit (a hang still stops), the check runs after the builder's turn instead of inside it, a stop carries its real cause (deadline, environment) and offers the retry, one machine retry for a deadline stop, and the ask to a person states what happened with real line counts and a ready command. From the machinery's first run 2026-10-23 (landed-proof killed twice at exactly 15m0s after 28 of 46 green units); analyses: first-machinery-run-20261023/partial-build-1042.md and outside-read-fable-partial-build.md.
- Origin: human
- Next step: Design and brief from the two analyses (Fable's fixes 2-6), Astra under the rare-case rule, build by hand (it blocks the machinery's first run), then round 4 of a-done-goals-branch-is-swept on the new engine
- OpenedAt: 2026-10-23T09:59:14Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-10-23T09:59:32Z revision=2 opid=ZC66SKK6SPCQDB2CXDXKEA3YKX-m1e-79fa16f1 authority=proven digest=b578788a2be7cc599785744438128220308e4126c93e45acbe7bf6a75baeae53 episode=2

History:
- 2026-10-23T09:59:14Z D7PCCK2WZZKFTA7T9Y9DCMND2R-m1e-79fa16f1 open actor=human:Wido targets=a-unit-check-ends-on-progress
- 2026-10-23T09:59:32Z ZC66SKK6SPCQDB2CXDXKEA3YKX-m1e-79fa16f1 approve actor=human:Wido targets=a-unit-check-ends-on-progress
Integrity: sha256=6963b34ccac75be5a0975e11bef9d9061d1e5765e4fb172d2dd6d6b33d93052d
