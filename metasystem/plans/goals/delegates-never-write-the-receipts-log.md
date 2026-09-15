# delegates-never-write-the-receipts-log

- State: approved
- Priority: 1
- Sequence: 62
- Risk: severity=1 novelty=1 exposure=3 accumulation=2 basis="severity 1: stray lines cost a read finding and a removal, no wrong landing; novelty 1: a path refusal at an existing round boundary; exposure 3: nearly every delegate round on every seat; accumulation 2: silent until a read catches it"
- Tier: 1
- Intent: Codex builders append their own lines to metasystem/memory/receipts.log in nearly every round, with the wrong builder and 'shipped' claims; on 2026-09-15 m1c spent about an hour a day on read findings and removals, and a dirty receipts.log blocked engine fast-forwards on m1e and m1b. DONE: a delegate round whose diff changes memory/receipts.log is refused at the round boundary with a named refusal, the builder brief template says receipts are the seat's, and a fixture proves a delegate diff touching receipts.log is refused while the seat's own receipt line still records.
- Origin: human
- Next step: Build, tier 1: refuse a delegate diff that changes memory/receipts.log in the round's boundary check, add the rule to the builder brief template, and prove both with a fixture.
- OpenedAt: 2026-09-15T05:54:25Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-15T05:54:35Z revision=2 opid=X8P0FE8ET9YAE9YAK621DB8HAP-m1e-c6925449 authority=proven digest=68a106ae2cbf42173582fb5847d29c51ae8170f4233eb5e49085311be6e6f3e4

History:
- 2026-09-15T05:54:25Z JXB761T8YVQ8QDKVVMKV7D8WT2-m1e-c6925449 open actor=human:Wido targets=delegates-never-write-the-receipts-log
- 2026-09-15T05:54:35Z X8P0FE8ET9YAE9YAK621DB8HAP-m1e-c6925449 approve actor=human:Wido targets=delegates-never-write-the-receipts-log
- 2026-09-15T05:58:22Z 637D9KKTAN70EAVA6N5Q2CY57E-m1e-c6925449 set-priority actor=human:Wido targets=delegates-never-write-the-receipts-log reason=priority-order subject=delegates-never-write-the-receipts-log from=unranked to=1:62 requested-sequence=62
Integrity: sha256=bd0e4933841a5611decbe4c18d017889feb39c312d386672243232f30570e27c
