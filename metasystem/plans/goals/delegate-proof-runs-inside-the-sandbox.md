# delegate-proof-runs-inside-the-sandbox

- State: queued
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: no delegate can meet the engine-appended testing requirement, so every round verifies by hand and nothing reuses; novelty 2: the git environment scrub and the detached worktrees are security boundaries the fix must keep; exposure 2: every delegate round with a goal; accumulation 1: one engine path"
- Tier: 2
- Intent: A delegate running the engine-appended testing requirement (metasystem test run) inside its job worktree fails before admission. Two causes, both seen 2026-09-12 on chain implementer-5881d3816c94692315588ba0 (rounds 1 and 2): the engine's git invocations scrub GIT_OBJECT_DIRECTORY and GIT_ALTERNATE_OBJECT_DIRECTORIES (gittree.ScrubbedEnviron), so git write-tree for the staged candidate writes into the shared object store, which the sandbox keeps read-only (the quarantine of issue 5): 'fatal: git-write-tree: error building trees'; and the runner materializes each group in a detached worktree (gittree.NewDetachedWorktree, three sites in proofrun/test_build.go), which writes .git/worktrees under the main repository, outside every root the envelope grants. DONE means: an engine run started by a verified hook delegate inside its worktree writes its objects into that worktree's quarantine and materializes its candidate beds under a root the envelope grants (the worktree's git dir), the shared store and the main .git stay read-only to it, and a dispatch-fixtures leg on the fake runtime proves test plan and test run complete inside a worktree; the measured chain of goal delegate-rounds-reuse-a-warm-gate (design page section 4) then reaches an attempt, which is that goal's remaining DONE item. RUNTIME INDEPENDENCE: the mechanism is the engine's and the adapter contract's, never one runtime's sandbox.
- Origin: main
- Next step: Read gittree.ScrubbedEnviron, gittree.NewDetachedWorktree and the hook-delegate admission in cmd/metasystem/proof_run.go; design how an engine run started by a verified hook delegate writes objects into its worktree's quarantine and materializes beds under the worktree's git dir; critique once; build behind a dispatch-fixtures leg on the fake runtime; then rerun the measured chain of delegate-rounds-reuse-a-warm-gate.
- OpenedAt: 2026-09-12T11:22:49Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0

History:
- 2026-09-12T11:22:49Z 39G69YXKB5EFS5TZ4SV9XE66ED-m1b-30a7e141 open actor=m1b+main-1789191336-90295-e4b24b targets=delegate-proof-runs-inside-the-sandbox,delegate-rounds-reuse-a-warm-gate
Integrity: sha256=19d1500660cddc9ade38f3da855201d0f319c7c5749bdf7b67d7f0597590f2c7
