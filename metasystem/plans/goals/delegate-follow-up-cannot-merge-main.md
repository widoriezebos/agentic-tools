# delegate-follow-up-cannot-merge-main

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=2 basis="severity 2: every correction round on a busy fleet day risks a stall or a hand-done merge; novelty 2: a new lawful path through the commit guard or the dispatcher; exposure 2: every chain whose files another seat lands on; accumulation 2: the hand-done stash-and-reapply recipe will be repeated and will drift"
- Tier: 2
- Intent: A delegate's follow-up round cannot bring its worktree up to date when main moved on the chain's own files: the dispatcher warns WORKTREE-BEHIND and asks the orchestrator to merge main into the worktree, but the worktree holds the round's uncommitted changes, git refuses a merge over them, and no lawful checkpoint commit exists - the pre-commit guard admits only wrapper-token commits and the wrapper is the landing lane; a delegate in its sandbox cannot even reach the lease test the wrapper runs. Seen 2026-09-06 on chain idle-escalate-build1 after landing c1525b90 touched the same four files: the follow-up round stopped on the gap and the orchestrator resorted to a tagged stash, a fast-forward and a re-apply by hand. DONE means a follow-up round whose base is behind main on the chain's files gets a lawful rebase or merge of its worktree, either by the dispatcher before the round starts (stash, fast-forward, re-apply, conflicts left for the builder) or by a checkpoint commit verb the guard admits on agent branches, and a fixture proves it
- Origin: main
- Next step: Decide between the two mechanisms (dispatcher pre-round rebase versus a guard-admitted checkpoint on agent branches) with the facts from chain idle-escalate-build1; tier 2; build behind a dispatch fixture with a follow-up whose base is behind main on a conflicting file
- OpenedAt: 2026-09-06T13:23:13Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T13:23:13Z AGAHF86TZVX4XSR5AJQSN9NYSD-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=delegate-follow-up-cannot-merge-main
Integrity: sha256=45e80c052664a054b248c50bf0a062cdc508170cb8b2e5711d26459eeb1bf105
