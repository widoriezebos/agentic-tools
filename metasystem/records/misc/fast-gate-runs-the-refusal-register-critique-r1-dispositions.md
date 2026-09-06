# fast-gate-runs-the-refusal-register, critique round one: dispositions

Chain fgr-build1 after its round-one build, critic fgr-critic1 (claude,
claude-fable-5-1, xhigh), reviewed tree
6c8811fbf09889126777756ecdd79485b30ccacb. The critic returned zero
material findings and two wording notes. The critic had no shell (goal
code-critic-runtime-has-no-shell); the orchestrator's seat-side run on
the reviewed worktree (the refusal package green; the fast gate green
with its new "refusal register" stage named in the passed line) stands
as the executed proof. Both notes fold in round two.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| F-1 | noted | The commit wrapper's static re-proof comment omitted the new stage; outside the round-one workspace. | Folded in round two (plans/fast-gate-runs-the-refusal-register-fold-r2-brief.md): the comment names the refusal register. |
| F-2 | noted | The exclusion reason said the sentinel "refuses nothing" while the verdict path that writes it does block a stop; the token is a digest slot the steward compares, not a refusal code. | Folded in round two: the reason says that. |
