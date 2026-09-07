# malformed-return-field-loses-the-round

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=2 basis="severity 2: wasted rounds and an unlandable tree, never wrong code; novelty 1: one validation branch plus the existing repair seam; exposure 2: every implementer round on every chain; accumulation 2: the third sighting of a round lost to its envelope rather than its work, and each costs a full cap plus a human budget raise"
- Tier: 2
- Intent: One malformed entry in an implementer return's diffBoundary voids the whole round: on 2026-09-07 brain-build1b-20260907-r7 folded a code review across 41 real files, then returned a 42nd boundary entry of keyboard noise ('lkjoiuytrewq'). The engine stamped DIFF_BOUNDARY_INVALID, recorded the job failed with error protocol_error, charged its 120-minute cap, and validate conformance refuses the same way, so two hours of correct work in the worktree cannot be certified or landed and only a fresh round can recover it. The round's own paid repair (job repair-claim) cannot help: it requires status running and the job is already terminal. Same family as critic-return-identity-loses-the-round, different field and role. DONE means: (1) the return validator, before failing the job, drops entries it can prove are not repository paths and reports them, or asks the round for one corrected return through the existing paid-repair seam while the job is still running; (2) a round whose only defect is an unparseable boundary entry does not consume its cap; (3) validate conformance names the offending entry and offers the same recovery rather than only refusing; (4) fixtures: a fake-runtime round returning one junk boundary entry beside valid ones is recovered, its cap unspent, and its diff certified; a round whose junk cannot be separated from real paths still fails.
- Origin: main
- Next step: Tier 2, MECHANICAL. Read internal/dispatch's return validation (the DIFF_BOUNDARY_INVALID site and record-protocol-error), internal/dispatch/record.go RepairClaim (the one paid repair, running-only), internal/validate/conformance.go's boundary check, and internal/dispatch/budget.go for the cap accounting. Sol builds behind the fixtures, a code review, land. Relates to critic-return-identity-loses-the-round and runtime-limit-is-not-a-protocol-error: three ways a round dies for a reason that is not the work.
- OpenedAt: 2026-09-07T10:09:29Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-07T10:09:29Z 4QWGH9X1RR3XG26MH6N86T6T9S-m1-a4f8999f open actor=m1+main-1788594343-3833-fb64b9 targets=malformed-return-field-loses-the-round
Integrity: sha256=e75c90300f87034b6d70120dfaea795332e12ec6f4b707d683b5b9a6f0218dc5
