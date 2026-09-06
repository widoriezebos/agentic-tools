# tracked-orig-files-litter

- State: approved
- Risk: severity=1 novelty=1 exposure=1 accumulation=1 basis="severity 1: dead files, no behavior; novelty 1: a deletion and one guard line; exposure 1: readers and critics only; accumulation 1: one occurrence class, already recurred once (four files, one removed)"
- Tier: 1
- Intent: Three patch backup files are tracked on main and ship with every checkout: cmd/metasystem/dispatch_verbs.go.orig, internal/dispatch/finding_register.go.orig and internal/dispatch/record.go.orig (a fourth, build.go.orig, was removed at 78018dd5). Go ignores them, but they are stale copies of live files that mislead a reader and a critic (design-close-crit1 flagged them on 2026-09-06). DONE means no file ending in .orig is tracked and the pre-commit guard or a landing check refuses a new one
- Origin: main
- Next step: Mechanical: one implementer round deletes the three files and adds the .orig refusal to the guard that already screens landings (scripts/agents/pre-commit-guard.sh or the landing path classes), with its fixture; tier 1, no critique
- OpenedAt: 2026-09-06T10:03:04Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=360 activeJobLimit=1 reviewRoundLimit=0
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T14:49:24Z revision=2 opid=WCPJTSBBGBRS6M6F7GFEWRBWEM-m1-c6925449 authority=proven digest=7143e38c3d583e0493dc9ad8c1f0a28c6d29b0714a0e472e2d0ee1165124a0cf

History:
- 2026-09-06T10:03:04Z XSXGFN2RS50H2XR4PZKJ4EP4FX-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=tracked-orig-files-litter
- 2026-09-06T14:49:24Z WCPJTSBBGBRS6M6F7GFEWRBWEM-m1-c6925449 approve actor=human:Wido targets=delegate-follow-up-cannot-merge-main,tracked-orig-files-litter
Integrity: sha256=e87540eedc9054e7ea6a6fd00f6b6eff8c83e6034506bf1075ba86de1b91466e
