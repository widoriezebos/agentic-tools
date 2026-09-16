# goals-live-on-branches: design, revision 1

Wido asked for this on 2026-09-16: a goal's unlanded work lives on one machine
under `/tmp`, so a lost machine or a cleaned temp folder loses it, and no
release lane can take units from another host. This page rules that a goal's
work lives on a pushed git branch. It does not design the later human approval
before landing; it names the commit id that approval points at. Evidence:
`/Users/wido/LocalStorage/hact-20260912/`, `internal/goal/`, brief B, the
branches on origin. Paths are under
`metasystem/` unless absolute. Machinery is a `metasystem` Go verb; hand steps
are the hand lane and last until the verb lands.

## 1. The branch

Seats may push `goal/<goal-id>` branches to `origin` (GitHub). Wido,
2026-09-16 21:07Z, under four conditions, each a ruling here: only the claim
holder pushes its goal branch; a rewrite is pushed only with
`--force-with-lease`; `main` is written by the landing lane alone; landing the
last unit or concluding the goal deletes the branch.

One branch per goal, named `goal/<goal-id>`. The first `goal branch commit`
for the goal creates it from the trunk tip `goal.ResolveEndpoint` names
(`refs/heads/main` today; integration-branch may move it and these verbs
follow). It is pushed after every build, fix and record,
and mirrored to `transport` with `scripts/agents/sync-transport.sh
goal/<goal-id>` when a proof on the VM needs it; transport receives origin's
ref, never a local branch.

Only the claim holder writes the branch, from its worktree. The builder leaves
a staged tree and never commits or pushes: one identity signs every commit,
and a second writer would leave a unit whose bytes nobody can certify against
a read.

A rewrite has two shapes, both by the holder: replacing the open unit's commit
after a fix (section 2) and rebasing onto a moved trunk. Every push is
`--force-with-lease` against the tip the holder last pushed, kept in
`refs/metasystem/goals/pushed/<goal-id>`; a moved tip refuses
`GOAL_BRANCH_LEASE_MOVED` and pushes nothing. A non-holder refuses
`GOAL_BRANCH_NOT_HOLDER`. The verb enforces this; `docs/project-rules.md`
states it for the hand lane. Proof: units 1 and 2.

## 2. The unit

A unit is one commit on the goal branch whose message ends with
`Goal-Unit: <goal-id>/<unit>`. A fix replaces that commit (`goal branch
commit --kind unit --amend`); a unit never becomes a range, because a read
binds to one commit id and the landing certifies one diff. Earlier read
records sit after the unit as their own commits (section 3) and are rebased
onto the replacement; they name the replaced id and digest, so a refusal stays
on record without the object.

The unit's certified output is its canonical diff, `git diff --binary
--full-index <commit>^ <commit>`, hashed with sha256: the existing read
subject of kind commit in `internal/dispatch/read_subject_compute.go`
(`commitReadSubject`, field `DiffDigest`), reused unchanged. A read record
names `commit:<id>` and this digest instead of a diff file and its checksum.

A rebase keeps a read valid when the unit's canonical diff against its new
parent hashes to the digest the record names; otherwise the unit is read
again. With `--full-index`, a trunk change to a file the unit touches changes
the pre-image blob id and so the digest: that file's base is gone, and a new
read is right. A change elsewhere leaves the digest equal. Proof: unit 3.

## 3. What the branch carries and what it never carries

Three commit kinds, told apart by trailer:

- `Goal-Unit: <goal-id>/<unit>`: code, tests, docs; the certified unit.
- `Goal-Read: <goal-id>/<unit> <unit-commit-id>`: one read record under
  `records/misc/`, nothing else, committed by the holder.
- `Goal-Plan: <goal-id>`: design pages, amendments, decision records under
  `plans/` and `records/`, committed before the unit they govern, so the
  unit's tree contains them.

The branch never carries the append-only registers (`memory/receipts.log`,
`records/narrator-digest.log`) or the goal store (`plans/goals/`,
`plans/goals.md`, `plans/goals-accepted.json`, `records/goals`,
`records/counselor`): `landing.WorkspaceExclusions()` plus the receipt ledger.
The receipt row is appended when the unit lands, as today; the goal store
keeps publishing through `Publish` on the trunk tip.

One list, three places: `goal branch commit` refuses a staged path on it
(`GOAL_BRANCH_CARRIES_REGISTER`), `goal branch push` refuses a branch whose
`origin/main...tip` names meet it, and the landing (section 6) refuses a
branch commit touching it. Proof: units 1 and 4.

## 4. Parking and moving

`goal park` requires the branch pushed (`GOAL_PARK_UNPUSHED` otherwise) and
writes a `Next step` line naming the last unit, its commit id and its state
(built, read clean, needs read); the holder then removes the worktree.
Resuming on any enrolled machine: claim, fetch, `git worktree add <path>
goal/<goal-id>`, continue; records and plans come with the branch.

The store gains no field: the name derives from the goal id, the tip is one
`git ls-remote` away, and `Next step` carries the narrative. A tip field would
drift on every push and cost a trunk commit through `Publish` each time.
Proof: unit 3.

## 5. Land-ready

A unit is land-ready when its commit is on the pushed branch, a `Goal-Read`
commit later on the branch carries a record naming that commit id, its digest
and the verdict LAND, and the record names the cheap gates it observed on that
tree (fast gate with ratchet, changed packages' tests). A goal's land-ready
set is the longest prefix of its units, in branch order, that are all
land-ready; a unit behind one that needs a read is out: its parent is not
what the trunk receives.

`goal branch status --goal G` prints every commit with its kind, each unit's
digest and read state (unread, needs read, LAND with the record's path), and
the land-ready prefix. This is what a release lane takes from any machine
that can fetch the branch, and the state a human will later approve: the
human's word names a branch commit id, the last unit of the approved prefix.
Nothing more is decided here. one-approval-gate wants one human word per
goal; an approval to land is a second unless it is that word's landing half,
and that goal owns the answer. Proof: unit 3.

## 6. Landing from a branch

Both lanes take the unit's diff from the commit, not a file, and land one
trunk commit per unit with its receipt row.

Hand lane: `goal branch land-prep --goal G --unit <commit> --root <checkout>
--out <dir>` fetches the trunk tip, seeds a temporary index from it, applies
in branch order the `Goal-Plan` commits since the previous unit, the unit's
canonical diff and its `Goal-Read` commits, appends the receipt row, and
writes the diff file, the message file and a fold manifest (folded commit ids
with their paths). The message carries `Goal-Unit: G/<unit>` and one
`Goal-Source: <commit-id>` per folded commit. `land.sh` then runs as today;
the human commits from the enrolled terminal until brief B's verbs replace it.
Before writing any file the verb checks that the prepared diff, restricted to
paths outside the fold manifest and the receipt ledger, hashes to the read's
digest; otherwise `GOAL_UNIT_REREAD` names the paths, the holder rebases and
the unit is read again. A fold path overlapping the unit's paths refuses
`GOAL_LAND_FOLD_OVERLAP`.

After the push, `goal branch verify --landed <trunk-commit> --unit <commit>`
recomputes the landed commit's restricted diff against the digest, a pure
function of trunk history. The check moves from a diff file's sha256 to this
digest; the byte-equality does not change.

The landing applies the diff, not a cherry-pick: the landed commit is a
different object anyway (receipt row, folded records, the terminal's human
author), and a cherry-pick would change the author and add nothing the
trailers do not give. Proof: unit 4.

## 7. The testrun lock and the proof

The lock stays per host: it bounds engine runs on one host; the tree id of a
prefix is the same everywhere, so a proof of tree T anywhere is a proof of T. Two hosts proving the same tree waste one run and
never disagree; a disagreement is a host-dependent test, fixed at its cause.
The host that proves lands: the receipt is minted where the proof ran.

A second machine needs an enrolled engine (`metasystem up`), the lock library
(`testrun-lock.sh` today, BA8 later) and fetch access to origin; the VM also
needs the transport mirror. The rest is its rollout.

## 8. Cleanup

Landing the last unit or concluding the goal deletes the branch: `goal branch
sweep --goal G` deletes `goal/G` on origin and transport and removes its
worktrees; the landing runs it after the last unit's push and `goal done` at
conclusion. It refuses while a `Goal-Unit` commit has no trunk commit carrying
its id as a `Goal-Source` trailer, unless the conclusion's `Next step` names
that unit as dropped with its commit id and digest; a concluded goal keeps no
work outside the trunk. A parked goal's branch lives as long as the goal. The
anomaly is a claimed or parked goal naming a unit commit in `Next step` with
no branch on origin; `goal branch sweep` without `--goal` lists it, and
branches of done goals. An abandoned goal's branch stays:
abandoning the goal and deleting its work are two decisions, so the sweep
lists it as deletable and a human deletes it with `--abandoned`. Proof: unit 5.

## 9. The first mover

Goal fixture-children-cannot-outlive-their-test moves onto
`goal/fixture-children-cannot-outlive-their-test`, by the hand lane until
units 1 and 2 land:

1. Units 4a, 4b and 5a land from their diff files as planned.
2. The branch is created at the trunk tip containing them.
3. The two amendments and the decision record that exist only in the backup
   become one `Goal-Plan` commit at the front. A plan change inside a unit's
   diff file stays in that unit's commit.
4. Units 5b0, 5b and 5b2, in order: `git apply --index` of the diff file in
   the branch worktree, then `goal branch commit --kind unit`. The new
   commit's diff against its parent, cut with the command that cut the file,
   is byte-equal to the file, so the record's sha256 still names these bytes
   and the read carries; the parent differs from the read's base only in
   records and the receipt ledger, which the units do not touch.
   `records/misc/fixture-children-branch-move.md` binds each:
   diff file sha256, commit id, canonical digest. Each unit's read records
   follow it as `Goal-Read` commits.
5. Unit 5c is committed as it stands; its open fix replaces the commit and the
   next read names the new id.
6. Unit 5d is committed; its first read names the commit.
7. Push after every step; the backup directory becomes redundant.

## 10. What changes in brief B

Brief B stands; three inputs are added.

- BA2 gains a branch reader as row BA2b: `landing batch join --goal G --unit
  <commit>` fetches `goal/G`, requires the commit reachable from the tip and a
  `Goal-Read` LAND record after it, and takes as certified output the
  canonical diff plus the folded plan and read diffs; transport is byte-exact
  as in BA2; a missing record refuses `BATCH_JOIN_UNREAD`.
- BA4's assembly, after a branch unit's `--3way` apply, checks that the prefix
  diff restricted outside the fold paths hashes to the digest, else
  `BATCH_JOIN_REREAD`; the function is BA2b's.
- BA14a's message carries `Goal-Unit` and `Goal-Source`; BA15 shows branch
  and commit.

BA4 and BA14a re-estimate; a row passing 300 splits at its witness boundary.

## 11. What changes in existing behaviour

Under R-115-m1e: a reader names `commit:<id>` and its digest instead of a
diff file and its sha256; a landing checks the same byte-equality computed
from commits, before the human commit and provably after; a seat pushes
`goal/<goal-id>` refs to origin, mirrored to transport by the existing script.
Untouched: the receipt rule, `land.sh`, `commit.sh`, `ledger-preserve.sh`,
the lock, the goal store, the ledgers, brief B's DONE, and integration-branch's
boundary (goal branches integrate into the designated integration branch,
`main` today). human-carried-landing's carry can name a branch commit; no
conflict. No question is open for Wido.

## Proof of DONE

1. Every unit built after this page lands is a `Goal-Unit` commit on `origin`
   `goal/<goal-id>` before its read starts; `goal branch status` and `git
   ls-remote` agree.
2. `origin` `goal/fixture-children-cannot-outlive-their-test` carries 5b0,
   5b, 5b2, 5c, 5d, their read records and the plan commit, and a fresh clone
   on another enrolled machine shows them.
3. No commit on any `goal/*` branch touches a register or the goal store;
   unit 1's refusal witness is green.
4. `goal branch verify --landed` reports equal for every unit landed from a
   branch and refuses a one-byte mutation in a fixture.
5. `landing batch join --unit` takes a branch unit once brief B's BA2 and BA4
   exist; production availability stays brief B's rule.
6. `docs/project-rules.md` states what a seat pushes, what a goal branch
   carries and who may rewrite it.

## Units

Dependency order; at most 300 changed lines each, ending on runnable
behaviour; witnesses use a bare-repository fixture as origin, no clocks, sleeps
or process timing, and fail on trunk without their unit.

| Unit | Files | Rule | Witness | Lines |
|---|---|---|---|---|
| 1. Commit verb and rule. `goal branch commit --goal G --kind unit\|read\|plan --unit N [--commit C] [--amend]`; branch from the endpoint trunk; trailer; register refusal; holder check. | `cmd/metasystem/goal_branch.go`, `internal/goal/branch/commit.go`, `internal/refusal/register.go`, `docs/project-rules.md` | 1 to 3 | Staged `receipts.log` refuses; non-holder refuses; the commit sits on `goal/<id>` with its trailer. | 290 |
| 2. Push with lease and mirror. `goal branch push --goal G [--transport]`; pushed-tip ref; name check; `sync-transport.sh` for the ref. | `internal/goal/branch/push.go`, `cmd/metasystem/goal_branch.go` | 1 | Push after an amend succeeds; a tip moved behind the lease refuses and pushes nothing; transport holds origin's ref. | 220 |
| 3. Status, land-ready, park. `goal branch status`; digest via `commitReadSubject`; needs-read after rewrite; prefix; `goal park` requires a pushed branch and writes `Next step`. | `internal/goal/branch/status.go`, `internal/goal/verbs.go`, `cmd/metasystem/goal.go` | 2, 4, 5 | Prefix of one on clean, unread, clean; rebase over an untouched file carries, over a touched file needs read; unpushed park refuses. | 280 |
| 4. Landing input, hand lane. `goal branch land-prep`, `goal branch verify --landed`; fold manifest; trailers; the two refusals. | `internal/goal/branch/land.go`, `cmd/metasystem/goal_branch.go` | 6 | The prepared diff's non-fold part hashes to the digest; a trunk change in a unit file refuses before any file is written; a planted register path refuses; a one-byte change in a landed commit fails verify. | 300 |
| 5. Cleanup and anomalies. `goal branch sweep`; deletion on both remotes after every `Goal-Source` is on trunk; anomaly list; abandoned kept. | `internal/goal/branch/sweep.go`, `cmd/metasystem/goal_branch.go` | 8 | An unlanded unit refuses deletion; a parked goal without a branch is listed; an abandoned branch survives a plain sweep. | 240 |
| 6. Batch-lane input. BA2b reader for `landing batch join --unit`; BA4's re-read check; BA14a trailers; BA15 fields. After BA2 and BA4. | `internal/landing/batch/branch.go`, `join.go`, `land_apply.go` | 10 | Join from a fixture branch certifies the digest's bytes; no LAND record refuses; a moved trunk in a unit file refuses re-read. | 280 |
| 7. First mover. Section 9 by the hand lane, then the verbs; the binding note. After unit 2. | `records/misc/fixture-children-branch-move.md` | 9 | Proof of DONE 2. | 60, records only |
