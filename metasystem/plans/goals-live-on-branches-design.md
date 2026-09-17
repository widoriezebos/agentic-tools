# goals-live-on-branches: design, revision 3

Revision 3, 2026-09-17, Wido's ruling: the goal lands, not the slice. An amendment of revision 2, which Wido approved
on 2026-09-17 with three rulings: a goal's slices land on its branch and only the goal lands on the endpoint, under
one full suite at the merge; a red found at that landing is fixed at once as part of the landing, with no new goal
and no round trip; what counts is a goal that can be finished and closed, not a commit. What changed: section 3's
receipt-row sentence; section 5 makes the cheap gate the unit's only machine check and defines a land-ready goal;
section 6 is one landing act per goal, rules 6.1 to 6.8 (the landing set, one candidate and one proof, one trunk
commit per unit under one receipt, the landing record, the red loop, trunk red, budget, amendments at the landing);
section 7 takes the lock once per landing and admits an optional interim run; section 9 lands the first mover as a
goal; section 10 makes a batch member a goal's landing set; section 11 ends the hand lane's per-unit proof; Proof of
DONE 4 and 5; units 4, 7 (split into 7a and 7b), 8, 10a and 10b. Displaced: revision 2's per-unit trunk landing with
a per-unit test receipt (`GOAL_LAND_UNPROVEN` per unit), a revision 1 shape that the critique fold "the landing
pushes with a lease against the trunk commit it prepared from" had kept; the lease rule itself stands, for the series.

Boundary amendment, 2026-09-17 18:05, on a conflict that the build of unit 10b found; the answer is Wido's, relayed by
m1e. Section 10 puts a goal branch into a batch as one member, brief B sends every batch commit through `commit.sh`
(R15, BA14a), and section 11 said `commit.sh` is untouched. The boundary had no declaration for a branch member: a
branch member has no implementer chain, so `landing observe --chain` returns a would-refuse code for its critic root
or its branch tip, and under the boundary's observe mode that commit would land with the would-refuse verdict, without
`Goal-Revision`, and without the checks every declaration meets. Wido's answer is yes: the boundary admits a branch
member's commit on a critic-root attestation that is validated again at the landing root and bound to the landed
bytes, without an implementer job record, as a fourth declaration beside `--chain`, `--direct-fix` and `--carried`
(10.1, unit 10c). Unit 10c is built right after this goal merges and does not gate the merge. Until it lands, the
batch lane reads the verdict trailer of each commit it makes for a branch member and fails the series with
`BATCH_LAND_UNPROVENANCED` unless the verdict is `pass` (unit 10b); nothing is pushed and the goal is ejected, so a
branch goal lands through the single-goal lane (section 6). A chain member is unchanged. The batch lane takes
critic-root attestations only, and Wido asked that this be one verb rather than a rule a seat must remember: unit 10c
also adds `goal branch read`, which gates the unit's tree, dispatches the critic and records the attestation (10.1).
What stays a restriction is what carries safety: the batch lane admits a unit only on an engine-launched review bound
to its bytes. Seats read branch units through the verb when 10c lands. Second, the 13:53 amendment's red record line
also carries the candidate's retry identity (the tree id after the record and the reads are filtered out), and the
retry check compares that stored identity, so the check never reads L's objects and works from any clone. What
changed: sections 5, 10 and 11, unit 10b, and the new section 10.1 and unit 10c. Witness: a branch member whose commit
carries a would-refuse verdict fails its series before the push, and a chain member's does not.

Landing amendment, 2026-09-17 13:53, rulings of the goal's holder (m1c) and the coordinator (m1e) on two conflicts
that the read of units 5 to 7b found in this page. No design round and no critique round. First, where L lives before
its proof: 6.2 pushes the landing branch only after a green receipt, while 6.4, 6.5 and 6.9 name L at a red proof.
`land-prep` builds L locally, in its landing worktree, before the proof; the proof runs on L's project tree; the
leased push of `landing/<goal-id>` happens only after a green receipt, so 6.2 stands, and on red nothing is pushed.
The red record line and the red loop name the local L commit id (40 hex digits, in the landing worktree's object
store) and its tree id, and the red loop runs on the host that built L, so it needs no remote ref. Witness: a red
proof reaches the red loop with L unpushed and origin's landing ref unchanged, and the command that calls the red loop
computes the change set from the prefix's entries and folded paths against E (6.5). Second, the human's word for
`--through` (6.1) is only a `Next step` or history line of the exact form `land through <full commit id>`. Park's
summary line (`goal/G last unit U commit <id> is <state>`) never matches, and no other field of the page is read.
Witness: a page whose `Next step` is park's line refuses `--through`; the same id in a `land through` line lands.
Third, `--last` refuses when the branch carries a unit beyond the land-ready prefix, and no `Goal-Last` is written,
since the verb can see that unit. Witness: an unread u4 past a prefix ending at u3 refuses `--last`. What changed:
6.1, 6.9 and units 7a and 7b. Nothing else moves.

Build-grain amendment, 2026-09-17 12:12, Wido's ruling: one build is one commit, whose `Goal-Unit` joins all of that
build's unit names with `+`; reads, replacement, status and landing identity bind to that same list.

Attribution amendment, 2026-09-17 08:20Z, Wido's ruling: "When it is possible to attribute commits to a human, when a
human is in the loop granting authority, then I think that will always have my preference", for the verbs, not the
hand lane's script. What changed: rule 6.10 (every commit the landing verbs write is authored and committed as the
goal's approving human; the seat rides in a trailer; no human act is added); Publication's attribution sentence;
section 10's BA14a; units 7a and 8. A human ruling: no critique round. Nothing else moves.

Revision 3 amendment, 2026-09-17 07:30Z, Wido's ruling: the landing itself is a branch. Wido asked whether the landing
would be clever enough to compose on a separate branch, so that a member with a serious issue can be taken out and the
landing retried without untangling a merged tree, and ruled yes. What changed: rule 6.9 (the landing branch,
`landing/<goal-id>` or `landing/<batch-id>`, pushed at land-prep, proved at its tip, landed by one fast-forward push,
rebuilt on ejection, deleted on land or dissolve); 6.2 composes on that branch; Publication fast-forwards instead of
committing on the endpoint; section 7 gains the fetchable candidate; section 10's BA12b rebuilds the batch's landing
branch and BA14a lands by fast-forward; units 7a, 8, 9 and 10b. A human ruling: no critique round. Nothing else moves.

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

The branch never carries `landing.WorkspaceExclusions()`: the two append-only registers and the goal store. A
receipt row is appended in each trunk commit of the goal's landing (section 6), as today. The goal store keeps
publishing through `Publish` on the endpoint; that compare-and-swap of coordination state is the standing exception
to "the landing lane alone writes the integration branch", which binds product-workspace bytes. Proof: unit 1.

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
subject, source, verdict, the cheap gate observed on the unit's tree (`scripts/agents/go-gate.sh --fast`: kind,
tree, run id), the carry origin when section 2 wrote it, and a sha256 over the rest. Two sources. `--root-job J`: a
closed code-critic root whose subject is `commit:<id>`; the verb checks the closure with `readsubject` (critic root,
round, `ReturnBindsSubject` on the tree, a clean register) and records them. `--reader-record <path>`: a prose read
under `records/misc/` naming the commit id and the unit digest; the verb records its sha256. A reader-record
attestation is as strong as the record, the hand lane's trust today, unchanged; the batch lane takes critic-root
attestations only, at the join and again at the commit boundary, and `goal branch read` produces them
(section 10, 10.1).

5.1 The cheap gate is the unit's only machine check (revision 3). `scripts/agents/go-gate.sh --fast` (gofmt, vet,
staticcheck, the refusal register, the engine build) runs on the unit's tree and its observation is recorded in the
attestation; the verb refuses `GOAL_READ_UNGATED` when none is recorded for that tree. No deep proof binds to a unit:
the deep suite runs once, at the goal's landing (section 6), or as the optional interim run of 7.2. A unit commit
that modifies an existing test file is named in the attestation's `testsChanged` list with the reader's word on the
change; the verb computes the list from the raw entries and refuses `GOAL_READ_TESTS_UNNAMED` on a mismatch.
Witness: unit 4.

Every consumer re-validates an attestation before trusting it: the path names its goal and commit, the subject
matches the commit, the self-digest matches, the source rules hold. A copy for another commit sits at the wrong path
and names the wrong tree; a stale one names a replaced commit; an edited one fails its digest.

A unit is land-ready when its commit is on origin's branch, its attestation records the cheap gate on its tree, and
a later `Goal-Read` commit carries that attestation, valid, with verdict LAND. A goal's land-ready set is the longest
prefix of its units, in branch order, that are all land-ready. `goal branch status --goal G` prints every commit
with its kind, each unit's digest and read state, and the prefix. This is what a release lane takes from any machine
that can fetch the branch.

5.2 A goal is land-ready when its prefix is every unit of its plan (revision 3). The plan lives in prose, so the
holder's word `--last` says so (6.1) and the whole goal lands as one set. A shorter prefix lands only on the human's
word naming its last unit's commit id on the goal page (6.1); that word is decided here. Whether every landing needs
a human word is goal-landing-needs-a-human-word's question, and one-approval-gate owns whether that word is a second
word. Proof: units 4, 5 and 7a.

## 6. Landing from a branch: the goal lands

Revision 3 replaces revision 2's per-unit trunk landing with one landing act per goal. Slices land on the branch
(sections 2 and 5); the goal lands on the endpoint. Both lanes take the units' entries from their commits, not
files, and land one trunk commit per unit with its receipt row, applied as a diff, under one proof of the series tip.
Since the landing-branch amendment (6.9) the series is composed as a pushed branch and the landing is one fast-forward
of the endpoint to its tip.

6.1 The landing set. A landing takes the goal's land-ready prefix (section 5) as one set. Two words start it, never a
schedule: `--last`, the holder's word that the prefix is every unit of the goal's plan, which also writes `Goal-Last`;
or `--through <commit>`, a shorter prefix, which lands only on the human's word naming that commit on the goal page, a
`Next step` or history line of the exact form `land through <full commit id>` and nothing else, else
`GOAL_LAND_PARTIAL` names the missing word. Before a `--last` landing the holder runs `goal land-ready` (the landing
slot, `docs/backlog-mechanism.md`), since the goal is built; a partial landing runs under the claim without the slot.
Witness: unit 7a.

6.2 One candidate, one proof. `goal branch land-prep --goal G (--last | --through <commit>) --root <checkout> --out
<dir> --test-receipt <path>` fetches the endpoint tip E, starts the landing branch (6.9) at E, and applies the prefix
in branch order as commits on that branch, one per unit in 6.3's shape. The fold range of unit i is the `Goal-Plan`
and `Goal-Read` commits after unit i-1's commit and before unit i+1's, to the branch tip for the last unit; for each
unit the verb applies its folds, then the unit, then its receipt row, and takes that trunk commit's tree; the last
tree is the candidate C. Before writing any file it checks two things. Every unit's entries against its predecessor
tree, folded paths and exclusions removed, hash to that unit's attestation digest; otherwise `GOAL_UNIT_REREAD` names
the unit and the paths and the holder rebases and reads again. The one test receipt (schema 3) names
`ProjectWorkspaceTree(C)` as its tree, or `landing observe` accepts it for C by identity; otherwise
`GOAL_LAND_UNPROVEN`. That receipt is the landing's proof: no intermediate tree of the series is proved, and none is
pushed alone (publication below). A candidate equal to one the landing record (6.4) already holds as red refuses
`GOAL_LAND_RETRY`. The verb then pushes the landing branch under its lease (6.9) and writes into `<dir>` the record
line's draft and a `trunk` file naming E and the landing tip L; the diffs and messages it also writes are the hand
lane's artefacts until unit 8 lands. Witness: unit 7a.

6.3 The shape on the endpoint. One trunk commit per unit, in branch order, diff-applied, with revision 2's trailers
as the landing manifest: `Goal-Unit: G/<unit>`, `Goal-Digest: <unit digest>`, one `Goal-Source: <id>` for the unit
commit and each folded commit, one `Goal-Fold: <path>` per folded path, and `Goal-Last: G` on the last commit of a
`--last` landing only. Every receipt row names the goal, the landing's last unit and the one receipt id; the register
rule in `docs/project-rules.md` wants a row in every commit that changes code, and the hand lane appends them so
today. One squash commit per goal was weighed and rejected: `verify` compares one commit's non-fold entries to one
`Goal-Digest`, and a squash of two units that touch one file merges their entries so no unit digest can be recovered
from the landed object; the sweep's `Goal-Source` check (section 8) and BA14b's recognition (section 10) read
per-commit trailers and stay as they are; per-unit commits change nothing in units 8 and 9, and unit 7 changes from
one unit to a loop over the prefix with the receipt checked once at the tip. Witness: units 7a and 8.

6.4 The landing record. `records/misc/<goal-id>-landing.md` is the landing record. It lives on the branch as a
`Goal-Plan` commit after the units it describes, so it folds into the last unit's trunk commit (6.2's fold range) and
lands with the goal. One line per landing proof: the proof number n, E, C, the landing tip L (6.9), the attempt id,
the verdict, the red groups, the canary run that showed the fix clean (6.5), and the fix commit. `land-prep` writes
the draft line into `<dir>`; the holder commits it before the next proof, the verb from unit 8 on. The goal's `Next
step` carries the count in the same words: "landing G through <unit>: proof n of C red on <groups>, fix <commit>", and
after the push "LANDED G through <unit> after n proofs". Witness: units 7a and 7b.

6.5 Red at the landing. A red proof of C is part of the landing: no new goal, no design round, no critique round, no
brief. The lane classifies first. When a failing group's input manifest (`testing.json`) names a file in the goal's
change set (the prefix's entries and folded paths against E), it is the goal's red. Otherwise the lane runs the
failing groups on E, `test run --purpose diagnostic --no-reuse --mode canary --groups <F> --tree <E> --goal G`,
charged to G: red on E is a trunk red (6.6); green on E is the goal's red, the integration itself. The holder fixes
the goal's red on the branch at once as a `Goal-Unit` commit: a fix that belongs to one unit replaces that unit's
commit (`--amend`, section 2; later units carry or are read again by section 2's rule); a fix of the integration
that belongs to no unit is a new unit `G/land-fix-<n>` at the tip, a unit of the landing set. The fix takes the
cheap gate and a read as any unit: a fix that only changes production code and adds a witness needs the read; a fix
that changes an existing test's assertion needs the read and the reader names the test in `testsChanged` (5.1). The
holder then runs only the red groups plus the fast gate on the branch tip (`test run --purpose diagnostic
--no-reuse --mode canary --groups <F> --tree <tip>`), records that run in the record line, and the lane takes a new
candidate from E and one new deep proof; `land-prep` refuses `GOAL_LAND_UNCHECKED` when the record's last red line
names no clean canary run on the tip it prepares from. Never a retry of the same tree (`GOAL_LAND_RETRY`), never
`t.Skip`, never a raised bound or a lowered floor. Witness: unit 7b.

6.6 Trunk red. A red on E is a trunk red: the lane writes one entry on the red register (`plans/goals/trunk-red.json`,
owned by goal red-on-main-gets-an-owner-on-the-ledger, Wido 2026-09-17 05:52Z) referencing the branch as that
register's rule wants (branch name, the commit it pointed at, open or merged), through that goal's ledger-owner seam,
and the entry gets an owner the same hour under that goal's rule; the landing holds `GOAL_LAND_TRUNK_RED` and writes
nothing on the endpoint; until that seam lands the hand lane writes the entry by hand and the hold stands. When the
fix lands on the endpoint the holder rebases the branch past it (section 2's carry rule applies) and the lane takes
a new candidate from the new E. Only the goal's own change or the integration itself is fixed inside the landing.
Witness: unit 7b.

6.7 Budget. Every landing proof and every diagnostic run of 6.5 is admitted under the goal's claim and charges its
box: attempts and reserved job minutes refuse admission as any proof does; elapsed keeps counting, and in the landing
slot the fence is suspended and past the box the ledger reads `LANDING OVERDUE`, while outside the slot the fence
bites as on any claimed goal. A landing that loops red therefore surfaces on the ledger as a budget refusal on the
goal, never as integration work without a trace; the loop count is n in the record and in `Next step` (6.4).
Witness: unit 7b.

6.8 Rules at the landing. A fix that changes a rule, schema or interface of the design is an amendment on the goal's
own design page, a `Goal-Plan` commit on the branch, folded at the landing by 6.2's fold range; still no new goal.
Witness: unit 7a (the fold range) and unit 1 (code never rides as a plan).

6.9 The landing branch (Wido's ruling, 2026-09-17). The candidate is a branch, not an index: `land-prep` composes the
series of 6.3 as commits on `landing/<goal-id>` starting at E, by applying each unit's entries, its folds and its
receipt row in turn; no merge commit and no squash, so 6.3's per-unit shape, trailers and digests are exactly what
lands. The tip L has tree C. The verb pushes the branch to origin with a lease as goal branches are pushed (section
3): created against an absent ref, replaced against the tip it last pushed; a ref moved behind the lease refuses
`GOAL_LAND_BRANCH_MOVED` and nothing is written. `land-prep` builds L locally before the proof, the proof of 6.2 runs
on L's project tree, and the attempt and the record line name L beside C. The leased push comes only after a green
receipt (6.2), so a proved candidate is a commit any enrolled host can fetch; a red L stays unpushed on the host that
built it, where the red loop (6.5) runs. A new candidate (6.5, 6.6) is a new L: `land-prep` rebuilds the branch from E
and the fixed prefix and replaces it under the lease, never amending in place, and the record keeps one line per L. In
a batch (section 10) the branch is `landing/<batch-id>`, built from the members in join order at the seal, and
ejecting a member rebuilds it from the survivors: the ejected goal's commits are simply not re-applied, which is the
ruling's reason. The branch lives from `land-prep` to the landing or the dissolve: `land-push` deletes it, against L,
after the fast-forward; a landing given up deletes it the same way; a `landing/` branch whose id names no live landing
is listed by the sweep (section 8) and deleted on the same word as an abandoned goal branch. The range rule of section
1 governs goal branches; the landing branch's commits carry receipt rows and are checked by `verify` (6.3, unit 8),
not by `goal branch check`. Reads change nothing: a read binds to the unit commit and its digest (section 4), and 6.2
checks that digest against the re-applied entries as before. Witness: units 7a, 8, 9 and 10b.

6.10 Attribution (Wido's ruling, 2026-09-17). When a human in the loop granted the authority, the landed commits name
that human. Every commit `land-prep` writes on the landing branch has as author and committer the person on the goal's
approval record (the ledger's `Approved.By`, written by `goal approve`; a human word at the landing, once
one-approval-gate records one, takes its place), whose git identity the checkout's configuration names in
`goal.human.<name>` as `Name <email>`, read as `goal.sync-branch` is read. The ambient `user.name` and `user.email`
are never used, so a seat's shell cannot stand in for the approver; a goal whose approver has no identity there
refuses `GOAL_LAND_AUTHOR_UNBOUND` before any commit is written, and no machine identity is a fallback. The machine's
part rides in trailers: `Landed-By: <seat>` names the seat that ran the verb, beside 6.3's trailers, and the unit
commit's `Co-Authored-By` lines are carried over. No human act is added: the approval already happened, and the
landing stays automatic under Wido's ruling of 2026-09-16. The fast-forward of Publication keeps the commit objects,
so the endpoint shows the approver. Nothing tells a human commit from an agent landing by identity: `landing observe`,
the census and the sweep read trailers and the verb path, so a landed commit that names the approver is still an agent
landing to them. In a batch (section 10) each member's unit commits name that member's approver. The hand lane's
`land.sh` is out of scope (Wido: the script is temporary). Witness: units 7a and 8.

Publication. `goal branch land-push --prepared <dir>` fetches the endpoint and the landing branch. If the endpoint tip
is not E it refuses `GOAL_LAND_TRUNK_MOVED` and pushes nothing, and the holder returns to `land-prep` for a new
candidate and its proof; if the landing tip is not L it refuses `GOAL_LAND_BRANCH_MOVED`. Otherwise it fast-forwards
the endpoint to L with `--force-with-lease=<endpoint>:E`, one push, the whole series or nothing, no prefix ever; then
it deletes the landing branch against L. Nothing rebases between the check and the push, so no trunk change reaches
the integration branch unread. The commits that reach the endpoint are the landing branch's, made by the verb under
Wido's ruling of 2026-09-16 that the landing is automatic and attributed by 6.10 to the goal's approver; the human act
at a landing is the holder's word of 6.1, and none is added. Until unit 8 lands the hand lane does the same by hand:
it commits the series on the landing branch from the enrolled terminal, pushes it, proves its tip and fast-forwards
with the lease; `land.sh` changes from a commit on the endpoint to a commit on the landing branch and a leased
fast-forward, and takes E and L from the `trunk` file.

Verification. `goal branch verify --landed <trunk-commit>` reads the trailers, takes the landed commit's entries
against its parent, removes the `Goal-Fold` paths and the exclusions, hashes, and compares with `Goal-Digest`; a
landed goal verifies commit by commit. It needs no object outside the integration branch, so the branch may be
deleted. Proof: units 7a, 7b and 8.

## 7. The testrun lock and the proof

The lock stays per host: it bounds engine runs on one host, and a candidate's tree id is the same everywhere, so a
proof of C anywhere is a proof of C. Two hosts proving one tree waste one run and never disagree; a disagreement is a
host-dependent test, fixed at its cause. The receipt lives on the host that ran it, so the host that proves is the
host that prepares and lands. A second machine needs an enrolled engine (`metasystem up`), the lock library
(`testrun-lock.sh` today, BA8 later) and fetch access to origin; the VM also needs the transport mirror. The landing
branch (6.9) is what a second host fetches: a proof of L's tree anywhere is a proof of C, and the record names L.

7.1 The lock is taken once per goal landing, for the one proof of 6.2, and once per interim run of 7.2; never per
unit (revision 3). Witness: unit 7a takes one receipt for a three-unit series.

7.2 The interim run (revision 3). A seat may take one deep run on the branch tip's project tree (`test run --goal G
--tree <tree>`) when the lock is free, to find a red early on a long goal. It is a precondition of nothing; a red it
finds is fixed on the branch as 6.5, without a landing act, and without a register entry unless E is red; its
receipt is not a landing receipt: the landing takes only a receipt that names C by 6.2's tree check, and when the
interim tree equals C that check accepts it as it accepts any proof of C (this section's first sentence). It is
admitted under the goal's claim and charges its attempts as any proof. Witness: unit 7a (a receipt on a tree that is
not C refuses).

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
the hour of revision 2, `origin/main` carried units 4a (41a545307), 4b (c91c3ac21) and 5a (7b32588a3); 5b0, 5b and
5b2 were staged in the m1c checkout; 5c, 5d and 5e existed as worktrees, diff files and read records on m1c alone.
At the hour of revision 3 (2026-09-17 06:07Z) units 1, 2, 3a, 3b and 3c are landed too; 5b0, 5b, 5b2, 5c, 5d and 5e
are read clean and stacked on m1c as one tree (315baddb7 on 3f978dd1c) whose deep proof admission refused on
elapsed; 5f is building; 5g and units 6 to 12 are unbuilt. The round-5 read of 5b0, 5b and 5b2 names one aggregate
diff for the three, so no per-unit commit can equal what it read.

Rule: no read carries from a diff file to a commit. A read binds to a commit, or it is history.

Rule (revision 3): the first mover lands as a goal, under one proof (section 6). The stacked six do not land per
unit; they land with the goal under `--last`, or earlier as a prefix only on Wido's word naming 5e's commit on the
goal page (6.1), by hand until unit 7a lands.

The move, by the holder, the day unit 3 lands, starting from whatever `git log origin/<endpoint>` then shows
unlanded; until then the hand lane keeps the stack as diffs and lands nothing of this goal per unit:

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
6. The goal lands once: `land-prep --last` over every unit of its plan, one proof of the series tip, the record of
   6.4 folded in; a red at that landing follows 6.5 to 6.7.

## 10. What changes in brief B

Brief B stands; its member is a goal's landing set (the land-ready prefix of section 5 taken whole, revision 3), not a
unit, so one batch proof covers several goals landing together. A member's units are one closed set of branch commits
standing where a chain stands, beside chain members in the same batch; the commit boundary admits them by their
attestations (10.1), and these rows take it:

- BA2 gains a branch reader as row BA2b: `landing batch join --goal G (--last | --through <commit>)` fetches
  `goal/G`, validates the range, requires for every unit of the prefix a critic-root attestation with verdict LAND
  that records the cheap gate, and takes as certified output the prefix in branch order, each unit's entries plus
  its folded plan and read commits in 6.2's fold range. A reader-record attestation on any unit refuses
  `BATCH_JOIN_UNREAD`, as a unit outside the job domain does today; the closed-chain gate is not weakened. One member
  per goal: BA5a's cardinality holds as written, and a second member of a goal refuses `BATCH_GOAL_ELSEWHERE`.
- BA4 compares, inside a member, each unit's own transition `T(i-1)..Ti`, that unit's folded paths removed, to that
  unit's digest, else `BATCH_JOIN_REREAD`; a member's units are contiguous in join order and the cumulative prefix
  tree stays for receipts.
- BA12b's ejection names the member: the goal whose unit's files the failing group names is ejected whole with the
  failure attached to its `Next step` by BA6b's return; rule 6.5's fix loop runs on that goal's branch while the
  survivors are proved as a new tree, and the goal rejoins as a new member with a new candidate. The batch's landing
  branch `landing/<batch-id>` (6.9) is rebuilt from the survivors in join order and replaced under its lease; the
  ejected member's commits are not re-applied. A red that names no
  member is the trunk red of BA12a and 6.6.
- The seal composes the batch as `landing/<batch-id>` (6.9), one trunk commit per unit of each member in join order
  with 6.3's trailers and receipt rows, each commit naming its member's approver (6.10), and pushes it before the
  proof, whose attempt names the tip; BA13 receipts each unit commit of a member by identity from the one tip proof,
  as it receipts prefixes today; BA14a lands by one fast-forward of the endpoint to that tip under the lease, the
  whole batch or nothing, and deletes the branch; BA1's record, BA6b's `Next` edit, BA13's receipt identity and
  BA14a's message carry the member's commit ids and its last unit; BA14b recognises each landed unit by `Goal-Source`
  and finishes a member once, when every one of its units has its trailer on origin; BA5b's handover precedes any
  sweep and changes no field; BA15's status and moved-effects inventory show branch, prefix and last unit.

Every named row re-estimates; a row passing 300 splits at its witness boundary. Brief B's DONE (1) to (5) stand,
with "unit" read as the member.

10.1 The commit boundary for a branch member (unit 10c). Every batch commit goes through `scripts/agents/commit.sh`
(brief B, R15, BA14a). A branch member has no implementer chain, so its commits carry a fourth declaration,
`--attested <unit commit> --attested-snapshot <branch tip> --attested-base <endpoint tip>` (names as the build lands
them), decided in `landing observe` beside `--chain`, `--direct-fix` and `--carried`; `commit.sh` forwards it and
decides nothing. The verb refuses when: the id is not a commit in the repository; the attestation at
`metasystem/records/reads/G/<commit>.json` in the snapshot fails section 5's checks (canonical form, self-digest,
goal, unit, verdict LAND, the subject recomputed from the commit, the fold range, the fast-gate observation on the
unit's tree, the test-change rule), run by the same function the join runs; the source is not a critic root whose
closure `readsubject` re-reads from the landing root's job store and binds to `commit:<id>` at the recorded round;
`--goal` is not the attestation's goal; the candidate change from HEAD, less the folded paths, the receipt ledger and
the reads directory, does not hash to the attested unit digest, or a folded path does not hash to its recorded fold
digest; the change reaches a destructive path and the member's fold range carries no `Goal-Plan` commit; or the checks
every declaration meets refuse (path classes, the held goal at its revision, the receipt ledger row, the test
receipt's series rule). It passes as `Landing-Provenance: attested=<commit> goal=G unit=<unit> critic=<root>/<round>
change=<digest>` with `Landing-Provenance-Verdict: pass` under its own bar and the `Goal-Revision` trailer. A
reader-record attestation refuses. `--attested` beside `--chain` or `--direct-fix` refuses `conflicting-declarations`.
The batch lane reads the verdict trailer of every commit it makes for a branch member and treats anything but `pass`
as a landing failure before the push (unit 10b), so a would-refuse under the boundary's observe mode never reaches
trunk. The closed-chain gate is not touched: a chain member still needs its chain, and a branch member still needs a
critic-root attestation per unit at the join. A branch member and chain members join the same batch in join order.
What the boundary no longer asks of a branch member is an implementer job record; the critic's closure over the commit
and the holder's pushed branch stand in its place (section 11). A seat gets that attestation from one verb, `goal
branch read --goal G --unit <commit>` (name as the build lands it). It runs `scripts/agents/go-gate.sh --fast` on the
unit's tree when no observation of that tree is recorded, dispatches a code-critic root on `commit:<id>` through
`delegate` with a brief the verb writes, and returns at once with the job id; it never waits on a model. Once the root
closes, the same verb with `--collect` writes the attestation through `goal branch commit --kind read --root-job` with
the recorded gate run; a root still open writes nothing and says so, and a root whose register is not clean refuses,
so the unit is fixed and read again. A seat never chooses a source. `--reader-record` stays for the hand lane only,
where a person lands on their own authority; the join and the boundary refuse its attestations. Witness: unit 10c.

## 11. What changes in existing behaviour

Under R-115-m1e: a reader names `commit:<id>` and an attestation instead of a diff file and its sha256; a landing
checks the digest before the human commit and `verify` checks it after; `land.sh` resets to the prepared trunk
commit, does not rebase, and pushes the series once with a lease; a seat pushes `goal/<goal-id>` refs to origin and,
through the verb, to transport; `goal park` requires a pushed branch only when branch work exists. Revision 3: one
landing per goal. The hand lane's per-unit deep proof ends when unit 7a lands; until then the hand lane already
lands a goal's whole land-ready prefix under one proof, as lane 12 did for five units under proof-mu53bjy2 on
2026-09-17, and a shorter prefix only on the human's word (6.1); a red at a landing is fixed inside the landing (6.5
to 6.7), by hand under the same rules until unit 7b lands; the lock is taken once per landing (7.1). Untouched: the
receipt rule (one row per trunk commit), `ledger-preserve.sh`, `sync-transport.sh` for `main`, the lock, the goal
store and its publication, the ledgers, brief B's DONE, integration-branch's boundary. `commit.sh` gains one forwarded
flag and no decision, and `goal branch` gains `read` (10.1). human-carried-landing's carry can name a branch commit;
no conflict. No question is open for Wido: he answered yes to 10.1 on 2026-09-17.

## Proof of DONE

1. Every unit built after unit 3 lands is a `Goal-Unit` commit on `origin` `goal/<goal-id>` before its read starts;
   `goal branch status` and `git ls-remote` agree.
2. `origin` `goal/fixture-children-cannot-outlive-their-test` carries every unit of that goal that was not on the
   endpoint at the move, each with a fresh attestation, and the plan commit; a fresh clone on a second enrolled
   machine prints the same `goal branch status` prefix.
3. `goal branch check` passes on every `goal/*` range on origin; unit 1's disguise witnesses refuse.
4. `goal branch verify --landed` reports equal for every trunk commit of every goal landed from a branch, and that
   goal's landing record names one test receipt for its series tip; in a fixture, a one-byte mutation fails it and
   a trunk moved after preparation refuses before any push.
5. `landing batch join --goal G --last` takes a goal's landing set from its branch once brief B's BA2 and BA4 exist;
   production availability stays brief B's rule.
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
| 4. Attestation and carry. | `attest.go`, `commit.go` | 2, 5 | A critic-root source validates; a reader-record source records the sha256; a copy for another commit refuses; an edited file fails its digest; no fast-gate observation on the unit's tree refuses `GOAL_READ_UNGATED`; a commit that modifies an existing test file with no `testsChanged` naming it refuses `GOAL_READ_TESTS_UNNAMED`; a rebase over an untouched file writes a carry naming both commits; a rebase over a touched file or a changed fold refuses `GOAL_READ_STALE`. | 300 |
| 5. Status, land-ready, park. | `status.go`, `internal/goal/verbs.go`, `cmd/metasystem/goal.go` | 4, 5 | Prefix of one on clean, unread, clean; a goal with no branch parks; a goal with a branch and an unpushed tip refuses; `Next step` names the last unit and its commit. | 240 |
| 6. Transport mirror and leased delete. | `mirror.go`, `cmd/metasystem/goal_branch.go` | 1, 8 | After an amend transport equals origin; a transport tip moved behind its lease refuses; a delete against a moved oid refuses and the ref stays. | 180 |
| 7a. Goal landing preparation. | `land.go`, `cmd/metasystem/goal_branch.go`, `internal/refusal/register.go` | 6.1 to 6.4, 6.8, 7.1, 7.2 | Over a three-unit prefix: every unit's non-fold entries hash to its digest; a trunk change in a unit file refuses before any file is written; one receipt naming C suffices for the series, and a receipt naming an intermediate tree or an interim tree that is not C refuses `GOAL_LAND_UNPROVEN`; each message carries every trailer and only the last carries `Goal-Last`, under `--last` only; `--through` a shorter prefix without the human's word on the goal page refuses `GOAL_LAND_PARTIAL`; a `Goal-Plan` commit after the last unit folds into the last trunk commit; the record draft names the proof number, E, C, L and the attempt; a candidate equal to a recorded red one refuses `GOAL_LAND_RETRY`; the series is pushed as `landing/G` with its tip tree equal to C, a second `land-prep` replaces it under the lease, and a ref moved behind the lease refuses `GOAL_LAND_BRANCH_MOVED` with nothing written; every landing-branch commit's author and committer equal the `goal.human.<name>` identity of the goal's `Approved.By` with a `Landed-By` trailer naming the seat, an ambient `user.email` set to another person changes nothing, and an approver without an identity refuses `GOAL_LAND_AUTHOR_UNBOUND` before any commit (6.10). | 370 |
| 7b. The landing red loop. After 7a. | `red.go`, `cmd/metasystem/goal_branch.go`, `internal/refusal/register.go` | 6.5 to 6.7 | With a fake runner and a fake ledger-owner seam: a failing group whose manifest names a change-set file is the goal's red and no run on E happens; a group outside the change set runs on E first, and red on E writes one register entry referencing the branch and holds `GOAL_LAND_TRUNK_RED` with nothing written on the endpoint; green on E is the goal's red; the next `land-prep` refuses `GOAL_LAND_UNCHECKED` until the record's red line names a clean canary run on the tip; the record gains one line per proof and `Next step` carries the count; every run is admitted under the goal's claim, so an exhausted attempt box refuses before any runner. | 280 |
| 8. Landing publication and verify. | `publish.go`, `verify.go`, `internal/refusal/register.go`, `/Users/wido/LocalStorage/hact-20260912/land.sh` | 6, 6.9, 6.10 | A moved endpoint refuses before any push; a moved landing tip refuses `GOAL_LAND_BRANCH_MOVED`; the leased fast-forward lands the whole series on E or nothing and deletes the landing branch against L; `verify` reports equal for each commit of a landed series; a one-byte mutation in a fixture's landed commit fails it; every commit on the endpoint after the fast-forward names the goal's approver as author and committer. | 280 |
| 9. Sweep and conclusion. | `sweep.go`, `internal/goal/verbs.go`, `cmd/metasystem/goal.go` | 8 | A plan-only tail refuses; an unknown commit refuses; a tip advanced between check and delete refuses; a `Goal-Last` landing sweeps; `goal done` sweeps; a parked goal without a branch is listed; an abandoned branch survives a plain sweep; a `landing/` branch of no live landing is listed and deleted only on the word (6.9). | 300 |
| 10a. Batch reader and per-member check. After BA2 and BA4. | `internal/landing/batch/branch.go`, `join.go` | 10 | A fixture branch of three units with critic-root attestations joins as one member and certifies each unit's bytes; a reader-record attestation on one unit refuses `BATCH_JOIN_UNREAD` for the member; a two-goal batch passes with each unit compared to its own transition inside its member; a moved trunk in a unit file refuses `BATCH_JOIN_REREAD`; a second member of one goal refuses `BATCH_GOAL_ELSEWHERE`. | 290 |
| 10b. Batch identity rows and goal ejection. After 10a, BA12b, BA14b and BA15. | `record.go`, `return.go`, `red.go`, `receipts.go`, `land_apply.go`, `recovery.go`, `cmd/metasystem/landing_batch_verbs.go`, the inventory | 10 | Record and receipt carry the member's commit ids and last unit; `Next` names the last unit; a failing group that names one member's unit ejects that goal whole with the failure on its `Next step`, the survivors form a new tree and `landing/<batch-id>` is rebuilt from them under its lease (6.9); recovery finds each landed unit by `Goal-Source` and finishes the member once; `validate moved-effects` reports zero problems. | 320 |
| 10c. Commit boundary for a branch member. After 10b; built after this goal merges. | `internal/landing/observe.go`, `internal/landing/attested.go`, `attest.go`, `cmd/metasystem/landing_verbs.go`, `internal/landing/batch/transport.go`, `internal/landing/batch/join.go`, `cmd/metasystem/landing_batch_land.go`, `internal/landing/batch/recovery.go`, `internal/goal/attention.go`, `internal/refusal/register.go`, `scripts/agents/commit.sh`, `scripts/agents/landing-promotion.json`, `cmd/metasystem/goal_branch.go`, `internal/goal/branch/read.go`, `docs/project-rules.md` | 10.1 | `landing observe --attested` on a fixture unit commit with a critic-root attestation passes with a `pass` verdict and a goal revision, and `commit.sh` lands it with its trailers; a reader-record source, a critic root missing from the job store or a round off by one, an extra changed file or a changed fold byte, a held goal at a moved revision, and a destructive change without a `Goal-Plan` fold each refuse and nothing is committed; `--attested` with `--chain` refuses; `CommitBuild` passes the attested declaration and never a critic root or a branch tip as `--chain`; a branch member joins beside a chain member and both land; recovery finds a branch member's commits by `Goal-Source`; the census reads the new provenance line; `goal branch read` on a fixture unit runs the fast gate once for a tree, dispatches one code-critic root on `commit:<id>` through a fake delegate and returns without waiting, and its `--collect` writes a critic-root attestation that `landing observe --attested` passes when the root closed clean, writes nothing while the root is open, and refuses a root with open findings. | 550 (the read verb about 250), witnesses about 320 more |
| 11. First mover. After unit 3. | `records/misc/fixture-children-branch-move.md` | 9 | Proof of DONE 2. | 60, records only |

### Builds (process reset, 2026-09-17)

Unit 1 builds alone (in flight when the reset was made). Build A = units 2, 3 and 4. Build B =
units 5, 6, 7a and 7b. Build C = units 8, 9, 10a, 10b and 11, after batch build C (BA12a,
BA12b, BA17, BA13). One builder per build, from this page and nothing else; one independent read
per build; the goal lands under one deep proof. The reasons and the numbers:
records/misc/delivery-process-reset-2026-09-17.md. The landing-branch amendment (6.9) touches units 7a, 8, 9 and
10b only; Build A is unchanged.

### Critique round 2 dispositions (revision 3, 2026-09-17)

The round returned ten findings; under the reset rule (one round, material = cannot be built as
written or builds the wrong behaviour) they disposition as follows, and no fold round runs.

- M-01, process: the critic read a revision-3 copy in its worktree; this page is the canonical
  one and carries revision 3.
- M-02, build B: the landing record gains the red that opens the loop and the final green
  (section 6.5 to 6.7); the builder adds the fields to the record and its schema.
- M-03, known defect: the component retry fence is goal proof-admission's (priority 1, sequence
  4). Build B proves the red loop against a fake runner; the live loop needs that goal.
- M-04, another goal: the trunk-red register's branch reference (name, commit, open or merged)
  is carried by red-on-main-gets-an-owner-on-the-ledger; build B reads it and does not define it.
- M-05 and M-06, build C with batch build C: the member schema, the closed-chain gate and BA13's
  receipts move to goal grain together (units 10a, 10b, BA13).
- M-07, build B: the optional interim proof never counts as the landing proof; the lock rule
  gets its witness in 7a.
- M-08, M-09, M-10, builder: witness bookkeeping. The builder proves each rule by mutation and
  returns the witness names; unit 4's two refusals are in build A's scope.
- Unit 1 correction (m1c, 2026-09-17 06:50Z): a branch may lag the endpoint. The range is
  `merge-base(E, tip)..tip`, first-parent, and the check is that the merge-base lies on the
  endpoint's history, never that the tip descends from the endpoint tip; unit 1's brief said
  otherwise and build A corrects `ValidateRange` and the verb.
