# delegate-exec-channel

- State: approved
- Intent: Graded implementer delegates cannot run their own builds: v1 approvals=deny blocks every exec, so verification pressure escapes to the host (bm-2dc rep 2, 2026-08-24)
- Origin: main
- Next step: Design the lawful build-and-verify channel for graded delegates: options are (a) grade exec allow scoped to the job worktree, (b) a gate-runner the delegate can invoke that executes declared commands in its worktree and returns captured output, or (c) approvals=ask with the turn driver auto-granting a declared command allowlist. Decide with a design note first; the bm-2dc rep-2 evidence (implementer produced a lawful patch but could not run mvnw/gate.sh) is the acceptance scenario.
- OpenedAt: 2026-08-24T11:40:52Z
- Revision: 3
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-06T06:53:37Z revision=3 opid=A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f authority=proven digest=27c1652c96324997bfbb020460cf27eac06b6fbaa2dded50fb68ec36f07660ec

History:
- 2026-08-24T11:40:52Z NQP0ZBA2W4HWGKNVH3208BT7RS-m2-bc1be9cb open actor=m2+mac-coordinator targets=delegate-exec-channel
- 2026-09-01T20:28:22Z AKQAPABMJX50FXMD6BNW4Z31QA-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=delegate-exec-channel
- 2026-09-06T06:53:37Z A35EHY2TGAWCCFA1Q1J0WZGS35-m1-a4f8999f approve actor=human:Wido targets=delegate-exec-channel reason=sweep
Integrity: sha256=e7a59299c659d711eb9f56a7da766d7bb2b694cf46c9a0d400b7d737c9c58a3c
