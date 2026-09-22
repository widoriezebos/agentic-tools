# tests-parallel-and-deterministic

- State: claimed
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="Test scheduling and isolation protect every application landing; false greens can admit defects, the concurrency owners already exist, and broad shared-state changes affect the complete suite."
- Tier: 3
- Intent: Make every test safe for parallel execution, with configurable concurrency based on available CPU cores and deterministic outcomes on slow or loaded supported hosts.
- Origin: human
- Next step: Complete the collected C1 test failures as a batch, then obtain sufficient selected proof and publish/install the compatibility release. Finish fixture and timing corrections and their available independent reads, compose the parallel worker activation, prove C2 with the installed C1 engine, publish/install, and run the full current macOS and Linux validation. Retain deterministic matching green results; no budgets, coverage floors or review limits raised. Current evidence and pending work: plans/handoff-test-parallelism-and-determinism.md. No human decision is pending for this approved work.
- ReadItem: id=c1-native-delegate-budget-read-1-1 read=c1-native-delegate-budget-read-1 state=accepted addedAt=2026-09-21T12:11:17Z changedAt=2026-09-21T12:15:57Z closingReference="A one-line discrepancy in builder prose is not a product defect: independent read verified exact tree f648c06b and diff602316e3, actual54 lines stays below250. Acceptance dispositions record the corrected count; source and runtime proof unchanged." text="N-1: builder prose counted53 changed lines; exact reviewed diff has54. Correct count recorded in acceptance dispositions; source/hash and250-line boundary unchanged; no product fix required."
- OpenedAt: 2026-09-21T03:36:50Z
- Revision: 10
- Budget: elapsedLimit=3d6h attemptLimit=12 reservedJobMinutesLimit=720 activeJobLimit=8 reviewRoundLimit=3
- BudgetExceptions: 4
- Approved: by=human:Wido at=2026-09-22T01:18:20Z revision=10 opid=WZP3BT6XX9Y5JJHEQY1SCKD8JG-m1e-226b0953 authority=proven digest=74ae49f1dfb53c826faf0a7611ef0ee3aafbe5abbcd36efa27a541479999a393 episode=10
- Claimed: machine=m1e lineage=main-1789893000-16595-1a7a6a at=2026-09-22T01:18:20Z revision=10 accountingRevision=10 episodeAt=2026-09-21T03:36:57Z episodeRevision=3
- StopCapability: generation=10 revision=10 machine=m1e claimEpoch=3 fenceEpoch=0

History:
- 2026-09-21T03:36:50Z E8NGT9NQ58VWERY27EW0GATBKS-m1e-226b0953 open actor=human:Wido targets=tests-parallel-and-deterministic
- 2026-09-21T03:36:54Z 2YPPWS0593N435RWNRGSWW8ZZ7-m1e-226b0953 approve actor=human:Wido targets=tests-parallel-and-deterministic
- 2026-09-21T03:36:57Z C5YYDZ2TZDGQQTM09JESMJGGQM-m1e-226b0953 claim actor=m1e+main-1789893000-16595-1a7a6a targets=tests-parallel-and-deterministic
- 2026-09-21T11:58:41Z 2BAHATHTGNFP4ZEG53MNTATWKF-m1e-226b0953 edit actor=m1e+main-1789893000-16595-1a7a6a targets=tests-parallel-and-deterministic
- 2026-09-21T12:11:17Z 1W7HHZ7XNEPSHHN9FCSZ9CKSGA-m1e-226b0953 read-items-add actor=m1e+main-1789893000-16595-1a7a6a targets=tests-parallel-and-deterministic
- 2026-09-21T12:15:25Z 37M0GYPF2KGZ739XG25KSPAPPA-m1e-226b0953 set-budget actor=human:Wido targets=tests-parallel-and-deterministic
- 2026-09-21T12:15:57Z YMPYSW5QZKVC5ASQMYFA2VSRNW-m1e-226b0953 read-items-close actor=m1e+main-1789893000-16595-1a7a6a targets=tests-parallel-and-deterministic
- 2026-09-21T17:41:51Z CTST7XJESN5RPRZWERP2R52H5Z-m1e-226b0953 set-budget actor=human:Wido targets=tests-parallel-and-deterministic
- 2026-09-21T18:39:41Z 9FXDRPSD03KVG26ENJDMXYV0W3-m1e-226b0953 set-budget actor=human:Wido targets=tests-parallel-and-deterministic
- 2026-09-22T01:18:20Z WZP3BT6XX9Y5JJHEQY1SCKD8JG-m1e-226b0953 set-budget actor=human:Wido targets=tests-parallel-and-deterministic
Integrity: sha256=80637c7c3c14ed2d606705cea0abc2b2586ea0c70e33371a90ed6cd7c2ead717
