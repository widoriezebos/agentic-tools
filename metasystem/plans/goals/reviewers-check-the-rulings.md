# reviewers-check-the-rulings

- State: queued
- Risk: severity=2 novelty=1 exposure=3 accumulation=1 basis="review process change for every critic dispatch; no product behaviour change; every seat"
- Tier: 2
- Intent: What: Every design critic and code critic checks the work against the standing human rulings, the project's register of Wido's decisions and the decisions recorded on the goal itself, and reports any conflict as a finding that always counts as material, naming the ruling it breaks. Why: today the critic roles are bound only by their brief, their skill and the project rules; the rulings register (175 rulings) and a goal's own human decisions reach a critic only if whoever wrote the brief happened to quote them, so a design or change can quietly contradict a ruling and still pass review. Wido, 2026-09-30: it should definitely be one of the tasks of the reviewer to check the rulings. Pros: decisions the human already made are enforced at every review instead of depending on someone remembering them. Cons: each review reads more material, and the register needs to stay findable by topic as it grows.
- Origin: human
- Next step: Next: add a binding step to the design-critic and code-critic roles and skills: read the rulings register and the goal's human decisions, check the work against them, and report each conflict as a material finding that cites the ruling; add a test that a dispatched critic's brief names both sources. Done when: a critic dispatched on a goal lists the rulings it checked in its return, and a planted conflict with a ruling is reported as material.
- OpenedAt: 2026-09-30T20:38:45Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-30T20:38:45Z 7PTYSF4TX48MAA3GD0YVH8B9KE-ui-bc2fda53 open actor=human:Wido targets=reviewers-check-the-rulings
Integrity: sha256=9b574aa60137f051c1ee79288db362542b8c964fcfd3544e78c4391f47ab90d4
