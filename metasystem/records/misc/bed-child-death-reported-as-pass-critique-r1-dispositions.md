# bed-child-death-reported-as-pass, critique round one: dispositions

Chain bcd-build1 after its round-two build, critic bcd-critic1 (claude,
claude-fable-5-1, xhigh), reviewed tree
24d426277ab18bbe01920433dbab62bc8f83054f. The critic had no shell; the
orchestrator's seat-side runs on the reviewed worktree under the stock
bash 3.2 (the direct self-test child probe exiting 70; the whole bed with
the self-test reported passed under its expected-to-die rule, every
ordinary scenario green, and the re-arm scenario rearm-launch-fails
genuinely red for a reason the old bed had hidden) stand as the executed
proof.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | accepted | Real: the operator-layout empty-runtime mode leaves the child with an explicit exit 0 before the completion line, which the sentinel rule reads as a death; the case the round-two brief asked the critic to check. | Folded in round three (plans/bed-child-death-reported-as-pass-fold-r3-brief.md): the completion line precedes that exit; the audit found no other child exit 0. |
| F-2 | accepted | Real: the self-test's deliberate death kept an evidence directory and printed a preserved line into every green run. | Folded in round three: cleanup removes the self-test's directory on every status and prints nothing. |
| F-3 | noted | Four initialization lines moved after the traps and the self-test block so the self-test needs no engine; the critic found no defect and the move lets an early failure keep evidence through cleanup. | none |
| F-4 | noted | A narrow race between two signal handlers can replace 130 with 143; both are signal deaths the parent treats as failure; the old code had a comparable window. | none |
