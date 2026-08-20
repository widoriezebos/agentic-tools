# Snapshot-scope critique, round 5 dispositions

Round 5 verdict: 5 material, none fixture-expressible (3 requirement
failures, 2 shape), loop-verdict CONTINUE. All five adjudicated and
accepted; one was demonstrated live by the critic (the GIT_DIR
redirection). Rounds 4-6 are the chain's second focused budget: if
non-fixture material findings survive round 6, the design parks
awaiting the human per the skill's exhaustion rule.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| WSS-R5-01 | ACCEPTED | Demonstrated: an inherited GIT_DIR steered git -C to a linked worktree's HEAD; gitAt inherits os.Environ and only replacement vars were scrubbed | The scrub is the whole repository-steering environment on every runner git surface (D120 posture); WSS-10 extended |
| WSS-R5-02 | ACCEPTED | write-tree refuses any unmerged entry, so even a copied repo-wide index refuses a lawful workspace beside a sibling conflict | StagedTree reconstructs a workspace-only isolated index from ls-files --stage; sibling entries never enter |
| WSS-R5-03 | ACCEPTED | The pseudoref census sat outside the captured posture, so a REBASE_HEAD written after rule evaluation escaped the final comparison | Both censuses are inside the capture the verification compares |
| WSS-R5-04 | ACCEPTED | The census admitted recorded worktrees by membership while their private HEAD/index (real files under .git/worktrees/) moved unobserved | The census records per-worktree posture; delegate worktrees free until consumption, stationary after, judged from the recorded posture |
| WSS-R5-05 | ACCEPTED | Append-then-verify without durable pending state conflicted with HIW-O13's single commit point and left a crash window over consumed authorizations | The acceptance append stays the single commit point but stops concluding the turn; a post-verification entry concludes; acceptance-without-verification is a defined state resume completes |
