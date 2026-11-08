# a-run-holds-its-worktree

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A change made to a run worktree between two engine steps is attributed to the builder or unseen (a1 stashed ui U3b worktree for 30 seconds on 2026-11-08 while attempt 2 built); the fix adds an observation and a stop class to the unit runner and a recovery transition for seat and lane runs, with no new verb"
- Tier: 2
- Intent: A run holds its worktree: a change between two steps or rounds stops the round with a plain class naming the paths (shared refs are reported, never compared); a seat recovers through work revise, which records the observed state as the new baseline; a lane-owned fix settles its run, clears the fix job, records the baseline and admits the replacement; work status shows who holds the worktree.
- Origin: human
- Next step: Build U1 from the design page (D5a): the step-end snapshot after restoration, the comparison at step and round start, the stop class and evidence; brief: the page.
- OpenedAt: 2026-11-08T20:51:22Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-08T20:51:22Z DHVYGG4CCEB4VHXH4XBHN9A29S-m1e-34255095 open actor=human:Wido targets=a-run-holds-its-worktree
Integrity: sha256=4908bc2f0dbe37712d12432e600f2cb5b3b636a3a1c3a9aacc912673d1c16bd1
