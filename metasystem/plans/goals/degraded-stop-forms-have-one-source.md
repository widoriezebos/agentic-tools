# degraded-stop-forms-have-one-source

- State: approved
- Risk: severity=2 novelty=2 exposure=2 accumulation=3 basis="severity 2: churn rather than wrong decisions; novelty 2: a new shared renderer across Go, shell and a generated template; exposure 2: every change to a degraded Stop form; accumulation 3: every form change multiplies across all rows that copy it"
- Tier: 2
- Intent: The Stop hook's fixed degraded texts are duplicated as literals across the fixture beds, so every change to a degraded form breaks rows that are otherwise correct; three bed rounds on 2026-09-13 were spent on exactly that. DONE: one side-effect-free renderer produces every degraded Stop form from a cause plus typed qualifiers in canonical order; the hook and the fixtures both take the forms from it; one contract fixture holds the full literal goldens for every base and qualifier form, so a wrong shared constant still fails on its own; the enforcement template's bootstrap fallback is generated from the same declarations.
- Origin: human
- Next step: Build the renderer and the golden contract fixture, move the hook and every bed row onto it, and generate the bootstrap fallback from the same source.
- OpenedAt: 2026-09-14T16:16:27Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-14T18:54:36Z revision=2 opid=K66ACW9K1RS0SEV51Y6EY7J55R-m1e-c6925449 authority=proven digest=c3abed22409f6e8dcbfe811a72d193d6880c0311fa6a4839f71727739cde1757

History:
- 2026-09-14T16:16:27Z 3S8BJ1GWC9X88JFR7AMXCR0HT8-m1e-c6925449 open actor=human:Wido targets=degraded-stop-forms-have-one-source
- 2026-09-14T18:54:36Z K66ACW9K1RS0SEV51Y6EY7J55R-m1e-c6925449 approve actor=human:Wido targets=degraded-stop-forms-have-one-source
Integrity: sha256=fd8e62f4a2d43c5ee54356625e0384205a9dd3383cdeadac4263a516dd6ee510
