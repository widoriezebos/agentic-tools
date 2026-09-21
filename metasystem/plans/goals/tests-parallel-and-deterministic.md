# tests-parallel-and-deterministic

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="Test scheduling and isolation protect every application landing; false greens can admit defects, the concurrency owners already exist, and broad shared-state changes affect the complete suite."
- Tier: 3
- Intent: Make every test safe for parallel execution, with configurable concurrency based on available CPU cores and deterministic outcomes on slow or loaded supported hosts.
- Origin: human
- Next step: Inspect prior parallelism work and current shared state, clocks, fixture scheduling and resource owners; design and critique the smallest complete change, implement it with Sol delegates, then verify full coverage, failure detection and measured runtime.
- OpenedAt: 2026-09-21T03:36:50Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=12 reservedJobMinutesLimit=720 activeJobLimit=8 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-21T03:36:54Z revision=2 opid=2YPPWS0593N435RWNRGSWW8ZZ7-m1e-226b0953 authority=proven digest=ae6b56343ee73052e7721718cdf6634cf98a71927277aaca222aa88351014e9a episode=2

History:
- 2026-09-21T03:36:50Z E8NGT9NQ58VWERY27EW0GATBKS-m1e-226b0953 open actor=human:Wido targets=tests-parallel-and-deterministic
- 2026-09-21T03:36:54Z 2YPPWS0593N435RWNRGSWW8ZZ7-m1e-226b0953 approve actor=human:Wido targets=tests-parallel-and-deterministic
Integrity: sha256=a0ffbd0d350fb25602ebfa05970b5d82901f49998e3b0696184adaf550c7a4e3
