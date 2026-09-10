# receipt-section-beds-do-not-export-the-engine

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="severity 2: every landing on every seat is refused until the two beds pass again; novelty 1: the change restores the bed environment of the day before; exposure 3: the receipt gate runs on every seat; accumulation 1: nothing is built on the variable"
- Tier: 3
- Intent: Since 016b83e2 the receipt's section beds run with METASYSTEM_BIN set to the candidate engine in the receipt worktree; every agent script copied into a fixture's scratch repository honours that variable, so up, steward arm and the pre-commit guard inside scratch repositories run the worktree engine instead of the copy enrolled there: supervision-and-census-fixtures fails every process scenario with ENROLLMENT_DRIFT and adoption-fixtures fails its new-plan guard leg, on every receipt whose enrolled engine carries 016b83e2 (proof-mtvn9elr on tip 53d89e2c). The candidate is already installed at the bed's own bin path where the default resolves. DONE: section beds carry no METASYSTEM_BIN (inherited or added; the go groups that consume the candidate engine keep theirs), the section test asserts the installed path and the absent variable, and a receipt taken on a tip at or after 016b83e2 passes both beds.
- Origin: main
- Next step: Small-change lane, high risk: one MECHANICAL implementer round (two files, a handful of lines), then one code-critic read because the receipt gate is fleet-wide; receipt; land with the chain. Opened by m1b 2026-09-10 17:45Z under the four-day authority; member of the-metasystem-validates-itself-with-itself (repairs member 1, landed as 016b83e2). Until it lands, no seat can take a sufficient receipt on a tip at or after 016b83e2.
- OpenedAt: 2026-09-10T15:39:22Z
- Revision: 1
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T15:39:22Z AVMNAVT6BFK2RJ1KMT8W35B8SV-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=receipt-section-beds-do-not-export-the-engine
Integrity: sha256=04738b83c92832d361a21c330bbb552c30ddc2021156f6718a0b92f38592668f
