# a-run-holds-its-worktree

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A change made to a run worktree between two engine steps is attributed to the builder or unseen (a1 stashed ui U3b worktree for 30 seconds on 2026-11-08 while attempt 2 built); the fix adds an observation and a stop class to the unit runner and a recovery transition for seat and lane runs, with no new verb"
- Tier: 2
- Intent: A run holds its worktree: a change between two steps or rounds stops the round with a plain class naming the paths (shared refs are reported, never compared); a seat recovers through work revise, which records the observed state as the new baseline; a lane-owned fix settles its run, clears the fix job, records the baseline and admits the replacement; work status shows who holds the worktree.
- Origin: human
- Next step: Build U1 from the design page (D5a): the step-end snapshot after restoration, the comparison at step and round start, the stop class and evidence; brief: the page.
- OpenedAt: 2026-11-08T20:51:22Z
- Revision: 3
- Pinned: m1e
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-11-08T20:54:28Z revision=2 opid=4VYBB8F43KXDR8DG56SCSJ9304-m1e-34255095 authority=proven digest=4b1ee8975bf8f3b0f133f90854aab9a383e41d68ce0b60e3cb7cf4afb7769907 episode=2

History:
- 2026-11-08T20:51:22Z DHVYGG4CCEB4VHXH4XBHN9A29S-m1e-34255095 open actor=human:Wido targets=a-run-holds-its-worktree
- 2026-11-08T20:54:28Z 4VYBB8F43KXDR8DG56SCSJ9304-m1e-34255095 approve actor=human:Wido targets=a-run-holds-its-worktree
- 2026-11-08T20:56:59Z 9FF56M5PKNB7J5EC71MEQ5YPWH-m1e-34255095 set-pin actor=human:Wido targets=a-run-holds-its-worktree
Integrity: sha256=019eba316266592f16dabbca4234a8f371605613ef2c9a0094c26395374ca576
