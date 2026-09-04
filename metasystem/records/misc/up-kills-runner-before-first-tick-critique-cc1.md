# The steward-runner fix: code review (chain ukr-build1-cc1)

Reviewed tree 96550e07f84675c4cf57b5475bc89e16f8e5eb5e (chain ukr-build1, round 1). Critic: Claude Fable 5.1. Brief: plans/up-kills-runner-code-critique-brief.md. Zero material findings; the chain closes and the fix lands.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| UKR-01 | noted | The whole-package timing comparison is inconclusive (the baseline hit the ten-minute default under contention); the batch-read fixture proves the mechanical change (one git cat-file --batch process instead of one per file). Re-measure when the machine is quiet. | none |
| UKR-02 | noted | No named equivalence test between the old and new readers; byte identity rests on reading the code and the goal suite passing. Backlog if a divergence ever surfaces. | none |
| UKR-03 | noted | A generation whose first tick never completes keeps the 120-second floor, so a machine whose first tick genuinely exceeds 120 seconds still loops, at a 120-second cadence. Follows the brief's formula; the batched read is expected to bring the first tick under the floor. Watch on m2. | none |
| UKR-04 | noted | A tick that runs past the bound but completes records one Dead verdict against itself; the count resets on the next healthy observation. | none |
