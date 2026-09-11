# blast-radius-beside-risk

- State: queued
- Risk: severity=2 novelty=3 exposure=3 accumulation=1 basis="severity 2: proof cost, not correctness; novelty 3: a new dimension in the selection rule; exposure 3: every receipt; accumulation 1: nothing built on it"
- Tier: 3
- Intent: Wido, 2026-09-10, on the small-change lane's first decision: 'it's always risk that should determine how deep the test should go. Later we can maybe do something with locality, because if it is a one-line change with high impact, that might still mean that we can have a single test proving it to be correct, and there's no need to run the entire suite. Locality is a thing next to risk. It's the blast radius basically.' Today the testing contract selects groups from the four risk answers only (severity, novelty, exposure, accumulation) and the diff's shape never lowers depth, which is right and stays. What is missing is a second, independent dimension: the blast radius of the change, meaning w || REWRITTEN 2026-09-11 (backlog consolidation; landed parts: the 'today' paragraph is stale: since a584e5c8 (R-3 amended 2026-09-11) per-landing depth is decided by the change and the risk answers scale cadence weight, so 'risk alone decides depth' no longer holds; the locality dimension itself is unbuilt (Wido's word 2): DONE means a design that defines blast radius mechanically (from the contract's surfaces, their dependsOn edges and each group's declared inputs, or call graphs where the language allows), states how it combines with the landed rule of a584e5c8 (the change decides per-landing depth and risk scales cadence weight; locality may narrow WHICH groups run, never below that floor or the tier's critique requirement), names what it must never do (a size or line count is not locality), and its fixtures; then the build.
- Origin: main
- Next step: Design first (Fable lane), after the bootstrap members land. Opened by m1b at Wido's word 2026-09-10 11:10Z. Backlog consistency: every record that speaks of test depth follows the four risk answers; the small-change lane's page keeps the contract's depth rule (its decision 1).
- OpenedAt: 2026-09-10T08:05:43Z
- Revision: 2
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T08:05:43Z S3VTJ7MZ0RH2M454T6530E144D-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=blast-radius-beside-risk
- 2026-09-11T22:07:40Z H965S18FS4EMPKWCD07R9TG8T6-m1-c6925449 edit actor=human:Wido targets=blast-radius-beside-risk
Integrity: sha256=7212de51f8e9d7fc4ecf0fbbb5cb778254f65e4d4c7fcadfd0500e5666d114dd
