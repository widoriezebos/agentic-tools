# Dispositions: followup-rebase-crit2, round 1

Chain under review: followup-rebase-build1 (reviewed tree
e962f76a1db88ee0c1a8a07237bcd38c3264537f, work round
followup-rebase-build1-r4). Critic: followup-rebase-crit2 (fresh),
three material findings, three noted. Orchestrator: m1b. The critic ran
without a shell; each material claim was confirmed by the orchestrator
reading the reviewed dispatch.sh (the restore drops the stash
unconditionally; the message variable is replaced by the
paragraph-prefixed file before brief authority runs on it; the
paragraph names conflicted paths in backticks and asks for resolution
without staging).

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| FRF-01 | accepted | Read: restore_follow_up_rebase_worktree runs reset, clean and stash apply each with a flag on failure, then drops the tagged entry regardless; a failed restore loses the round's work to an unreferenced commit and the refusal never names the hash. | Round 5 (metasystem/plans/follow-up-rebase-fold3-brief.md, D12): a failed restore keeps the tagged stash and the refusal names its hash and tag. |
| FRF-02 | accepted | Read: the paragraph cites each conflicted path in backticks; the authority extractor checks metasystem-prefixed tokens against the fast-forwarded HEAD; a path the trunk deleted (modify/delete) is absent there, so the follow-up and every retry refuse. | Round 5, D13: brief authority admits the caller's brief, not the dispatcher's prefixed message. |
| FRF-03 | accepted | Read: the paragraph says resolve, not stage; unmerged index entries block the next stash and are re-recorded as conflicts on every later round. | Round 5, D14: the paragraph asks the builder to resolve and stage each path; a fixture asserts a follow-up after a staged resolution plans no unmerged paths. |
| FRF-04 | noted | True in shape; the landing script creates no merge commits, so trunk history stays linear. | None. |
| FRF-05 | noted | Transitional: a follow-up already running when this lands has a record without the rebase fields, and its repeated wrapper refuses until it ends. | None. |
| FRF-06 | noted | True: a clean filter or a symlink in the untracked tree hashes differently and refuses as a collision, in the safe direction; this repository configures no such filter. | None. |
