# Dispositions: collaboration-loop design, critique round 1

Chain: design-critic-20260806t202601z-4f2f. All nine findings accepted and folded at f-round-1.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| CL-1-1 | accepted | Nothing bound the chain to the merged bytes; any chain satisfied the gate. | C-1: critic records implementer job id + reviewedTree; gate compares tree hashes. |
| CL-1-2 | accepted | A balanced join is bookkeeping, not agreement. | C-1: merge additionally requires the final round report zero material findings over the merged tree. |
| CL-1-3 | accepted | Exit rule contradicted the shipped round budget; no outcome existed for exhaustion. | C-1a: budget adopted; critiqueExhausted recorded; merge stays refused; work returns to implementation or the human. |
| CL-1-4 | accepted | Five steps described only the happy path. | C-1b: reverse edges stated — gap-stop reopens design, design-indicting finding reopens design critique, failed gate returns to implementation. |
| CL-1-5 | accepted | An unbounded waiver swallows the rule it excuses. | C-2a: waiver classes verifiable against the diff; mismatched class refused; waivers counted and surfaced in retro. |
| CL-1-6 | accepted | The must-not was unenforceable with recorded evidence. | Independence restated as checkable: critic and implementer differ in runtime or model, from job records; otherwise a declared independence=session-only, visible in evidence. |
| CL-1-7 | accepted | The code-critic role had no configured model; the gate would deadlock every merge. | C-2: role added to template with placeholder, pinned locally, and the unconfigured-role error names the key to set. |
| CL-1-8 | accepted | No handoff mechanics existed for critic rounds over a worktree. | C-1c: review object is diff + tree hash; fixes fold in the same implementer worktree; critic follow-ups name the new tree. |
| CL-1-9 | accepted | Correction representation and write-time validation were unspecified. | C-3: receipt.sh refuses the skill without a chain id at write time; append-only CORRECTION lines reference the original epoch. |
