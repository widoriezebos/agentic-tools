# delegate-proof-runs-inside-the-sandbox

- State: claimed
- Risk: severity=2 novelty=2 exposure=2 accumulation=1 basis="severity 2: no delegate can meet the engine-appended testing requirement, so every round verifies by hand and nothing reuses; novelty 2: the git environment scrub and the detached worktrees are security boundaries the fix must keep; exposure 2: every delegate round with a goal; accumulation 1: one engine path"
- Tier: 2
- Intent: A delegate running the engine-appended testing requirement (metasystem test run) inside its job worktree fails before admission. Two causes, both seen 2026-09-12 on chain implementer-5881d3816c94692315588ba0 (rounds 1 and 2): the engine's git invocations scrub GIT_OBJECT_DIRECTORY and GIT_ALTERNATE_OBJECT_DIRECTORIES (gittree.ScrubbedEnviron), so git write-tree for the staged candidate writes into the shared object store, which the sandbox keeps read-only (the quarantine of issue 5): 'fatal: git-write-tree: error building trees'; and the runner materializes each group in a detached worktree (gittree.NewDetachedWorktree, three sites in proofrun/test_build.go), which writes .git/worktrees under the main repository, outside every root the envelope grants. DONE means: an engine run started by a verified hook delegate inside its worktree writes its objects into that worktree's quarantine and materializes its candidate beds under a root the envelope grants (the worktree's git dir), the shared store and the main .git stay read-only to it, and a dispatch-fixtures leg on the fake runtime proves test plan and test run complete inside a worktree; the measured chain of goal delegate-rounds-reuse-a-warm-gate (design page section 4) then reaches an attempt, which is that goal's remaining DONE item. RUNTIME INDEPENDENCE: the mechanism is the engine's and the adapter contract's, never one runtime's sandbox.
- Origin: main
- Next step: 2026-09-12 m1b: approved and claimed (blocker of delegate-rounds-reuse-a-warm-gate). Design page plans/delegate-proof-runs-inside-the-sandbox-design.md written: the engine honors the workspace's own quarantine (and only that) after the scrub; inside a quarantined workspace a candidate bed is a private repository over alternates, not a linked worktree. Design-critique read in flight; then build behind the gittree tests and a dispatch-fixtures leg, one code read, human commit, rerun the measured chain.
- OpenedAt: 2026-09-12T11:22:49Z
- Revision: 4
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=2
- BudgetExceptions: 0
- Approved: by=human:Wido at=2026-09-12T11:24:37Z revision=2 opid=DHDRSCC796KT7ZT4NQHBF5PETZ-m1b-c6925449 authority=proven digest=38ed0cf0205612a195497a181814c6dcceeb5fe91916a5130e5059501c02c8ae
- Claimed: machine=m1b lineage=main-1789191336-90295-e4b24b at=2026-09-12T11:24:41Z revision=3 accountingRevision=3 episodeAt=2026-09-12T11:24:41Z episodeRevision=3
- StopCapability: generation=3 revision=3 machine=m1b claimEpoch=2 fenceEpoch=0

History:
- 2026-09-12T11:22:49Z 39G69YXKB5EFS5TZ4SV9XE66ED-m1b-30a7e141 open actor=m1b+main-1789191336-90295-e4b24b targets=delegate-proof-runs-inside-the-sandbox,delegate-rounds-reuse-a-warm-gate
- 2026-09-12T11:24:37Z DHDRSCC796KT7ZT4NQHBF5PETZ-m1b-c6925449 approve actor=human:Wido targets=delegate-proof-runs-inside-the-sandbox
- 2026-09-12T11:24:41Z RJBNG6ESCVQQZXJ0QWZRW2NKZF-m1b-30a7e141 claim actor=m1b+main-1789191336-90295-e4b24b targets=delegate-proof-runs-inside-the-sandbox
- 2026-09-12T11:25:47Z V4QXGGFGE0W1174HPZSN1PRCX2-m1b-30a7e141 edit actor=m1b+main-1789191336-90295-e4b24b targets=delegate-proof-runs-inside-the-sandbox
Integrity: sha256=85b27e554523e0252824b90ce5d16a55fab35625c10b74938487925c1d6c93bf
