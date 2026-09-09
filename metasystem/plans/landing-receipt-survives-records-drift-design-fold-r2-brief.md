Working Mode: design
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, coordinator under goal landing-receipt-survives-records-drift)
Date: 2026-09-09

# Fold brief: revision 3 of the receipt-survives-drift design — delete the rare case

## Your authority to author this revision

You are dispatched to REVISE a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1b and R-25 put design authoring on this lane. Author the
revision; do not stop to report an authority conflict.

## What you are revising, and the shape of this fold

`metasystem/plans/landing-receipt-survives-records-drift-design.md`,
revision 2, landed at f4abc699. Revise IN PLACE to revision 3 with a
revision record. The read is
`metasystem/records/misc/landing-receipt-survives-records-drift-critique-r2.md`;
the coordinator's disposition there is binding. This fold is a DELETION
plus four bounded specifications. Revision 3 must be shorter than revision
2. It goes to the build without a further design read, so every sentence
that remains must be one an implementer can build from and a code critic
can verify.

## LRE-01, critical: delete the rare case

Decision 3e steps F and G are deleted, with the capture directory, the
manifest, the link-a-complete-file step, the exported `narratordigest.Lock`,
the window table, the LOST rows, the restore self-check, the crash-resume
paragraph, the concurrent-writer fixtures that exist only for them, and the
probes that only they needed. Step D's classification stays; its outcome for
a register in the overlap becomes one refusal, `advance-register-contended`,
naming the register and N. The bytes stay exactly where they are in the
worktree. The repair route, stated on the page: the seat lands them with
the register-carriage landing that already exists
(`metasystem/internal/landing/observe.go:647-677` accepts record-only
carriage relative to current HEAD) on top of the local landing commit, then
runs `landing advance` again, which now takes the common path. Say why
re-emission was never an option: `metasystem/internal/retrodebt/debt.go`
lines 174-212 and `metasystem/internal/narratordigest/digest.go` lines
247-313 bind exact prefixes.

## LRE-08 and LRE-04: serialize advance and close the stage-after-check race

Today the checkout mutation lock is held only while commit.sh's wrapper
child runs (`metasystem/internal/lease/verbs.go` lines 462-481); land.sh's
post-commit check, fetch, advance and push run after it is released. Two
advances can start at the same HEAD, and a stage on a path N does not change
can land between classification D and `git reset --keep N`, be reset away
(git documents that `--keep` resets index entries), and the verb reports
success.

Specify: advance takes the checkout mutation lock for its whole run, from
precondition A through the reset, using the lock the lease package already
owns (name the function and what it returns); and the branch move is
guarded by a compare-and-swap, HEAD must still equal L at the moment of the
reset, refusing `advance-head-moved` otherwise. Say what the second
concurrent advance sees (a lock refusal with the holder named) and what
the seat does.

## LRE-05: name the gittree operations

`metasystem/internal/gittree/gittree.go` lines 104-154 and
`metasystem/internal/gittree/snapshotscope.go` line 41 keep `git`,
`gitTop`, `gitAt` and `gitProbe` unexported, and the package exports no
rebase, abort, reset-keep or ancestor operation. Name the exported
operations advance needs, each with its typed outcome: an ancestor probe;
rebase and abort inside `NewDetachedCommitWorktree`; `reset --keep` at the
toplevel. Every one goes through the bounded wrapper; an implementer must
never reach for `os/exec` from package landing. Put them in the
implementation map.

## LRE-07: the refusal register

Every advance refusal code the page keeps (`advance-not-on-branch`,
`advance-index-not-empty`, `advance-rebase-conflict`,
`advance-unstaged-drift`, `advance-register-removed`,
`advance-register-contended`, `advance-head-moved`, and the lock refusal)
gets a row in `metasystem/internal/refusal/register.go` with its owner,
site, shape and repair route, or is excluded there with a reason. The
register's test (`metasystem/internal/refusal/register_test.go` lines
16-47) collects only certain suffixes automatically, so these rows are
written by hand; say so in the implementation map.

## LRE-06: one rationale

The post-commit drift check protects the checkout's immediate posture after
commit, index and worktree, not transport (`sync-transport.sh` pushes refs
only) and not the proof-to-commit equation (commit.sh proves that itself).
Say exactly that.

## What must not change

Decisions 1 and 2 as landed. Decision 4 as landed in revision 2 (the read
verified the compatibility rule and both mixed-form pins). The drift verb's
two per-mode rule tables. The common path E and the private detached
worktree of step C. The sentinel-stash canary stays, because it proves the
stash list is never touched.

## Constraints

Wall-clock budget: 35 minutes. Design only; no code, no bed. Shorter than
revision 2, or say why a sentence had to stay.
