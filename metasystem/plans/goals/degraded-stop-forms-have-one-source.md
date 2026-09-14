# degraded-stop-forms-have-one-source

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=3 basis="severity 2: churn rather than wrong decisions; novelty 2: a new shared renderer across Go, shell and a generated template; exposure 2: every change to a degraded Stop form; accumulation 3: every form change multiplies across all rows that copy it"
- Tier: 2
- Intent: The Stop hook's fixed degraded texts are duplicated as literals across the fixture beds, so every change to a degraded form breaks rows that are otherwise correct; three bed rounds on 2026-09-13 were spent on exactly that. DONE: one side-effect-free renderer produces every degraded Stop form from a cause plus typed qualifiers in canonical order; the hook and the fixtures both take the forms from it; one contract fixture holds the full literal goldens for every base and qualifier form, so a wrong shared constant still fails on its own; the enforcement template's bootstrap fallback is generated from the same declarations.
- Origin: human
- Next step: Build the renderer and the golden contract fixture, move the hook and every bed row onto it, and generate the bootstrap fallback from the same source.
- OpenedAt: 2026-09-14T16:16:27Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-14T16:16:27Z 3S8BJ1GWC9X88JFR7AMXCR0HT8-m1e-c6925449 open actor=human:Wido targets=degraded-stop-forms-have-one-source
Integrity: sha256=0436d4b27d9d5d86ec3123567be42c092a21d3acc2e1ce6d6e2198a4dee5e9f9
