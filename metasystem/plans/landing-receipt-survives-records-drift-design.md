# Design: a landing receipt that survives an append to a register

Goal: `plans/goals/landing-receipt-survives-records-drift.md`.
Revision 3, folding the round-2 read
(`records/misc/landing-receipt-survives-records-drift-critique-r2.md`) of
revision 2 (f4abc699). Every line number this revision adds (`lease/`,
`refusal/`, `gittree.go:104-144`, `snapshotscope.go:41`,
`observe.go:647-677`, `retrodebt/debt.go`, `narratordigest/digest.go`,
`commit.sh:31-40` and `:539-578`, `.gitattributes`) was read at e8b6e6e8;
the citations carried over from revision 2 were not re-read, and no
commit between f4abc699 and e8b6e6e8 touched the files they name. Paths
are relative to `metasystem/`. The revision record at the end names each
finding and what moved.

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
3. **Tolerate register appends; never carry, stash, or rewrite them.** Both
   land.sh cleanliness sites call `metasystem landing drift`, which
   tolerates exactly an unstaged append to a declared register and nothing
   else. The rebase runs in a private detached worktree, and the real
   checkout advances to the rebased commit with `git reset --keep` under
   the checkout mutation lock; the reset never touches a register that
   origin did not change. When origin did change a register this checkout
   is appending to, advance refuses with the bytes exactly where they are,
   and the seat lands them with the register-carriage landing that already
   exists. The narrator digest joins the behavior-surface coordination set
   so commit.sh's LANDING comparison stops refusing on it, the way it
   already does not refuse on the receipts log. Staging the drift as
   append-only carriage is rejected because it moves the candidate tree
   out from under the receipt.
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
`:77-83` reads the REAL workspace before the run and refuses unless both
postures equal the supplied tree; `:103-109` reads the ISOLATED candidate
(a detached worktree at the tree, `:88`) the same way; `:129-135` reads
the isolated candidate again after the command and refuses with "the
candidate changed while the command ran"; `:143-153` builds `Binding` from
the four CANDIDATE reads.

**Correction 1.** The brief says the after bindings are the real
workspace's postures. They are the isolated candidate's (`:129`, `:151`);
`TestCreateTestReceiptIgnoresLiveWorkspaceMotion` (`receipt_test.go:16-58`)
proves today that a live append during the battery does not break them.
What breaks is the landing-time read in `readTestReceipt` (`:306-309`):
"the index or working tree moved after the test receipt was created". The
battery is discarded on the tier-1 path because `create_test_receipt`
(`land.sh:382-388`) re-runs the command on every attempt and
`CreateTestReceipt` removes the previous receipt first (`:59-62`); on the
chain path the receipt file survives a refusal.

`readTestReceipt` (`:260-311`): `:290` requires schema 1; `:296-305` all
four bindings equal to the candidate; `:306-309` both real postures too.

**The candidate tree.** `land.sh:351-359` `staged_candidate_tree` is `git
write-tree` on the real index, subtree by prefix. `commit.sh:292` and
`:421` take the same write-tree and `:443-447` resolve the prefix subtree
for `landing observe`. None of these read the worktree.

**The land.sh cleanliness sites.** `stage_changes` (`land.sh:315-334`)
refuses at `:324-326` on unstaged changes and at `:328-333` on untracked
paths. `require_clean_after_commit` (`:390-398`) refuses when `git status
--porcelain --untracked-files=normal` prints anything; it runs at `:557`
on the ordinary path and at `:550-555` on the recertified path, where a
refusal parks the chain with `chain-recertification-source-changed`.
`rebase_origin` (`:404-406`) is a plain `git rebase` at `:571` and inside
the push retry loop at `:589`; its step names are asserted by
`land-fixtures.sh:437` and `:498` and are kept.

What the post-commit drift check protects: the checkout's immediate
posture after the commit, index and worktree, against a change that
appeared while commit.sh ran. It does not protect transport:
`sync-transport.sh:31-35` fetches origin's branch head into the tracking
ref and pushes that ref, reading no worktree. It does not protect the
proof-to-commit equation either: commit.sh proves that itself, comparing
the landed tree with the proved tree and rolling the commit back softly on
a mismatch (`commit.sh:539-578`). Revision 1's phrase "transport needs the
whole worktree clean" was wrong and is withdrawn; the message text at
`land.sh:394` is kept unchanged so no fixture moves.

**The lock commit.sh holds, and where it ends.** commit.sh re-executes
itself under `lease run-held` (`commit.sh:31-40`); `RunHeld`
(`lease/verbs.go:464-482`) takes the checkout's flock at
`artifacts/agents/mains/worktree-lease.lock` (`lease.go:54-62`) through
`acquireBounded` (`lock.go:31-53`: a bounded wait of `lockWaitSeconds()`,
10 s by default, `METASYSTEM_LEASE_LOCK_WAIT_SEC` overrides) and releases
it when the wrapped child exits; a HUMAN caller runs ungated (`:470-471`).
The post-commit check, fetch, rebase and push (`land.sh:547-592`) run with
the lock released. `LockBounded(path, what string) (func(), error)`
(`lock.go:67-73`) is the exported form; `landing` already imports `lease`
(`park.go:21`) and `lease` does not import `landing`.

**Correction 2, a sixth site.** `commit.sh:321-342` enumerates `git diff
--name-only` at the toplevel plus untracked files, filters them through
`behavior-surface select --projection LANDING`, and refuses: "the LANDING
comparison found projected working-tree bytes that are not what the commit
would record at its index endpoint"; `:425-431` repeats it after the
proofs. LANDING membership (`behaviorsurface/policy.go:279-301`) excludes
every `coordinationPaths` match, and `policy.v2.json:29-35` lists
`memory/receipts.log` but not `records/narrator-digest.log`: a dirty
receipts log passes commit.sh today and a dirty digest does not. The
fixture stub of commit.sh (`land-fixtures.sh:51-135`) has no such check,
so a unit test on the policy pins this site.

**The two engines at a landing.** `land.sh:14` selects
`${METASYSTEM_BIN:-$root/bin/metasystem}` for every landing verb, from the
brain fence at `:117` to the receipt mint at `:386`. `commit.sh:297-319`
builds a separate proof engine from the checkout's source (`go-gate.sh
--fast --proof-out`, `go-gate.sh:463-468`) and reads the receipt with it
at `:463`. `go-build.sh:38-66` stamps a build `dev-<commit>-dirty`
whenever the ENGINE projection differs from HEAD, and automatic enrollment
refuses that stamp (`dispatch.sh:235-271`). So on the landing that lands
this change the live binary is necessarily older than the candidate, and
it cannot be rebuilt and re-armed from the dirty tree.

**The filtering primitive.** `gittree.FilterTree` (`gittree.go:284-312`)
reads a tree into an isolated index and runs `update-index --force-remove`
from the toplevel with the paths verbatim, so a workspace-space tree is
filtered by workspace-relative names (`:296-299`; `gittree_test.go:666-686`
proves it nested). Its one caller today is `missionrunner/wall.go:510-526`.

**The detached worktree primitive.** `gittree/detached.go:26-77`
`NewDetachedWorktree(tree)` runs `git worktree add --detach <tmp> HEAD`
under `os.MkdirTemp` (`:38`, `:53`), then grafts the candidate tree into
it (`:58-68`); `Close` (`:116-131`) removes it with `worktree remove
--force --force` and never runs `worktree prune`. Advance needs only the
first half and gets a sibling constructor.

**The git wrapper is package-private.** `gittree.go:104-144` `git`,
`gitTop`, `gitAt` and `snapshotscope.go:41` `gitProbe` are the bounded,
config-pinned, environment-scrubbed ways this tree runs git; `gitProbe`
types a nonzero exit as an answer and a failed spawn or timeout as
`RunFailure` (`:27-36`). Neither is exported, and the package exports no
rebase, abort, reset or ancestor operation; 3e adds them by name.

**The writers and their consumers.** `narratordigest.Append`
(`digest.go:133-172`) and `AppendPayload` (`:177-217`) take an exclusive
flock (`:90-115`), read the whole file, and replace it through
`atomicfile.WriteText`. `receipt.appendLine`
(`internal/receipt/receipt.go:445-459`) opens the receipts log with
`O_APPEND|O_CREATE` per call and writes one line; no lock exists. Their
lines reach main through register-carriage landings (a1273d6d, 937b30eb,
both `Landing-Provenance: direct-fix class=register-carriage`), so the
unstaged register bytes are real work in flight that the seat's manual
`git checkout --` has been discarding. Their consumers bind exact
prefixes: `retrodebt.Raise` records the receipts log's length and the
SHA-256 of those bytes (`debt.go:178-181`) and `Open` refuses "receipt
ledger prefix changed beneath retro debt" when that prefix no longer
hashes the same (`:211-212`); the digest cursor's `Pending` refuses
"narrator digest changed before the last check-in cursor"
(`digest.go:268-269`) and `Advance` refuses a cursor that does not name
the emitted prefix (`:298-299`).

**Git facts probed at design time** (git 2.50.1 Apple Git-155, scratch
clones of a bare origin under a private temporary root, with the harness's
`GIT_OBJECT_DIRECTORY` and `GIT_ALTERNATE_OBJECT_DIRECTORIES` unset the way
`land-fixtures.sh:15` does; not fixtures):

- `git rebase <upstream>` refuses with "cannot rebase: You have unstaged
  changes" even when the branch is already up to date.
- The stash ref is one file under the common directory: every worktree
  and every session on the machine push to and pop from the same list
  (verified by the round-1 critic on two worktrees of this repository).
  That is why the sentinel-stash canary exists.
- In a private detached worktree, `git rebase <upstream>` leaves the real
  checkout's branch, index, and dirty register untouched, on success and
  on a content conflict (`UU` in the private worktree, `rebase --abort`,
  `worktree remove`, real checkout still at the landing commit, ` M`).
- `git reset --keep <N>` with a dirty register identical between HEAD and
  N: exit 0, branch moved, the register's bytes untouched, status ` M`;
  git keeps the index entry's stale stat and never opens the file.
- `git reset --keep <N>` with a dirty path that differs between HEAD and
  N: "error: Entry 'reg.log' not uptodate. Cannot merge." exit 128, and
  nothing changed. This all-or-nothing is what makes a refused reset safe.
- `git update-index --force-remove` accepts a path absent from the index,
  so `FilterTree` is safe on a tree that lacks a register.
- `git status --porcelain` prints toplevel-relative paths from a
  subdirectory.
- `git check-attr merge` answers `union` for both registers, from
  `.gitattributes:1-2`.

## Decision 1: where the register set is declared

New file `internal/landing/registers.go`:

```go
// appendOnlyRegisters names the tracked registers that background writers
// and turn-boundary hooks append to at times a landing does not control.
// Paths are workspace-relative, the landing package's one path space.
// One declaration, four consumers: recordCarriageError carries a register
// only append-only, receiptPosture excludes it from the working-tree
// projection, WorktreeDrift tolerates an unstaged append to it, and
// Advance refuses to move the checkout over an upstream change to it.
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

Why not elsewhere: `gittree` is generic and names no metasystem path
(its callers pass their own lists, `wall.go:514`); the path-class manifest
(`scripts/agents/path-classes.txt:29,31`) classes `memory/` and `records/`
by directory, cannot name two files, and is loaded from the landing base
tree (`observe.go:259`), so the receipt's reads would depend on which base
is checked out; `behaviorsurface/policy.v2.json` is a different policy
(which bytes the static proofs bind) that must agree with the declaration
but does not own it; `stateroot` owns the directories
(`stateroot.go:218-227`), not the files.

Pins, in `internal/landing/registers_test.go` (package `landing` already
depends on `dispatch`, which imports `behaviorsurface`, so the test import
adds no cycle; `behaviorsurface` imports nothing from `landing`):

1. `appendOnlyRegisters` equals `{RelativeRoot(Receipts)+"/receipts.log",
   RelativeRoot(Records)+"/narrator-digest.log"}`.
2. For every declared path, `policy.Includes(Landing, "metasystem/"+path,
   "metasystem/")` is false and `Includes(Landing, path, "")` is false.
   This is the sweep that catches a register added to the Go declaration
   without the policy row.

Revision 1's third pin, a literal `merge=union` line in `.gitattributes`
per register, is withdrawn: no step of this design merges register bytes.
The attribute acts only in the rebase of a register-carriage commit over
an upstream change to the same register, which exists today
(`.gitattributes:1-2`) and now happens in the private worktree, where a
conflict is a loud `advance-rebase-conflict`, not a silent loss.

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

Both use `workspace.FilterTree(x, appendOnlyRegisters)`. A filtered
projection can never equal the raw candidate tree, so it is compared
against `receiptIdentity(tree)`, the way `wall.go:580-596` compares
observed against committed with both sides filtered.

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
  worktree is discarded at `:136`, the registers are bookkeeping, and the
  command's verdict on T cannot depend on bytes the battery itself
  appended. All four reads go through one function so the bindings have
  one meaning everywhere.
- Content is not examined by the receipt: a rewritten or deleted register
  is as invisible to it as an appended one. The drift verb examines the
  shape (3a) and the `:779` rule examines content when someone lands it.

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
([]StatusEntry, error)` in `snapshotscope.go`, built on `gitProbe` (`:41`):
toplevel-scoped, `-z` parsed, fields `Index byte`, `Worktree byte`, `Path
string` (toplevel-relative), no rename records. Paths are mapped to
workspace space by stripping `Workspace.Prefix()` (`gittree.go:195`); a
path outside the prefix is never a register.

The append shape, `isRegisterAppend(workspace, path) (bool, error)`: the
worktree entry is a regular file (`os.Lstat`); the index blob is
`FileAt(StagedTree(), path)` (`gittree.go:443-457`); the file's bytes have
the index blob as a byte prefix and end in a newline. This is
`appendOnly`'s rule (`observe.go:971-981`) applied to index versus
worktree, so what the drift verb tolerates is exactly what the carriage
rule will later accept. An index blob that is the whole file cannot show
as ` M` except through a stale stat, which `git status` refreshes, so
equality is not a case.

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
register append: <path>`. Exit 0 with no drift, 1 with drift, 2 on usage
or git failure.

**3b. land.sh.**

- `stage_changes` `:324-333`: replace the `git diff --quiet` and
  `ls-files --others` pair with one `"$ms" landing drift --root "$root"`
  call captured into a variable. On exit 1: if any line is of kind
  `unstaged` or `register-not-append`, print the existing message "land
  refused: unstaged changes remain after staging; transport requires a
  clean tree after commit", otherwise the existing untracked message; then
  the lines indented; return 2. The empty-staging-set check at `:320-323`
  stays first and unchanged.
- `require_clean_after_commit` `:390-398`: replace the `git status` call
  with `"$ms" landing drift --root "$root" --require-empty-index`. On exit
  1 print the existing message "land refused: commit succeeded but the
  tree is not clean, so transport will not start" and the lines; return 1.
  The recertified park at `:550-555` is unchanged.
- `rebase_origin` `:404-406` becomes `"$ms" landing advance --root "$root"
  --upstream "refs/remotes/origin/$branch"`. The step names at `:571` and
  `:589` and the retry loop at `:588-589` do not change. An advance
  refusal exits 1 with `advance refused: <code>: <detail>` on stderr;
  `run_required_step` fails the step and land.sh exits with the commit
  made and unpushed, the state every repair route in 3e starts from.
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
`internal/behaviorsurface/consumer_wiring_test.go:84`. The implementer
runs the behaviorsurface tests and `static-reproof-fixtures.sh` after the
edit, because that run is the record.

**3d. Why not carriage.** Staging the drifted registers into the landing
commit fails three ways. After the receipt exists, staging a register
changes the commit tree, so `receipt.Tree != CandidateTree` at
`receipt.go:290` and `land.sh:376` void the receipt. Staging before the
receipt leaves the drift during and after the battery uncovered, which is
the defect. And a chain landing that carries register bytes without
`--direct-fix register-carriage` refuses `chain-has-uncarried-paths`
(`observe.go:284-286`). The writers' bytes stay in the worktree for the
register-carriage landing that already exists for them.

**3e. The advance verb: the rebase without a stash.** `metasystem landing
advance --root <root> --upstream <ref>`, implemented by
`landing.Advance(root, upstream string, stdout, stderr io.Writer) error` in
a new `internal/landing/advance.go`, replaces `git rebase` in the real
checkout. Its design rule: the verb never opens, writes, moves or restores
a register file; when origin changed a register this checkout is appending
to, it refuses and the seat lands the bytes by register carriage.

Names: L is the landing commit (HEAD at entry); U is the commit the
upstream ref names; N is L rebased onto U. Refusals are one type,
`advanceRefusal{code, detail string}`, whose `Error()` is `advance refused:
<code>: <detail>`, printed to stderr with exit 1; usage errors and a git
that could not run (`gittree.RunFailure`) exit 2.

**The lock.** `Advance` takes the checkout mutation lock first and holds
it until it returns:

```go
release, err := lease.LockBounded(lease.LockPath(root), "landing advance")
if err != nil { return advanceRefusal{"advance-checkout-locked", err.Error()} }
defer release()
```

`LockBounded` (`lock.go:67-73`) returns the release function or the
bounded refusal from `acquireBounded`. `lease.LockPath(root string)
string` is new: one exported line returning `leasePaths(root).Lock`
(`lease.go:54-62`), the flock `RunHeld` takes around commit.sh's
`__lease-held` child. Held from precondition A through the HEAD check
after the reset, it means an agent commit and an advance on one checkout
never overlap, and two advances never overlap.

The refusal keeps today's text (`lock.go:47-49`: another lease-gated
operation holds it). Coordinator's disposition on revision 3: a holder
note would touch a primitive every lease holder shares for one message;
the seat waits and retries on the text as it is.

What the second of two concurrent advances sees: it waits in
`acquireBounded` for up to the bounded wait. A private rebase of this
repository takes well under a second, so usually the first finishes, the
second acquires the lock, reads HEAD, which is now N, finds U an ancestor
of it, and exits 0 through the fast path; its land.sh then pushes what is
already pushed. When the first holds past the wait, the second refuses
`advance-checkout-locked`, and nothing was touched. What the seat does, here and for
every refusal below that leaves the commit made and unpushed: deal with
the named condition, then run land.sh's three post-commit steps by hand,
`git fetch origin +refs/heads/<branch>:refs/remotes/origin/<branch>`,
`metasystem landing advance --root <root> --upstream
refs/remotes/origin/<branch>`, `git push origin <branch>`, then
`sync-transport.sh <branch>`.

**Steps**, all under the lock:

A. **Preconditions, nothing touched.** `SymbolicHead()`
   (`snapshotscope.go:102`) names a branch, else refuse
   `advance-not-on-branch`. `HeadCommit()` (`:76`) is L.
   `TopStagedPosture()` (`:308`) has no unmerged entries and its tree
   equals `ResolveRef(L+"^{tree}")`, else refuse `advance-index-not-empty`.
   `ResolveCommit(upstream)` (`disjoint_merge.go:655`) is U; an
   unresolvable upstream is a git failure, exit 2.
B. **Fast path.** `IsAncestor(U, L)` true: print `advance: up to date with
   <upstream>` and return 0. On a serialized seat this is the usual outcome.
C. **Private rebase.** `NewDetachedCommitWorktree(L)`, then
   `Rebase(upstream)` in it. `Conflicted` (git has already aborted):
   `Close`, refuse `advance-rebase-conflict` with git's output; the real
   checkout is untouched (probed). Otherwise N is `Head`; `Close`. The
   worktree's own `.gitattributes` are in force there, so a
   register-carriage commit merges the way it does today.
D. **Classification, nothing touched.** `changed` is
   `ChangedPaths(ResolveRef(L+"^{tree}"), ResolveRef(N+"^{tree}"))`
   (`gittree.go:331`, toplevel trees, so toplevel-relative paths like
   `Status()`'s); `dirty` is `Status()`. Any entry in `dirty` with a
   non-blank index column refuses `advance-index-not-empty` (only a
   writer that bypasses the lock can produce one now; the check stays).
   The overlap is the dirty paths also in `changed`, mapped to workspace
   space for the register test, and is classified in full before any
   refusal, in this precedence: a register that N deletes or whose mode N
   changes refuses `advance-register-removed`; a path that is not a
   declared register, or a register without the append shape of 3a,
   refuses `advance-unstaged-drift` naming the path and N; otherwise every
   overlap path is a register with the append shape and the verb refuses
   `advance-register-contended` naming each register and N.
E. **Common path, overlap empty: compare, then swap.** Re-read
   `HeadCommit()` and `SymbolicHead()`; unless HEAD is still L on the same
   branch refuse `advance-head-moved` naming L and what HEAD is now. Then
   `ResetKeep(N)`. `Moved` false: refuse `advance-unstaged-drift` with
   git's output; the branch is still at L (the all-or-nothing probed
   above). Re-read `HeadCommit()`; unless it is N refuse
   `advance-head-moved`. Print `advance: <L> -> <N>; registers untouched:
   <dirty registers>` and return 0. The dirty registers are never opened
   (probed: git keeps the index entry and its stale stat).

The compare-and-swap, stated exactly: the compare is the last read of
HEAD before the swap, and the swap is git's own `reset --keep`, which
moves the branch only when every path it must check out is clean. Under
the lock no lease-gated writer (an agent's commit.sh, another advance) can
act between the two. The lock does not fence a writer that never takes
it, a HUMAN's commit.sh (`verbs.go:470-471`) or a raw git command; against
those the compare sits two process spawns from the swap, a reset they race
is in the branch's reflog, and the post-reset check names the result
instead of reporting success.

**The repair route for `advance-register-contended`.** The bytes stay
where they are; the verb has not opened the file. The seat lands them
with the register-carriage landing that already exists: `land.sh
--direct-fix register-carriage` with the named registers as pathspecs.
commit.sh's `landing observe` judges the carriage against current HEAD
(`registerCarriage`, `observe.go:647-649`, takes `workspace.HeadTree()`
as its base), which is L, so the carriage commit C sits on top of the
unpushed landing commit. That landing's own fetch, advance and push steps
(`land.sh:570-591`) then run: advance finds the registers clean and the
overlap empty, the common path moves the branch to L and C rebased onto
origin, and the push carries both. The rebase of C over origin's change
to the same register is the carriage-over-carriage case that exists
today, governed by `merge=union` (`.gitattributes:1-2`) in the private
worktree; a conflict there is `advance-rebase-conflict` with the real
checkout untouched. That merge rewrites the worktree register the way
today's rebase does; this design neither widens nor fixes it.

Why re-emission was never an option. Revision 2 restored the local append
by writing the rebased blob and then the local suffix into the register.
Every local consumer that had recorded an offset into those bytes,
`retrodebt` (`debt.go:178-181`, refused at `:211-212`) and the digest
cursor (`digest.go:268-269`, `:298-299`), would then name a prefix that no
longer hashes the same, and refuse: the landing would have manufactured,
outside the writers' own protocols, a file its own readers reject. The
carriage landing commits the worktree bytes byte for byte.

**The gittree operations advance needs.** Every git call in `Advance`
goes through one of these; package `landing` never reaches for the
standard library's `exec` package. Each is built on `gitProbe`
(`snapshotscope.go:41`): a nonzero exit is a typed answer, a failed spawn
or timeout is a `RunFailure`, and the local timeout applies.

```go
// IsAncestor: merge-base --is-ancestor. Exit 0 true, exit 1 false, any
// other exit an answer error.
func (w Workspace) IsAncestor(ancestor, descendant string) (bool, error)
// NewDetachedCommitWorktree: NewDetachedWorktree's sibling, worktree add
// --detach under os.MkdirTemp at the given commit, no graft, same Close.
func (w Workspace) NewDetachedCommitWorktree(commit string) (*DetachedWorktree, error)
type RebaseResult struct {
	Head       string // the rebased commit when Conflicted is false
	Conflicted bool   // git rebase exited nonzero; rebase --abort has run
	Output     string // git's stderr and stdout when Conflicted
}
// Rebase runs git rebase <upstream> in the detached worktree; an abort
// that itself fails is joined into the error. Head is HeadCommit() after.
func (d *DetachedWorktree) Rebase(upstream string) (RebaseResult, error)
type ResetKeepResult struct {
	Moved  bool   // exit 0: the branch, index and changed paths moved
	Output string // git's stderr when Moved is false
}
// ResetKeep runs git reset --keep <commit> from the repository toplevel.
func (w Workspace) ResetKeep(commit string) (ResetKeepResult, error)
```

Already exported and used as they are: `HeadCommit`, `SymbolicHead`,
`TopStagedPosture`, `ResolveCommit`, `ResolveRef`, `ChangedPaths`,
`StagedTree`, `FileAt`, `Prefix`, `TopLevel`; `Status` is added by 3a.

**The refusal register.** `internal/refusal/register.go` gets one row per
advance code in `Rows` (`:32`): shape `Agent`, owner `internal/landing`,
site the emitting line in `advance.go` as landed, override the repair
route below, and its command count (the three post-commit steps by hand
count three).

| Code | Override | Commands |
| --- | --- | --- |
| `advance-checkout-locked` | wait, then fetch, advance, push | 3 |
| `advance-not-on-branch` | `git switch <branch>`; fetch, advance, push | 4 |
| `advance-index-not-empty` | land or unstage the staged paths; fetch, advance, push | 4 |
| `advance-rebase-conflict` | carry dirty registers first, then `git rebase` in the real checkout and resolve; push | 4 |
| `advance-unstaged-drift` | land or restore the named path; fetch, advance, push | 4 |
| `advance-register-removed` | restore the register from the upstream by hand; fetch, advance, push | 4 |
| `advance-register-contended` | `land.sh --direct-fix register-carriage <registers>` | 1 |
| `advance-head-moved` | read `git reflog <branch>`; fetch, advance, push | 4 |

None is excluded. The rows are written by hand: the register's test
(`register_test.go:16-47`) collects hyphenated codes only when they end in
`refused`, `unreadable`, `malformed` or `unavailable` (`:17`), and in
package `landing` only `carriageError{code:}` literals and `wouldRefuse`
arguments (`:142-144`, `:175-199`); the advance codes have none of those
suffixes and live in `advanceRefusal`, so a missing row would never fail
the test. The other two register tests pass as written: no override
begins with `goal`, no row is pending, every `Agent` row has an override.

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
pointer with `omitempty` lets a version-1 file decode without the object;
a version-2 file never omits it. The `binding` field names stay, with the
projection object telling a reader what the two worktree keys mean. The
comment on `TestReceipt` (`:28-29`) says the worktree observations are
filtered in version 2 and raw in version 1, the index ones exact in both.

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
tree), so every fact a version-2 receipt asserts is asserted by a
version-1 receipt a fortiori; the rule accepts nothing weaker than
version 1 demanded.

Why it cannot reintroduce the silent raw-versus-filtered misreading: the
comparison target is selected by the version field, written in the same
struct literal as the bindings (`:143-153`), so a file's version and the
meaning of its bindings travel together; version 2 compares filtered with
filtered, version 1 raw with raw. The two mixed forms are loud: a
version-1 receipt whose worktree bindings hold `receiptIdentity(T)` fails
rule 3; a version-2 receipt whose worktree bindings hold raw T fails rule
2 whenever T contains a register, which every real candidate does. Both
are pinned in the fixture list. What made revision 1 refuse version 1 was
a reader that kept the field names and did not move the version; that
reader does not exist here.

The crossover, stated as procedure because the code cannot enforce it on
the pre-change side: the landing that lands this change runs land.sh with
`METASYSTEM_BIN` set to an engine built from the candidate (`scripts/
agents/go-gate.sh --fast --proof-out <path>`, the build commit.sh makes at
`:302`), because `landing drift` and `landing advance` exist only there
and the live binary cannot be rebuilt and re-armed from the dirty tree
(`go-build.sh:38-66`). With that engine the tier-1 path mints and reads
version 2. On the chain path the seat mints the receipt with the same
engine; a receipt minted with the stale live binary is version 1 and rule
3 accepts it, so the battery is not lost either way. If the seat forgets
`METASYSTEM_BIN`, the live binary fails at `stage_changes` on the unknown
verb, before the battery on the tier-1 path and before the commit on the
chain path, where the receipt file survives for the retry. After the
landing the engine is rebuilt and re-armed as `dispatch.sh:269` demands.

The reverse crossover, a candidate whose engine source is older than the
live binary (an exact revert, a stale branch), mints version 2 and reads
it with a pre-change proof engine, which refuses on the unknown field:
loud, the same as for any receipt field ever added, and outside this goal.

The version-1 rule stays in the reader; its removal is a one-line change
with the pin flipped to a refusal, is crossover-safe because both engines
then agree on version 2, and is not scheduled by this design.

## What must not change, and how each is kept

- Index tree and candidate tree exact: `StagedTree` untouched; the index
  comparisons at `:81`, `:107`, `:133`, `:296-305`, `:307` stay exact; the
  write-trees in land.sh and commit.sh are untouched.
- The `:779` rule stays; only its case expression reads the declaration.
  The drift verb's append shape is exactly what that rule accepts.
- Non-register drift between battery and landing refuses with the existing
  message: `:308` keeps its text; the Go refusal canary proves it before
  and after, and the shell refusal canary reaches the same refusal at
  `stage_changes` with a real staged candidate and receipt.
- `Snapshot`, `StagedTree`, and `FilterTree` keep their contracts.
- No landing verb writes a register: drift reads them, advance never does.

## Fixtures

Go tests in `internal/landing`, proving run
`go test ./internal/landing/ -run 'TestCreateTestReceipt|TestReadTestReceipt|TestWorktreeDrift|TestAdvance|TestAppendOnlyRegisters'`,
ceiling three minutes (receipt commands are `true` and short `printf`
lines; each advance test creates one private worktree):

- **Passing canary.** `TestReadTestReceiptSurvivesRegisterAppendAfterReceipt`:
  `newObserveFixture` (it tracks both registers, `observe_test.go:64-65`);
  stage a change to `product.txt`; T is `StagedTree`; `CreateTestReceipt(T,
  "true")`; append one line to `records/narrator-digest.log` in the live
  root; `readTestReceipt` for T returns nil. Repeat with
  `memory/receipts.log`. The implementer runs this test against the
  untouched tree first and records that it fails with "the index or
  working tree moved after the test receipt was created".
- **During-the-battery canary.** Extend
  `TestCreateTestReceiptIgnoresLiveWorkspaceMotion` (`receipt_test.go:16-58`):
  the expected worktree bindings become `receiptIdentity(candidate)`, the
  index bindings stay `candidate`, and afterwards `readTestReceipt`
  against the live root, whose digest carries the mid-command line,
  accepts. Add `TestCreateTestReceiptToleratesCandidateRegisterAppend`:
  the command appends to the digest in its own working directory (the
  isolated candidate) and the receipt is still created.
  `TestCreateTestReceiptRefusesIsolatedCandidateMotion` (`:60-90`) stays
  green.
- **Refusal canary.** `TestReadTestReceiptRefusesNonRegisterDrift`: the
  passing canary with `product.txt` appended instead; the error contains
  the message at `:308`. A second case stages an append to
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
  contains both registers, so (b) and (c) are real refusals.
- **Drift rule, both modes.** `TestWorktreeDrift` over the shapes, each
  asserted with and without `--require-empty-index`: clean; ` M` register
  append tolerated in both; ` M` register with a rewritten first line
  `register-not-append` in both; `MM` and `AM` register appends tolerated
  without the flag and `staged` with it; `TM` register (the file replaced
  by a symlink after a staged type change) `unstaged` without the flag and
  `staged` with it; `M ` product tolerated without the flag and `staged`
  with it; ` M` product `unstaged` in both; ` D` register `unstaged` in
  both; `??` `untracked` in both; the nested `metasystem/` prefix of
  `newObserveFixture` and an adopted toplevel fixture's empty prefix both
  map correctly.
- **Advance, common path.** `TestAdvanceLeavesRegistersUntouched`: a bare
  origin cloned twice, the seed tracking both registers and a
  `.gitattributes` with the two `merge=union` lines; the peer pushes a
  `product.txt` change; the local clone commits a
  `scripts/agents/go-gate.sh` change, appends to both registers, runs
  `Advance`; the branch is at the rebased commit, both registers' worktree
  bytes are byte-identical, status shows exactly the two ` M` lines, `git
  worktree list` has one line, the stash list is empty, and a second
  `Advance` prints the up-to-date line.
- **Advance, contended register, and the repair route.**
  `TestAdvanceRefusesContendedRegister`: as above but the peer pushes a
  digest append and the local clone has an unstaged digest append.
  `Advance` refuses `advance-register-contended` naming
  `records/narrator-digest.log` and N; the branch is at L; the digest's
  bytes are byte-identical; status is the one ` M` line; one worktree.
  Then the test commits the digest on top of L by hand (the carriage
  commit's shape) and runs `Advance` again: the branch is at the rebased
  commit, status is clean, and the committed digest has exactly three
  lines, the seed line first and the peer's and the local line each once
  (their order is git's union order, not asserted).
- **Advance refusals.** `TestAdvanceRefusals`: a dirty non-register that
  origin changed refuses `advance-unstaged-drift` with the branch at L and
  the file intact; a conflicting rebase refuses `advance-rebase-conflict`
  with the branch at L and no leftover worktree; a register origin deleted
  refuses `advance-register-removed`; a staged path refuses
  `advance-index-not-empty`; a detached HEAD refuses
  `advance-not-on-branch`. For `advance-head-moved` a package-level hook
  `advanceBeforeSwap func()`, nil in production and injectable like
  `receipt.syncFile` (`internal/receipt/receipt.go:462`), runs between D
  and the compare of E; the test's hook commits on the branch with raw
  git, and `Advance` refuses `advance-head-moved` naming L and that commit.
- **Advance under the lock.** `TestAdvanceRefusesWhenCheckoutLocked`
  holds `lease.LockBounded(lease.LockPath(root), "test-holder")`, sets
  `METASYSTEM_LEASE_LOCK_WAIT_SEC=0`, and `Advance` refuses
  `advance-checkout-locked` with a message containing `held by test-holder
  pid=`; HEAD is unchanged. In `internal/lease`,
  `TestLockRefusalNamesHolder`: one `acquireBounded` holds, a second with
  the zero wait refuses naming the first's note; after release the file is
  empty.
- **Pins.** `TestAppendOnlyRegistersPins`, the two pins of Decision 1.
- **Policy row.** The digest row in `behaviorsurface/policy_test.go`.

Shell canaries in `scripts/agents/land-fixtures.sh`, scenario
`full-width-chain`, after the matching-receipt landing at `:741-769`.
Proving run: `bash scripts/agents/land-fixtures.sh` (the scenario harness
at `fixture-bed-scenarios.sh:32-79` runs each leg as its own child, so the
whole bed is the runnable unit; its per-leg cap is the ceiling). The bed
uses the real engine (`land-fixtures.sh:50`) and a reduced commit.sh that
performs the real `landing observe` with `--test-receipt` (`:109-123`), so
the receipt's landing-time read is exercised. The reduced commit.sh takes
no lease lock; advance takes it on the leg's own `artifacts/agents/mains/`
lock file, which the seed's `.gitignore` keeps out of every status.

- **Seed.** Inside the `full-width-chain` branch of `make_leg`
  (`:136-146`, `:161-163`) write and track `memory/receipts.log`
  (`receipt=seed\n`), `records/narrator-digest.log` (`digest=seed\n`), and
  a `.gitattributes` with the two `merge=union` lines, because the repair
  leg rebases a carriage commit over the peer's change to the same register.
- **Second candidate and its receipt.** After the matching-receipt landing
  the index is clean, so a fresh candidate is needed: append a second line
  to `scripts/agents/go-gate.sh`, `git add` it, write a second chain record
  `full-chain-2` and its `rounds/1/{diff.patch,review.json}` the way
  `:675-697` does for the new candidate tree, and mint its receipt with
  `$source_engine landing test-receipt` and `$full_battery_command`.
- **Refusal canary, first.** Append `payload=drift\n` to `payload.txt`
  (tracked, not a register) in `leg_local`. Land `--chain full-chain-2
  --test-receipt <the new receipt> --staged-only --skip-transport`. Expect:
  exit 2; the exact line "land refused: unstaged changes remain after
  staging; transport requires a clean tree after commit"; a line
  `unstaged<TAB> M<TAB>payload.txt`; no `== STEP: commit`; HEAD unchanged.
  Restore the file with `git checkout -- payload.txt` (fixture cleanup).
  The staged candidate and its receipt survive the refusal and are reused
  below. Before the change this leg reaches the refusal through the old
  `git diff --quiet` at `:324`; the tab line tells the two apart.
- **Passing canary: one landing, common path, both registers dirty,
  sentinel stash, concurrent writer.** From `leg_peer`, append
  `payload=peer\n` to `payload.txt`, commit, push to origin. In
  `leg_local`: append `digest=drift\n` to the digest; push a sentinel
  stash by dirtying and stashing a scratch tracked path (`git stash push
  -m sentinel -- plans/existing.md`) and record `git rev-parse stash@{0}`;
  start a background appender by recorded PID that appends
  `receipt=bg-<n>\n` lines to `memory/receipts.log` every 10 ms and mirrors
  each line into `$leg_root/bg.log`. Land `--chain full-chain-2
  --test-receipt … --staged-only --skip-transport` (fetch, advance, and
  push still run, `:570-591`; only `sync-transport.sh` is skipped). Stop
  the appender by its PID. Expect: exit 0; origin main equals local HEAD;
  `git show HEAD:payload.txt` equals `seed\npayload=peer\n`; HEAD's two
  registers equal their seeds (no widening); the worktree digest equals
  `digest=seed\ndigest=drift\n` and the worktree receipts log equals
  `receipt=seed\n` followed by exactly the bytes of `bg.log` (the common
  path never opened either file, so every concurrent line is there in
  order); `git status --porcelain` is exactly the two ` M` register lines;
  `git stash list` has one entry and `git rev-parse stash@{0}` equals the
  recorded id; `git worktree list` has one line. This one landing proves
  `stage_changes` tolerance, the receipt's landing-time read under drift,
  `require_clean_after_commit` tolerance, the private rebase against a
  moved origin, the common path under a live lockless writer, and the
  untouched stash.
- **Contended canary and its repair.** From `leg_peer`, append
  `digest=peer\n` to the digest, commit, push. In `leg_local`, with the
  digest still dirty from the passing canary, land `-m <message>
  --direct-fix register-carriage --skip-transport -- memory/receipts.log`
  (the receipts log is carried; the digest is not staged). Expect: a
  nonzero exit at the step "rebase onto origin/main" with `== STEP:
  commit` before it; a stderr line `advance refused:
  advance-register-contended: records/narrator-digest.log` naming N; `git
  log -1 --format=%s` is the carriage message (L exists, unpushed); origin
  main unchanged; the worktree digest byte-identical to before; `git
  status --porcelain` exactly ` M records/narrator-digest.log`; one
  worktree; the stash entry and its id unchanged. Then the repair route:
  land `-m <message> --direct-fix register-carriage --skip-transport --
  records/narrator-digest.log`. Expect: exit 0; origin main equals local
  HEAD; `git show HEAD:records/narrator-digest.log` has exactly three
  lines, `digest=seed` first, `digest=peer` and `digest=drift` each once;
  `git show HEAD~1:memory/receipts.log` equals `receipt=seed\n` followed by
  the bytes of `bg.log`; `git status --porcelain` is empty; the worktree
  digest equals the committed digest; one worktree; the stash entry and
  its id unchanged. This leg proves the refusal leaves every byte in place
  and that the route it names carries both commits to origin.

## Implementation map

In this order, each step leaving the gate green:

1. `internal/gittree/snapshotscope.go`: `Workspace.Status()` and
   `StatusEntry`, `IsAncestor`, `ResetKeep` and `ResetKeepResult`;
   `internal/gittree/detached.go`: `NewDetachedCommitWorktree`, `Rebase`
   and `RebaseResult`; tests over a nested checkout, including a
   conflicting rebase that answers `Conflicted` and leaves no worktree and
   a `reset --keep` over a dirty changed path that answers `Moved` false
   with the branch unmoved.
2. `internal/lease/lease.go`: `LockPath`, one exported line.
3. `internal/landing/registers.go`, `registers_test.go`; the switch at
   `observe.go:773`.
4. `internal/landing/receipt.go`: `receiptPosture`, `receiptIdentity`,
   schema 2, the version rules; the receipt tests above.
5. `internal/landing/drift.go` and `drift_test.go`; the verb in
   `cmd/metasystem/landing_verbs.go` and its registry line in `main.go`.
6. `internal/landing/advance.go` and `advance_test.go`; the verb and its
   registry line; the eight hand-written rows in
   `internal/refusal/register.go` with their `advance.go` sites.
7. `internal/behaviorsurface/policy.v2.json`, the `policy_test.go` row,
   `consumer_wiring_test.go:84`, the three `static-reproof-fixtures.sh`
   case lists.
8. `scripts/agents/land.sh`: the three edits of 3b.
9. `scripts/agents/land-fixtures.sh`: the seed, the second candidate, and
   the three canaries.
10. The crossover landing itself, as the procedure in Decision 4: land.sh
    under `METASYSTEM_BIN` at a proof build of the candidate, the chain
    receipt minted with that build, rebuild and re-arm after.

No document describes the receipt's JSON fields or land.sh's clean-tree
rule (grep over `docs/`, `AGENTS.md`, `wow.md`, `development/`, `skills/`
finds neither), so the type comment in `receipt.go`, the doc comment on
`Advance`, and the registry summaries in `main.go` are the records.

## What the code cannot answer

- Whether the static-reproof bed needs the digest in its skip lists beyond
  the three listed case lists: answered by the run after step 7.
- Whether the eight refusals on 2026-09-09 included commit.sh's LANDING
  refusal on the digest: the goal record does not say. The design covers
  that site regardless.

## Revision record

Revision 3.1, 2026-09-09, coordinator's disposition: the holder note the
fold added to the shared lease lock primitive is struck; the lock refusal
keeps its existing text. Nothing else moves.

Revision 3, 2026-09-09, from the round-2 read
(`records/misc/landing-receipt-survives-records-drift-critique-r2.md`);
it goes to the build without a further design read.

- **LRE-01 (critical).** The rare case is deleted: steps F and G, the
  capture directory, the manifest, the link-a-complete-file step, the
  exported digest lock, the window table, the LOST rows, the restore
  self-check, the crash-resume paragraph, the refresh and livelock probes,
  and the two concurrent-writer Go fixtures that existed only for them.
  Step D's classification stays; a register in the overlap is one
  refusal, `advance-register-contended`, with the bytes untouched. The
  repair route and why re-emission was never an option are on the page.
- **LRE-02 (critical) and LRE-03 (high).** Fall with the rare case.
- **LRE-08 (critical) and LRE-04 (high).** Advance holds the checkout
  mutation lock for its whole run (`lease.LockBounded` on
  `lease.LockPath(root)`, the flock `RunHeld` holds around commit.sh), the
  branch move is compare-then-swap under it refusing `advance-head-moved`,
  and the lock's refusal names its holder. The page says what the second
  concurrent advance sees and what the seat does.
- **LRE-05 (medium).** The gittree operations advance needs are named
  with typed outcomes, all on `gitProbe`; they are step 1 of the map.
- **LRE-07 (medium).** The eight advance refusal codes get hand-written
  register rows; the page says why the test cannot flag their absence.
- **LRE-06 (low).** The post-commit check's rationale is corrected.
- **Consequential edits, not decision changes.** Decision 1's code
  comment and its `merge=union` paragraph, and the summary of Decision 3,
  described the deleted restore and now describe the refusal. The shell
  passing canary moves the peer's change from the digest to
  `payload.txt`, because a peer change to a dirty register is now the
  contended refusal, which has its own canary with the repair route; the
  shell seed gains a `.gitattributes` for that leg. Decisions 2 and 4, the
  drift verb's two rule tables, the common path and the private worktree,
  and the sentinel-stash canary are as landed.

Revision 2, 2026-09-09, from the round-1 read
(`records/misc/landing-receipt-survives-records-drift-critique-r1.md`):
LRD-01 withdrew `git rebase --autostash` for the private rebase and `git
reset --keep`; LRD-02 removed the `merge=union` pin; LRD-03 chose the
version-1 read and the crossover procedure; LRD-04 split the drift rule
into two per-mode tables; LRD-05 gave the shell refusal canary a real
staged candidate and receipt.
