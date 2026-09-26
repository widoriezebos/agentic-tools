# Fable critique r2 (final): manual work through public review

- Design: plans/designs/intent-manual-review.md, draft, id 01M3EJ5W1JX95BA2NHD4G6Z1N7
- Reviewed bytes: git blob 7a9e5c581db592543f18744b5c83ef2f896faa2d, sha256 80e94a9d3f832e9bacb107efe6184243cffdfdcb22f54341eb0b662c360b7f29, at HEAD 7987f2a93 (page committed there); dispositions page read in full
- Source read from the same working tree (uncommitted Opus edits present); citations are working-tree lines
- Round 2 of 2. Read-only. No edits except this report. 8 tool calls before this write.

## Verdict

REVISE-BOUNDED. MATERIAL: 2 (IM-C5 step 1, IM-C6 slice 2). Both are one-clause corrections, not shape failures. Round 1's IM-C1..3 folds are correct against source: the branch range, CommitStaged's consistent index, update-ref compare-and-swap and the push journal are real owners, and no new registry is needed. Slice 2 is honestly buildable from this page once IM-C6 is fixed. No invariant failure found.

## Criterion answers

Step 1, DIFFERENT or WRONG: yes, once. The page promises capture of the checkout's "complete" changes, but the primitive it names projects only the invoking workspace prefix (IM-C5). Step 1 WORKS and is SAFE without it: no authority is minted, the source is untouched, the brief binding it relies on exists (read.go:468-495).

Slice 2, honest buildability: yes. The staging flag copied from the cited scratch owner would leave a shared goal checkout conflicted (IM-C6). Concurrency and replay hold through owners the page misattributes (IM-C4, IM-C7), which are fixture corrections.

## Findings

### IM-C5 MATERIAL (step 1): "complete checkout changes" versus prefix-scoped Snapshot

Page: "the capture is the invoking Git checkout's complete tracked and untracked, unignored changes against its HEAD" and "Diagnostic snapshot/archive preparation belongs to the launch owner using existing gittree/Git primitives".

Source (read): gittree.Workspace.Snapshot projects "every addition ... UNDER THE WORKSPACE" and "the returned tree is scoped to the workspace's own prefix ... worktree changes outside a nested workspace are not the workspace's to project" (internal/gittree/gittree.go:343-352); the prefix is `rev-parse --show-prefix` of Workspace.Dir (271-274). This repository is nested (checkout root above metasystem/). A builder taking the Workspace at the invoking directory captures a subset while help says complete; a builder at top level captures everything. Different builds; the first is wrong against the page's own words and its "show the selected file list" only softens it.

Smallest correction: one clause. "Capture at the checkout's top level (`git rev-parse --show-toplevel`); paths in the result are top-level relative." The same rule applies to the replay tree comparison in slice 2, which otherwise compares a prefix-scoped tree with a full commit tree and never matches.

### IM-C6 MATERIAL (slice 2): staging must apply WITHOUT --3way

Page: "Stage the frozen patch through Git's existing --index application so both index and destination files agree" and, in Grounded owners, "buildCommitOnto applies a binary patch with git apply --index --3way" as the model.

Source (read): the scratch owner's Apply is `git apply --index --3way -` in a disposable worktree (commit_repository.go:89-91, opened at 70-88 and closed). In a persistent destination, --3way on a non-applying hunk writes conflict markers into files and conflict stages into the index; git does not roll that back. The page's promises "a failed staging operation restores only its own index/files", "A supplied patch that does not apply is refused with its inputs preserved" and "no automatic conflict resolution" are all violated if a builder copies the cited flag. The next caller then hits "must be clean or already hold exactly this captured candidate" and is refused for damage the tool made. Plain `git apply --index` checks every hunk before touching anything and is the safe form.

Smallest correction: "apply with `git apply --index --binary` and no --3way in the destination; 3-way belongs only to the commit owner's scratch worktree." Add that case to TestIntentManualWorkDelivery: a patch that would 3-way-merge but not apply cleanly is refused and the destination's index and files are byte-identical afterwards.

### IM-C4 non-material fixture: the checkout commit token is not the exclusion

Page: "These comparisons are owner reads under the same checkout token as commit, so concurrent repeated callers cannot both install."

Source (read): withGoalBranchCommitTokenAt requires the lease holder, writes one fixed identity file with pid, start time and nonce, and removes it on return (goal_branch.go:1025-1046). Its consumer is scripts/agents/pre-commit-guard.sh, which verifies that file (cmd/metasystem/lease.go:216-218). Two callers in one root overwrite the same file; the first to finish deletes the second's token. Nothing in the token serialises them. What guarantees "cannot both install" is elsewhere: installCommitOnto publishes through `update-ref --stdin` with expected old values (push.go:176-193), so the loser gets StaleCode and a checkout rollback (commit.go:457-487), and amendUnit refuses a repeated unit name in range (551-563). The contract holds; the attribution is wrong, and the loser's public outcome may be a guard or Stale error rather than "rejoin existing".

Correction: name update-ref compare-and-swap and the range check as the proof sites of IM-4; require the loser's next identical call to rejoin. Optional: take the existing lease Lock (lease.go:51-63 LockPath) around stage-plus-commit if the CLI wants a clean single-winner result rather than a retry.

### IM-C7 non-material fixture: replay comparison must use the owner's application semantics

Page: "if frozen patch applied to that unit's actual parent produces its exact tree, rejoin that authentic commit."

Source (read): the installed unit is produced by `apply --index --3way` onto state.baseTip in scratch (commit.go:382-405), and baseTip is the remote tip on adoption (152-233). gittree.Apply is a plain `apply --cached` in an isolated index (gittree.go:500-515). After an adoption where the 3-way succeeded, a plain apply onto the same parent can fail, so an identical resubmission is reported as "different tree, requires --after COMMIT". The page then refuses rather than duplicating (safe), but the promised rejoin is missed and the person is pushed to amend their own identical work, which would drop its read for nothing.

Correction: compare with the same application the owner used (scratch worktree, --index --3way onto the unit's parent, tree equality), or compare per-path post-image blobs of the frozen patch against the unit tree. One test in TestManualSubmissionReplayAndAmend: adopt-then-resubmit rejoins.

## Confirmed claims (read, not recited)

- Brief binding exists: BriefInputSHA256 and FrozenBriefSHA256 are checked on repeat (read.go:390, 468-495). "Changed brief is checked by the existing read owner" is true.
- Amendment removes only the target's Read commit and replays the suffix, then requires the replayed tree to equal the staged tree minus skipped read paths (commit.go:585-650). Manual --after COMMIT through CommitStaged with Amend and the unit name is honest, provided the staged diff is against the current tip, which the design's destination staging gives.
- Install preflight refuses overwriting unstaged tracked paths only (commit.go:407-427); untracked collisions are caught by `read-tree -m -u` (commit_repository.go:165-168). For the design's flow the index tree equals the new tip after staging, so install is a no-op checkout. Safe.
- Push journal: reconcilePushTransactions resolves an uncertain push to "reconciled" (push.go:300-323, 337-348). No commit OpID idempotence is claimed; correct.
- Launch "request/lock primitives": Manager.Store is `struct{ Root string }` with Create/Update/StateDir/AppendRefusal; the keyed entry-with-lock lives in UnitRunner (unit_named.go:391-447). The extraction's retained read request belongs beside those, not in Store. Precision note only.
- review goal G: the dispatcher's two-argument switch (intent_delivery.go:496-540) takes a new kind cleanly; reserved words resolve IM-C3.

## Unexamined

- `stop review REF` over a split sequence with several children (Manager.Cancel cancels one launch id).
- Pack and goal budget admission for a goal-associated diagnostic read (admit.go CheckPack).
- Whether the pre-commit guard also runs in the scratch worktree under a shared hooks path; relevant only to IM-C4's error text.
- gittree.Apply's handling of --binary against `--cached` for symlink and mode-only hunks.
- Concurrent Opus edits in cmd/metasystem; cited lines are the working tree, not 7987f2a93.
