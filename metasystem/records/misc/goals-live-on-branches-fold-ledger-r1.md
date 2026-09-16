# Fold ledger: goals-live-on-branches, critique round 1 into revision 2

Page: `goals-live-on-branches-design-rev2.md` (same directory). Every finding was checked against the code or page
it cites before the fix was chosen. Section numbers below are the revision 2 page's.

F1: fixed in sections 1 and 3 (unit 1): one range check, `branch.ValidateRange`, run by every verb and by a
read-only `goal branch check`: linear history, exactly one kind trailer per commit, and disjoint path classes
(`Goal-Unit` product paths only; `Goal-Plan` only `plans/` outside the goal store and `records/` outside
`records/reads/`; `Goal-Read` exactly one attestation file plus at most one named prose record). Code disguised as
a plan or a read, and a unit touching `plans/`, `records/` or `WorkspaceExclusions()`, refuse `GOAL_BRANCH_RANGE`.
Checked: `docs/project-rules.md` has no goal-branch rule and revision 1 relied on prose alone.

F2: fixed in section 5 (unit 4): the read is a machine-written JSON attestation at
`records/reads/<goal-id>/<unit-commit-id>.json` whose subject (commit, parent, tree, unit digest) the verb computes
from the repository; source is either a closed code-critic root checked through `readsubject` (`Closure`,
`ReturnBindsSubject`, clean register) or a prose reader record bound by sha256; self-digest; every consumer
re-validates path, subject, digest and source. A reader-record source is named as exactly today's hand-lane trust;
the batch lane refuses it (section 10). Checked: `internal/readsubject/closure.go` gives the closure fields;
`records/reads/` does not exist yet, so the path is free.

F3: fixed in section 6 (unit 7): `land-prep` takes `--test-receipt` and refuses `GOAL_LAND_UNPROVEN` unless a
schema-3 receipt names `ProjectWorkspaceTree(C)` (the `exactCandidate` rule at `internal/landing/receipt.go:666`)
or `landing observe` admits the candidate by identity; the receipt row is derived from it. Checked: revision 1 named
only cheap gates and left the deep proof to the hand lane's habit.

F4: fixed in section 6 (unit 8) and section 11: the landing owns the trunk commit E it prepared from (`trunk`
file); the human commits on E; `land-push` refuses `GOAL_LAND_TRUNK_MOVED` when the endpoint moved, else pushes
`--force-with-lease=<endpoint>:E`; no rebase between check and push. Section 11 now lists the three `land.sh` line
changes (reset target, dropped rebase, leased push) and the hand lane's interim. Checked:
`/Users/wido/LocalStorage/hact-20260912/land.sh` lines 17, 33, 34 (`reset --hard origin/main`, `rebase
origin/main` after the preflight, `push HEAD:main`).

F5: fixed in section 2 (unit 4), one cause with F6 and F13: a rebase never reuses a read; `--carry` writes a new
attestation naming both commits and both trees, and only when the unit digest and every earlier fold commit's
digest are unchanged and the old attestation is clean; otherwise `GOAL_READ_STALE`. With `--full-index` a trunk
change to a touched file changes the pre-image blob, so the base moving under the unit forces a new read.
Checked: `internal/readsubject/subject.go` `Equal` for a commit subject binds Commit, Tree and DiffDigest, so a
digest-only carry was not the existing identity.

F6: fixed in section 2 (unit 1): the digest is sha256 over raw `git diff-tree -r -z --no-renames --full-index`
entries, not a patch. Checked on this repository (c91c3ac21..7b32588a3): the raw-entry hash is identical under
default config and under `-c diff.renames=true -c diff.algorithm=patience -c diff.noprefix=true -c
diff.mnemonicPrefix=true -c core.quotePath=false`, while `git diff --binary --full-index` gives two different
hashes under `-c diff.algorithm=patience -c diff.noprefix=true`. `gitRawOutput` in `internal/dispatch/gitcmd.go`
pins no config, so `commitReadSubject`'s patch digest is machine-dependent; it is kept for its present callers and
the attestation carries the unit digest beside it.

F7: fixed in section 6 (units 7 and 8), one cause with F8, F9 and F10: the landed commit carries `Goal-Unit`,
`Goal-Digest`, one `Goal-Source` per unit and folded commit, `Goal-Fold` per folded path and `Goal-Last`; `verify
--landed` recomputes from the trunk commit and its parent alone, so branch deletion destroys nothing verify needs.

F8: fixed in section 1 (unit 3) and section 8 (unit 6): no local pushed-tip ref is an authority. Each push fetches
origin's oid O, checks the claim, records `O -> N` in a txn ref, pushes with lease O, classifies the outcome as
`goal.PublishCAS` does (`internal/goal/txn.go` lines 346-375), reconciles an unknown outcome by re-fetch, and
re-reads the claim after the push (`GOAL_BRANCH_CLAIM_LOST`). The sweep and the transport delete lease against the
oid just observed. A new holder adopts origin's tip after `ValidateRange`. Checked: revision 1's
`refs/metasystem/goals/pushed/<goal-id>` was local-only and had no crash or race story.

F9: fixed in section 1 (unit 6) and section 11: the mirror is the verb `goal branch push --transport`, which
fetches transport's oid T and pushes origin's fetched oid with `--force-with-lease=...:T`; `sync-transport.sh` is
left serving `main` and is named as unable to mirror a rewrite. Checked: `scripts/agents/sync-transport.sh` line
35 pushes `refs/remotes/origin/$branch:refs/heads/$branch` without force or lease.

F10: fixed in section 8 (unit 9): the sweep matches every branch-only commit of every kind (unit, plan, read) to a
`Goal-Source` trailer on the endpoint or to a dropped line in the conclusion's `Next step`, else
`GOAL_SWEEP_UNLANDED`; "last unit" is never inferred from the tip: `land-prep --last` writes `Goal-Last` on the
holder's word, and only that landing or `goal done` sweeps. Checked: revision 1 deleted when every `Goal-Unit`
had a source, which orphaned plan and read commits and guessed the last unit.

F11: fixed in section 9 (unit 11), rejected in part: the claim that `fcu5b-r5.diff` and the `wt-fcu5b` worktree do
not exist is false; the files exist in the seat's scratchpad and their sha256 values match the read records
(`fcu5b-r5.diff` = 129492488e53...7032, `fcu5c-r5.diff` = fa3cb9a67bf5...93b7, `fcu5d-r1b.diff` =
cf7c633996ed...8687), and the worktrees are present at b9a97d465. The substantive point stands and is folded: the
round-5 read of 5b0/5b/5b2 named one aggregate diff, so no per-unit commit can equal what it read; the page now
rules that no read carries from a diff file to a commit, every moved unit gets a fresh read bound to its commit,
the earlier reads become history in `records/misc/fixture-children-branch-move.md`, and the move starts from
whatever `git log origin/<endpoint>` shows unlanded on the day unit 3 lands (4a, 4b, 5a on main; 5b0/5b/5b2
landing from the m1c index; 5c, 5d, 5e on m1c alone). Counted as fixed.

F12: fixed in section 10 (units 10a, 10b): BA2b requires a critic-root attestation (closed chain, closed critic
root) and refuses `BATCH_JOIN_UNREAD` for a reader-record attestation, so the closed-chain gate is not bypassed;
the affected rows are enumerated: BA1, BA2b, BA4, BA5b, BA6b, BA13, BA14a, BA14b, BA15. Checked against
`plans/units-land-in-batches-under-one-proof-brief-b.md` sections 3 and 4.

F13: fixed in section 10 (unit 10a): BA4 compares each member's own transition `T(i-1)..Ti`, folded paths removed,
to that member's digest (`BATCH_JOIN_REREAD`), not the cumulative prefix against one unit's digest. Checked: brief
B section 4 defines the prefix trees T1..Tk cumulatively.

F14: fixed in sections 1, 3 and 11 (units 1, 2): every comparison uses the endpoint `goal.ResolveEndpoint` names
(`internal/goal/txn.go` lines 49-63, default `refs/heads/main`); the hand lane refuses
`GOAL_BRANCH_ENDPOINT_UNSUPPORTED` when the endpoint is not main until integration-branch moves `land.sh`; the
"landing lane alone writes the integration branch" ruling is scoped to product-workspace bytes, and the goal
store's `Publish`/`PublishCAS` on the exclusion paths is named as the standing exception, so the page no longer
contradicts the goal store.

F15: fixed in section 4 (unit 5): `goal park` requires a pushed, equal branch only when `goal/<goal-id>` exists on
origin or `Next step` names a unit commit; a goal with no branch parks as today; the check sits in `Park()` before
`Publish` (`internal/goal/verbs.go` line 2111, whose `parkRequest` has no branch condition today). Section 11
lists the change.

F16: fixed in the Units section: twelve units in dependency order, each at most 300 lines with the files it
touches: digest and range rule (1); commit verb with an unavailable marker (2); push with lease and reconcile,
which supplies the marker (3); attestation and carry (4); status, land-ready and park, naming `verbs.go` and
`cmd/metasystem/goal.go` (5); transport mirror and leased delete (6); land-prep with proof admission (7);
land-push, verify and the `land.sh` change (8); sweep and `goal done` hook, naming `verbs.go` and `goal.go` (9);
BA2b and BA4 (10a); BA1/BA6b/BA13/BA14a/BA14b/BA15 owners (10b); first-mover records (11). Revision 1's unit 4
(fold, digest, trailers, verify, land.sh in 300) is split over 7 and 8; its unit 1 ended before any push, so
unit 2 now refuses until unit 3's marker exists.

Short notes:
- "plus the receipt ledger": dropped; `landing.WorkspaceExclusions()` already covers both registers and the goal
  store (`internal/landing/registers.go`). Fixed in section 3.
- `git diff` vs `diff-tree -p`: resolved by naming `git diff-tree -r -z --no-renames --full-index` raw entries
  once, in section 2.
- Section 9 step 1 stale: section 9 rewritten from `git log origin/main` at the time of writing, and told to start
  from whatever is unlanded when unit 3 lands.
- Prose cannot substitute for validators: every push and land rule now names its check and refusal code and the
  unit that carries it, with the hand lane's interim stated (sections 1, 4, 6, 8).

Left unchanged (would not change what gets built): revision 1's cherry-pick-versus-apply answer, the lock section
except one sentence tying the proving host to the landing host, the "no store field" ruling, and the
one-approval-gate and human-carried-landing conflict statements. Not folded: a fourth commit kind for carried reads
(a carry is a `Goal-Read` with a `carriedFrom` field, which the validator already covers), and a rollout plan for
a second machine (section 7 still leaves it to that machine's rollout). Length: revision 2 is about 3,550 words
against revision 1's 2,500 (`wc -w`), carried by the attestation, push and trailer rules and five more unit rows;
a further cut would remove refusal names or witnesses.
