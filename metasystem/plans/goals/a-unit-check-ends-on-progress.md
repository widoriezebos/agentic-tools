# a-unit-check-ends-on-progress

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="changes when every unit check is ended and how a stopped unit is classified and asked about; a wrong progress clock lets a hung check run on, a wrong cause retries a real failure"
- Tier: 2
- Intent: A unit's frozen check ends on its result, not on a wall clock that load or selection size can exhaust: proof.deadline means minutes without a finished unit (a hang still stops), the check runs after the builder's turn instead of inside it, a stop carries its real cause (deadline, environment) and offers the retry, one machine retry for a deadline stop, and the ask to a person states what happened with real line counts and a ready command. From the machinery's first run 2026-10-23 (landed-proof killed twice at exactly 15m0s after 28 of 46 green units); analyses: first-machinery-run-20261023/partial-build-1042.md and outside-read-fable-partial-build.md.
- Origin: human
- Next step: Design and brief from the two analyses (Fable's fixes 2-6), Astra under the rare-case rule, build by hand (it blocks the machinery's first run), then round 4 of a-done-goals-branch-is-swept on the new engine
- OpenedAt: 2026-10-23T09:59:14Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-23T09:59:14Z D7PCCK2WZZKFTA7T9Y9DCMND2R-m1e-79fa16f1 open actor=human:Wido targets=a-unit-check-ends-on-progress
Integrity: sha256=02865393688227fcaf46f6f2d093322cfeb4d558d2d2b00311d4674330f6840d
