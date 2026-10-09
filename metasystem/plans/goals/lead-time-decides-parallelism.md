# lead-time-decides-parallelism

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="A wrong number misguides the parallelism limit and is corrected by reading the records again (severity 2); it extends an existing reader with a goal-level projection and one setting (novelty 2); it shows on goal status, goal list and receipt status (exposure 2); a few units in one reader and its faces (accumulation 1)"
- Tier: 2
- Intent: Lead time, from a goal opening to its landing on main, with the wait for a person and the review rounds shown apart, is measured per goal and per period from recorded acts and is the number that decides how many goals run in parallel; throughput is never claimed without it
- Origin: human
- Next step: Design from plans/lead-time-decides-parallelism-design-brief.md at this goal tier, then build; waits for machinery-measures-its-own-process because it extends that goal measures reader and status faces. INTENT: one lead-time line per goal split into its waits and work, review rounds per goal, period numbers in receipt status, and a setting for goals with coding started at once shown beside the measured lead time. CONSTRAINTS: derive from recorded acts, unknown stays unknown, no second reader or totals store, at most five units of 250 lines, one gate; step one is the per-goal lead-time line. FREEDOMS: the split of the journey, period boundaries, where rounds are read from.
- OpenedAt: 2026-10-09T14:26:04Z
- Revision: 1
- BlockedBy: machinery-measures-its-own-process
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-10-09T14:26:04Z P4WCQ07DWWN3PKGFVZ9FQ69Y3R-m1e-718ba0eb open actor=human:Wido targets=lead-time-decides-parallelism,machinery-measures-its-own-process
Integrity: sha256=24efb8e71c0d0195ade28340058b3fc84b7167fc77dff9658fba54ab66f00d80
