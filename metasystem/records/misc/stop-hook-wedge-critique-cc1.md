# The stop-hook wedge fix: code review (chain shw-build1-cc1) and the orchestrator's gate run

Reviewed tree 8b8cbf02560095d168d6308156b3210a98aebd66 (chain shw-build1, round 2). Critic: Claude Fable 5.1. Brief: plans/stop-hook-wedge-code-critique-brief.md. Zero material findings in the sandbox; the orchestrator's run of the process-owning fixture suite outside the sandbox was red on the fix's tree and green on main (recorded as ORCH-01, material); one correction round follows.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| ORCH-01 | accepted | supervision-hook-fixtures.sh red at its first leg on the fix's tree (a block carrying the open-work sentence), green on main, same machine; the mechanism is SHW-02: the deadline parent's engine work before the worker launch and after an overrun exceeds the provider cap on a loaded machine, so the overrun is recorded as a first occurrence and blocks. Material. | Correction round (plans/stop-hook-wedge-fix-brief.md): launch the worker first, cheap bookkeeping while waiting, one engine call on overrun. |
| SHW-01 | noted | After a first external cause, later stops end the turn before the open-work verdict runs for transient engine causes. For the design owner. | none |
| SHW-02 | noted | The deadline parent's pre-work and post-overrun work inside the five-second cap; folded through ORCH-01. | Correction round. |
| SHW-03 | noted | A verdict exiting non-zero still blocks on every stop; residual loop of a non-seat cause. For the design owner. | none |
| SHW-04 | noted | Unquoted canonical engine path in three places; folded in the correction. | Correction round. |
| SHW-05 | noted | Non-blocking record lock turns an overlapping first occurrence into a surface; folded in the correction (bounded blocking lock). | Correction round. |
