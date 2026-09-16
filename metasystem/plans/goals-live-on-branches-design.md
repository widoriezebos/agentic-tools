# goals-live-on-branches: design, revision 2

Revision 2 folds the critique of 2026-09-16 (16 material findings). What changed: one range rule keeps each commit
kind to its own paths; a read is a machine-written attestation bound to a commit; the digest comes from raw tree
entries so machines agree; a rebase writes a new attestation or refuses; the landing pushes with a lease against the
trunk commit it prepared from and never rebases after its check; the landed commit carries what `verify` and `sweep`
need; no local ref is a push's authority; the transport mirror is leased; parking a goal without a branch works as
today; the first mover carries no read from a diff file to a commit; the unit table was redone.

Wido asked for this on 2026-09-16: a goal's unlanded work lives on one machine under `/tmp`, so a lost machine or a
cleaned temp folder loses it, and no release lane can take units from another host. This page rules that a goal's
work lives on a pushed git branch. It does not design the later human approval before landing; it names the commit
id that approval points at. Paths are under `metasystem/` unless absolute. Machinery is a `metasystem` Go verb; the
hand lane's steps last until the verb lands.

## 1. The branch

Seats may push `goal/<goal-id>` branches to `origin` (Wido, 2026-09-16 21:07Z) under four conditions, each a ruling
here: only the claim holder pushes its goal branch; a rewrite is pushed only with `--force-with-lease`; the
integration branch is written by the landing lane alone; landing the last unit or concluding the goal deletes the
branch.

One branch per goal, `goal/<goal-id>`, created from the tip of the endpoint `goal.ResolveEndpoint` names
(`refs/heads/main` unless `goal.sync-branch` says otherwise). Every comparison on this page is against that
endpoint; nothing names `main` on its own. While the endpoint is not `refs/heads/main` the hand lane refuses
`GOAL_BRANCH_ENDPOINT_UNSUPPORTED`, because `land.sh` lands on main; integration-branch moves the landing when it
moves the endpoint. Only the claim holder writes the branch, from its worktree; the builder leaves a staged tree and
never commits or pushes.

The range rule. `branch.ValidateRange(endpoint-tip, tip)` checks every commit in between: one parent, exactly one
kind trailer (section 3), only paths of that kind's class. Every verb on this page runs it before acting. `goal
branch check --goal G` runs it alone and prints each commit's kind; it is the hand lane's check until the commit and
push verbs land. Failure is `GOAL_BRANCH_RANGE` with the commit and the reason. `docs/project-rules.md` states the
rule for people; the verb holds it.

The push. Origin is the authority; no local ref is. `goal branch push --goal G` fetches origin's `refs/heads/goal/G`
(oid O, absent on the first push), refuses `GOAL_BRANCH_NOT_HOLDER` unless the claim is this seat's, validates the
range, then pushes `--force-with-lease=refs/heads/goal/G:O`. The outcome is classified as `goal.PublishCAS`
classifies its push: landed; refused (`GOAL_BRANCH_LEASE_MOVED`, nothing pushed); unknown, which fetches again and
reconciles from what origin holds. A rewrite writes `O -> N` to `refs/metasystem/goals/txn/<opid>` first, so a crash
anywhere reconciles by fetching. After the push the verb reads the claim again; a moved claim reports
`GOAL_BRANCH_CLAIM_LOST` and stops; the pushed tip stands, since the lease proved nothing of the new holder's was
overwritten. A new holder on any machine adopts origin's tip after `ValidateRange`. Deletion (section 8) uses the
same lease.

The mirror. `goal branch push --transport` fetches transport's ref (oid T) and pushes the oid it fetched from origin
with `--force-with-lease=refs/heads/goal/G:T`. `scripts/agents/sync-transport.sh` pushes without a lease and cannot
mirror a rewrite; it keeps serving `main`, untouched. Proof: units 1, 3 and 6.

## 2. The unit

A unit is one commit on the goal branch whose message ends with `Goal-Unit: <goal-id>/<unit>`. A fix replaces that
commit (`goal branch commit --kind unit --amend`); a unit never becomes a range, because a read binds to one commit
id and the landing certifies one diff.

The unit digest is sha256 over the raw entries of `git diff-tree -r -z --no-renames --full-index <parent> <commit>`:
mode, pre-image blob, post-image blob, status and path per entry. A blob id is the hash of its bytes, so the digest
names the bytes, and the pre-image blob binds the base the unit was built on. Raw entries do not depend on git
configuration; a patch does (checked here: under conflicting `diff.algorithm` and `diff.noprefix` the raw entries
hash equal and `git diff --binary --full-index` does not). `commitReadSubject` in `internal/dispatch/` keeps its
patch digest for its present callers; the attestation (section 5) records that subject as persisted and the unit
digest beside it, and consumers compare commit, tree and unit digest.

A rebase never reuses a read. After the holder rebases onto a moved endpoint, `goal branch commit --kind read
--carry <old-commit>` writes a new attestation for the new commit when the unit digest is unchanged, every earlier
fold commit's digest in the range is unchanged, and the old attestation is clean; it names both commits and both
trees. Otherwise it refuses `GOAL_READ_STALE` and the unit is read again. A trunk change to a file the unit touches
changes the pre-image blob and so the digest: the base is gone and a new read is right. Proof: units 1 and 4.

## 3. What the branch carries and what it never carries

Three commit kinds, told apart by trailer, with disjoint path classes:

- `Goal-Unit: <goal-id>/<unit>`: product paths only; never `plans/`, `records/`, or a path in
  `landing.WorkspaceExclusions()`.
- `Goal-Plan: <goal-id>`: `plans/` outside the goal store and `records/` outside `records/reads/` and the
  exclusions: design pages, amendments, decision records, committed before the unit they govern.
- `Goal-Read: <goal-id>/<unit> <unit-commit-id>`: exactly one attestation at
  `records/reads/<goal-id>/<unit-commit-id>.json`, plus at most one prose record under `records/misc/` that the
  attestation names by sha256.

No path belongs to two classes, so a fold never overlaps a unit and code cannot ride as a plan or a read. A commit
with no kind, two kinds, two parents, or a path outside its class is `GOAL_BRANCH_RANGE`.

The branch never carries `landing.WorkspaceExclusions()`: the two append-only registers and the goal store. The
receipt row is appended when the unit lands, as today. The goal store keeps publishing through `Publish` on the
endpoint; that compare-and-swap of coordination state is the standing exception to "the landing lane alone writes
the integration branch", which binds product-workspace bytes. Proof: unit 1.

## 4. Parking and moving

`goal park` checks before it publishes: when `goal/<goal-id>` exists on origin or the goal's `Next step` names a unit
commit, the local branch must equal origin's tip (`GOAL_PARK_UNPUSHED` otherwise), and park writes a `Next step` line
naming the last unit, its commit id and its state (built, read clean, needs read); the holder then removes the
worktree. A goal with no branch parks as today. Resuming on any enrolled machine: claim, fetch, `git worktree add
<path> goal/<goal-id>`, continue; records, attestations and plans come with the branch.

The store gains no field: the name derives from the goal id, the tip is one `git ls-remote` away, and `Next step`
carries the narrative. Proof: unit 5.

## 5. Land-ready and the attestation

An attestation is written by `goal branch commit --kind read`, never by hand. The verb computes the subject from the
repository, not from its flags: commit, parent, tree, unit digest. The file carries schema version, goal, unit,
subject, source, verdict, the cheap gates observed on the unit's tree (kind, tree, run id), the carry origin when
section 2 wrote it, and a sha256 over the rest. Two sources. `--root-job J`: a closed code-critic root whose subject
is `commit:<id>`; the verb checks the closure with `readsubject` (critic root, round, `ReturnBindsSubject` on the
tree, a clean register) and records them. `--reader-record <path>`: a prose read under `records/misc/` naming the
commit id and the unit digest; the verb records its sha256. A reader-record attestation is as strong as the record,
the hand lane's trust today, unchanged; the batch lane takes critic-root attestations only (section 10).

Every consumer re-validates an attestation before trusting it: the path names its goal and commit, the subject
matches the commit, the self-digest matches, the source rules hold. A copy for another commit sits at the wrong path
and names the wrong tree; a stale one names a replaced commit; an edited one fails its digest.

A unit is land-ready when its commit is on origin's branch and a later `Goal-Read` commit carries a valid attestation
for it with verdict LAND. A goal's land-ready set is the longest prefix of its units, in branch order, that are all
land-ready. `goal branch status --goal G` prints every commit with its kind, each unit's digest and read state, and
the prefix. This is what a release lane takes from any machine that can fetch the branch, and the state a human will
later approve: the human's word names a branch commit id, the last unit of the approved prefix. Nothing more is
decided here; one-approval-gate owns whether that word is a second word. Proof: units 4 and 5.

## 6. Landing from a branch

Both lanes take the unit's entries from the commit, not a file, and land one trunk commit per unit with its receipt
row, applied as a diff: the landed commit is a different object anyway, and a cherry-pick would add nothing the
trailers do not give.

Preparation. `goal branch land-prep --goal G --unit <commit> --root <checkout> --out <dir> --test-receipt <path>
[--last]` fetches the endpoint tip E, seeds a temporary index from it, applies in branch order the `Goal-Plan` and
`Goal-Read` commits since the previous unit, then the unit, then the receipt row, and takes the candidate tree C.
Before writing any file it checks two things. The candidate's entries against E, folded paths and exclusions
removed, hash to the attestation's unit digest; otherwise `GOAL_UNIT_REREAD` names the paths and the holder rebases
and reads again. The test receipt (schema 3) names `ProjectWorkspaceTree(C)` as its tree, or `landing observe`
accepts it for C by identity; otherwise `GOAL_LAND_UNPROVEN`. This is the deep proof the hand lane runs by hand
today, now required by the verb; the receipt row names the receipt. The verb then writes the diff, the message and a
`trunk` file naming E. The message's trailers are the landing manifest: `Goal-Unit: G/<unit>`, `Goal-Digest: <unit
digest>`, one `Goal-Source: <id>` for the unit commit and each folded commit, one `Goal-Fold: <path>` per folded
path, and `Goal-Last: G` when `--last` was given.

Publication. The human commits on E from the enrolled terminal, as today. Then `goal branch land-push --prepared
<dir>` fetches the endpoint; if its tip is not E it refuses `GOAL_LAND_TRUNK_MOVED` and pushes nothing, and the
holder returns to land-prep for a new candidate tree and its proof; otherwise it pushes
`--force-with-lease=<endpoint>:E`. Nothing rebases between the check and the push, so no trunk change reaches the
integration branch unread. `land.sh` changes in three lines: reset to E instead of `origin/main`, no rebase, push
with the lease. Until unit 8 lands the hand lane makes those edits by hand and takes E from the `trunk` file.

Verification. `goal branch verify --landed <trunk-commit>` reads the trailers, takes the landed commit's entries
against its parent, removes the `Goal-Fold` paths and the exclusions, hashes, and compares with `Goal-Digest`. It
needs no object outside the integration branch, so the branch may be deleted. Proof: units 7 and 8.

## 7. The testrun lock and the proof

The lock stays per host: it bounds engine runs on one host, and a candidate's tree id is the same everywhere, so a
proof of C anywhere is a proof of C. Two hosts proving one tree waste one run and never disagree; a disagreement is a
host-dependent test, fixed at its cause. The receipt lives on the host that ran it, so the host that proves is the
host that prepares and lands. A second machine needs an enrolled engine (`metasystem up`), the lock library
(`testrun-lock.sh` today, BA8 later) and fetch access to origin; the VM also needs the transport mirror.

## 8. Cleanup

`goal branch sweep --goal G` deletes `goal/G` on origin and transport, each with a lease against the oid it just
observed, and removes its worktrees. It deletes only when every commit in `<endpoint-tip>..goal/G`, of every kind, is
named by a `Goal-Source` trailer on the endpoint since the branch's base, or by the conclusion's `Next step` as
dropped with its commit id and digest; otherwise `GOAL_SWEEP_UNLANDED` names the commits. A tip moved between the
check and the delete is refused by the lease.

Nothing is inferred from the tip. Two things run the sweep: `goal done`, at conclusion, and the landing after it
pushes a trunk commit carrying `Goal-Last`, which only `land-prep --last` writes on the holder's word; a fully landed
branch of a live, unmarked goal stays. A parked goal's branch lives as long as the goal; the anomaly is a claimed or
parked goal whose `Next step` names a unit commit with no branch on origin, and `goal branch sweep` without `--goal`
lists it, with the branches of done goals. An abandoned goal's branch stays: abandoning the goal and deleting its
work are two decisions, so the sweep lists it and a human deletes it with `--abandoned`. Proof: units 6 and 9.

## 9. The first mover

Goal fixture-children-cannot-outlive-their-test moves onto `goal/fixture-children-cannot-outlive-their-test`. At
the hour of writing `origin/main` carries units 4a (41a545307), 4b (c91c3ac21) and 5a (7b32588a3); 5b0, 5b and 5b2
are staged in the m1c checkout and landing by the hand lane; 5c, 5d and 5e exist as worktrees, diff files and read
records on m1c alone. The round-5 read of 5b0, 5b and 5b2 names one aggregate diff for the three, so no per-unit
commit can equal what it read.

Rule: no read carries from a diff file to a commit. A read binds to a commit, or it is history.

The move, by the holder, the day unit 3 lands, starting from whatever `git log origin/<endpoint>` then shows
unlanded; until then the hand lane keeps landing from diffs as today:

1. Inventory every unit of the goal not on the endpoint, with its worktree or diff file and its recorded base tree.
   A unit with neither is rebuilt.
2. Create the branch at the endpoint tip and push it.
3. One `Goal-Plan` commit carries the amendments and decision records that exist only in the backup
   (`fixture-children-amendment-witness9.md`, `-amendment-rev6.md`, `-u5d-decisions.md`,
   `-custodian-seat-rulings.md`) and `records/misc/fixture-children-branch-move.md`, which lists every earlier read
   record with the diff sha256 it named and the commit that replaced it.
4. Each unlanded unit in order: its diff against its recorded base tree, `git apply --index` in the branch worktree,
   `goal branch commit --kind unit`, `goal branch push`. A unit with an open fix is committed as it stands; the fix
   replaces the commit.
5. Each moved unit gets a fresh read bound to its commit. After that the backup directory and the scratchpad are
   redundant.

## 10. What changes in brief B

Brief B stands; its unit gains a second identity, a branch commit beside a chain, and these rows take it:

- BA2 gains a branch reader as row BA2b: `landing batch join --goal G --unit <commit>` fetches `goal/G`, validates
  the range, requires a critic-root attestation with verdict LAND for the commit, and takes as certified output the
  unit's entries plus the folded plan and read commits. A reader-record attestation refuses `BATCH_JOIN_UNREAD`, as
  a unit outside the job domain does today; the closed-chain gate is not weakened.
- BA4 compares each member's own transition `T(i-1)..Ti`, that member's folded paths removed, to that member's
  digest, else `BATCH_JOIN_REREAD`; the cumulative prefix tree stays for receipts.
- BA1's record, BA6b's `Next` edit, BA13's receipt identity and BA14a's message carry the commit id and the section
  6 trailers; BA14b recognises a landed unit by `Goal-Source`; BA5b's handover precedes any sweep and changes no
  field; BA15's status and moved-effects inventory show branch and commit.

Every named row re-estimates; a row passing 300 splits at its witness boundary.

## 11. What changes in existing behaviour

Under R-115-m1e: a reader names `commit:<id>` and an attestation instead of a diff file and its sha256; a landing
checks the digest before the human commit and `verify` checks it after; `land.sh` resets to the prepared trunk
commit, does not rebase, and pushes with a lease; a seat pushes `goal/<goal-id>` refs to origin and, through the
verb, to transport; `goal park` requires a pushed branch only when branch work exists. Untouched: the receipt rule,
`commit.sh`, `ledger-preserve.sh`, `sync-transport.sh` for `main`, the lock, the goal store and its publication, the
ledgers, brief B's DONE, integration-branch's boundary. human-carried-landing's carry can name a branch commit; no
conflict. No question is open for Wido.

## Proof of DONE

1. Every unit built after unit 3 lands is a `Goal-Unit` commit on `origin` `goal/<goal-id>` before its read starts;
   `goal branch status` and `git ls-remote` agree.
2. `origin` `goal/fixture-children-cannot-outlive-their-test` carries every unit of that goal that was not on the
   endpoint at the move, each with a fresh attestation, and the plan commit; a fresh clone on a second enrolled
   machine prints the same `goal branch status` prefix.
3. `goal branch check` passes on every `goal/*` range on origin; unit 1's disguise witnesses refuse.
4. `goal branch verify --landed` reports equal for every unit landed from a branch; in a fixture, a one-byte
   mutation fails it and a trunk moved after preparation refuses before any push.
5. `landing batch join --unit` takes a branch unit once brief B's BA2 and BA4 exist; production availability stays
   brief B's rule.
6. Two clones with conflicting diff configuration compute one digest for one commit.
7. `docs/project-rules.md` states what a seat pushes, what a goal branch carries and who may rewrite it.

## Units

Dependency order; at most 300 changed lines each, ending on runnable behaviour. Witnesses use a bare-repository
fixture as origin (a second as transport) and a scripted claim store; they assert states, never durations, and fail
on trunk without their unit. Files are under `internal/goal/branch/` unless a path says otherwise.

| Unit | Files | Rule | Witness | Lines |
|---|---|---|---|---|
| 1. Digest, range rule, `goal branch check`. | `digest.go`, `range.go`, `cmd/metasystem/goal_branch.go`, `internal/refusal/register.go` | 1, 2, 3 | Two clones with conflicting diff config hash one commit equal; no trailer, two trailers, a merge, code under `Goal-Plan` or `Goal-Read`, a unit touching `plans/` or a register each refuse `GOAL_BRANCH_RANGE`; a clean range passes. | 290 |
| 2. Commit verb, unit and plan kinds. | `commit.go`, `cmd/metasystem/goal_branch.go`, `docs/project-rules.md` | 1, 3 | First commit creates the branch at the endpoint tip with the trailer; `--amend` replaces one commit; a non-holder refuses; without unit 3's marker the verb refuses `GOAL_BRANCH_UNAVAILABLE`. | 260 |
| 3. Push with lease and reconcile. | `push.go`, `cmd/metasystem/goal_branch.go`, the marker | 1 | Push after an amend lands; a tip moved behind the lease refuses and origin is unchanged; an unknown outcome reconciles from origin; a claim moved during the push reports lost without retry; a crash between push and txn ref reconciles; a second clone adopts origin's tip. | 300 |
| 4. Attestation and carry. | `attest.go`, `commit.go` | 2, 5 | A critic-root source validates; a reader-record source records the sha256; a copy for another commit refuses; an edited file fails its digest; a rebase over an untouched file writes a carry naming both commits; a rebase over a touched file or a changed fold refuses `GOAL_READ_STALE`. | 300 |
| 5. Status, land-ready, park. | `status.go`, `internal/goal/verbs.go`, `cmd/metasystem/goal.go` | 4, 5 | Prefix of one on clean, unread, clean; a goal with no branch parks; a goal with a branch and an unpushed tip refuses; `Next step` names the last unit and its commit. | 240 |
| 6. Transport mirror and leased delete. | `mirror.go`, `cmd/metasystem/goal_branch.go` | 1, 8 | After an amend transport equals origin; a transport tip moved behind its lease refuses; a delete against a moved oid refuses and the ref stays. | 180 |
| 7. Landing preparation. | `land.go`, `cmd/metasystem/goal_branch.go` | 6 | Non-fold entries hash to the digest; a trunk change in a unit file refuses before any file is written; a missing or wrong-tree receipt refuses `GOAL_LAND_UNPROVEN`; the message carries every trailer; `--last` writes `Goal-Last`. | 300 |
| 8. Landing publication and verify. | `publish.go`, `verify.go`, `/Users/wido/LocalStorage/hact-20260912/land.sh` | 6 | A moved endpoint refuses before any push; the leased push lands on E; `verify` reports equal; a one-byte mutation in a fixture's landed commit fails it. | 240 |
| 9. Sweep and conclusion. | `sweep.go`, `internal/goal/verbs.go`, `cmd/metasystem/goal.go` | 8 | A plan-only tail refuses; an unknown commit refuses; a tip advanced between check and delete refuses; a `Goal-Last` landing sweeps; `goal done` sweeps; a parked goal without a branch is listed; an abandoned branch survives a plain sweep. | 290 |
| 10a. Batch reader and per-member check. After BA2 and BA4. | `internal/landing/batch/branch.go`, `join.go` | 10 | A fixture branch with a critic-root attestation certifies the digest's bytes; a reader-record attestation refuses `BATCH_JOIN_UNREAD`; a two-unit batch passes with each member compared to its own transition; a moved trunk in a unit file refuses `BATCH_JOIN_REREAD`. | 280 |
| 10b. Batch identity rows. After 10a, BA14b and BA15. | `record.go`, `return.go`, `receipts.go`, `land_apply.go`, `recovery.go`, `cmd/metasystem/landing_batch_verbs.go`, the inventory | 10 | Record and receipt carry the commit id; `Next` names it; recovery finds a landed unit by `Goal-Source`; `validate moved-effects` reports zero problems. | 300 |
| 11. First mover. After unit 3. | `records/misc/fixture-children-branch-move.md` | 9 | Proof of DONE 2. | 60, records only |
