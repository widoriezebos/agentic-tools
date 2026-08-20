# Snapshot-scope critique, round 6 — exhaustion record

Round 6 verdict: 6 material findings, none fixture-expressible,
loop-verdict PARK. This was the final round of the chain's second
focused budget, so the exhaustion rule in
skills/design-critique/SKILL.md stops the loop here: the findings
below are ASSESSED for the record with sketched remedies but
deliberately NOT folded — the design is parked awaiting the human
ruling named in its status header. Exhausting rounds is not
agreement; none of these findings is refuted.

| Finding id | Assessment | Sketched remedy (not folded) |
| --- | --- | --- |
| WSS-R6-01 | REAL, and the reason for the park: the post-verification entry re-creates the capture-to-append gap one level down; any finite probe chain has a last probe | Human ruling required: bounded-and-recorded detection window as the wall's stated posture (each verification records its capture instant; later motion lands in the next probe or next admission), or repository-wide custody during acceptance — isolation-tier machinery this design was scoped away from |
| WSS-R6-02 | Real schema omission: the worktree-census posture round 5 requires is absent from the acceptance payload's exact field list | Add worktreeCensusPost to the payload and to continuity/resolution origins |
| WSS-R6-03 | Real, demonstrated live: linked worktrees carry PRIVATE pseudorefs (the guest worktree resolves its own ORIG_HEAD) outside the single-git-directory census | The pseudoref census runs per admitted worktree's git directory |
| WSS-R6-04 | Real: a raw index-file digest is stat-volatile (ctime/mtime/inode churn) and would falsely park lawful missions | The per-worktree staged posture is the logical ls-files --stage serialization read through that worktree's index, mirroring the toplevel representation |
| WSS-R6-05 | Real: the self-publication-ref exclusion names only refMapPost while birth and openTurn ref maps carry the same causal impossibility | The exclusion covers every recorded ref map; the anchor machinery authenticates those refs at every capture, as already designed for refMapPost |
| WSS-R6-06 | Real requirement failure: judging side tips by immediate-parent delta admits a sibling payload buried under an empty accounted commit entering via an ours-merge | Repository-scope judgment for a side chain compares the side TIP's whole toplevel tree against the chain's merge base with the first-parent line, not per-commit deltas |
