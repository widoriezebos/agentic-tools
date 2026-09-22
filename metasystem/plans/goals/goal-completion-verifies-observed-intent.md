# goal-completion-verifies-observed-intent

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="False confidence during testing and false completion can ship behavior that contradicts accepted intent across every adopted application. Existing verification, steward, and orchestration roles reduce novelty, but the verifier return has no explicit intent verdict and goal progression does not consume one."
- Tier: 3
- Intent: Require each goal to define concise expected observable behavior at intake, and require independent verifier judgment during meaningful testing and at closure that the candidate's observed behavior completely satisfies the original intent.
- Origin: human
- Next step: Design against existing goal, verifier, steward, orchestrator, testing, proof, and completion owners; reuse existing goal metadata fields if adequate, with no new schema or framework by default. Acceptance criteria are a labeled requirement defined up front and tied to original intent: concise expected observable behavior, relevant operating conditions, the actual public entry point and candidate-bound evidence, and contradiction signals. Every meaningful testing round records each required criterion as satisfied, failed, or inconclusive with exact evidence; missing evidence is inconclusive, and the verifier judges completeness against the whole original intent rather than only listed components. For every failed or inconclusive criterion, classify the cause as implementation, configuration, missing test, inadequate observation, or an intent question; route it to the existing owner, make the smallest justified correction, and recheck narrowly before any necessary full run. The steward ensures observation and follow-through, while the orchestrator remains accountable for certification; closure refuses while any required criterion is failed or inconclusive. Avoid universal performance floors and premature implementation prescriptions. Parallelism criteria require independent native tests or fixtures actually overlapping within grants, improved elapsed duration only against equivalent work, platform, census and cache conditions, deterministic scheduling semantics, complete test census, and cleanup; concurrency settings alone never satisfy them.
- OpenedAt: 2026-09-22T07:32:19Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-22T07:32:19Z HZARDWH6N2BVY784P5ABK28WXD-m1e-d090af05 open actor=human:Wido targets=goal-completion-verifies-observed-intent
- 2026-09-22T07:55:40Z WYZ01HCKREP80K2SS5W8CD5WCS-m1e-d090af05 edit actor=human:Wido targets=goal-completion-verifies-observed-intent
- 2026-09-22T08:03:17Z GB36YRXW98SQB76FEYSVSTY8DD-m1e-d090af05 edit actor=human:Wido targets=goal-completion-verifies-observed-intent
Integrity: sha256=51f72c49f42edd0fea9e4cae4e5897dee16c4160a672f0bec4bfa1b29855ba86
