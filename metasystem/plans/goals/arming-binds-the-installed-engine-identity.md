# arming-binds-the-installed-engine-identity

- State: queued
- Priority: 2
- Sequence: 26
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a run can execute a binary the seat did not build without any record saying so; novelty 2: identity binding on top of projection comparison is new; exposure 3: every proof run; accumulation 1: the arming record and one check"
- Tier: 2
- Intent: What: When a seat is armed, the record stores exactly which engine binary was installed (its build stamp, a digest of its bytes, and who installed it and when), and a later run against a different binary refuses or says so loudly, naming both. Why: Today the check only compares which source version the engine was built from, so two different builds of the same source look identical; on 2026-09-16 two seats could not tell whose binary they were running and had to compare file sizes by hand. Pros: A swapped or foreign binary is caught at once, and the answer to which binary is running comes from the record. Cons: One more field to keep in sync, and a too-strict check could refuse a harmless reinstall, which is why a seat reinstalling its own build must pass silently.
- Origin: human
- Next step: Next: Read how the arming record is written and how the drift check compares it, and check whether the steward's existing digest of its enrolled binary can serve as the identity source; then add the identity fields and the comparison as one change. Done when: A test that installs a byte-different build of the same source is refused with both identities named, and a test where a seat reinstalls its own build passes.
- OpenedAt: 2026-09-16T06:43:19Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T06:43:19Z XC5NW53TNHSRSHVJ081AZ5S6YE-m1e-c6925449 open actor=human:Wido targets=arming-binds-the-installed-engine-identity
- 2026-09-16T06:45:17Z NYMW6TQA3RFF8ACHKPZ91VXR87-m1e-c6925449 set-priority actor=human:Wido targets=arming-binds-the-installed-engine-identity reason=priority-order subject=arming-binds-the-installed-engine-identity from=unranked to=2:26 requested-sequence=append
- 2026-09-30T18:46:21Z BQ8BH92A7RTV6VN7HF7KV7S4KQ-ui-bc2fda53 edit actor=human:Wido targets=arming-binds-the-installed-engine-identity
Integrity: sha256=e1e5976589978da5169204fab7c2e21265e957ec7636a0b373bd54b374f9f3a6
