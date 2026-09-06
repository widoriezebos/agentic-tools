# follow-up-brief-cannot-cite-round-created-files

- State: queued
- Risk: severity=1 novelty=1 exposure=2 accumulation=2 basis="severity 1: a refusal with a workaround that weakens the check; novelty 1: one more source of truth in an existing admission; exposure 2: every fold brief on a chain whose rounds create files; accumulation 2: every such brief is softened by hand and the softening spreads"
- Tier: 2
- Intent: Follow-up brief authority checks every cited repository path against the worktree HEAD commit, but a round's own new files stay uncommitted in the worktree because delegates never commit, so a fold brief cannot name a file the previous round created. On 2026-09-06 the round-2 brief for chain followup-rebase-build1 was refused for internal/dispatch/followup_rebase.go and its test, both created by round 1 and standing in the worktree; the orchestrator had to drop the repository prefix from those paths, which removes them from the check altogether. DONE means a follow-up brief's cited path is admitted when it is in the worktree HEAD tree or exists on the worktree disk (modified or untracked, not ignored), refused otherwise, and a fixture pins a brief that cites a round-created file.
- Origin: main
- Next step: Give ValidateBriefAuthority a worktree disk root for follow-ups: base commit or worktree disk admits; fixture with a round that creates a file and a follow-up brief naming it
- OpenedAt: 2026-09-06T17:47:33Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-06T17:47:33Z SVM499TBB84YJEZB4JTVEQHKX6-m1b-c6925449 open actor=m1b+main-1788680071-18713-e76d5d targets=follow-up-brief-cannot-cite-round-created-files
Integrity: sha256=7e95ec8a160538cbfcb8ef8e0dea02f99191ddd809abf1619f3dc433ac7fcbd4
