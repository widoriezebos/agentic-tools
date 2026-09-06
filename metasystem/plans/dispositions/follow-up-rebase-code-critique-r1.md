# Dispositions: followup-rebase-crit1, round 1

Chain under review: followup-rebase-build1 (reviewed tree
9bbf59a01623ec588a67460db043e7c79de36061). Critic: followup-rebase-crit1,
four material findings, four noted. Orchestrator: m1b.

The critic ran without a shell, so its two gaps were closed here: the
git ordering behind FRB-01 was reproduced live on this Mac's git 2.50.1
(a stash apply with one tracked conflict and one untracked collision
exits 1 with "could not restore untracked files from stash", leaves the
tracked path unmerged and the trunk's copy at the collided path, and the
round's copy survives only in the stash's untracked tree). The Go tests
and the fixture bed were run by the orchestrator and the implementer as
the critique brief lists.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| FRB-01 | accepted | Reproduced live: git merges tracked changes first and restores untracked files afterwards, so a round-created file that the trunk now tracks fails to restore while the tracked merge still leaves unmerged entries. The dispatcher's only failure test is an empty unmerged list, so it drops the stash and the round's copy is gone. Severe and irreversible. | Round 2 (metasystem/plans/follow-up-rebase-fold2-brief.md, D7): the apply counts as conflicted success only when every path in the stash's untracked tree stands in the worktree with the stash's bytes; anything else takes the restore branch; a fixture pins the collision. |
| FRB-02 | accepted | By construction: a follow-up is admitted after a protocol-error round, and such a round's return may lack a diffBoundary; the plan verb then refuses as soon as the trunk moved at all. The critic-chain half is unreachable today (every critic chain is shared-checkout), but the design-critic worktree path exists in the dispatcher. Worktrees never commit, so the dirty path set already carries every round's changes and a missing boundary loses nothing. | Round 2, D8: a round without a readable diffBoundary contributes no paths; only an undecodable return still refuses; unit tests for both tolerated shapes. |
| FRB-03 | accepted | Read: the repeated wrapper rebuilds the message from the parent record and skips the rebase block, while the delivered prompt hash is part of the launch fingerprint; a rebased round's repeat is refused as REFUSED-OPID-MISMATCH. | Round 2, D9: the paragraph is a pure function of the record's three rebase fields, and the repeated path reproduces it from the standing child record. |
| FRB-04 | accepted | Read: brief authority, the exhaustion gate, composition, preflight and the claim all refuse after the worktree moved and the stash was dropped; the retry plans behind zero and records nothing, so the builder is never told about the markers. | Round 2, D10: the plan verb reports the worktree's unmerged index entries; a wrapper that finds them without rebasing records them and prepends the paragraph; a fixture pins the refuse-then-retry path. |
| FRB-05 | noted | True: the drop is positional because git accepts no hash for drop, and the window is two consecutive git commands on the stack the brief decided to share. The post-drop check proves only that our own entry is gone. No lock covers hand stashes; left as the decided design. | None. |
| FRB-06 | noted | True as read: a drop failure after a successful apply refuses with a message that overstates the restore, and the entry leaks. Reachable only when git stash drop itself fails. | None; the message is corrected in passing under D7 if the builder touches that branch. |
| FRB-07 | noted | True: the block is gated on the record's launchMode while the fallback derivation runs later. Every live worktree chain carries the field, so it is a silent skip in theory only. Folded because it is one reordering. | Round 2, D11: the launch mode is derived before the block. |
| FRB-08 | noted | True: no Go test covers an overlap that comes only from a dirty path, and no fixture exercises a refusal branch. | Round 2: D7's and D10's fixtures each exercise a refusal branch; D8 adds the dirty-path-only unit test. |
