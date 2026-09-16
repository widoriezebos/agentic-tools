# goal-landing-needs-a-human-word

- State: queued
- Risk: severity=2 novelty=2 exposure=3 accumulation=1 basis="severity 2: a missing or forged word lands work a human did not want, reversible on main; novelty 2: a human gate on a landing, beside the goal approval that exists; exposure 3: every landing; accumulation 1: a landing that waits is visible"
- Tier: 2
- Intent: A land-ready goal lands on main only after a human approves that landing, naming the branch commit the word covers; today that step does not exist. Wido 2026-09-16 23:00 CEST: 'it would also be easier for a human being to approve a goal to land before it lands. So that step currently does not exist. But approval of a goal to be put to main landing is something that we will have to implement as well. Not as part of [goals-live-on-branches]. A separate backlog item.' Builds on goals-live-on-branches, whose design names the seam: the approved thing is a branch commit id. The design must reconcile it with units-land-in-batches-under-one-proof (Wido 2026-09-16 18:25 CEST: automatic landing, no human act from join to push), one-approval-gate (one human word, not two) and human-carried-landing: it decides where the word sits, which goals need it, and how it composes with the goal approval that exists. Not designed yet.
- Origin: human
- Next step: Waits for the design of goals-live-on-branches (the seam). Then a Fable design author, when Wido orders it. Priority and sequence are Wido's.
- OpenedAt: 2026-09-16T21:01:04Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-16T21:01:04Z PXNF9PEJGGJYJJ76MFX60FCX3T-m1c-fde8080b open actor=human:Wido targets=goal-landing-needs-a-human-word
Integrity: sha256=7e099a80a7d17d5186ef985a5cc646ae7ec8019ddbc048678df03b43d0cba21d
