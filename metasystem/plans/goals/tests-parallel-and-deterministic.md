# tests-parallel-and-deterministic

- State: claimed
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="Test scheduling and isolation protect every application landing; false greens can admit defects, the concurrency owners already exist, and broad shared-state changes affect the complete suite."
- Tier: 3
- Intent: Make every test safe for parallel execution, with configurable concurrency based on available CPU cores and deterministic outcomes on slow or loaded supported hosts.
- Origin: human
- Next step: Compose the accepted worker-propagation and scheduler changes, complete the final isolation inventory, obtain matching selected and full native macOS/Linux proof, then publish, install and verify the generic application witness. Retain matching green evidence; no budgets, coverage floors or review limits are raised. Current evidence and pending work: plans/handoff-test-parallelism-and-determinism.md. No human decision is pending for this approved work. Acceptance criteria, checked in every implementation and testing round with exact evidence and a satisfied, failed, or inconclusive verdict; missing or non-comparable evidence is inconclusive: PAR-1 overlap — independent eligible native tests and fixture processes actually overlap; contradiction: eligible work remains serial. PAR-2 capacity — configured worker grants reach nested runners, and blocked work does not occupy capacity that fitting ready work can use; contradiction: a nested runner loses or exceeds its grant, or blocked work leaves useful capacity idle. PAR-3 elapsed — equivalent work finishes sooner, and the complete suite records elapsed duration with platform, census and cache conditions; contradiction: comparable work does not improve, or a speedup is claimed without a comparable baseline. PAR-4 semantics — concurrency and slow scheduling leave semantic verdicts deterministic; contradiction: scheduling order or host speed changes a pass, failure or expected result. PAR-5 completeness — the complete required test census, race checks and coverage floors run, and any required failure blocks integration; contradiction: a required item is absent, silently skipped or red while integration proceeds. PAR-6 cleanup — no owned process remains after success, failure or cancellation; contradiction: the post-run census finds an owned survivor. PAR-7 generic control — the same worker and cancellation controls govern another application command adapter; contradiction: correct bounds or cleanup depend on metasystem-specific command behavior. For each failed or inconclusive criterion, diagnose the cause and make the smallest focused correction before deciding whether full proof must run again. Goal closure is refused while any required criterion is failed or inconclusive.
- ReadItem: id=c1-native-delegate-budget-read-1-1 read=c1-native-delegate-budget-read-1 state=accepted addedAt=2026-09-21T12:11:17Z changedAt=2026-09-21T12:15:57Z closingReference="A one-line discrepancy in builder prose is not a product defect: independent read verified exact tree f648c06b and diff602316e3, actual54 lines stays below250. Acceptance dispositions record the corrected count; source and runtime proof unchanged." text="N-1: builder prose counted53 changed lines; exact reviewed diff has54. Correct count recorded in acceptance dispositions; source/hash and250-line boundary unchanged; no product fix required."
- OpenedAt: 2026-09-21T03:36:50Z
- Revision: 11
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
- 2026-09-22T09:15:48Z A7DHVWS0YE92EGVHRWS5J24T0Q-m1e-226b0953 edit actor=human:Wido targets=tests-parallel-and-deterministic
Integrity: sha256=f275c7f48f3ad26c2c224df5a8d1a633392cb39a174b4d7e85eb6e199adbd4bb
