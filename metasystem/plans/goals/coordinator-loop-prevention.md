# coordinator-loop-prevention

- State: approved
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="Severity 3: a false proof decision could admit unverified source or fail to stop runaway execution. Novelty 2: connect existing proof, landing and governed-admission owners. Exposure 3: common validation and landing paths used by every runtime. Accumulation 2: crosses proof launch, evidence carriage and commit consumption."
- Tier: 3
- Intent: Prevent repeated proof and repair cycles across Claude, Codex, Devin and every supported runtime by enforcing shared admission of ordinary coordinator proof runs, propagating failed gates without automatic retries, and consuming valid coverage proof at landing while retaining source, platform, review and failure checks.
- Origin: human
- Next step: Settle the bounded shared admission and proof-handoff design, then independently review, implement and prove the stop and reuse behavior through all runtime entrypoints.
- OpenedAt: 2026-09-08T05:24:18Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-08T05:27:41Z revision=2 opid=NE7398PFN4BGN18QJ282PRQZXY-m1c-7cd0bd60 authority=proven digest=be0f8168c931f51591e4adffc3d6b8205c34fbcbd65f15302817327d44131574

History:
- 2026-09-08T05:24:18Z CCPXF4Z60MFM3TR8HMWPSM2K8S-m1c-1274caf4 open actor=m1c+main-1788759014-39092-24fa5c targets=coordinator-loop-prevention
- 2026-09-08T05:27:41Z NE7398PFN4BGN18QJ282PRQZXY-m1c-7cd0bd60 approve actor=human:Wido targets=coordinator-loop-prevention
Integrity: sha256=58df070b01417b710569eb9c2ad1ae68b2f85ca6ca37e38bd1cd8ddffde7e5bd
