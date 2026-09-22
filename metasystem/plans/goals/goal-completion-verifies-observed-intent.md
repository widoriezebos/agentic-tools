# goal-completion-verifies-observed-intent

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="False confidence during testing and false completion can ship behavior that contradicts accepted intent across every adopted application. Existing verification, steward, and orchestration roles reduce novelty, but the verifier return has no explicit intent verdict and goal progression does not consume one."
- Tier: 3
- Intent: Require independent verifier judgment throughout meaningful test execution and at closure that observed application behavior satisfies accepted user intent; deterministic checks support this judgment but cannot replace it.
- Origin: human
- Next step: Start from the runnable candidate and inspect the existing verifier, steward, orchestrator, completion, testing, proof, and goal owners. Design the smallest generic flow in which the verifier observes real public-entrypoint outcomes at meaningful test checkpoints, interprets uncertainty, distinguishes outcomes from green components, and returns satisfied, failed, or inconclusive against intent; the steward ensures required observation and follow-through occur, and the orchestrator remains accountable for certification. Reuse candidate-bound runtime evidence and add adverse controls for deterministic observable properties. Route implementation, check, design, and intent faults to their existing owners. Refuse closure without a satisfied verdict, and allow design-conforming code to fail when behavior misses intent. For parallelism, component tests may be green but the goal cannot close unless actual overlap is observed with limits and census. Machinery enforces process and evidence; the verifier owns the semantic judgment.
- OpenedAt: 2026-09-22T07:32:19Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-22T07:32:19Z HZARDWH6N2BVY784P5ABK28WXD-m1e-d090af05 open actor=human:Wido targets=goal-completion-verifies-observed-intent
Integrity: sha256=97fe6a50eb6caf945d0adcdb1765d9785d0240fd2a9ad677890eb7ab4b31e0e6
