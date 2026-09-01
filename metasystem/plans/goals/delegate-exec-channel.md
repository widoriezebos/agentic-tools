# delegate-exec-channel

- State: queued
- Intent: Graded implementer delegates cannot run their own builds: v1 approvals=deny blocks every exec, so verification pressure escapes to the host (bm-2dc rep 2, 2026-08-24)
- Origin: main
- Next step: Design the lawful build-and-verify channel for graded delegates: options are (a) grade exec allow scoped to the job worktree, (b) a gate-runner the delegate can invoke that executes declared commands in its worktree and returns captured output, or (c) approvals=ask with the turn driver auto-granting a declared command allowlist. Decide with a design note first; the bm-2dc rep-2 evidence (implementer produced a lawful patch but could not run mvnw/gate.sh) is the acceptance scenario.
- OpenedAt: 2026-08-24T11:40:52Z
- Revision: 2
- Budget: elapsedLimit=1d attemptLimit=6 reservedJobMinutesLimit=240 activeJobLimit=1

History:
- 2026-08-24T11:40:52Z NQP0ZBA2W4HWGKNVH3208BT7RS-m2-bc1be9cb open actor=m2+mac-coordinator targets=delegate-exec-channel
- 2026-09-01T20:28:22Z AKQAPABMJX50FXMD6BNW4Z31QA-m0b-6638932d set-budget actor=m0b+main-1788250419-3170380-8a1fb3 targets=delegate-exec-channel
Integrity: sha256=bab90bee818404d051e47cbd86c24c6d924726d0aec6329c87c02bf0be3b88ba
