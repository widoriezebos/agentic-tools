# delegate-exec-channel

- State: queued
- Intent: Graded implementer delegates cannot run their own builds: v1 approvals=deny blocks every exec, so verification pressure escapes to the host (bm-2dc rep 2, 2026-08-24)
- Origin: main
- Next step: Design the lawful build-and-verify channel for graded delegates: options are (a) grade exec allow scoped to the job worktree, (b) a gate-runner the delegate can invoke that executes declared commands in its worktree and returns captured output, or (c) approvals=ask with the turn driver auto-granting a declared command allowlist. Decide with a design note first; the bm-2dc rep-2 evidence (implementer produced a lawful patch but could not run mvnw/gate.sh) is the acceptance scenario.
- OpenedAt: 2026-08-24T11:40:52Z
- Revision: 1

History:
- 2026-08-24T11:40:52Z NQP0ZBA2W4HWGKNVH3208BT7RS-m2-bc1be9cb open actor=m2+mac-coordinator targets=delegate-exec-channel
Integrity: sha256=47203fc205c7e61fa2d3416a1c0a18bf7937f0853ee4d79bfbae6e6fee8c789b
