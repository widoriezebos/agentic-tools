# goal-completion-verifies-observed-intent

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="False confidence during testing and false completion can ship behavior that contradicts accepted intent across every adopted application. Existing verification, steward, and orchestration roles reduce novelty, but the verifier return has no explicit intent verdict and goal progression does not consume one."
- Tier: 3
- Intent: Require each goal to define concise expected observable behavior at intake, and require independent verifier judgment during meaningful testing and at closure that the candidate's observed behavior completely satisfies the original intent.
- Origin: human
- Next step: Design against existing goal, verifier, steward, orchestrator, testing, proof, and completion owners; reuse existing goal metadata fields if adequate, with no new schema or framework by default. The intake definition must state Expected observable behavior tied to intent: relevant operating conditions, the actual public entry point and candidate-bound evidence, and a contradiction signal. The verifier observes meaningful checkpoints and returns satisfied, failed, or inconclusive; missing evidence is inconclusive, and completeness is judged against the original intent rather than only listed components. The steward ensures observation and follow-through, while the orchestrator remains accountable for certification; closure refuses failed or inconclusive verdicts. Avoid universal performance floors and premature implementation prescriptions. Parallelism acceptance observes independent native tests or fixtures actually overlapping within grants, improved elapsed duration only against equivalent work, platform, census and cache conditions, deterministic scheduling semantics, complete test census, and cleanup; concurrency settings alone never satisfy the expectation.
- OpenedAt: 2026-09-22T07:32:19Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-22T07:32:19Z HZARDWH6N2BVY784P5ABK28WXD-m1e-d090af05 open actor=human:Wido targets=goal-completion-verifies-observed-intent
- 2026-09-22T07:55:40Z WYZ01HCKREP80K2SS5W8CD5WCS-m1e-d090af05 edit actor=human:Wido targets=goal-completion-verifies-observed-intent
Integrity: sha256=6c16f9a01b7ae0b1eab10335a73fa5c4152d949de75420c03c47076a4a1f5619
