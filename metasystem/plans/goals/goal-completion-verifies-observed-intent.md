# goal-completion-verifies-observed-intent

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=2 basis="False confidence during testing and false completion can ship behavior that contradicts accepted intent across every adopted application. Existing verification, steward, and orchestration roles reduce novelty, but the verifier return has no explicit intent verdict and goal progression does not consume one."
- Tier: 3
- Intent: What: Each goal states up front what a person should be able to see when it is done, and closing it needs an independent check that this really happens. Why: Goals get closed on green tests even when the behaviour you asked for is incomplete. Pros: "Done" means what you asked for works, not only that tests pass. Cons: More work when a goal is opened and closed; it must not grow into a heavy framework.
- Origin: human
- Next step: Next: Write a short design on the existing goal, verifier and completion code: where the expected behaviour is written when a goal opens, how each check round records pass, fail or unknown with its evidence, and how closing refuses while anything is failed or unknown. Consider folding it into examine-evidence-before-retesting. Done when: you have accepted the design.
- OpenedAt: 2026-09-22T07:32:19Z
- Revision: 4
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-22T07:32:19Z HZARDWH6N2BVY784P5ABK28WXD-m1e-d090af05 open actor=human:Wido targets=goal-completion-verifies-observed-intent
- 2026-09-22T07:55:40Z WYZ01HCKREP80K2SS5W8CD5WCS-m1e-d090af05 edit actor=human:Wido targets=goal-completion-verifies-observed-intent
- 2026-09-22T08:03:17Z GB36YRXW98SQB76FEYSVSTY8DD-m1e-d090af05 edit actor=human:Wido targets=goal-completion-verifies-observed-intent
- 2026-09-30T18:49:08Z HW2NCF9RA79EJ2DENGR4HF5FRH-m1e-b6a4eb0a edit actor=human:wido targets=goal-completion-verifies-observed-intent
Integrity: sha256=e8726683791afc35904e6dc4f9999eb4eb57d22d2329b44d3b140fe1519c79ce
