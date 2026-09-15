# delegates-never-write-the-receipts-log

- State: queued
- Risk: severity=1 novelty=1 exposure=3 accumulation=2 basis="severity 1: stray lines cost a read finding and a removal, no wrong landing; novelty 1: a path refusal at an existing round boundary; exposure 3: nearly every delegate round on every seat; accumulation 2: silent until a read catches it"
- Tier: 1
- Intent: Codex builders append their own lines to metasystem/memory/receipts.log in nearly every round, with the wrong builder and 'shipped' claims; on 2026-09-15 m1c spent about an hour a day on read findings and removals, and a dirty receipts.log blocked engine fast-forwards on m1e and m1b. DONE: a delegate round whose diff changes memory/receipts.log is refused at the round boundary with a named refusal, the builder brief template says receipts are the seat's, and a fixture proves a delegate diff touching receipts.log is refused while the seat's own receipt line still records.
- Origin: human
- Next step: Build, tier 1: refuse a delegate diff that changes memory/receipts.log in the round's boundary check, add the rule to the builder brief template, and prove both with a fixture.
- OpenedAt: 2026-09-15T05:54:25Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-15T05:54:25Z JXB761T8YVQ8QDKVVMKV7D8WT2-m1e-c6925449 open actor=human:Wido targets=delegates-never-write-the-receipts-log
Integrity: sha256=1810fd6c284662ee27b9547cdda3273109ea5290a7fd1cde6a26edb72ce5437e
