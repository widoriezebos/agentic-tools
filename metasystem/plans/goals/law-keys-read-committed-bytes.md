# law-keys-read-committed-bytes

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: a law silently raised by an uncommitted edit; novelty 1: member-size-gate r2 already designs the committed-bytes reader for its own keys; exposure 3: every seat, every budget check; accumulation 1: one reader"
- Tier: 2
- Intent: Budget-law keys (slice-norm-hours, the tier boxes, review-round-max, carry-open-max and their siblings) are read from the working-tree metasystem.conf, so an uncommitted edit silently raises a law on the seat that made it and no record shows it. member-size-gate design r2 (2026-09-16) fixes only its own keys by reading committed bytes via git show HEAD:./metasystem.conf. DONE: one committed-bytes reader serves every law key; a dirty-raise witness per key shows a working-tree edit changes nothing until committed; the refusal register names the case where HEAD lacks the key; proven on two runtimes.
- Origin: human
- Next step: Inventory every law key and its reader (file:line); reuse the member-size-gate reader; one unit per key family with its dirty-raise witness; Opus read; land.
- OpenedAt: 2026-09-16T06:43:11Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T06:43:11Z BJ12FA41CRC9F8V4VE8EK64REX-m1e-c6925449 open actor=human:Wido targets=law-keys-read-committed-bytes
Integrity: sha256=7d8f8cdc1c227b007aeccad5a90b613ee96a831c6b89b932cc3e8e3ff054699e
