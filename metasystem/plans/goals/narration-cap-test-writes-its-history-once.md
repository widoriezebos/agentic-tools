# narration-cap-test-writes-its-history-once

- State: claimed
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: each item removes measured waste inside one test or fixture and changes no law; novelty 1: known shapes; exposure 1: one group's duration; accumulation 1: none"
- Tier: 1
- Intent: TestNarrationCapsItsHistory in internal/steward/narrate_test.go builds its 2000-line history through 2025 Narrate calls, each rewriting the file through atomicfile with an fsync, and takes 41 s in governed-standard. DONE means the test writes the history to disk directly, then calls Narrate a handful of times to prove the cap, and governed-standard drops by about 40 s.
- Origin: human
- Next step: Slice 4 item 3 of plans/suite-speed-plan.md. The cap's behaviour under test must not weaken: the test still proves the cap at the boundary. Code critique only.
- OpenedAt: 2026-09-10T12:02:46Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-11T14:32:31Z revision=2 opid=KVQYZGJXTHWVX301E628KFBX5F-m1-f47a9d40 authority=proven digest=89e35000d474cdefc8a44da0b0653bc00680d9903fc795d499b0a65866bd0f50
- Claimed: machine=m1e lineage=main-1789030447-51011-5722fc at=2026-09-11T14:53:37Z revision=3 accountingRevision=3 episodeAt=2026-09-11T14:53:37Z episodeRevision=3
- StopCapability: generation=3 revision=3 machine=m1e claimEpoch=1 fenceEpoch=0

History:
- 2026-09-10T12:02:46Z DPGDD6PEGVCFSPJ6RCWKB9ZKY8-m1e-892cdaec open actor=m1e+main-1789030447-51011-5722fc targets=narration-cap-test-writes-its-history-once
- 2026-09-11T14:32:31Z KVQYZGJXTHWVX301E628KFBX5F-m1-f47a9d40 approve actor=human:Wido targets=narration-cap-test-writes-its-history-once
- 2026-09-11T14:53:37Z D582VB519KB1J9V7Q7BGYW7ZQN-m1e-892cdaec claim actor=m1e+main-1789030447-51011-5722fc targets=narration-cap-test-writes-its-history-once
Integrity: sha256=43aec1f1c31256cf1c8960e3634188921ca764070c20614d16daf636f63f5cde
