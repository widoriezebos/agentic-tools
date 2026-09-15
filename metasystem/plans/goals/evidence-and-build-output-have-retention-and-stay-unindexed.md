# evidence-and-build-output-have-retention-and-stay-unindexed

- State: approved
- Risk: severity=2 novelty=2 exposure=3 accumulation=3 basis="severity 2: disk and indexing churn that slows every seat, no correctness harm; novelty 2: durable-copy acknowledgement and pins before deletion, and path moves across every reader; exposure 3: every proof run writes these stores; accumulation 3: grows by gigabytes per day until pruned"
- Tier: 2
- Intent: Preserved suite-failure copies (23 GB across three seats), engine pins (10 GB), proof-run payloads and scratch build output in the temp directory (test binaries and gate outputs, about 9 GB) have no retention, and the checkout copies are indexed by Spotlight (63,410 Go files under one checkout, 37,089 of them preserved copies) and scanned by XProtect (records/misc/leaked-processes-diagnosis-2026-09-15.md, class D). DONE: suite-failures, engine pins, proof-run payloads and temp build output have count-and-age retention that never deletes the only copy of evidence an open diagnosis pins or a binary a live runner executes, and preserved evidence, candidate trees, pins and build output live where Spotlight does not index them, with every reader and writer moved; proven by index and disk counts that stay under stated thresholds for a day of three-seat work. Relates to run-scoped-build-caches-have-a-janitor. Seed material: section 5 and rules R1 to R5 and P1 to P6 of the revision-2 design page of seat-machines-shed-leaked-processes, and its critique rounds 1 and 2.
- Origin: human
- Next step: Design first by a Claude Fable delegate from the seed material named in the intent, reconciling scope with run-scoped-build-caches-have-a-janitor first and answering critique findings SMLP-13, -14, -216 and -217; then a Codex critique; then the build.
- OpenedAt: 2026-09-15T09:05:39Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T09:05:45Z revision=2 opid=BMQ1E2X76RTM6PEWT07FJ4KAKD-m1e-c6925449 authority=proven digest=95a04369bcf84610390f5c2c0747b2dcd547af93e797d71a0e6740d368da9d10

History:
- 2026-09-15T09:05:39Z MPR8S7Y9HX5FSMQRACQYQ2PF34-m1e-c6925449 open actor=human:Wido targets=evidence-and-build-output-have-retention-and-stay-unindexed
- 2026-09-15T09:05:45Z BMQ1E2X76RTM6PEWT07FJ4KAKD-m1e-c6925449 approve actor=human:Wido targets=evidence-and-build-output-have-retention-and-stay-unindexed
Integrity: sha256=9758bf8e8b6784e5c19f1df154b0b513093109dfa7bcf31ce73e8f92ceac0468
