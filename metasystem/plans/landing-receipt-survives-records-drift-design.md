# Design: a landing receipt that survives an append to a register

Goal: `plans/goals/landing-receipt-survives-records-drift.md`.
Revision 2. Revision read: c77f117b (branch agent/lrsrd-design5). Every
line number this revision adds or leans on (the writers, land.sh,
commit.sh, dispatch.sh, go-build.sh, go-gate.sh, sync-transport.sh,
gittree, the carriage rule, the fixture bed) was re-read at that revision;
the citations carried over from revision 1 (d533caf1) that this revision
does not touch (`receipt_test.go`, `gittree_test.go`, `wall.go`,
`policy.go`, `policy_test.go`, `static-reproof-fixtures.sh`,
`path-classes.txt`, `observe.go:259` and `:284-286`) were not re-read, and
no file between the two revisions changed them according to `git diff
--stat d533caf1..c77f117b`. Paths are relative to `metasystem/`. The
revision record at the end names each finding of the round-1 read and
what moved.

## The four decisions

1. **One declaration.** The register set lives in package `landing`, in a
   new file `internal/landing/registers.go`, as `appendOnlyRegisters`. The
   carriage classifier (`observe.go:779`), the receipt's posture reads, the
   drift verb and the advance verb all read that slice. Two pin tests keep
   the neighbours that also name these files in step: the writers' state
   roots and the behavior-surface LANDING projection.
2. **Filter the working-tree projection only.** At every one of the
   receipt's posture reads the projection is `FilterTree` over
   `Snapshot("HEAD")`, compared against the equally filtered candidate
   tree. Index trees, the candidate tree, and the commit tree stay exact.
   The orchestrator's reading holds; this design refutes none of it.
3. **Tolerate register appends, never carry them, never stash them.** Both
   land.sh cleanliness sites call `metasystem landing drift`, which
   tolerates exactly an unstaged append to a declared register and nothing
   else. The rebase runs in a private detached worktree, and the real
   checkout advances to the rebased commit with `git reset --keep`, which
   never touches a register that origin did not change. When origin did
   change one, a Go verb restores the local append by a prefix-verified
   suffix append, with the captured bytes held in a per-landing directory
   under `artifacts/agents/landing/`, not in the stash. The narrator
   digest joins the behavior-surface coordination set so commit.sh's
   LANDING comparison stops refusing on it, the way it already does not
   refuse on the receipts log. Staging the drift as append-only carriage is
   rejected because it moves the candidate tree out from under the receipt.
4. **Receipt schema 2, with a version-1 read.** The receipt gains a
   `worktreeProjection` object naming the excluded paths and the filtered
   candidate identity that the two worktree bindings must equal. The reader
   selects its comparison by the version field: version 2 under the
   projection rule, version 1 under version 1's own four-exact-bindings
   rule. That is the cutover rule for the landing that lands this change.

## The mechanism as read, with two corrections to the brief

**The carriage rule.** `observe.go:773-784` switches on the changed path.
`:774` handles `memory/rulings.md`; `:779` names `memory/receipts.log` and
`records/narrator-digest.log` and refuses anything but an append with
`register-carriage-not-append-only` through `appendOnly` (`:949-984`).
`appendOnly` is a byte rule: the base blob must be a prefix of the
candidate blob (`:980`), both newline-terminated (`:971`, `:977`). Nothing
else in the package consults those two names.

**The posture reads.** `receipt.go:248-258` `receiptPosture` returns
`workspace.StagedTree()` (real index, reconstructed from `ls-files
--stage`, `snapshotscope.go:246-257`) and `workspace.Snapshot("HEAD")`
(`gittree.go:255-277`: `read-tree HEAD` into an isolated index, `git add
-A -- .` from the workspace, `write-tree`, then the workspace subtree).
Every tracked file's working bytes enter the projection, the two registers
included.

**Where the reads happen.** `CreateTestReceipt` (`receipt.go:55-170`):

- `:77-83` reads the REAL workspace before the run and refuses unless both
  postures equal the supplied tree.
- `:103-109` reads the ISOLATED candidate (a detached worktree at the tree,
  `:88`) and refuses unless both postures equal the tree.
- `:129-135` reads the ISOLATED candidate again after the command and
  refuses with "the candidate changed while the command ran".
- `:143-153` builds `Binding` from the CANDIDATE reads:
  `candidateIndexBefore`, `candidateWorktreeBefore`, `candidateIndexAfter`,
  `candidateWorktreeAfter`.

**Correction 1.** The brief says the after bindings are the real
workspace's postures. They are the isolated candidate's (`:129`, `:151`).
A register appended in the real workspace DURING the battery does not
break the after binding; `TestCreateTestReceiptIgnoresLiveWorkspaceMotion`
(`receipt_test.go:16-58`) proves exactly that today by appending to the
live digest from inside the command. What breaks is the landing-time read
in `readTestReceipt` (`:306-309`): "the index or working tree moved after
the test receipt was created". The twenty-minute battery is discarded on
the tier-1 path because `create_test_receipt` (`land.sh:382-388`) re-runs
the command on every attempt, and `CreateTestReceipt` removes the previous
receipt first (`:59-62`). On the chain path the receipt file survives a
refusal, so the cost there is the manual checkout and the retry.

`readTestReceipt` (`:260-311`): `:290` requires schema 1; `:296-305`
requires all four bindings to equal the candidate; `:306-309` re-reads the
real postures and requires both to equal the candidate.

**The candidate tree.** `land.sh:351-359` `staged_candidate_tree` is `git
write-tree` on the real index, subtree by prefix. `commit.sh:292` and
`:421` take the same write-tree and `:443-447` resolve the prefix subtree
for `landing observe`. None of these read the worktree.

**The land.sh cleanliness sites.** `stage_changes` (`land.sh:315-334`)
refuses at `:324-326` when `git diff --quiet --` finds unstaged changes
and at `:328-333` when untracked paths exist. `require_clean_after_commit`
(`:390-398`) refuses when `git status --porcelain --untracked-files=normal`
prints anything; it runs at `:557` on the ordinary path and at `:550-555`
on the recertified path, where a refusal parks the chain with
`chain-recertification-source-changed`. `rebase_origin` (`:404-406`) is a
plain `git rebase` and runs at `:571` and again inside the push retry loop
at `:589`. The step names "rebase onto origin/<branch>" and "rebase onto
origin/<branch> after push attempt <n>" are asserted by
`land-fixtures.sh:437` and `:498` and are kept.

What the post-commit clean check actually protects: the rebase, which
refuses on any unstaged change, and the equation "the tree the proofs
bound is the tree that was committed". It does not protect transport:
`sync-transport.sh:31-35` fetches origin's branch head into the tracking
ref and pushes that ref; it reads no worktree. Revision 1's phrase
"transport needs the whole worktree clean" was wrong and is withdrawn; the
message text at `land.sh:394` is kept unchanged so no fixture moves.

**Correction 2, a sixth site.** `commit.sh:321-342` enumerates `git diff
--name-only` at the toplevel plus untracked files, filters them through
`behavior-surface select --projection LANDING`, and refuses: "the LANDING
comparison found projected working-tree bytes that are not what the commit
would record at its index endpoint". `:425-431` repeats the enumeration
after the proofs. LANDING membership (`behaviorsurface/policy.go:279-301`)
excludes every `coordinationPaths` match. `policy.v2.json:29-35` lists
`memory/receipts.log` and does not list `records/narrator-digest.log`. So a
dirty receipts log passes commit.sh today and a dirty narrator digest does
not. The fixture stub of commit.sh in `land-fixtures.sh:51-135` has no such
check, so the shell bed cannot see this site; a unit test on the policy
pins it instead.

**The two engines at a landing.** `land.sh:14` selects
`${METASYSTEM_BIN:-$root/bin/metasystem}` and uses it for every landing
verb, from the brain fence at `:117` to the receipt mint at `:386`.
`commit.sh:297-319` builds a separate proof engine from the checkout's
source (`go-gate.sh --fast --proof-out`, which is `go-build.sh --out`,
`go-gate.sh:463-468`) and reads the receipt with that engine at `:463`.
`dispatch.sh:235-271` compares the live engine's stamp only with commits
between the stamp and HEAD; staged source is invisible to it, and a `dev`
stamp bypasses it (`:244`). `go-build.sh:38-66` stamps a build
`dev-<commit>-dirty` whenever the ENGINE projection differs from HEAD, and
its comment says automatic enrollment refuses that stamp. So on the
landing that lands this change the live binary is necessarily older than
the candidate, and it cannot be rebuilt and re-armed from the dirty tree.

**The filtering primitive.** `gittree.FilterTree` (`gittree.go:284-312`)
reads a tree into an isolated index and runs `update-index --force-remove`
from the toplevel with the paths verbatim, so a tree in workspace path
space is filtered by workspace-relative names (`:296-299`;
`gittree_test.go:666-686` proves it in a nested checkout). Every caller
passes the mission's own ledger path (`missionrunner/wall.go:510-526`).

**The detached worktree primitive.** `gittree/detached.go:26-77`
`NewDetachedWorktree(tree)` runs `git worktree add --detach <tmp> HEAD`
under `os.MkdirTemp` (`:38`, `:53`) and then grafts the candidate tree
into it (`:58-68`). `Close` (`:116-131`) removes the worktree with
`worktree remove --force --force` and never runs `worktree prune`. The
advance verb below needs only the first half, a detached worktree at a
commit, and gets a sibling constructor for it.

**The writers and where their bytes go.**

- `narratordigest.Append` (`digest.go:133-172`) and `AppendPayload`
  (`:177-217`) take an exclusive flock on
  `artifacts/agents/steward/narrator-digest.flock` (`:90-115`;
  `stateroot.go:232-233`), read the whole file, append lines to the body in
  memory, and replace the file through `atomicfile.WriteText` (`:164`,
  `:209`). `Pending` (`:248-256`) takes the same lock to read. A holder of
  that lock therefore owns the digest's bytes: every digest writer blocks in
  `acquire` (`:104-109`) until release.
- `receipt.appendLine` (`internal/receipt/receipt.go:445-459`) opens the
  receipts log with `O_APPEND|O_CREATE` per call, writes one line, syncs,
  closes. No lock exists; the callers at `:212`, `:287`, `:304` are the
  steward's `PrepareIntent` path and the receipt verbs. Two facts follow:
  each line is one `write` at the file's end, so concurrent appenders
  interleave whole lines and never tear each other; and nothing can hold
  this writer off, so any protocol that must own the path for a moment has
  a window against it, stated per window below.

Their lines reach main through register-carriage landings: `git log --
memory/receipts.log` shows a1273d6d "Receipt for the stop verb landing"
and 937b30eb, both `Landing-Provenance: direct-fix class=register-carriage`.
The unstaged register bytes are therefore real work in flight, and the
seat's manual `git checkout --` has been discarding them.

**Git facts probed at design time** (git 2.50.1 Apple Git-155, scratch
clones of a bare origin under a private temporary root, with the harness's
`GIT_OBJECT_DIRECTORY` and `GIT_ALTERNATE_OBJECT_DIRECTORIES` unset the way
`land-fixtures.sh:15` does; not fixtures):

- `git rebase <upstream>` refuses with "cannot rebase: You have unstaged
  changes" even when the branch is already up to date.
- The stash ref is one file under the common directory: every worktree of
  a repository and every session on the machine push to and pop from the
  same list (the round-1 critic verified this on two worktrees of this
  repository). `git rebase --autostash` saves a failed re-apply there.
- `git worktree add --detach <dir> <commit>` on a clone of this repository
  (3187 tracked files) took 0.3 s; `worktree remove --force` 0.15 s.
- In a private detached worktree, `git rebase <upstream>` leaves the real
  checkout's branch, index, and dirty register untouched, both on success
  and on a content conflict (`UU product.txt` in the private worktree,
  `rebase --abort`, `worktree remove`, real checkout still at the landing
  commit with ` M reg.log`).
- `git reset --keep <N>` with a dirty register that is identical between
  HEAD and N: exit 0, branch moved to N, the register's worktree bytes
  untouched, status ` M reg.log`. The register's index entry keeps its
  stale stat and git never opens the file.
- `git reset --keep <N>` with a dirty path that differs between HEAD and
  N, register or not: "error: Entry 'reg.log' not uptodate. Cannot merge."
  exit 128, and nothing changed: branch, index, and every worktree file
  are as before. This is the all-or-nothing the rare-case protocol relies
  on.
- Writing the index blob's bytes back into the dirty register and then
  running `reset --keep` still aborts with the same message, because the
  index entry's cached stat no longer matches the file. `git update-index
  --refresh -- <path>` before the reset makes it succeed. The protocol
  below has that refresh as a mandatory step.
- A shell model of the rare-case protocol (move the file away, create the
  index blob at the path with O_EXCL, refresh, `reset --keep`, retry on
  abort, append the captured suffixes) was run against a shell appender
  that opens, appends one line, and closes per line, the shape of
  `appendLine`. At full speed (thousands of lines per second) the loop
  never won while the writer ran, because every moved file was recreated
  before the next create; it had reached 44 591 capture files when the
  writer was stopped by hand, after which the reset succeeded and the
  restore returned 9 613 419 of 9 613 426 lines, no duplicates, the
  rebased blob a prefix of the result. The 7 missing lines were written
  into the freshly created empty file before the probe's `cat-file` wrote
  the blob into it at offset 0, which overwrote them; step F.2 below
  therefore links a fully written file into place instead of creating and
  then filling. At one line per 10 ms all 50 attempts aborted: the two git
  process spawns between the create and the reset's check are wider than
  that period. Both facts shape 3e: every loop is bounded, the bound is a
  rate bound, and hitting it refuses loudly with every byte on disk.
- `git update-index --force-remove` accepts a path absent from the index
  (exit 0), so `FilterTree` is safe on a tree that lacks a register.
- `git status --porcelain` prints toplevel-relative paths from a
  subdirectory.

## Decision 1: where the register set is declared

New file `internal/landing/registers.go`:

```go
// appendOnlyRegisters names the tracked registers that background writers
// and turn-boundary hooks append to at times a landing does not control.
// Paths are workspace-relative, the landing package's one path space.
// One declaration, four consumers: recordCarriageError carries a register
// only append-only, receiptPosture excludes it from the working-tree
// projection, WorktreeDrift tolerates an unstaged append to it, and
// Advance restores that append across a rebase.
var appendOnlyRegisters = []string{
	"memory/receipts.log",
	"records/narrator-digest.log",
}

// AppendOnlyRegisters returns a copy for callers outside the package.
func AppendOnlyRegisters() []string

func isAppendOnlyRegister(path string) bool
```

The name says what the landing does with these files (carries appends
only), not what the receipt does (ignores them), because the carriage rule
at `:779` is the older policy and the receipt's exclusion follows from it.

`observe.go:773` changes from `switch changedPath {` to a tagless switch:
`case changedPath == "memory/rulings.md":` and `case
isAppendOnlyRegister(changedPath):`. The bodies at `:775-778` and
`:780-783` do not change. The two sets are disjoint, so order is
irrelevant.

Why not elsewhere:

- `gittree` is generic and names no metasystem path; its callers pass
  their own lists (`wall.go:514`).
- The path-class manifest (`scripts/agents/path-classes.txt:29,31`) marks
  `memory/` and `records/` as class `record` by directory. It cannot name
  two files, and it is loaded from the landing base tree
  (`observe.go:259`), so the receipt's reads would depend on which base is
  checked out.
- `behaviorsurface/policy.v2.json` is a different policy (which bytes the
  static proofs bind). It must agree with the declaration, and a pin test
  makes it, but it does not own it.
- `stateroot` owns the directories (`stateroot.go:218-227`), not the files.

Pins, in `internal/landing/registers_test.go` (package `landing` already
depends on `dispatch`, which imports `behaviorsurface`, so the test import
adds no cycle; `behaviorsurface` imports nothing from `landing`):

1. `appendOnlyRegisters` equals `{RelativeRoot(Receipts)+"/receipts.log",
   RelativeRoot(Records)+"/narrator-digest.log"}`.
2. For every declared path, `policy.Includes(Landing, "metasystem/"+path,
   "metasystem/")` is false and `Includes(Landing, path, "")` is false.
   This is the sweep that catches a register added to the Go declaration
   without the policy row.

Revision 1 had a third pin, a literal `merge=union` line in
`.gitattributes` for each register. It is withdrawn. No step of this
design merges register bytes through git's merge driver any more (Decision
3e restores by prefix and append, in Go), so the attribute is not a
premise of anything here. The attribute still governs the pre-existing
case of a register-carriage commit rebased over an upstream carriage of
the same register; that rebase now happens in the private worktree, and if
the attribute were ever absent its conflict is a loud `advance-rebase-
conflict` refusal with the real checkout untouched, not a silent loss.

## Decision 2: which trees are filtered

`receiptPosture` keeps its shape and changes its second value:

```go
// receiptPosture returns the exact index tree and the FILTERED working-tree
// projection: Snapshot("HEAD") with appendOnlyRegisters removed.
func receiptPosture(workspace gittree.Workspace) (indexTree, projection string, err error)

// receiptIdentity is what a filtered projection must equal for candidate
// tree: the same tree with the registers removed.
func receiptIdentity(workspace gittree.Workspace, tree string) (string, error)
```

Both use `workspace.FilterTree(x, appendOnlyRegisters)`. Because
`FilterTree` removes the paths, a filtered projection can never equal the
raw candidate tree; it is compared against `receiptIdentity(tree)`, the
same way `wall.go:580-596` compares observed against committed with both
sides filtered.

The reads and their new comparisons:

| Site | Index comparison | Projection comparison |
| --- | --- | --- |
| `receipt.go:81` real workspace before | `== tree` | `== receiptIdentity(tree)` |
| `:107` isolated candidate before | `== tree` | `== receiptIdentity(tree)` |
| `:133` isolated candidate after | `== tree` | `== receiptIdentity(tree)` |
| `:296-305` recorded bindings | index bindings `== candidate` | worktree bindings `== receiptIdentity(candidate)`, recomputed at landing time |
| `:306-309` real workspace at landing | `== candidate` | `== receiptIdentity(candidate)` |

Messages at `:82`, `:108`, `:134`, `:303`, and `:308` keep their text; `:82`
now prints the filtered projection id, which is what was compared.

Not filtered, and not given an option: `StagedTree` at both index reads,
`staged_candidate_tree` (`land.sh:351-359`), the write-trees in
`commit.sh:292,421`, and the defaults of `Snapshot` and `StagedTree`. The
filtering composes at the landing's call sites, which the brief allows.

Path space: the candidate is the workspace subtree (`land.sh:354-357`,
`commit.sh:443-447`), `Snapshot` returns the subtree (`gittree.go:276`),
and `FilterTree` matches workspace-relative names against a subtree
(`:296-301`, `gittree_test.go:676-686`). In an adopted checkout at the
toplevel (`newAdoptedObserveFixture`, `observe_test.go:35-39`) the prefix
is empty and the same names match.

What proof is kept:

- A staged change to any path, registers included, moves the index tree
  and refuses. A receipt for tree T stays unlandable against any index
  other than T.
- An unstaged change to any tracked non-register path moves the filtered
  projection and refuses with the existing message.

What proof is given up, by construction:

- Unstaged motion of a register path in the real workspace, before the
  receipt and at landing time. That is the goal.
- Motion of a register path inside the isolated candidate while the
  command runs: `:133` no longer fires for those two paths. The candidate
  worktree is discarded at `:136`, the registers are bookkeeping and not
  product, and the command's verdict on T cannot depend on bytes the
  battery itself appended. All four reads go through one function so the
  bindings have one meaning everywhere.
- Content is not examined by the receipt. A rewritten or deleted register
  in the worktree is as invisible to the receipt as an appended one. The
  drift verb examines the shape (Decision 3a) and the `:779` rule examines
  content when someone lands it, which is where content belongs.

Filtering the index as well is not done: it would let a staged register
change into a receipted landing unseen, which widens carriage.

## Decision 3: the cleanliness sites and transport

**3a. The drift verb.** `metasystem landing drift --root <root>
[--require-empty-index]`, registered in `cmd/metasystem/main.go:111-119`
beside `observe`, `park`, and `test-receipt`, implemented by
`landing.WorktreeDrift(root string, requireEmptyIndex bool) ([]DriftEntry, []string, error)`
in a new `internal/landing/drift.go`; the second result is the tolerated
register paths.

Mechanics: one `git status --porcelain=v1 -z --no-renames
--untracked-files=normal` at the repository toplevel
(`Workspace.TopLevel()`, `gittree.go:188`), because the rebase refuses on
any unstaged change anywhere in the worktree, which is what `land.sh:392`
checks today. This needs one new gittree primitive, `Workspace.Status()
([]StatusEntry, error)` in `snapshotscope.go`, built on `gitProbe`
(`:41`) like the other census probes: toplevel-scoped, `-z` parsed, fields
`Index byte`, `Worktree byte`, `Path string` (toplevel-relative), no
rename records because of `--no-renames`. Paths are mapped to workspace
space by stripping `Workspace.Prefix()` (`gittree.go:195`); a path outside
the prefix is never a register.

The append shape, `isRegisterAppend(workspace, path) (bool, error)`: the
worktree entry is a regular file (`os.Lstat`); the index blob is
`FileAt(StagedTree(), path)` (`gittree.go:443-457` on the index subtree);
the file's bytes have the index blob as a byte prefix and end in a newline.
This is `appendOnly`'s rule (`observe.go:971-981`) applied to index versus
worktree instead of base versus candidate, so what the drift verb
tolerates is exactly what the carriage rule will later accept. An index
blob that is the whole file (no growth) cannot show as ` M` except through
a stale stat, which `git status` refreshes, so equality is not a case.

The rule, applied to each entry `XY path`. `X` is the index column, `Y`
the worktree column. With `--no-renames`, `R` and `C` never appear.

With `--require-empty-index` (the post-commit site), in this order:

1. `??` is drift of kind `untracked`.
2. `X != ' '` (`M`, `A`, `D`, `T`, `U`, and every unmerged pair) is drift
   of kind `staged`, whatever `Y` is and whether or not the path is a
   register. After the commit the index must be exactly HEAD; a staged
   register append here would be a change that appeared after the proved
   commit, and it is refused before the register exception is reached.
3. ` M` on a register with the append shape is tolerated.
4. ` M` on a register without the append shape is drift of kind
   `register-not-append`.
5. Everything else (` M` non-register, ` D`, ` T`) is drift of kind
   `unstaged`.

Without the flag (the pre-commit site), in this order:

1. `??` is drift of kind `untracked`.
2. `Y == ' '` with `X` in `M`, `A`, `D`, `T` is the candidate: tolerated.
3. `X` in `' '`, `M`, `A` with `Y == 'M'` on a register with the append
   shape (against the INDEX blob, which for `MM` and `AM` is the staged
   carriage content) is tolerated: a register-carriage candidate with a
   further unstaged append is the same defect in its own form.
4. `X` in `' '`, `M`, `A` with `Y == 'M'` on a register without the append
   shape is drift of kind `register-not-append`.
5. Everything else (`TM` on a register, since a type change is not an
   append; `MM`, `AM`, `TM` on a non-register; ` M` non-register; ` D`;
   ` T`; every unmerged pair) is drift of kind `unstaged`.

`DM` is not an ordinary porcelain-v1 state and is not a case. The
documented unmerged pairs (`DD`, `AU`, `UD`, `UA`, `DU`, `AA`, `UU`) reach
rule 2 with the flag and the final rule without it.

Output: one line per drift entry on stdout, `kind<TAB>XY<TAB>path` with
the path as git printed it; tolerated entries on stderr as `tolerated
register append: <path>` so a landing log shows what was ignored. Exit 0
with no drift, 1 with drift, 2 on usage or git failure.

**3b. land.sh.**

- `stage_changes` `:324-333`: replace the `git diff --quiet` and
  `ls-files --others` pair with one `"$ms" landing drift --root "$root"`
  call captured into a variable. On exit 1: if any line is of kind
  `unstaged` or `register-not-append`, print the existing message "land
  refused: unstaged changes remain after staging; transport requires a
  clean tree after commit"; otherwise print the existing untracked
  message; then print the lines indented; return 2. The empty-staging-set
  check at `:320-323` stays first and unchanged.
- `require_clean_after_commit` `:390-398`: replace the `git status` call
  with `"$ms" landing drift --root "$root" --require-empty-index`. On exit
  1 print the existing message "land refused: commit succeeded but the
  tree is not clean, so transport will not start" and the lines; return 1.
  The recertified park at `:550-555` is unchanged and still fires on real
  drift.
- `rebase_origin` `:404-406` becomes `"$ms" landing advance --root "$root"
  --upstream "refs/remotes/origin/$branch"`. The step names at `:571` and
  `:589` do not change. The retry loop at `:588-589` calls the same
  function and needs no change.
- Nothing else in land.sh changes. `check_supplied_test_receipt` (`:361-380`)
  reads only the `tree` field, which keeps its name.

**3c. commit.sh's LANDING comparison.** Add `"records/narrator-digest.log"`
to `coordinationPaths` in `internal/behaviorsurface/policy.v2.json:29-35`,
directly after `memory/receipts.log`. No `records/**` pattern exists in
`payloadRoots` (`:11` onward), so the digest's projection row becomes
`{"records/narrator-digest.log", Coordination, false, false, false}` in
the table at `policy_test.go:91-115`, unless a `tailoredPaths` pattern
matches it, in which case the class column reads `Tailored` and the three
booleans stay false. The fixture stubs of the same policy list
`memory/receipts.log` by hand and gain the digest beside it:
`scripts/agents/static-reproof-fixtures.sh:313`, `:329`, `:484`, and
`internal/behaviorsurface/consumer_wiring_test.go:84`. The round-1 critic
searched the Go call sites of `Classify` and `ClassifyChanges` and found
only the behavior-surface classify command in production; the implementer
still runs the behaviorsurface tests and `static-reproof-fixtures.sh`
after the edit, because that run is the record.

**3d. Why not carriage.** Staging the drifted registers into the landing
commit fails three ways. After the receipt exists, staging a register
changes the commit tree, so `receipt.Tree != CandidateTree` at
`receipt.go:290` and `land.sh:376` void the receipt. Staging before the
receipt leaves the drift during and after the battery uncovered, which is
the defect. And a chain landing that carries register bytes without
`--direct-fix register-carriage` refuses `chain-has-uncarried-paths`
(`observe.go:284-286`), so every chain landing would have to declare
carriage it did not intend. The writers' bytes stay in the worktree,
untouched, for the register-carriage landing that already exists for them.

**3e. The advance verb: the rebase without a stash.** `metasystem landing
advance --root <root> --upstream <ref>`, implemented by
`landing.Advance(root, upstream string, stdout, stderr io.Writer) error` in
a new `internal/landing/advance.go`. It replaces `git rebase` in the real
checkout. Its one design rule: the real checkout's register files are
touched only when origin changed that register, and then only under a
protocol whose every window is named below.

Names used below, per register R: L is the landing commit (HEAD at entry);
N is L rebased onto the upstream; B0 is R's blob in the index, which
equals L's blob because the index is clean; B1 is R's blob in N; W is R's
worktree bytes; S is W with the B0 prefix removed, the local append.

Steps:

A. **Preconditions, nothing touched.** HEAD is a branch (else refuse
   `advance-not-on-branch`). The index equals HEAD's tree at the toplevel
   (`git write-tree` equals `HEAD^{tree}`; else refuse
   `advance-index-not-empty`). The upstream ref resolves to a commit.
B. **Fast path.** `git merge-base --is-ancestor <upstream> HEAD` exits 0:
   print `advance: up to date with <upstream>` and exit 0. On a serialized
   seat this is the usual outcome, and it touches nothing.
C. **Private rebase.** A new gittree constructor
   `NewDetachedCommitWorktree(commit)` in `detached.go`: the same
   `worktree add --detach` under `os.MkdirTemp` as `:38-56`, at the given
   commit instead of HEAD, without the graft and `read-tree` at `:58-68`,
   with the same `Close`. In it, `git rebase <upstream>`. On failure:
   `rebase --abort`, `Close`, refuse `advance-rebase-conflict` with git's
   output; the real checkout is untouched (probed). On success N is its
   HEAD; `Close`. Its own `.gitattributes` are in force there, so a
   register-carriage commit merges the way it does today.
D. **Classification, nothing touched.** `changed` is `git diff --name-only
   --no-renames L N` at the toplevel; `dirty` is `Workspace.Status()`. Any
   entry in `dirty` with a non-blank index column refuses
   `advance-index-not-empty` (a stage between the post-commit check and
   now). The overlap is the dirty paths that are also in `changed`. Any
   overlap path that is not a declared register, or is a register whose
   worktree entry lacks the append shape of 3a, refuses
   `advance-unstaged-drift` naming the path and N, so the seat can `git
   reset --keep <N>` by hand once the path is dealt with. A register in
   the overlap that N deletes or whose mode N changes refuses
   `advance-register-removed`.
E. **Common case, overlap empty.** `git reset --keep N`. The branch moves,
   every changed path is checked out, and the dirty registers are never
   opened (probed: git keeps the index entry and its stale stat). Print
   `advance: <L> -> <N>; registers untouched: <paths>` and exit 0. No
   window exists here for either writer, because the landing never reads
   or writes a register.
F. **Rare case, overlap is one or both registers.** The capture directory
   is `artifacts/agents/landing/advance/<L>/`, ignored by `.gitignore:1`,
   private to this landing by the commit id, on the same filesystem as the
   register so a rename is atomic. First a `manifest.json` naming L, N,
   and per register its path, B0, B1 is written and synced. If a manifest
   for L already exists (an interrupted run), its capture files are folded
   into this run's set and the counter continues. If the digest is in the
   overlap, the digest flock is acquired now through a new exported
   `narratordigest.Lock(repoRoot) (release func(), err error)` that wraps
   `acquire` (`digest.go:96-115`), and held until step G ends; package
   `landing` already has `narratordigest` in its dependency closure (`go
   list -deps`), so the import adds no cycle. Then, for the register set,
   at most 8 attempts:

   1. Move: `rename(R, <dir>/<k>.strip)`. The first move takes the
      original W, later moves take the B0 file this loop created and a
      writer grew. A `.strip` file is known to begin with B0. An appender
      that already opened the old inode keeps writing into the moved file,
      which is read again at the end.
   2. Create: write B0 to `<dir>/b0.<k>` (`O_CREATE|O_EXCL`, write, sync,
      close), then `os.Link(<dir>/b0.<k>, R)` and remove the temporary
      name. The file is complete before the path exists, so no writer can
      land a line in it ahead of B0 (the probe lost 7 lines exactly that
      way). On `EEXIST` a writer created R in the gap: `rename(R,
      <dir>/<k>.raw)` (a `.raw` file is appender lines only, nothing to
      strip) and try the link again, at most 8 times; past that, refuse
      `advance-register-contended`: every byte is in R or in the capture
      files, the branch is still at L, and the message says to run
      `landing advance` again when the writer is quiet (the manifest makes
      that run resume).
   3. Refresh: `git update-index --refresh -- R` (probed: without it the
      reset aborts on the stale stat).
   4. Reset: `git reset --keep N`. Success leaves the loop. The abort
      "not uptodate" means a writer appended to the fresh B0 file between
      steps 2 and 4; continue at step 1, which moves that grown file as
      `.strip`. Past 8 attempts refuse `advance-register-contended` as
      above. The window between the link and the reset's check is two git
      process spawns wide (tens of milliseconds, measured at more than
      10 ms); a writer whose period is shorter than that wins every
      attempt, and the refusal is the answer to it. The real writers
      append once per landing event and once per turn boundary, seconds
      to minutes apart, so the first attempt is the expected outcome and
      the bound is a guard, not a path.

G. **Restore.** R now holds B1 in an inode git created. Read every capture
   file in `k` order; strip B0 from each `.strip` file after verifying the
   prefix (a `.strip` file without it is a corruption and refuses
   `advance-capture-corrupt` with nothing removed); concatenate into S.
   One `O_APPEND` write of S onto R, then sync. Then read every capture
   file again; if any grew since its last read (a straggler on a moved
   inode), append the growth the same way; repeat until two consecutive
   reads agree, at most 8 rounds. Then remove the capture directory and
   release the lock. Print `advance: <L> -> <N>; <R>: <len(S)> captured
   bytes restored after <B1>` per register and exit 0.

The merge rule, named: **prefix-verified suffix append**. The rebased blob
B1 comes first, the local append S follows, in the order the local writer
produced it; upstream lines that arrived inside B1 therefore precede local
lines that may be older by the clock. What makes it true is this verb's
own code: the prefix check `bytes.HasPrefix(W, B0)` before any move, and
the single `O_APPEND` write of S in step G. No git merge driver and no
attribute takes part, which is why the `merge=union` pin is gone. The
result satisfies `appendOnly` (`observe.go:980`) against the new base B1,
so the next register-carriage landing carries it without complaint.

What happens to each writer's bytes, rare case only (the common case has
no window):

| Window | Digest (flock; read, modify, replace) | Receipts log (no lock; one `O_APPEND` write per line) |
| --- | --- | --- |
| Before the first move | In W, captured as `.strip`, restored. | Same. |
| Between the move and the link | Blocked in `acquire` until step G releases; then appends to the final file. | The writer's `O_CREATE` makes a new R; the link sees `EEXIST`, the loop moves it as `.raw`, restores it. |
| Between the link and the reset's check | Blocked. | Lands in the complete B0 file after B0; the reset aborts; the loop moves it as `.strip`, restores it. A writer faster than this window on every attempt ends in `advance-register-contended`, nothing lost. |
| Inside the reset, while git unlinks and recreates R | Blocked. | A writer that opened the B0 file before git's unlink and wrote after it wrote to a dead inode: LOST. Width: the gap between `os.OpenFile` and `WriteString` at `receipt.go:446-450`. |
| Between the reset and the append of S | Blocked. | Lands in git's B1 file ahead of S: preserved, ordered before S. |
| After the append, on a moved inode | Blocked. | Caught by the re-read rounds; a write after the last agreeing read is LOST, same width as above. |
| After step G | Appends to the final file. | Same. |

How the landing proves it: the verb's final self-check reads R and refuses
`advance-restore-mismatch` unless R has B1 as a prefix and contains S as
one contiguous run after it; the per-register line names the byte count,
and `git status` afterwards shows ` M <R>` for every restored register and
nothing else. The two LOST rows are the limit of this design against a
writer that has no lock, stated plainly: they are a few instructions wide,
they exist only when origin changed that register during this landing's
window, and closing them means giving the receipts log the digest's lock,
which changes a writer and is outside this goal. Everything the stash did
badly is gone: no shared list is touched (the shell canary proves a
sentinel entry survives), and no append disappears between a snapshot and
a reset because there is no snapshot-and-reset of the whole tree, only a
move of one file that a straggler keeps writing into.

A crash inside step F leaves R absent or at B0 with the bytes in the
capture directory; the next landing's drift verb refuses on ` D` or on the
non-append shape, naming the path, and `landing advance` run again resumes
from the manifest. That is loud and loses nothing; it is not made
invisible.

The private worktree costs 0.3 s on this repository and only when origin
moved. `Advance` uses `gittree`'s bounded git wrapper (`gittree.go:121-144`),
so every git call carries the local timeout.

## Decision 4: what the receipt records

Schema version 2:

```json
{
  "schemaVersion": 2,
  "tree": "<candidate tree T>",
  "command": "...",
  "exitStatus": 0,
  "time": "...",
  "binding": {
    "indexTreeBefore": "<exact index tree>",
    "worktreeTreeBefore": "<filtered projection>",
    "indexTreeAfter": "<exact index tree>",
    "worktreeTreeAfter": "<filtered projection>"
  },
  "worktreeProjection": {
    "excludes": ["memory/receipts.log", "records/narrator-digest.log"],
    "tree": "<FilterTree(T, excludes)>"
  }
}
```

Go: `WorktreeProjection *TestReceiptProjection \`json:"worktreeProjection,omitempty"\``
with `Excludes []string` and `Tree string`, filled from
`appendOnlyRegisters` and `receiptIdentity(tree)` at `:143-153`. The
pointer with `omitempty` is what lets a version-1 file decode without the
object and a version-2 file never omit it. The `binding` field names stay,
so a reader sees the same four keys with the projection object telling it
what the two worktree keys mean. The comment on `TestReceipt` (`:28-29`)
says the worktree observations are filtered projections in version 2 and
raw in version 1, and the index observations are exact in both.

Reader rules in `readTestReceipt`, in order after the existing decode
(`DisallowUnknownFields` at `:281` stays):

1. `SchemaVersion` is 1 or 2; anything else refuses with the existing
   message at `:291`. `Tree`, `Command`, `ExitStatus`, and `Time` are
   checked as today.
2. Version 2: `WorktreeProjection` must be present; `Excludes` must equal
   `appendOnlyRegisters` element for element, else "test receipt excludes
   a different register set than this engine"; `WorktreeProjection.Tree`
   and both worktree bindings must equal `receiptIdentity(candidate)`
   recomputed now; both index bindings must equal the candidate; any
   mismatch keeps the message at `:303`.
3. Version 1: `WorktreeProjection` must be absent (a version-1 receipt
   carrying a projection object is a mixed form and refuses with "test
   receipt mixes schema versions"); all four bindings must equal the
   candidate exactly, which is what the version-1 writer itself required
   before publishing (`:81`, `:107`, `:133`). No register set is checked,
   because nothing in a version-1 receipt was filtered.
4. The landing-time read at `:306-309` as in Decision 2, for both versions.
   It is the reader's own observation of the checkout now, so it follows
   the reader's rule, not the writer's.

Why the version-1 rule is sound: a version-1 receipt says the isolated
candidate's raw projection equalled T before and after the command. Raw
equality implies filtered equality (`FilterTree` is a function of the
tree), so every fact a version-2 receipt asserts about the candidate is
asserted by a version-1 receipt a fortiori. The version-1 rule accepts
nothing weaker than version 1 demanded.

Why it cannot reintroduce the silent raw-versus-filtered misreading: the
comparison target is selected by the version field, and the version field
is written in the same struct literal as the bindings (`:143-153`), so a
file's version and the meaning of its bindings travel together. There is
no branch in which a filtered tree is compared with a raw one: version 2
compares filtered with filtered, version 1 compares raw with raw. The two
mixed forms are loud: a version-1 receipt whose worktree bindings hold
`receiptIdentity(T)` fails rule 3 (they do not equal T); a version-2
receipt whose worktree bindings hold raw T fails rule 2 (they do not equal
`receiptIdentity(T)`) whenever T contains a register, which every real
candidate does. The pin tests for both mixed forms are in the fixture
list. What made revision 1 refuse version 1 was a reader that kept the
field names and did not move the version; that reader does not exist in
this design.

The crossover, stated as procedure because the code cannot enforce it on
the pre-change side: the landing that lands this change runs land.sh with
`METASYSTEM_BIN` set to an engine built from the candidate (`scripts/
agents/go-gate.sh --fast --proof-out <path>` in the checkout, the build
commit.sh makes at `:302`), because `landing drift` and `landing advance`
exist only there and the live binary cannot be rebuilt and re-armed from
the dirty tree (`go-build.sh:38-66`). With that engine the tier-1 path
mints version 2 and reads version 2. On the chain path the seat mints the
receipt with the same engine; if it mints with the stale live binary
instead, the receipt is version 1 and rule 3 accepts it, so the battery is
not lost either way. If the seat forgets `METASYSTEM_BIN` altogether, the
live binary fails at `stage_changes` on the unknown verb, which is before
the battery on the tier-1 path and before the commit on the chain path,
where the receipt file survives for the retry. After the landing the
engine is rebuilt and re-armed the way `dispatch.sh:269` already demands.

The reverse crossover, a candidate whose engine source is older than the
live binary (an exact revert of this change, a stale branch), mints
version 2 and reads it with a pre-change proof engine, which refuses on
the unknown field. That is loud, it is the same behaviour as for any
receipt field ever added, and it is outside this goal.

The version-1 rule stays in the reader; its removal is a one-line change
with the pin flipped to a refusal, is crossover-safe because both engines
then agree on version 2, and is not scheduled by this design.

## What must not change, and how each is kept

- Index tree and candidate tree exact: `StagedTree` untouched; the index
  comparisons at `:81`, `:107`, `:133`, `:296-305`, `:307` stay exact; the
  write-trees in land.sh and commit.sh are untouched.
- The `:779` rule stays; only its case expression reads the declaration.
  The drift verb's append shape and the advance verb's restore both
  produce exactly what that rule accepts.
- Non-register drift between battery and landing refuses with the existing
  message: `:308` keeps its text; the Go refusal canary proves it before
  and after, and the shell refusal canary reaches the same refusal at
  `stage_changes` with a real staged candidate and receipt.
- `Snapshot`, `StagedTree`, and `FilterTree` keep their contracts; the
  landing composes them at its own call sites.

## Fixtures

Go tests in `internal/landing`, proving run
`go test ./internal/landing/ -run 'TestCreateTestReceipt|TestReadTestReceipt|TestWorktreeDrift|TestAdvance|TestAppendOnlyRegisters'`,
ceiling three minutes (the receipt tests run subprocess commands of `true`
and short `printf` lines; the advance tests create one private worktree
each):

- **Passing canary.** `TestReadTestReceiptSurvivesRegisterAppendAfterReceipt`:
  `newObserveFixture` (it already tracks both registers,
  `observe_test.go:64-65`); stage a change to `product.txt`; T is
  `StagedTree`; `CreateTestReceipt(T, "true")`; append one line to
  `records/narrator-digest.log` in the live root; `readTestReceipt` for T
  returns nil. Repeat with `memory/receipts.log`. The implementer runs this
  test against the untouched tree first and records that it fails with
  "the index or working tree moved after the test receipt was created".
- **During-the-battery canary.** Extend
  `TestCreateTestReceiptIgnoresLiveWorkspaceMotion` (`receipt_test.go:16-58`):
  the expected worktree bindings become `receiptIdentity(candidate)`, the
  index bindings stay `candidate`, and after the receipt returns,
  `readTestReceipt` against the live root, whose digest now carries the
  mid-command line, accepts. Add
  `TestCreateTestReceiptToleratesCandidateRegisterAppend`: the command
  appends to `records/narrator-digest.log` in its own working directory
  (the isolated candidate) and the receipt is still created.
  `TestCreateTestReceiptRefusesIsolatedCandidateMotion` (`:60-90`,
  `product.txt`) stays as it is and stays green.
- **Refusal canary.** `TestReadTestReceiptRefusesNonRegisterDrift`: the
  passing canary with the appended file `product.txt` instead; the error
  contains the message at `:308`. A second case stages an append to
  `memory/receipts.log` (index moved) and refuses with the same message.
  Both pass before and after the change.
- **Version rules.** `TestReadTestReceiptVersions`, receipts written by
  hand at `TestReceiptPath`: (a) version 1, four bindings equal T, no
  projection object: accepted; (b) version 1 with worktree bindings equal
  to `receiptIdentity(T)`: refused, binding message; (c) version 2 with
  worktree bindings equal to raw T: refused, binding message; (d) version
  1 with a projection object: refused, "mixes schema versions"; (e)
  version 2 with `excludes` missing one register: refused, register-set
  message; (f) version 3: refused, `:291` message. The fixture tree
  contains both registers, so `receiptIdentity(T) != T` and (b) and (c)
  are real refusals.
- **Drift rule, both modes.** `TestWorktreeDrift` over the shapes, each
  asserted with and without `--require-empty-index`: clean; ` M` register
  append tolerated in both; ` M` register with a rewritten first line is
  `register-not-append` in both; `MM` register append tolerated without
  the flag and `staged` with it; `AM` register (a register added in the
  index) append tolerated without the flag and `staged` with it; `TM`
  register (the worktree file replaced by a symlink after a staged type
  change) is `unstaged` without the flag and `staged` with it; `M `
  product tolerated without the flag and `staged` with it; ` M` product
  `unstaged` in both; ` D` register `unstaged` in both; `??` `untracked`
  in both; the nested `metasystem/` prefix of `newObserveFixture` maps
  correctly and an adopted toplevel fixture maps with the empty prefix.
- **Advance, common case.** `TestAdvanceLeavesRegistersUntouched`: a bare
  origin cloned twice; the peer pushes a `product.txt` change; the local
  clone commits a `scripts/agents/go-gate.sh` change, appends to both
  registers, runs `Advance`; the branch is at the rebased commit, both
  registers' worktree bytes are byte-identical to before, status shows
  exactly the two ` M` lines, and `Advance` printed the up-to-date line on
  a second call.
- **Advance, rare case, digest, concurrent writer.**
  `TestAdvanceRestoresDigestUnderConcurrentAppend`: as above but the peer
  pushes a digest append; the local clone has an unstaged digest append; a
  goroutine loops `narratordigest.Append` with distinct entries until told
  to stop, throughout `Advance`. Assertions: the digest has N's blob as a
  prefix, then the pre-existing local line, then every line the goroutine
  wrote, each exactly once; no capture directory remains; the stash list
  is empty.
- **Advance, rare case, receipts log, injected race.**
  `TestAdvanceRestoresReceiptsLogAcrossRaces`: the peer pushes a receipts
  log append. A package-level hook `advanceRaceHook func(stage string)`
  (injectable the way `receipt.syncFile` is at `internal/receipt/receipt.go:462`),
  nil in production, is called at `after-move`, `after-link`, and
  `after-reset`; the test appends a distinct line to the register at each
  stage through a real `os.OpenFile(O_APPEND)` write. Assertions: the
  reset aborted at least once (the verb reports attempts), every injected
  line and the original local line are present exactly once after N's
  blob, and the capture directory is gone. A second case injects at
  `after-link` on every attempt and asserts `advance-register-contended`
  after 8 attempts with the branch still at L and every injected line
  present in R or in a capture file.
- **Advance refusals.** `TestAdvanceRefusals`: a dirty non-register that
  origin changed refuses `advance-unstaged-drift` with the branch at L and
  the file intact; a conflicting rebase refuses `advance-rebase-conflict`
  with the branch at L and no leftover worktree in `git worktree list`; a
  register origin deleted refuses `advance-register-removed`; a staged
  path refuses `advance-index-not-empty`.
- **Pins.** `TestAppendOnlyRegistersPins`, the two pins of Decision 1.
- **Policy row.** The digest row in `behaviorsurface/policy_test.go`.

Shell canaries in `scripts/agents/land-fixtures.sh`, scenario
`full-width-chain`, after the matching-receipt landing at `:741-769`.
Proving run: `bash scripts/agents/land-fixtures.sh` (the scenario harness
at `fixture-bed-scenarios.sh:32-79` runs each leg as its own child and
mints the child capability itself, so the whole bed is the runnable unit;
its per-leg cap is the ceiling). The bed uses the real engine
(`land-fixtures.sh:50`) and a reduced commit.sh that performs the real
`landing observe` with `--test-receipt` (`:109-123`), so the receipt's
landing-time read is exercised.

- **Seed.** Inside the `full-width-chain` branch of `make_leg`
  (`:136-146`, `:161-163`) write and track `memory/receipts.log`
  (`receipt=seed\n`) and `records/narrator-digest.log` (`digest=seed\n`).
  No `.gitattributes` is seeded; nothing below depends on one.
- **Second candidate and its receipt.** After the matching-receipt landing
  the index is clean, so a fresh candidate is needed before any refusal
  can be reached: append a second line to `scripts/agents/go-gate.sh`,
  `git add` it, write a second chain record `full-chain-2` and its
  `rounds/1/{diff.patch,review.json}` the way `:675-697` does for the new
  candidate tree, and mint its receipt with `$source_engine landing
  test-receipt` and `$full_battery_command`.
- **Refusal canary, first.** Append `payload=drift\n` to `payload.txt`
  (tracked, not a register) in `leg_local`. Land `--chain full-chain-2
  --test-receipt <the new receipt> --staged-only --skip-transport`. Expect:
  exit 2; the exact line "land refused: unstaged changes remain after
  staging; transport requires a clean tree after commit"; a line
  `unstaged<TAB> M<TAB>payload.txt`; no `== STEP: commit` in the output;
  HEAD unchanged. Restore the file with `git checkout -- payload.txt`
  (fixture cleanup, not seat recovery). The staged candidate and its
  receipt are untouched by the refusal and are reused below. This leg
  passes before and after the change, but before the change it reaches the
  refusal through the old `git diff --quiet` at `:324`, and the tab line
  assertion is what tells the two apart.
- **Passing canary, one landing, rare case for the digest, common case
  for the receipts log, sentinel stash, concurrent writer.** From
  `leg_peer`, append `digest=peer\n` to the digest, commit, push to origin.
  In `leg_local`: append `digest=drift\n` to the digest; push a sentinel
  stash by dirtying and stashing a scratch tracked path (`git stash push
  -m sentinel -- plans/existing.md`) and record `git rev-parse stash@{0}`;
  start a background appender by recorded PID that appends
  `receipt=bg-<n>\n` lines to `memory/receipts.log` every 10 ms and mirrors
  each line into `$leg_root/bg.log`. Land `--chain full-chain-2
  --test-receipt … --staged-only --skip-transport` (fetch, advance, and
  push still run, `:570-591`; only `sync-transport.sh` is skipped). Stop
  the appender by its PID. Expect: exit 0; origin main equals local HEAD;
  `git show HEAD:records/narrator-digest.log` equals
  `digest=seed\ndigest=peer\n` (no widening); the worktree digest equals
  `digest=seed\ndigest=peer\ndigest=drift\n` (the rare-case restore across
  a real rebase, upstream first, local after); `git show
  HEAD:memory/receipts.log` equals `receipt=seed\n`; the worktree receipts
  log equals `receipt=seed\n` followed by exactly the bytes of `bg.log`
  (the common case never opened the file, so every concurrent line is
  there in order); `git status --porcelain` equals exactly the two ` M`
  register lines; `git stash list` has exactly one entry and `git
  rev-parse stash@{0}` equals the recorded id; `git worktree list` has one
  line; no `artifacts/agents/landing/advance/` directory remains. This one
  landing proves `stage_changes` tolerance, the receipt's landing-time
  read under drift, `require_clean_after_commit` tolerance, the private
  rebase against a moved origin, the common-case advance under a live
  lockless writer, the rare-case restore, and the untouched stash.

The shell bed cannot drive the digest's real writer (the flock lives in
Go and the bed's appender is a shell loop), so the concurrent digest case
is the Go test above; the shell bed's concurrent writer targets the
receipts log, whose common case is deterministic by construction.

## Implementation map

In this order, each step leaving the gate green:

1. `internal/gittree/snapshotscope.go`: `Workspace.Status()` and
   `StatusEntry`; `internal/gittree/detached.go`:
   `NewDetachedCommitWorktree`; tests in `gittree_test.go` and
   `snapshotscope_test.go` over a nested checkout.
2. `internal/landing/registers.go`, `registers_test.go`; the switch at
   `observe.go:773`.
3. `internal/landing/receipt.go`: `receiptPosture`, `receiptIdentity`,
   schema 2, the version rules; the receipt tests above.
4. `internal/landing/drift.go` and `drift_test.go`; the verb in
   `cmd/metasystem/landing_verbs.go` and its registry line in `main.go`.
5. `internal/narratordigest/digest.go`: exported `Lock`.
   `internal/landing/advance.go` and `advance_test.go`; the verb and its
   registry line.
6. `internal/behaviorsurface/policy.v2.json`, the `policy_test.go` row,
   `consumer_wiring_test.go:84`, and the three case lists in
   `static-reproof-fixtures.sh`.
7. `scripts/agents/land.sh`: the three edits of 3b.
8. `scripts/agents/land-fixtures.sh`: the seed, the second candidate, and
   the two canaries.
9. The crossover landing itself, as the procedure in Decision 4: land.sh
   under `METASYSTEM_BIN` pointing at a proof build of the candidate; the
   chain receipt minted with that same build; rebuild and re-arm after.

No document describes the receipt's JSON fields or land.sh's clean-tree
rule (grep over `docs/`, `AGENTS.md`, `wow.md`, `development/`, and
`skills/` finds neither), so the type comment in `receipt.go`, the doc
comment on `Advance`, and the registry summaries in `main.go` are the
records.

## What the code cannot answer

- Whether the static-reproof bed needs the digest in its skip lists beyond
  the three listed case lists: answered by running the behaviorsurface
  tests and `static-reproof-fixtures.sh` after step 6.
- The width of the two LOST rows in the rare-case table. The shell probe
  measured 7 lost lines in 9.6 million, and all 7 came from a defect the
  Go protocol does not have (create-then-fill; step F.2 links a complete
  file instead), so the probe put no number on the dead-inode rows
  themselves. The injected-race Go test proves every window the verb can
  see; the dead-inode window is the writer's own two-syscall gap, which no
  test of this verb can widen or close.
- Whether the eight refusals on 2026-09-09 included commit.sh's LANDING
  refusal on the digest: the goal record does not say. The design covers
  that site regardless.

## Revision record

Revision 2, 2026-09-09, from the round-1 read
(`records/misc/landing-receipt-survives-records-drift-critique-r1.md`).

- **LRD-01 (critical).** `git rebase --autostash` is withdrawn from 3b.
  Decision 3e replaces it: the rebase runs in a private detached worktree,
  the real checkout advances with `git reset --keep`, which never opens a
  register origin did not change, and the rare case restores by
  prefix-verified suffix append from per-landing capture files under
  `artifacts/agents/landing/advance/<L>/`. Each of the two writers is
  covered window by window in the table in 3e, with the two residual
  windows against the lockless receipts-log writer named as LOST and
  bounded to that writer's own open-to-write gap. The shell canary runs a
  concurrent receipts-log writer through the landing and checks a
  sentinel stash entry afterwards; the Go tests run a concurrent digest
  writer and inject the receipts-log races. The probes that ground it are
  listed under "Git facts probed", including the stale-stat abort that
  makes the refresh step mandatory and the livelock that makes the loops
  bounded.
- **LRD-02 (high).** No merge of register bytes through git remains in the
  restoration, so the literal-line pin is removed from Decision 1 and the
  claim that content conflicts cannot occur is gone with the autostash. The
  attribute's remaining role, the pre-existing carriage-over-carriage
  rebase, now fails loudly in the private worktree if it ever lacks the
  attribute, and the design says so instead of pinning it.
- **LRD-03 (high).** Decision 4 chooses the narrowly proved compatibility
  read: version 1 is accepted under version 1's own four-exact-bindings
  rule, mixed forms are refused, and both mixed forms are pinned. The
  section says why the version field makes a raw-versus-filtered
  misreading impossible, and it states the crossover procedure the code
  cannot enforce on the pre-change side (`METASYSTEM_BIN` at a proof build
  of the candidate), with what happens if the seat forgets it.
- **LRD-04 (high).** 3a now has two ordered rule lists, one per mode. With
  `--require-empty-index` every non-blank index column is `staged` before
  the register exception; without it the staged-candidate-plus-register-
  append case stays tolerated. The append shape is part of the tolerance,
  so a rewritten register is `register-not-append` in both modes. The
  fixture list names the mode for every shape and asserts both.
- **LRD-05 (medium).** The shell refusal canary stages a fresh non-register
  candidate under a second chain record, mints its receipt, then adds the
  unstaged payload edit, and asserts the exact "unstaged changes remain
  after staging" line plus the drift verb's tab line and the absence of
  the commit step. The passing canary reuses that candidate and receipt.
- **Corrections not asked for.** "Transport needs the whole worktree
  clean" was wrong (`sync-transport.sh` pushes refs only) and is replaced
  by what the post-commit check protects; the `worktreeProjection` field
  is a pointer with `omitempty` so a version-1 file decodes; the fixture
  seed no longer tracks a `.gitattributes`, since nothing depends on it.
