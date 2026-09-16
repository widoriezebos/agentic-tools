# Findings: goals-live-on-branches, revision 1

The page is not safe to build. The most serious paths either let bytes outside the certified unit ride into a landing, or make the only post-landing check happen after those bytes are already on the integration branch.

## 1. Fold kinds can carry code that the unit read never certified

Section 3 says a `Goal-Read` commit contains one record and a `Goal-Plan` commit contains only `plans/` and `records/`, but the only executable cleanliness rule it gives is the register/goal-store exclusion at lines 82-85. Unit 1 likewise names only a register refusal (line 253), and section 6 excludes every fold-manifest path from the digest comparison (lines 132-136). A holder can therefore stage `internal/x.go` as a `Goal-Plan`, then make a normal unit commit touching a different path. The push passes because `internal/x.go` is not a register, the unit read covers only the unit commit's diff, and landing folds `internal/x.go` while removing it from the digest comparison. That lands code nobody read. An ordinary unclassified commit has the same problem because push never requires every branch-only commit to have exactly one recognized kind.

Smallest fix: make commit, push, status, join, land-prep and sweep independently validate the complete branch range. Every commit must be linear and have exactly one kind trailer; `Goal-Read` must change exactly one validated record under `records/misc/`; `Goal-Plan` must change only the declared plan/record classes; `Goal-Unit` must not use either fold class or any workspace exclusion. Add witnesses for code disguised as plan/read, a commit with zero or two kind trailers, and a plain commit.

## 2. A holder-authored prose record is treated as proof of a clean read

Sections 3 and 5 make the claim holder commit the read record and let status accept a record that “names” a commit, digest, LAND verdict and cheap gates (lines 68-70 and 102-112). No schema, canonical path, parser, closed critic root, return digest, or link to the persisted closure is defined. Existing commit-read authority is more specific: `internal/readsubject/subject.go` lines 50-53 binds commit, tree and digest, and `internal/readsubject/closure.go` lines 107-114 binds the critic return to the subject tree. Those artifacts are not carried by the proposed branch record. A copied, stale or hand-edited Markdown file can therefore say LAND for any digest and make the unit land-ready.

Smallest fix: define a machine-written, strict read-attestation schema and path. It must carry the exact persisted subject, critic root and closed return identity, verdict/finding closure, gate evidence identities, and a digest over the attestation. `goal branch commit --kind read` must derive and validate it from closed evidence rather than trust staged prose; every later consumer must revalidate it. Carry enough evidence on the branch for a fresh clone to do that without the original machine's `artifacts/` directory.

## 3. The hand lane has no enforced deep-proof admission

The checked brief says today one deep proof of the last tree runs under the testrun lock before landing (`design-brief-goals-live-on-branches.md` line 19). This page defines land-ready using only a LAND read and cheap gates (lines 102-117), and `goal branch land-prep` takes only goal, unit, root and output directory (lines 124-131). Section 7 says a host proves and lands, but supplies no proof receipt to land-prep, no exact candidate-tree binding, and no refusal when the proof is absent. The command can thus prepare a receipt row and feed `land.sh` without the existing deep proof. That weakens an existing gate under R-115-m1e.

Smallest fix: keep “land-ready” as queue eligibility if desired, but define a separate landing admission. Land-prep must require and validate the existing schema test receipt/deep-proof evidence for the exact folded candidate tree (including the workspace exclusions), under the existing lock and goal revision rules, and derive the receipt row from that evidence. Add an absent, wrong-tree and moved-tree refusal witness.

## 4. `land.sh` can rebase to different bytes after the last preflight check

Section 6 checks the prepared diff before writing it and relies on `goal branch verify` after the push (lines 124-141). The actual `/Users/wido/LocalStorage/hact-20260912/land.sh` fetches and hard-resets again at line 17, commits at line 31, fetches and rebases onto a possibly newer `origin/main` at line 33, and pushes at line 34. A concurrent trunk change in a unit file can arrive after land-prep. If Git applies/rebases without a textual conflict, the pushed commit's parent-side blob and canonical diff differ from the read digest. The proposed verify then reports the defect only after unread bytes are on main.

Smallest fix: make the landing transaction own an exact fetched trunk oid from preparation through push. Recompute the restricted digest after any rebase and before commit/push, and publish with an explicit lease on that oid; a moved trunk must return to preparation/proof, not rely on post-push detection. This necessarily changes `land.sh` or replaces its reset/rebase/push portion, so section 11 cannot call it untouched.

## 5. Digest equality is not the existing commit-read identity and does not safely carry across rebase

Section 2 says `commitReadSubject` is reused unchanged and that equal `DiffDigest` alone keeps a read valid after rebase (lines 52-62). The existing function records parent, tree, commit and digest (`internal/dispatch/read_subject_compute.go` lines 167-185), while subject equality includes commit, tree and digest (`internal/readsubject/subject.go` lines 50-53). The page also contradicts itself: section 5 requires the LAND record to name the current commit id, while the rebased record necessarily names the old id unless the holder edits a reader's attestation.

There is also an unsafe concrete carry. Put a `Goal-Plan` before unit U, read U, then rebase onto a trunk change to that plan path and resolve the plan conflict to different bytes. U's own code diff and digest can remain equal, so the page carries the read, yet landing folds plan bytes that were not in the reviewed tree. The fact that plans precede the unit so its tree contains them (lines 71-73) is exactly why dropping the tree binding is unsafe.

Smallest fix: describe this as a new recertification operation, not reuse unchanged. It must bind old subject to new commit and tree, prove both the unit delta and every folded source byte unchanged, and emit an immutable carry attestation naming both commits. Any changed fold or ambiguous mapping requires a new read.

## 6. The proposed “canonical diff” is not canonical across machines

`commitReadSubject` invokes ordinary `git diff --binary --full-index` (`internal/dispatch/read_subject_compute.go` lines 167-181) through `gitRawOutput`, which inherits repository/user Git configuration and supplies no deterministic `-c` settings (`internal/dispatch/gitcmd.go` lines 23-34). Settings such as the diff algorithm, rename detection, prefixes/order and text conversion can change patch bytes for the same two trees. That was acceptable as an existing local read subject; it cannot be reused unchanged as a cross-machine wire digest. Two enrolled machines can compute different digests for one immutable commit, needlessly invalidating a valid read or making status disagree.

Smallest fix: define and implement one configuration-independent serialization, preferably from sorted path, mode and pre/post blob ids plus blob content for changed entries. If the patch serialization is retained, pin every output-affecting Git option and disable attributes/config-driven transformations. Prove the same digest in two clones with deliberately conflicting Git diff configuration.

## 7. Deleting the branch destroys the evidence needed by `verify`

Section 6 calls post-landing verification “a pure function of trunk history” (lines 138-141), but the landed message carries only `Goal-Source` object ids (lines 129-130). It carries neither the unit digest nor the fold path manifest. A trailer containing an oid does not make that object reachable. Section 8 then deletes the only origin and transport refs that reach the unit, plan and read commits (lines 159-167). A fresh clone after deletion cannot fetch `--unit <commit>`, derive the fold paths, or recompute the source digest, and server garbage collection may remove the objects entirely.

Smallest fix: put a canonical, self-validating landing manifest in the landed commit (or another trunk-reachable object). It must contain the unit digest, source ids, source path sets/digests and exclusions needed to recompute the restricted landed diff without reading any unreachable object. `verify` and sweep must use that durable manifest, and a fresh-clone-after-delete witness must pass.

## 8. The local pushed-tip ref is neither transferable nor crash-recoverable

Sections 1 and 4 keep the lease expectation only in `refs/metasystem/goals/pushed/<goal-id>` on one clone, then say another machine merely claims, fetches and adds a worktree (lines 35-40 and 89-98). The new holder has no such ref. Worse, no ordering can atomically update a local ref and the remote branch: crash after the remote accepts but before the local ref advances leaves the next push permanently reporting `GOAL_BRANCH_LEASE_MOVED`; advancing the local ref first has the dual crash. A second machine can also retain a stale local ref forever.

The holder check is a separate check-then-push race. A loses its claim after its check; B claims; A can still win the remote branch lease if B has not pushed yet. Sweep has the same race and the page gives deletion no expected oid at all, so a stale sweep can delete a new holder's just-pushed work.

Smallest fix: make the remote observed oid, claim pair/epoch and intended new oid a recoverable transaction. A new holder adopts the freshly fetched remote oid only after validating ancestry and branch state; an ambiguous push is reconciled from the remote postcondition. Recheck the claim after publication and repair/refuse stale-epoch updates. Every rewrite and deletion must use an explicit expected remote oid. Add two-machine reclaim, crash-before/after-local-bookkeeping, lost-claim-during-push and advance-during-delete witnesses.

## 9. The existing transport mirror cannot mirror a rewrite

Section 1 says rewritten branches are mirrored with the existing `sync-transport.sh`, and unit 2 promises that transport holds origin after an amend (lines 24-27 and 254). The script fetches origin with force at `scripts/agents/sync-transport.sh` line 31, but pushes to transport without force or lease at line 35. Once transport has the old unit commit, an amend or rebase is a non-fast-forward update and that push is rejected. Unit 2 cannot meet its own witness with its stated file list.

Smallest fix: include the mirror implementation in unit 2 and update transport with an explicit lease against its freshly observed old tip, sourcing the new oid only from origin. Treat divergence and unknown outcomes with the same recovery protocol as origin; deletion also needs a leased transport update.

## 10. Sweep can delete branch-only work and cannot know what “last unit” means

Section 8 refuses deletion only for an unlanded `Goal-Unit` (lines 161-172). It does not require every `Goal-Plan`, `Goal-Read` or unclassified commit to appear in a landed manifest. A plan commit made before its future unit, or an accidental ordinary commit, therefore satisfies the stated sweep condition and is lost when the branch is deleted. Checking only a `Goal-Source` string also does not prove that landing folded the source's paths.

The automatic “after the last unit's push” trigger is not implementable from the proposed state. Section 4 deliberately adds no unit/tip field to the goal store (lines 95-98), and `Next step` is narrative. The current tip being fully landed does not mean the goal has no future unit; deleting then recreating also loses any unmatched plan/read commit.

Smallest fix: sweep validates every branch-only commit against durable landed manifests and refuses any unmatched source. Add an explicit, authoritative last-unit/conclusion signal (or require an explicit `--last` under the appropriate authority) rather than infer it from the current tip. The witness must include a plan-only tail, an unknown commit, a source trailer whose manifest omitted paths, and a concurrent tip advance.

## 11. The first mover's clean read cannot be split among the three proposed commits, and its source bytes are already absent

Section 9 claims 5b0, 5b and 5b2 each have a diff file whose bytes become that commit's canonical diff and whose old read therefore carries (lines 185-193). The actual read is one aggregate: `/Users/wido/LocalStorage/hact-20260912/backup-m1c-fixture-children/g18/read-fcu5b-round5.md` lines 5-10 names one `fcu5b-r5.diff` and one digest for all three, and lines 43-47 explicitly partitions that one reviewed change into 43, 299 and 100 changed-line units. No individual commit diff can equal the aggregate diff.

The hash record at `.../g18/fcu5b-r5-diff.sha256` line 1 points into the vanished `/private/tmp/.../scratchpad/fcu5b-r5.diff`; that file and `g18/wt-fcu5b` are absent now. The backup contains hashes and read prose, not that patch or worktree. The surviving `wt-fcu5d-snapshot-2248.diff` covers only four paths and is not the missing 5b2/5c candidate. Thus Proof of DONE 2 cannot be reached by the stated migration, and this is the loss scenario the design was meant to prevent.

Smallest fix: stop calling these reads transferable. Recover/rebuild the exact candidates from any remaining authoritative artifacts, make the intended individual commits, and obtain fresh reads for those commits; if exact bytes cannot be recovered, record the work as lost and rebuild it. Change the first-mover proof to inventory the actual objects before claiming safety.

## 12. BA2b bypasses brief B's closed-chain gate and the list of affected rows is incomplete

Brief B says DONE remains unchanged and defines a unit as one closed implementation chain with a closed code-critic root (`units-land-in-batches-under-one-proof-brief-b.md` lines 3-5 and BA2 at line 40). The proposed BA2b accepts a reachable commit plus a later `Goal-Read` LAND record (lines 203-207), which is the unauthenticated record from finding 2, and never requires the implementation chain or closed critic root. That weakens BATCH_JOIN_UNREAD rather than adding an equivalent transport.

Changing unit identity from chain to branch commit also affects BA1's record identity, BA3's gate input, BA5/BA6 claim handover and uniqueness, BA13 receipt identity, BA14b recovery and the moved-effects inventory. Section 10 names only BA2, BA4, BA14a and BA15, while saying the rest of brief B stands. Unit 6 therefore cannot be integrated into brief B as specified.

Smallest fix: either carry and validate the original closed chain/critic closure in the branch attestation, or amend brief B with an explicitly equivalent commit-attestation gate without weakening DONE. Enumerate and re-estimate every affected BA row, dependency, capability and moved effect; do not hide all of them in BA2b/BA4/BA14a/BA15.

## 13. BA4 compares an accumulated prefix to one unit's digest

Section 10 says that after applying a branch unit, BA4 hashes “the prefix diff” outside fold paths and compares it with that unit's canonical digest (lines 208-210). In brief B, prefix `Ti` is the cumulative tree after units 1 through i (`units-land-in-batches-under-one-proof-brief-b.md` lines 75-79). For the second unit, `baseTree..T2` contains unit 1 as well as unit 2, so it cannot hash to unit 2's parent-to-commit digest. A lawful multi-unit batch is refused even when every unit is unchanged.

Smallest fix: compare the per-member transition `T(i-1)..Ti`, after separating that member's folds, to that member's digest. Keep the cumulative prefix tree for receipts. Add a two-unit, disjoint-path witness and a same-file non-conflicting witness.

## 14. The target-branch and main-writer contracts contradict the code the page leaves unchanged

The page says goal branches start from the endpoint returned by `goal.ResolveEndpoint` and will follow a designated integration branch (lines 21-24 and 223-225), but its cleanliness check hard-codes `origin/main` (line 84), and the retained hand script hard-codes reset, rebase and push to `origin/main` (`land.sh` lines 17, 33 and 34). With `goal.sync-branch` set to a development branch, the goal branch is based there while landing bypasses it and writes main, contradicting `integration-branch.md` line 8.

There is a second literal conflict: section 1 says main is written only by the landing lane, while section 3 and section 11 leave goal-store `Publish` writing the trunk. `ResolveEndpoint` defaults to `refs/heads/main` (`internal/goal/txn.go` lines 49-63), and `PublishCAS` pushes that ref directly at lines 346-375. Both statements cannot hold.

Smallest fix: use one resolved integration endpoint in branch comparison, preparation, proof, push and verification; remove every hard-coded main from this feature. State and enforce the exact scope of the main-writer ruling. If it is literal, goal-store publication must move behind the landing owner; if ledger CAS publication is the intended existing exception, the page must say so rather than claim exclusivity.

## 15. Parking a claimed goal with no build becomes impossible

The first branch is created only by the first `goal branch commit` (lines 21-24), but section 4 makes every `goal park` require a pushed branch (lines 89-93). A newly claimed goal with no unit or plan commit has no branch and can be parked today through the existing park verb. The new unconditional refusal changes existing lifecycle behavior and can strand work that has nothing to preserve. Section 11 does not list this existing-behavior change.

Smallest fix: require a pushed branch only when branch work exists or `Next step` names a unit commit. An empty pre-build goal must park as it does today. Add both cases to unit 3 and list the intentional conditional behavior in section 11.

## 16. The unit plan cannot implement or prove the page within its own boundaries

The preceding omissions are also concrete unit failures. Unit 4 is already allocated exactly 300 lines, yet a safe implementation must add proof-receipt admission and replace or change `land.sh`'s post-prep rebase/push; neither file or behavior is in its row. Its one-byte landed mutation witness detects damage after publication rather than proving prevention. Unit 5 promises that `goal done` runs sweep, but lists only `internal/goal/branch/sweep.go` and `cmd/metasystem/goal_branch.go` (line 257); current done behavior lives in `internal/goal/verbs.go` lines 1667-1702 and `cmd/metasystem/goal.go` lines 219-229. Unit 6's 280 lines omit the many brief-B owners identified in finding 12. Unit 1 also ends before push, so it is not the brief's requested first safe slice of “commits pushed after every round.”

Smallest fix: redo the dependency table after the protocol is corrected. Split at independently runnable refusal/witness boundaries, list every production and test owner, make the initial rollout unavailable until commit plus recoverable origin push are both present, and give each race/crash witness a deterministic seam rather than a post-push observation.

## Short notes

- `landing.WorkspaceExclusions()` does contain both append-only registers and all named goal-store paths (`internal/landing/registers.go` lines 25-46 and 62-81). The page's “plus the receipt ledger” is redundant, not wrong.
- `commitReadSubject` uses `git diff`, not the `git diff-tree -p` wording required by the author brief. The larger problem is the unpinned serialization in finding 6.
- `origin/main` at the checked design commit already contains 4a, 4b and 5a. Section 9 step 1 is stale as an action, although creating the branch from a tip containing them is still the right base condition.
- The current project rules have no pre-existing goal-branch policy; adding one is appropriate. Prose cannot substitute for the push/land validators above.

MATERIAL: 16
