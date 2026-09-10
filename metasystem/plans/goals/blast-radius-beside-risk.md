# blast-radius-beside-risk

- State: queued
- Risk: severity=2 novelty=3 exposure=3 accumulation=1 basis="severity 2: proof cost, not correctness; novelty 3: a new dimension in the selection rule; exposure 3: every receipt; accumulation 1: nothing built on it"
- Tier: 3
- Intent: Wido, 2026-09-10, on the small-change lane's first decision: 'it's always risk that should determine how deep the test should go. Later we can maybe do something with locality, because if it is a one-line change with high impact, that might still mean that we can have a single test proving it to be correct, and there's no need to run the entire suite. Locality is a thing next to risk. It's the blast radius basically.' Today the testing contract selects groups from the four risk answers only (severity, novelty, exposure, accumulation) and the diff's shape never lowers depth, which is right and stays. What is missing is a second, independent dimension: the blast radius of the change, meaning which behaviours a change can actually reach (the surfaces it touches, the callers of what it changes, the fixtures whose inputs include it), so that a high-exposure change with a small reach can be proven by the groups that reach it rather than by everything the surface owns. DONE means: a design that defines blast radius mechanically (from the contract's surfaces, their dependsOn edges and each group's declared inputs, or from call graphs where the language allows), states how it combines with risk (risk sets the floor of proof depth and the critique requirement; locality may narrow WHICH groups run, never below the floor), names what it must never do (a size or line count is not locality), and its fixtures; then the build. Until it lands, risk alone decides depth and no goal, page or ruling may say otherwise.
- Origin: main
- Next step: Design first (Fable lane), after the bootstrap members land. Opened by m1b at Wido's word 2026-09-10 11:10Z. Backlog consistency: every record that speaks of test depth follows the four risk answers; the small-change lane's page keeps the contract's depth rule (its decision 1).
- OpenedAt: 2026-09-10T08:05:43Z
- Revision: 1
- Labels: bootstrap
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-10T08:05:43Z S3VTJ7MZ0RH2M454T6530E144D-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=blast-radius-beside-risk
Integrity: sha256=c31f8e50e5ac81a929766a98a6ef3a0077f56d4a37c1f86550b7d081ba6a3503
