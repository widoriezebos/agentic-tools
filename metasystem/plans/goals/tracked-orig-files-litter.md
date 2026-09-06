# tracked-orig-files-litter

- State: queued
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: dead files, no behavior; novelty 1: a deletion and one guard line; exposure 1: readers and critics only; accumulation 1: one occurrence class, already recurred once (four files, one removed)"
- Tier: 1
- Intent: Three patch backup files are tracked on main and ship with every checkout: cmd/metasystem/dispatch_verbs.go.orig, internal/dispatch/finding_register.go.orig and internal/dispatch/record.go.orig (a fourth, build.go.orig, was removed at 78018dd5). Go ignores them, but they are stale copies of live files that mislead a reader and a critic (design-close-crit1 flagged them on 2026-09-06). DONE means no file ending in .orig is tracked and the pre-commit guard or a landing check refuses a new one
- Origin: main
- Next step: Mechanical: one implementer round deletes the three files and adds the .orig refusal to the guard that already screens landings (scripts/agents/pre-commit-guard.sh or the landing path classes), with its fixture; tier 1, no critique
- OpenedAt: 2026-09-06T10:03:04Z
- Revision: 1
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0

History:
- 2026-09-06T10:03:04Z XSXGFN2RS50H2XR4PZKJ4EP4FX-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=tracked-orig-files-litter
Integrity: sha256=d9a71f5c8ebe6ccc3a3eae28d824b77075242b83ec76fd4544c4e0b32210cc14
