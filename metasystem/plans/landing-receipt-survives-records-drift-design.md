# Design: a landing receipt that survives an append to a register

Goal: `plans/goals/landing-receipt-survives-records-drift.md`.
Revision read: d533caf1 (branch agent/lrsrd-design3). Every line number
below is from that revision. Paths are relative to `metasystem/`.

## The four decisions

1. **One declaration.** The register set lives in package `landing`, in a
   new file `internal/landing/registers.go`, as `appendOnlyRegisters`. The
   carriage classifier (`observe.go:779`), the receipt's posture reads, and
   a new `metasystem landing drift` verb all read that slice. Three pin
   tests keep the neighbours that also name these files in step: the
   writers' state roots, `.gitattributes`, and the behavior-surface LANDING
   projection.
2. **Filter the working-tree projection only.** At every one of the
   receipt's posture reads the projection is `FilterTree` over
   `Snapshot("HEAD")`, compared against the equally filtered candidate
   tree. Index trees, the candidate tree, and the commit tree stay exact.
   The orchestrator's reading holds; this design refutes none of it.
3. **Tolerate register drift, never carry it.** Both land.sh cleanliness
   sites call `metasystem landing drift`, which tolerates exactly an
   unstaged modification to a declared register and nothing else. The
   rebase runs with `--autostash` and a guard. The narrator digest joins
   the behavior-surface coordination set so commit.sh's LANDING comparison
   stops refusing on it, the way it already does not refuse on the receipts
   log. Staging the drift as append-only carriage is rejected because it
   moves the candidate tree out from under the receipt.
4. **Receipt schema 2.** The receipt gains a `worktreeProjection` object
   naming the excluded paths and the filtered candidate identity that the
   two worktree bindings must equal. The reader accepts only schema 2.

## The mechanism as read, with two corrections to the brief

**The carriage rule.** `observe.go:773-784` switches on the changed path.
`:774` handles `memory/rulings.md`; `:779` names `memory/receipts.log` and
`records/narrator-digest.log` and refuses anything but an append with
`register-carriage-not-append-only` through `appendOnly` (`:949-984`).
Nothing else in the package consults those two names.

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
at `:589`.

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

**The filtering primitive.** `gittree.FilterTree` (`gittree.go:284-312`)
reads a tree into an isolated index and runs `update-index --force-remove`
from the toplevel with the paths verbatim, so a tree in workspace path
space is filtered by workspace-relative names (`:296-299`;
`gittree_test.go:666-686` proves it in a nested checkout). Every caller
passes the mission's own ledger path (`missionrunner/wall.go:510-526`).

**Git facts probed at design time** (git 2.50.1, scratch repository with a
nested `sub/` workspace, not fixtures):

- `git rebase <upstream>` refuses with "cannot rebase: You have unstaged
  changes" even when the branch is already up to date.
- `git rebase --autostash <upstream>` succeeds both when upstream moved and
  when it did not, restores the register append after the rebase, leaves
  ` M` on the register, and leaves no stash entry. With `merge=union` on
  the path (`.gitattributes:1-2`) a remote append and the local append
  union into `seed, remote, local`, so the local line is still a suffix
  relative to the new HEAD.
- `git update-index --force-remove` accepts a path absent from the index
  (exit 0), so `FilterTree` is safe on a tree that lacks a register.
- `git status --porcelain` prints toplevel-relative paths from a
  subdirectory.

**The writers and where their bytes go.** `narratordigest.Append`
(`digest.go:133-164`) takes a flock and rewrites the digest through
`atomicfile.WriteText`. The steward writes the receipts log through
`PrepareIntent` (`cmd/metasystem/goal.go:699`, `steward_verbs.go:462`).
Their lines reach main through register-carriage landings: `git log --
memory/receipts.log` shows a1273d6d "Receipt for the stop verb landing"
and 937b30eb, both `Landing-Provenance: direct-fix class=register-carriage`.
The unstaged register bytes are therefore real work in flight, and the
seat's manual `git checkout --` has been discarding them.

## Decision 1: where the register set is declared

New file `internal/landing/registers.go`:

```go
// appendOnlyRegisters names the tracked registers that background writers
// and turn-boundary hooks append to at times a landing does not control.
// Paths are workspace-relative, the landing package's one path space.
// One declaration, three consumers: recordCarriageError carries a register
// only append-only, receiptPosture excludes it from the working-tree
// projection, and WorktreeDrift tolerates an unstaged modification to it.
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
2. Every declared path has a `merge=union` line in `../../.gitattributes`
   (read the way `observe_test.go:56-62` reads `../../scripts/agents/`).
3. For every declared path, `policy.Includes(Landing, "metasystem/"+path,
   "metasystem/")` is false and `Includes(Landing, path, "")` is false.
   This is the sweep that catches a register added to the Go declaration
   without the policy row.

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
- Content is not examined. A rewritten or deleted register in the worktree
  is as invisible to the receipt as an appended one. The `:779` rule
  examines content when someone lands it, which is where content belongs.

Filtering the index as well is not done: it would let a staged register
change into a receipted landing unseen, which widens carriage.

## Decision 3: the cleanliness sites and transport

**3a. The verb.** `metasystem landing drift --root <root>
[--require-empty-index]`, registered in `cmd/metasystem/main.go:112-118`
beside `observe`, `park`, and `test-receipt`, implemented by
`landing.WorktreeDrift(root string, requireEmptyIndex bool) ([]DriftEntry, error)`
in a new `internal/landing/drift.go`.

Mechanics: one `git status --porcelain=v1 -z --no-renames
--untracked-files=normal` at the repository toplevel
(`Workspace.TopLevel()`, `gittree.go:188`), because transport needs the
whole worktree clean, which is what `land.sh:392` checks today. This needs
one new gittree primitive, `Workspace.Status() ([]StatusEntry, error)` in
`snapshotscope.go`, built on `gitProbe` like the other census probes:
toplevel-scoped, `-z` parsed, fields `Index byte`, `Worktree byte`,
`Path string` (toplevel-relative), no rename records because of
`--no-renames`. Paths are mapped to workspace space by stripping
`Workspace.Prefix()` (`gittree.go:195`); a path outside the prefix is never
a register.

The rule, applied to each entry `XY path` in this order:

1. `??` is drift of kind `untracked`.
2. `Y == ' '` is an index-only entry: kind `staged` when
   `--require-empty-index`, otherwise tolerated (it is the candidate).
3. `Y == 'M'` and the workspace-relative path is a declared register is
   tolerated, whatever `X` is. A staged register append with a further
   unstaged append is the register-carriage landing's form of this same
   defect and gets the same tolerance.
4. Everything else (` D`, ` T`, `MM` on a non-register, `UU`, `AA`, `DD`)
   is drift of kind `unstaged`.

Output: one line per drift entry on stdout, `kind<TAB>XY<TAB>path` with
the path as git printed it; tolerated entries on stderr as `tolerated
register append: <path>` so a landing log shows what was ignored. Exit 0
with no drift, 1 with drift, 2 on usage or git failure.

**3b. land.sh.**

- `stage_changes` `:324-333`: replace the `git diff --quiet` and
  `ls-files --others` pair with one `"$ms" landing drift --root "$root"`
  call captured into a variable. On exit 1: if any line is of kind
  `unstaged`, print the existing message "land refused: unstaged changes
  remain after staging; transport requires a clean tree after commit";
  otherwise print the existing untracked message; then print the lines
  indented; return 2. The empty-staging-set check at `:320-323` stays
  first and unchanged.
- `require_clean_after_commit` `:390-398`: replace the `git status` call
  with `"$ms" landing drift --root "$root" --require-empty-index`. On exit
  1 print the existing message "land refused: commit succeeded but the
  tree is not clean, so transport will not start" and the lines; return 1.
  The recertified park at `:550-555` is unchanged and still fires on real
  drift.
- `rebase_origin` `:404-406`: `git rebase --autostash
  "refs/remotes/origin/$branch"`. After a successful rebase, if the step
  output contains git's line `Applying autostash resulted in conflicts`,
  the step fails with "land refused: the rebase could not restore the
  register appends; they are in the newest stash entry; run git stash pop
  in this checkout, then land again". The push has not happened at that
  point, so the branch is rebased locally and nothing is lost. The
  implementer confirms the exact git wording by forcing the case in a
  scratch repository with a non-union file, since the probe only exercised
  the success path. The retry loop at `:588-589` calls the same function
  and needs no change.
- Nothing else in land.sh changes. `check_supplied_test_receipt` (`:361-380`)
  reads only the `tree` field, which keeps its name.

With rule 3 above, the only dirty paths that can reach the rebase are
register appends, so the autostash carries nothing else. With `merge=union`
their re-apply cannot conflict on content. The one residual failure is a
background writer touching a register between the autostash's reset and
its re-apply, a window of one rebase; the guard names it and the seat's
recovery is one `git stash pop`. That is the limit of this design, stated
plainly, and it is narrower than the manual checkout it replaces because
it loses no bytes.

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
`internal/behaviorsurface/consumer_wiring_test.go:84`. The implementer runs
the behaviorsurface tests and `static-reproof-fixtures.sh` after the edit;
the code does not say whether any other consumer of `Classify`
(`policy.go:243-276`) changes behaviour when the digest becomes
Coordination, and that run answers it.

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

Go: `WorktreeProjection TestReceiptProjection \`json:"worktreeProjection"\``
with `Excludes []string` and `Tree string`, filled from
`appendOnlyRegisters` and `receiptIdentity(tree)` at `:143-153`. The
`binding` field names stay, so a reader sees the same four keys with the
projection object telling it what the two worktree keys mean. The comment
on `TestReceipt` (`:28-29`) says the worktree observations are filtered
projections and the index observations are exact.

Reader rules in `readTestReceipt`, in order after the existing decode:

1. `SchemaVersion != 2` refuses with the existing message at `:291`. A
   schema-1 receipt is refused, not translated. The one-time cost is
   remaking a receipt produced by a pre-change engine; because the receipt
   path is keyed by tree (`:47-49`), a receipt for any post-change tree can
   only have come from a post-change engine unless a stale binary ran.
2. `Excludes` must equal `appendOnlyRegisters` element for element, else
   "test receipt excludes a different register set than this engine".
   This is a new condition and gets a new message.
3. `WorktreeProjection.Tree` and both worktree bindings must equal
   `receiptIdentity(candidate)` recomputed now; both index bindings must
   equal the candidate; any mismatch keeps the message at `:303`.
4. The landing-time read at `:306-309` as in Decision 2.

`DisallowUnknownFields` at `:281` stays: a pre-change reader meeting a
schema-2 receipt refuses on the unknown field, which is the right answer.

Why the version moves: the two worktree bindings now hold a different tree
than the raw projection of the same worktree. A reader that kept treating
`worktreeTreeAfter` as raw would be wrong in silence.

## What must not change, and how each is kept

- Index tree and candidate tree exact: `StagedTree` untouched; the index
  comparisons at `:81`, `:107`, `:133`, `:296-305`, `:307` stay exact; the
  write-trees in land.sh and commit.sh are untouched.
- The `:779` rule stays; only its case expression reads the declaration.
- Non-register drift between battery and landing refuses with the existing
  message: `:308` keeps its text; the Go refusal canary proves it before
  and after.
- `Snapshot`, `StagedTree`, and `FilterTree` keep their contracts; the
  landing composes them at its own call sites.

## Fixtures

Go tests in `internal/landing`, proving run
`go test ./internal/landing/ -run 'TestCreateTestReceipt|TestReadTestReceipt|TestWorktreeDrift|TestAppendOnlyRegisters'`,
ceiling two minutes (the package's receipt tests run subprocess commands of
`true` and short `printf` lines):

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
- **Drift rule.** `TestWorktreeDrift…` over the shapes: clean; ` M`
  register tolerated; `MM` register tolerated; ` M` product is `unstaged`;
  ` D` register is `unstaged`; `??` is `untracked`; `M ` product is
  tolerated without the flag and `staged` with it; the nested
  `metasystem/` prefix of `newObserveFixture` maps correctly.
- **Pins.** `TestAppendOnlyRegistersPins`, the three pins of Decision 1.
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
  Track a `.gitattributes` with the two `merge=union` lines so the moved-origin
  case below unions the way the real repository does.
- **Refusal canary, first.** Append `payload=drift\n` to `payload.txt`
  (tracked, not a register) in `leg_local`; attempt the landing with the
  existing matching receipt; expect exit 2, the existing "unstaged changes
  remain after staging" message, and HEAD unchanged. Restore the file with
  `git checkout -- payload.txt` (fixture cleanup, not seat recovery).
  Passes before and after.
- **Passing canary and fifth-site fixture, one landing.** From `leg_peer`,
  append `digest=peer\n` to the digest, commit, push to origin. In
  `leg_local`, stage a further change to `scripts/agents/go-gate.sh`,
  write a new chain record and review the way `:675-697` does, make the
  receipt for the new candidate with `$full_battery_command`, then append
  `digest=drift\n` to `records/narrator-digest.log`. Land with
  `--chain … --test-receipt … --staged-only --skip-transport` (fetch,
  rebase, and push still run, `:570-591`; only `sync-transport.sh` is
  skipped). Expect: exit 0; `git status --porcelain` in `leg_local` equals
  exactly ` M records/narrator-digest.log`; `git show HEAD:records/narrator-digest.log`
  equals `digest=seed\ndigest=peer\n` (no widening); the worktree digest
  contains `digest=peer` and ends with `digest=drift\n` (the autostash
  restored the append across a real rebase); origin main equals local
  HEAD. This one landing proves `stage_changes` tolerance, the receipt's
  landing-time read under drift, `require_clean_after_commit` tolerance,
  and the autostash rebase against a moved origin.

## Implementation map

In this order, each step leaving the gate green:

1. `internal/gittree/snapshotscope.go`: `Workspace.Status()` and
   `StatusEntry`; a test in `gittree_test.go` over a nested checkout.
2. `internal/landing/registers.go`, `registers_test.go`; the switch at
   `observe.go:773`.
3. `internal/landing/receipt.go`: `receiptPosture`, `receiptIdentity`,
   schema 2, reader rules; the receipt tests above.
4. `internal/landing/drift.go` and `drift_test.go`; the verb in
   `cmd/metasystem/landing_verbs.go` and its registry line in `main.go`.
5. `internal/behaviorsurface/policy.v2.json`, the `policy_test.go` row,
   `consumer_wiring_test.go:84`, and the three case lists in
   `static-reproof-fixtures.sh`.
6. `scripts/agents/land.sh`: the three edits of 3b.
7. `scripts/agents/land-fixtures.sh`: the seed and the two canaries.

No document describes the receipt's JSON fields or land.sh's clean-tree
rule (grep over `docs/`, `AGENTS.md`, `wow.md`, `development/`, and
`skills/` finds neither), so the type comment in `receipt.go` and the
registry summary in `main.go` are the records.

## What the code cannot answer

- Whether any consumer of `behaviorsurface.Classify` changes behaviour when
  the digest becomes Coordination class, and whether the static-reproof
  bed needs the digest in its skip lists: answered by running the
  behaviorsurface tests and `static-reproof-fixtures.sh` after step 5.
- The exact wording git prints when an autostash cannot be re-applied: the
  probe exercised only the success path; the implementer forces the
  failure in a scratch repository and copies the line into the guard.
- Whether the eight refusals on 2026-09-09 included commit.sh's LANDING
  refusal on the digest: the goal record does not say. The design covers
  that site regardless.
