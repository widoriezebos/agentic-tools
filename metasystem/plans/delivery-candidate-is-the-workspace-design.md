# delivery-candidate-is-the-workspace — design: the delivery candidate is the workspace, not the ledger (revision 3)

Goal: plans/goals/delivery-candidate-is-the-workspace-not-the-ledger.md
(sixth member of plans/goals/the-metasystem-validates-itself-with-itself.md).
Ruling: Wido, 2026-09-10 16:40, verbatim: "We should never have to wait
until it gets quiet." Every cite below is read at main 2fbd77535 (whole
tree 8cb35ec3); paths are repository-relative under `metasystem/` unless
they name the two unlanded patches.

Revision 2 folds the first critique (chain dcwl-crit1-20260910, seven
findings DCW-01 to DCW-07, six material; the register lands under
`records/misc/` with the chain; the coordinator's dispositions bind this
fold). Revision 1 made the workspace projection W the universal cross-tip
key and refused every non-ledger difference. That fails the sentence the
goal exists for: on the day before the read the coordinator counted 375
goal-verb commits and 45 other landings on `origin/main`, most of them
records only, and under revision 1 a sibling's critique register or
design page still voided a receipt whose every selected component identity
was unchanged. Revision 2 makes the key what the contract already names
and keeps W as the recorded floor. Revision 3 folds the second critique
(chain dcwl-crit2-20260910, six findings DCW-08 to DCW-13, all material;
the coordinator's dispositions bind this fold) and is the last design
round: what it says is what gets built. It changes six places and nothing
else: the observer no longer fails open (4.4), site 5's residual is stated
as the launcher makes it (3), the cutover leg uses a real older engine
(8.2), the shared scenario harness initialises its budget and reaps
process groups (8.3), and the engine identity binds the toolchain closure
(5). The revision record at the end names each finding and what moved.

## 0. What is wrong, and what the contract already promises

The goal ledger lives on `main`. Every `goal open`, `claim`, `edit`,
`park`, `done` on any seat is a commit that rewrites one file under
`plans/goals/` or moves it to `records/goals/` and pushes. The last forty
goal-verb commits on `main` before 2fbd77535 touch `plans/goals/**` and
`records/goals/**` and nothing else (read with `git log --name-only`; the
one `done`, eaf73ff97, moves `plans/goals/breach-stop-wedges-seat.md` to
`records/goals/`). Around them, other seats land records: critique
registers, design pages, notes under `records/misc/`. The shared testing
contract binds a delivery candidate to the whole-project index tree, so a
receipt for tree T is refused once the checkout has rebased onto a tip
whose difference touches nothing any selected test reads.

The contract's own design already names the fix.
plans/application-testing-contract-design.md line 231: "A changed
candidate consisting only of proven irrelevant coordination records can
consume the original receipt: schema-2 acceptance compares its component
input identities against the current candidate and rechecks current
index/worktree and provenance, rather than requiring irrelevant whole-tree
object-id equality. Record the old proved tree and current landing tree
distinctly." The receipt has both fields (`tree` and `provedTree`,
internal/landing/receipt.go:35 and :42), and every reader requires them
equal to the live index. The engine already computes the key that
sentence names: `RevalidateRetainedGroupExecutionIdentities`
(internal/proofrun/test_build.go:482-545) re-hashes each selected group's
declared and retained implicit inputs, environment and executable bytes on
a tree, and `groupExecutionIdentity` (:641-665) folds in the contract,
policy and section-engine digests. This design makes the receipt readers
use that key. It adds no new principle.

Three words used throughout:

- **T** is the exact whole-project tree the proof ran on. It stays
  recorded everywhere it is recorded today.
- **W** is the workspace projection of a tree: the same tree with the
  ledger paths of section 1 removed. W is a real tree object (a
  `write-tree` id), so it can be named, diffed with `ChangedPaths`, and
  checked out with `NewDetachedWorktree`. W is the auditable minimum
  projection: it is recorded in the receipt, printed by `landing
  workspace`, and used as a cheap fast path. It is never the sole reason
  for a refusal.
- **The key** is the set of selected component execution identities. A
  retained proof for the landing goal and accounting revision covers a
  tree when every selected group's revalidated identity on that tree
  equals the identity a retained successful attempt owns. This is what
  `reusedTestResult` (internal/proofrun/test_result.go:296-360) already
  composes for `test verify`.

The rule in one sentence: within one checkout, comparisons stay exact;
across a tip move, a receipt covers the current tree when the key holds,
with exact-tree equality and W equality as the two fast paths that make
the key's computation unnecessary.

## 1. Decision 1: the workspace projection W

### 1.1 The excluded set

W removes these workspace-relative paths from a tree:

| Path | Kind | Who writes it | Why it is ledger, not workspace |
| --- | --- | --- | --- |
| `plans/goals/` | directory prefix | every goal verb (the forty commits above) | the goal file is rewritten on every verb, on every seat |
| `records/goals/` | directory prefix | `goal done` (internal/goal/validate.go:38 names the records root; eaf73ff97 shows the move) | same writer, same cadence |
| `plans/goals.md` | file | the legacy goal store: `LedgerPath` (internal/goal/goal.go:123-125), read and rewritten by `Store.readState` and its writers (internal/goal/goalverbs.go:152, :207, :593) | a legacy-ledger publish or a migration on another seat moves it; absent at 2fbd77535 because the migration transaction deletes it (internal/goal/migrate.go:283-288), and an absent filtered path is a no-op (1.3) |
| `plans/goals-accepted.json` | file | the legacy acceptance baseline: `BaselinePath` (goal.go:127-129; goalverbs.go:159, :219) | same; deleted by the same migration commit |
| `records/counselor/` | directory prefix | `goal accept-risk` and a tier-raising `goal edit` append to it locally AFTER their goal transaction has published (cmd/metasystem/goalsync_mutations.go:341-350 and :1457-1461; the transaction commit is built from explicit changes only, internal/goal/txn.go:231-244), and later landings carry the appended lines (`git log` on the directory shows records landings such as 044aa3552) | goal verbs write it, no group in testing.json declares it, and its only other reader is the counselor report (internal/counselor/sources.go:78-90) |
| `memory/receipts.log` | file | the append-only register the carriage classifier names (internal/landing/observe.go:793) | already the carry chain's register |
| `records/narrator-digest.log` | file | same (observe.go:793) | same |

The counselor register is OUT of W (excluded) for the three reasons in
its row; revision 1's claim that the append rides in the same ledger
commit was wrong and is withdrawn. `memory/rulings.md` stays IN W: it is
a declared input of `fast-static-build` (testing.json:27) and is written
by landings, not by goal verbs.

The `goal-records` surface in testing.json:20 (`plans/**`, `records/**`,
`memory/rulings.md`, `memory/receipts.log`; two static section groups)
stays exactly as it is. It selects groups from the CANDIDATE'S OWN changed
paths (cmd/metasystem/test.go:281-290), and after a rebase the other
seats' ledger commits are in the base, so W never hides a candidate change
from the planner.

### 1.2 One file, two declarations, one union

The carry chain's patch (`artifacts/agents/dcwl-context/lrsrd-carry-round2.patch`,
new file `internal/landing/registers.go`) declares
`appendOnlyRegisters = []string{"memory/receipts.log", "records/narrator-digest.log"}`
and uses it in three places with append-only semantics: the carriage
classifier (`isAppendOnlyRegister`), the drift classifier
(`landing drift`, patch lines 1066-1107: a dirty register is tolerated
only when `isRegisterAppend` proves the change is an append), and the
advance verb (`landing advance`, patch lines 554-616: a dirty register
that the rebase would change is "contended", and the operator is sent to
register carriage).

Widening that slice with `plans/goals` is wrong, because a goal file is
rewritten, never appended: `landing drift` would classify a dirty goal
file as `register-not-append` and `landing advance` would refuse a rebase
that changes it with `advance-register-contended`. So the fold is one
FILE, not one slice. `internal/landing/registers.go` holds:

```go
// appendOnlyRegisters: unchanged from the carry chain (files, append-only).
var appendOnlyRegisters = []string{"memory/receipts.log", "records/narrator-digest.log"}

// ledgerPaths: directories and files that goal verbs rewrite on every seat.
// Files and directories mix because the filter enumerates both (1.3).
var ledgerPaths = []string{"plans/goals", "plans/goals-accepted.json", "plans/goals.md", "records/counselor", "records/goals"}

// WorkspaceExclusions returns appendOnlyRegisters and ledgerPaths together,
// sorted, as a copy. This is the one list W is defined by.
func WorkspaceExclusions() []string
```

Whichever chain lands second adds its declaration to this file and drops
its duplicate. `AppendOnlyRegisters()` and `isAppendOnlyRegister` keep the
carry chain's meaning. The receipt's `workspace.excludes` (section 2.3)
records `WorkspaceExclusions()`, and readers refuse a receipt whose list
differs from the engine's, exactly as the carry chain's
`validateVersionThreeReceipt` refuses a different register set (patch
lines 1670-1689).

### 1.3 How a directory prefix is filtered

`gittree.FilterTree` (internal/gittree/gittree.go:284) rewrites a tree by
seeding an isolated index and running `update-index --force-remove` from
the toplevel with the named paths. Its existing callers pass file paths.
Probed on this host (git 2.50.1, a throwaway repository with
`plans/goals/{a,b}.md`, `records/goals/r.md`, `internal/x.go`):

- `update-index --force-remove -- plans/goals` exits 0 and removes
  nothing. With a trailing slash it prints `Ignoring path plans/goals/`
  and removes nothing. FilterTree as written silently does nothing for a
  prefix.
- `git rm -r --cached -- plans/goals` removes the entries, but refuses
  ("staged content different from both the file and the HEAD") whenever
  the isolated index entry differs from HEAD and the worktree, which is
  the normal state when filtering an arbitrary tree. `-f` would be needed,
  and `--ignore-unmatch` for an absent prefix.
- `git ls-files -z -- plans/goals records/goals records/counselor` against
  the isolated index lists the three files (an absent prefix lists
  nothing), and piping that into `git update-index -z --force-remove --stdin`
  removes them; `write-tree` then names the same tree that `rm -r --cached`
  produced.

The third form is the one `SnapshotSeeded` already uses to drop a subtree
(internal/gittree/snapshotscope.go:360-376). So the primitive is a
sibling, not a change to FilterTree:

```go
// FilterTreePrefixes rewrites tree with every entry at or below the named
// paths removed. Paths are toplevel-relative and may name files or
// directories; an absent path removes nothing. Enumerates with
// ls-files -z against the isolated index and removes with
// update-index -z --force-remove --stdin, both run from the toplevel.
func (w Workspace) FilterTreePrefixes(tree string, paths []string) (string, error)
```

`FilterTree`, `Snapshot`, `StagedTree`, `SnapshotRelevant` keep their
contracts for every other caller.

### 1.4 The two path spaces

The exclusions are workspace-relative (the landing package's one path
space). Trees come in two shapes at the twelve sites: whole-project trees
(`receipt.tree`, `git write-tree`, `HEAD^{tree}`, `prepared.CandidateTree`)
and installation subtrees (`params.CandidateTree` in `landing observe`,
which commit.sh resolves through the prefix at scripts/agents/commit.sh:457-461).
Two helpers in `internal/landing`, both running `FilterTreePrefixes` from
the toplevel:

- `ProjectWorkspaceTree(root, tree)`: joins the installation prefix
  (`Workspace.Prefix()`) onto each exclusion, so in this repository the
  removed paths are `metasystem/plans/goals` and so on, and everything
  outside the installation (`benchmark/`, `development/`) stays in W.
- `InstallationWorkspaceTree(root, subtree)`: exclusions verbatim, for a
  tree that is already the installation subtree.

At a toplevel installation (prefix empty) the two coincide.

## 2. Decision 2: which identity each site compares

| Site | Where (2fbd77535) | Today | This design | What crosses a tip move |
| --- | --- | --- | --- | --- |
| 1 | `prepareTesting`, cmd/metasystem/test.go:234 | delivery: `candidateTree == indexTree` | **exact, unchanged** | nothing: every caller passes the index it just wrote (test-receipt passes `write-tree`; commit.sh:262 passes `write-tree`; land.sh:501 passes `HEAD^{tree}`, which equals the index after a clean rebase) |
| 2 | admission identity, test.go:551 | `IdentityInputs[0] = T`; manifest digest over T (:522, proof_run.go:439) | **unchanged** (revision 1 moved it to W; withdrawn): the reusable-success decision is the component path, which never reads the tree (section 3) | the whole-identity digest stays exact provenance |
| 3 | `ExactReusableTestResult`, internal/proofrun/test_result.go:267 | `CandidateTree`, `BaseCommit`, `PolicyBaseCommit`, `PlanDigest` must equal | **the key**: per-group identities and the digests it already checks; the four tree-bound fields become recorded provenance (section 3) | the exact committed payload is returned after any move that leaves the key intact |
| 4 | `groupExecutionIdentity`, test_build.go:641 | hashes group, inputs, env, tools, discovery, platform, four digests, section engine digest; never the tree | **unchanged**; the section engine digest must be a function of the engine build identity (section 5) | group identities already are tree-independent |
| 5 | `PrepareTestingReceiptPayload`, internal/landing/testing.go:29-45, called from the launcher's success callback at the end of a battery: test.go:648-649 for `test run`, and proof_run.go:168-169 for `proof-run launch`; the terminal result is committed by `commitProofTerminalWithTestResult` (proof_run.go:632-640) | `result.CandidateTree == tree`; index and working posture both `== tree`, before and after owner validation | `result.CandidateTree == tree` stays exact; working posture stays exact against T; **index posture: exact, then the W fast path**; the residual (a non-ledger index move under a running battery) is refused as today, the attempt is recorded as failed, and the battery runs again at today's cost (section 3); the signature does not move, so both callers inherit the fast path unchanged | the index moved under a running battery |
| 6 | `PublishCommittedReceipt`, receipt.go:199-201 | index and working posture `== receipt.Tree` | **index posture equals the tree the reuse decision was judged on** (the `--tree` of `landing test-receipt`, which site 1 proved equal to the index); working posture exact against `receipt.Tree`; payload published unchanged | the index moved between the proof and its projection |
| 7 | `readTestReceipt` schema-2, receipt.go:480-503 | `TreeOf(receipt.Tree) == params.CandidateTree`; four bindings `== receipt.Tree`; live posture `== receipt.Tree` | **exact, then W, then the key**: `landing observe` runs the verify core on the index and hands the composed result to `readTestReceipt`, which requires every selected group's current identity to equal the receipt's (section 4); four bindings stay exact against `receipt.Tree`; working posture exact against `receipt.Tree`; the field checks of 2.3 | the staged candidate after a rebase |
| 8 | `check_supplied_test_receipt`, scripts/agents/land.sh:385 | `receipt.tree == staged_project_tree` | **exact, then `landing workspace` on both sides, then `test verify --tree $(git write-tree)`**; gated on the receipt carrying `workspace` (section 4) | same |
| 9 | chain path, land.sh:596-597 and :614-616 | `test verify --tree HEAD^{tree}` | **unchanged call**: `test verify` is the key; on failure it adds one line naming the group and the moved paths (section 4) | the rebased HEAD |
| 10 | commit.sh:262, :272, :435-447, :462, :573-580 | whole index proved, settled, observed, landed | **unchanged**; the commit happens before the rebase | nothing |
| 11 | `landing test-receipt --mode`, cmd/metasystem/landing_verbs.go:49-107 | run, then publish committed payload or compose | **unchanged in structure**; passes its `--tree` to `PublishCommittedReceipt` as the accepted index tree (site 6) | none of its own |
| 12 | recertified path, land.sh:471-479, observe.go:86 and :213 | any origin move parks | **exact, out of scope** (section 6) | not this goal |

### 2.1 The index-versus-worktree drift check stays exact

Inside one checkout nothing is filtered. The working posture at sites 5,
6 and 7 is `SnapshotRelevant(candidate, declared inputs)`
(testing.go:186): it projects only declared inputs from the worktree over
the candidate tree, and no declared input is a ledger path, so a
ledger-only move leaves it equal to T while a staged or unstaged change to
any declared path still refuses with the existing messages
("testing receipt candidate moved before preparation", "the whole-project
index or working tree moved after the schema-2 test receipt"). land.sh's
`stage_changes` (:324-333) still refuses unstaged and untracked paths
before any receipt is read. The carry chain's rule "index never filtered"
is about this check, and this design keeps it: W and the key are compared
between two INDEX trees taken at different tips, never between the index
and the worktree.

### 2.2 Why the group identities are the key

`groupExecutionIdentity` (test_build.go:641-665) hashes the group
definition, the input digest, the environment digest, tool identities,
discovery, platform, the four contract and policy digests, and for
`section` groups the candidate engine digest. It does not hash the tree.
`RevalidateRetainedGroupExecutionIdentities` (:482-545) re-hashes declared
inputs and the retained implicit inputs (the `InputManifest` a run
records at :301, declared globs merged with discovered files) and the
executables in a detached worktree of the candidate; a path no selected
group declares or discovered cannot change a digest. This is why the
receipt breaks today and the proof does not, and why the composed reuse
(`reusedTestResult`, test_result.go:296-360) already serves `test verify`
after a rebase. What the key does not notice is an undeclared real read;
that is the contract's existing component-reuse boundary (line 231), not
a new hole.

Two other inputs to reuse deserve a sentence. `PolicyEngineDigest` is the
enrolled engine's bytes (test.go:371-402), stable across a records move;
its source check `VerifySourceAtDestination` compares ENGINE projections
of the two commits, not the commits (internal/steward/rearm_resolver.go:287-303),
so a records-only tip passes it. `AccountingRevision` is the landing
goal's own claim-time revision (test.go:889); a raise after claim on THIS
goal still voids reuse (test_result.go:262, :321), and that is right: it
is a change to the goal, not a bystander's publish.

### 2.3 What the receipt records

Schema-2 testing receipts gain one field and keep their version:

```json
"tree": "<T, the exact whole-project tree the proof ran on>",
"provedTree": "<T, as today>",
"workspace": {"excludes": ["memory/receipts.log", "plans/goals", "plans/goals-accepted.json", "plans/goals.md", "records/counselor", "records/goals", "records/narrator-digest.log"], "tree": "<W = ProjectWorkspaceTree(T)>"}
```

The type is the carry chain's `TestReceiptProjection{Excludes []string; Tree string}`
(patch lines 1391-1394), declared once under that name so the second chain
to land drops its copy. The field name is `workspace`, distinct from the
carry chain's `worktreeProjection` (a version-3 command-receipt field with
a different meaning), and the carry chain's "mixes schema versions"
refusal for `worktreeProjection` on schema 2 stays.

Version rule. Readers use `DisallowUnknownFields` (receipt.go:459 and
testing.go:149), so an engine older than this design refuses a receipt
with the new field as malformed rather than misreading it. A new engine
reading a schema-2 receipt WITHOUT the field applies the exact rule at
every site, as today (line 231: "Legacy receipts keep their stricter
original rules"); section 4 says how the shell and the observer stay on
today's calls for such a receipt. A new engine reading a receipt WITH the
field recomputes `ProjectWorkspaceTree(receipt.tree)` with its own
`WorkspaceExclusions()` and refuses when `workspace.tree` or
`workspace.excludes` differs. No reader ever trusts the field without
recomputing it.

`TestResult` does not change: W is computed from `receipt.tree` at
publication and recomputed by readers, and revision 1's `workspaceTree`
result field is withdrawn. `PlanDigest` (`TestPlanDigest`,
test_result.go:229, over contract, plan and T) does not move: the plan is
a function of the candidate's own changed paths, risk, mode and purpose
(test.go:281-290), and exact reuse compares the plan's content field by
field (section 3), so the digest becomes recorded provenance there and
keeps its tree-bound meaning everywhere else (the admission identity
inputs at test.go:551-552, the attempt path at :605).

## 3. Decision 3: the proof side

`test run` keeps the exact rule at site 1: the battery runs on exactly the
index, and the receipt binds the index posture. Nothing at site 1 crosses
a tip. What changes is what retained evidence is compared against.

Admission (site 2) is unchanged. A repeat after a records-only move does
not open a new attempt: `repeatDecisionLocked` (attempt.go:546-552) asks
the component decision first, and `componentDecisionLocked` (:558-643)
keys on goal, accounting revision and per-group identity, never on the
tree; with every group already successful it returns reusable success
(76). The whole-identity path (`noChildDecisionLocked`, :701-743) is
reached only without component identities and stays exact provenance of
the run. The brief's "admitted as a NEW attempt" becomes true under the
engine-bed chain's whole-tree stamp, which section 5 forbids.

Exact reuse (site 3). After 76 the CLI tries `ExactReusableTestResult`
(test.go:566) and today falls to the composed projection because
`CandidateTree`, `BaseCommit`, `PolicyBaseCommit` and `PlanDigest` all
moved. Revision 2 compares: `ContractDigest`, `BaseContractDigest`,
`PolicyEngineDigest`, `BehaviorPolicyDigest`, `Purpose`, `RequiredMode`,
`ExecutedMode`, `RequiredGroups`, `SelectedGroups`, `Delivery.Sufficient`,
`ValidateTestResult`, and every selected group's `ExecutionIdentity`
against the current identities (the per-group loop at test_result.go:276-288,
already there). `CandidateTree`, `BaseCommit`, `PolicyBaseCommit` and
`PlanDigest` become recorded provenance. Nothing they protect is lost:
the policy base contributed `BaseContractDigest` and `PolicyEngineDigest`,
the merge base contributed the changed paths and therefore the group
lists and modes, and all of those are still compared. With the key intact
the exact projection matches, `AttemptID` is kept, and `landing
test-receipt` takes `PublishCommittedReceipt` (site 11, landing_verbs.go:95-96).

Publication at the end of a battery (site 5). The launcher's success
callback hands the worker's result to `PrepareTestingReceiptPayload`
(test.go:648-649; the proof launcher's own callback at proof_run.go:168-169
calls the same function), which requires the index and working posture to
equal T before and after owner validation (testing.go:33-37 and :42-45).
The index may have moved while the battery ran, and the launcher's own
parity check does not see such a move: for a testing attempt it
re-captures the context with the prepared manifest digest instead of
re-hashing the root (launcher.go:421-422; execution_context.go:39-46), so
the wall is the index posture at testing.go:33-37. The rule there
becomes: exact, then W. A ledger-only move is accepted on the W fast path
(`ProjectWorkspaceTree(root, indexBefore) == ProjectWorkspaceTree(root, tree)`,
and the same for `indexAfter`) and the payload is committed with `tree` =
T; the working posture stays exact. The function's signature does not
change, so both callers inherit the widened index rule.

The residual is a non-ledger move of the index under a running battery,
and this design leaves it where the launcher puts it today. A
`PrepareSuccess` error sets the launch result to 1 (launcher.go:445-452),
`CommitTerminal` records the attempt as `TerminalFailed` because the exit
status is not 0 (proof_run.go:632-637), and a failed attempt owns
nothing: `validateTestingAttemptOwners` requires terminal success
(testing.go:115-118), `ExactReusableTestResult` requires success and a
committed receipt (test_result.go:262-263), the composed projection takes
the newest attempt per identity and requires it to be a success
(test_result.go:311-327), and the component decision classifies it
`failed` and asks for a retry (attempt.go:594-606). So that move costs
the battery again, exactly as it does today. Revision 2's claim that the
attempt "finalizes as a success without a committed receipt" and that the
next `landing test-receipt` composes a receipt with zero execution was
wrong and is withdrawn. The key is not computed at site 5: it would add
the metadata launches of `PrepareGroupExecutionIdentities` to every run
for a case whose recovery is a re-run either way. The W fast path is the
whole of what this design adds at site 5.

Projection of a committed payload (site 6, receipt.go:199-201). The
committed payload is published unchanged: `tree` stays T, `provedTree`
stays T, `workspace.tree` is W(T). It is never re-projected under the new
tip; rewriting `tree` would forge an observation that never happened, and
line 229 makes the committed payload immutable. `PublishCommittedReceipt`
gains the accepted index tree: `landing test-receipt` passes its `--tree`,
which site 1 proved equal to the index and on which the reuse decision
was judged, and the index posture must equal that tree; the working
posture is exact against `receipt.Tree`. When the two trees coincide this
is today's rule. The composed `CreateTestingReceipt` path serves what it
serves today: a selection that no single attempt owns; its `tree` and
`provedTree` name the tree the composition was verified against, and each
group's `reuseAttempt` names where it ran.

## 4. Decisions 4 and 5: the landing

### 4.1 One engine function

The body of `test verify` (cmd/metasystem/test.go:777-831: `prepareTesting`,
`RevalidateRetainedGroupExecutionIdentities`, `ReusedTestResult`) is
factored into one function in the same file:

```go
// verifyRetainedTesting composes the retained proof that covers request.Tree
// (the index when empty) for the request's goal and accounting revision. It
// launches nothing and creates no attempt. The result's per-group
// ExecutionIdentity is the identity on the current tree.
func verifyRetainedTesting(request testingSelectionRequest) (proofrun.TestResult, error)
```

Three callers: the `test verify` verb itself (site 9, and site 8 from the
shell), and `landing observe` (site 7). It is `test verify` itself.

### 4.2 Site 9: the rebased tree

`verify_current_testing_proof` (land.sh:497-505) does not change:
`test verify --root . --tree HEAD^{tree} --mode auto --purpose delivery
[--goal]`. After a records-only rebase every selected identity is
unchanged, the composition is sufficient, and the push proceeds. After a
rebase that moved a declared input, the composition reports the missing
groups and exits 1 as today. Revision 2 adds one line to that failure
when a retained successful attempt for the goal owns the group under a
different identity:

```
proof-input-moved-after-receipt: group section/land-fixtures was proved on tree <T> with a different input identity; moved declared paths: scripts/agents/coverage-delta.sh
```

The moved paths are `ChangedPaths(retained attempt's CandidateTree,
current tree)` filtered by the retained `InputManifest` (test_build.go:301),
with the same glob matcher the input digest uses. When that set is empty
(the environment digest or a tool identity moved, :537-538), the line
says `no declared path moved; the environment or a tool identity changed`.
`test verify` gains no flag, so an engine from before this design and an
engine after it are called the same way at site 9; only the failure text
differs.

The code replaces revision 1's `workspace-changed-after-proof`, which
named the wrong thing once W stopped being the key. Verbs and codes name
intent (Wido's word of 2026-09-06, recorded on
plans/goals/verbs-match-intent.md). It is registered in
internal/refusal/register.go as
`{Code: "proof-input-moved-after-receipt", Owner: "cmd/metasystem", Site: "test.go:<line>", Shape: Agent, Override: "reset to the pushed tip, re-stage, landing test-receipt --mode auto, land again", Commands: 4}`.

### 4.3 Site 8: the shell, before commit

`check_supplied_test_receipt` (land.sh:365-389) for a schema-2 receipt:

1. Read `schemaVersion`, `tree` and whether `workspace` is present with
   `$ms json get` (the pattern at :368 and :378).
2. `tree == $(git write-tree)`: pass, as today.
3. Otherwise, if `workspace` is absent: refuse with today's message, as
   today. This is the legacy path: no new verb is called, so an enrolled
   engine from before this design lands an old receipt whenever the tip
   did not move.
4. Otherwise, `landing workspace --root . --tree <receipt tree>` equals
   `landing workspace --root . --tree $(git write-tree)`: pass (the
   ledger-only fast path). `landing workspace` is a new read-only verb
   that prints `ProjectWorkspaceTree(tree)`.
5. Otherwise run `test verify --root . --tree $(git write-tree) --mode
   auto --purpose delivery [--goal]`: pass when it exits 0. When it fails,
   refuse with today's first sentence byte for byte ("names tree X but
   the staged candidate is Y; make the receipt against this exact
   candidate", which land-fixtures.sh:834 greps) followed by the last
   lines of the verify output, which carry the code and paths of 4.2.

Step 5 is the same read-only command commit.sh runs a moment later at
:272 on the same index; running it twice costs one detached worktree and
the input hashing, bounded by `suite.section-cap-min` through
`limits.sectionCap` (test.go:812, metasystem.conf:49), and buys the early,
legible wall with the receipt's path in the message.

### 4.4 Site 7: the observer, at commit

`landing observe` (cmd/metasystem/landing_verbs.go:40-46) runs from
commit.sh at the moment the index equals `proved_tree` and `settled_tree`
(commit.sh:441 refuses otherwise), after commit.sh's own `test verify` at
:272-277. Between that call and the observer's, retained attempts, the
destination policy state, physical inputs, the engine and the environment
can all differ, so the observer runs the verify core itself and never
trusts the earlier run. Revision 2 handed the core's raw result to
`readTestReceipt`, whose exact and W fast paths ran before the only
sufficiency check; that was an observer that could fail open, and it is
withdrawn.

`runLandingObserve` passes the core as a callback,
`ObserveParams.VerifyTesting func() (proofrun.TestResult, error)`, which
wraps `verifyRetainedTesting` with an empty tree (the index), purpose
delivery and the `--goal`. The callback is set only when `--test-receipt`
is given and `--recertification` is empty (the recertified path stays
exact, section 6). Revision 2's value field `VerifiedTesting` is
withdrawn: with a callback, the core runs only when the decoded receipt
carries `workspace`, and `runLandingObserve` never reads the receipt
itself. For a receipt without the field, or with `--test-receipt` unset,
nothing is computed and `readTestReceipt` behaves as today. This is the
cutover rule at site 7: the field is what a new engine wrote, so the
engine that reads it is by construction one that has the verify core.

`readTestReceipt` (receipt.go:480-503) for a schema-2 receipt that
carries the field, after the path and structure checks (:472-479) and
BEFORE every other check:

0. Call `VerifyTesting` once. Refuse when it returns an error; when the
   result's `Delivery.Sufficient` is false; or when
   `TreeOf(result.CandidateTree)` is not `params.CandidateTree`. The
   refusal is `chain-test-receipt-refused` (observe.go:179-183; the
   register row at internal/refusal/register.go:112 keeps its code and
   override) with `Detail` set, in the shape the recertified branch
   already uses at :229-231, to one of:
   - `testing-receipt: verify core failed: <error>`;
   - `testing-receipt: retained proof is not sufficient for the index; missing groups: <ids>`;
   - `testing-receipt: verify core judged tree <projected tree>, not the landing candidate <--tree>`.
   With the field and no callback (the recertified path), the receipt
   takes the field checks of step 1 and the exact rule only.

   Why a refusal and not a command error: commit.sh reads the verdict
   from the observation (:477-491) and maps a non-zero exit or an
   unreadable observation to `evaluator-unavailable`, whose repair text
   is "restore or rebuild the proof-built landing evaluator"
   (:496-498). That is the wrong instruction for a moved input or a
   missing attempt. A refusal is a durable observation with a registered
   code and a detail that names which of the three conditions held, and
   the bed's commit stub reads the same fields (land-fixtures.sh:128-130).

Then, in order:

1. Recompute `ProjectWorkspaceTree(receipt.Tree)` and require it equal to
   `workspace.tree`; require `workspace.excludes` equal to
   `WorkspaceExclusions()`.
2. The four bindings stay exact against `receipt.Tree` (:494-499). The
   working posture stays `SnapshotRelevant(receipt.Testing.CandidateTree, inputs) == receipt.Tree`.
3. Coverage of the candidate, first match wins:
   - exact: `TreeOf(receipt.Tree) == params.CandidateTree`, as today;
   - W: `InstallationWorkspaceTree(TreeOf(receipt.Tree)) == InstallationWorkspaceTree(params.CandidateTree)`;
   - the key: the result of step 0 has `SelectedGroups` and
     `RequiredGroups` equal to the receipt's, and for every selected
     group its `ExecutionIdentity` equals the receipt's group
     `ExecutionIdentity`.
   Otherwise refuse with `chain-test-receipt-refused` and the detail
   `testing-receipt: receipt does not cover the candidate: <groups whose
   identity differs>`.

The live index posture check at :500-503 becomes the exact-or-W check on
the index tree against `receipt.Tree`, and when neither holds, the key of
step 3 is what proves the index is covered (the verify core ran on this
very index). The receipt's own attempts are still validated through
`validateTestingAttemptOwners` (:487-489), after step 0: when a retained
attempt has been removed between commit.sh's run and the observer's, step
0 refuses first with the sufficiency detail, which names what to run
again, rather than the ownership message.

Cost: the core now runs three times on one landing, at site 8's step 5,
at commit.sh:272 and here, all on the same index; each run is read-only,
one detached worktree and the input hashing, bounded by `limits.sectionCap`
(test.go:812). Revision 2 accepted the second run for the legible wall;
the third is the price of an observer that cannot fail open.

### 4.5 The residual and its recovery

When a declared input really moved between the receipt and the rebased
tip, site 9 refuses with the code of 4.2 and the landing stops before the
push. That is a real re-proof, not a re-receipt: the group whose identity
moved has to run again. The rebased commit exists on the local branch and
was not pushed, and land.sh always stages and commits before it reaches
its rebase-and-push path (land.sh:525-567); it has no path that pushes an
existing commit. Recovery today, in order: reset the branch to the pushed
tip (`git reset --soft origin/<branch>` keeps the change staged), fetch
and check that the index now holds the intended change on top of the
moved tip, take a new receipt with `landing test-receipt --root . --tree
$(git write-tree) --mode auto --goal <goal>` (the unchanged groups reuse,
the moved group runs), and land again. Follow-up goal, one line:
`land-resumes-a-refused-push`: land.sh pushes an existing receipted commit
after a post-rebase refusal without recommitting. It is not this goal's
DONE.

A receipt for T stays unlandable against a staged candidate whose key
differs: site 8 refuses it before commit, site 7 refuses it at commit,
site 9 refuses it after the rebase. The three walls are one function in
three places.

## 5. Constraints on the two neighbouring chains

Carry chain (`lrsrd-carry-round2.patch`, goal
landing-receipt-survives-records-drift, land-ready). Fold rule in 1.2:
one file, `appendOnlyRegisters` unchanged, `ledgerPaths` and
`WorkspaceExclusions()` added beside it; `TestReceiptProjection` declared
once. Its `landing advance` replaces `git rebase` at land.sh:414 and is
compatible with site 9: advance moves the branch, then `test verify` runs
on the result. Its rule "index never filtered" is kept as 2.1 reads it.

Engine-bed chain (`rbce-build1-r4.patch`, goal
receipt-beds-run-the-candidate-engine, parked until this goal lands). Two
constraints, stated so its owner can apply them from this section alone.

**Constraint 1: the engine identity is the build identity, and the build is
deterministic inside it.** Today `bindMaterializedCandidateCommit` (patch
lines 179-221) makes a synthetic commit of the whole candidate tree and
`go-build.sh --trimpath --out` links that id into the binary (patch lines
153-158 and 1352-1358), so every records-only move changes the engine
bytes, `candidateEngine.Digest`, and every `section` group identity
through `groupExecutionIdentity` (test_build.go:642-645). The patch also
clears `GOFLAGS` and pins `GOTOOLCHAIN=local` (patch lines 95-108), while
go-build.sh passes only `-buildvcs=false` (go-build.sh:73 and :82) and
does not force module-readonly mode; go-gate.sh does
(`export GOFLAGS=-mod=readonly`, go-gate.sh:66). At 2fbd77535 there is no
`vendor/` directory (`git ls-files vendor` is empty) and go.mod:3 declares
`go 1.27`. The constraint:

- The proof build forces `-mod=readonly` (the contract already requires
  it for the frozen proof at plans/application-testing-contract-design.md:238).
- The engine identity that the stamp and the section identities bind is:
  the ENGINE projection digest of the candidate's installation subtree
  (the `enginePaths` list at internal/behaviorsurface/policy.v2.json:3-10,
  which already holds `go.mod` and `go.sum`; `archivedEngineDigestAtCommit`
  at rearm_resolver.go:307-323 computes it for enrolled engines), plus the
  toolchain closure identity, plus the platform (`GOOS/GOARCH`), plus the
  build flags and environment the build fixes (`CGO_ENABLED=0`,
  `-trimpath`, `-mod=readonly`, the pinned toolchain). Never T, never W.
- The toolchain closure identity is a new function in internal/proofrun
  beside execution_context.go:91, `ToolchainClosureIdentity(root, environment)`.
  It hashes what `CompleteToolchainIdentityAtWithEnvironment` hashes
  today (`go version` and the `go env` tuple, :92-110; the shell twin is
  `gate_toolchain_identity`, go-gate.sh:186-191) AND the SHA-256 of the
  selected `go` executable's bytes AND the bytes of `$(go env GOROOT)/VERSION`.
  The selected executable is the one the build's own environment
  resolves: `explicitEnvironmentCommand` resolves `go` against that
  environment's PATH (test_build.go:1083-1108), the resulting path is
  followed through symlinks (on this host `/opt/homebrew/bin/go` is a link
  to the Cellar binary) and hashed, and `GOROOT` is read from the same
  environment. The build pins `GOTOOLCHAIN=local` (patch lines 95-108),
  so that executable is the compiler front-end that runs; without the
  pin `go env GOROOT` names a switched toolchain, whose VERSION file then
  enters the identity beside the front-end's bytes. A missing VERSION
  file is an error, never an empty input.
- Why the closure and not a proven-once claim. Revision 2 hashed what
  `go version` and `go env` report, and two toolchains can report the
  same and emit different engine bytes. The alternative, dropping
  byte-identity for "equal identity, equal bytes, proven once per
  toolchain on this host", needs a host register of past proofs and
  still needs a digest to tell toolchains apart. The closure costs one
  hash of one file (16 MB on this host) and one small read per identity,
  and no state. The front-end's bytes come from the same release build as
  the tools under its GOROOT; a GOROOT whose tools were replaced under an
  unchanged front-end is tampering, out of scope.
- `CompleteToolchainIdentityAtWithEnvironment` itself does not change.
  It is the attempt identity (attempt.go:830) and the coverage evidence's
  toolchain (coverage.go:230 and :242), and widening it voids every
  retained coverage record once. That widening is a one-line follow-up
  goal, `toolchain-identity-binds-the-compiler-bytes`, not this goal's.
- Byte-identity of the proof engine is claimed only for an identical
  build context: two candidates with equal ENGINE projections built under
  the same toolchain closure, platform, flags and environment produce
  identical bytes. Whether the stamp is the synthetic commit id of an
  ENGINE-only tree (forty hex, the `commitBuildStamp` shape at
  rearm_resolver.go:37) or the `witness-<12 hex>` form (:38, the form
  go-build.sh:41-42 calls "the judged engine-input digest") is that
  chain's choice.
- One fixture in that chain builds the engine twice from two candidates
  whose ENGINE projections are equal and whose trees differ outside it
  (a records file), under one build context, and fails when the digests
  differ; and it builds once more after changing `go.sum` and fails when
  the identity did NOT change.
- The same-report, different-bytes case is a unit test of
  `ToolchainClosureIdentity` in that chain, not a two-compiler bed. The
  test writes a fake `go` script into a temporary directory that replays
  the real toolchain's `go version` and `go env` output and answers
  `go env GOROOT` with a temporary directory holding a copy of the real
  VERSION file, puts that directory first on the PATH of the environment
  it passes, and asserts that the identity differs from the real
  toolchain's. A second copy of the same script in another directory
  yields the same identity as the first (bytes, not path). Removing the
  VERSION file makes the function return an error.

**Constraint 2: retained candidate-engine evidence is located by the engine
build identity, never by T or W.** Patch lines 314-360
(`retainedCandidateEngineDigest`) look up the retained engine digest by
`result.CandidateTree == prepared.CandidateTree` and by a receipt at
`TestReceiptPath(root, prepared.CandidateTree)`. After any tip move
neither matches and `test verify` refuses with "candidate engine digest
is absent from retained evidence". Revision 1 keyed it by W, which loses
every section group again on a records-only landing. The lookup must
compute the candidate's engine build identity (constraint 1) and select
the retained result or receipt whose recorded engine build identity
equals it; the candidate engine digest is then that record's digest, and
a candidate whose build identity has no retained record is what "run
metasystem test run" is for. The result and the receipt record the build
identity next to the digest so the lookup never has to rebuild to compare.

## 6. Decision 6: the recertified path stays exact

Recertification binds `TargetCommit` (internal/validate/recertification.go:57)
and the verifier replays the certified merge onto `TargetTree` (:1044-1096:
resolve the target, replay `q` on `TargetTree`, require `MergedTree`,
require `MergedWholeTree` from a graft on `TargetCommit`). land.sh:471-479
and observe.go:86 and :213 park with `chain-recertification-target-moved`
when HEAD or origin differs from that commit, and :490 requires the
landed commit's parent to be it. Rebinding to the moved target means: on
every tip move, re-derive `MergedTree` on the new target tree, re-verify
the replay, and re-bind the receipt to the new merged whole tree. That is
a new recertification per tip move, not a projection rule, and it touches
the producer, the verifier, `landing park` and the promotion-policy load
(`applyPromotionAtTree`, observe.go:84).

So this goal leaves recertification exact, with the reason written. The
ordinary chain path is what the goal's sentence "the landing rebases onto
the moved tip and verifies the same W" describes, and it is the path the
three parked landings on this seat can re-enter once their reviewed trees
are re-based by their owners. Follow-up goal, one line:
`recertification-binds-the-workspace-not-the-tip`: a recertification
record binds the key of its target and the verifier replays the certified
merge onto any target the retained proof covers.

## 7. Decision 7: the documented rule

docs/project-rules.md:44 today: "The candidate index and every
delivery-relevant working-tree input must describe the same whole-project
bytes before selected testing evidence can authorize delivery; a mismatch
is repaired, never treated as permission to publish partial coverage."

As it will read: "The candidate index and every delivery-relevant
working-tree input must describe the same workspace bytes (the whole
project minus the goal ledger paths that goal verbs rewrite: `plans/goals`,
`plans/goals.md`, `plans/goals-accepted.json`, `records/goals`,
`records/counselor`, `memory/receipts.log`, `records/narrator-digest.log`)
before selected testing evidence can authorize delivery; the exact tree
the proof ran on is still recorded; a tip move that leaves every selected
test group's execution identity unchanged never voids that proof, however
many records it touched; a moved declared input is proved again, never
treated as permission to publish partial coverage."

## 8. Fixtures: the design names them, the build proves them

### 8.1 The bed

All shell fixtures extend scripts/agents/land-fixtures.sh as new bed
scenarios in the list at :30-32 (the success line's leg count moves from
9 to 13). Each is one isolated leg under `make_leg` (:50). The bed copies
the source engine into each leg (`source_engine=$root/bin/metasystem` at
:9-11, copied at :61), so the engine every leg's `land.sh` calls through
`$ms` (land.sh:14) is the candidate's own build, and every receipt the leg
takes carries the `workspace` field. That is the production
shell-to-engine path: land.sh's site 8 steps, commit.sh's `test verify`
and `landing observe`, and land.sh's post-rebase `test verify`, all
against the engine under test.

The leg needs a contract-bearing repository, which the full-width-chain
leg does not have (it adds and removes `testing.contract` at :801-811 and
takes a legacy command receipt at :844-846). The recipe is the Go test's
(cmd/metasystem/landing_verbs_test.go:1352-1375): `metasystem.conf` with
`testing.contract=testing.json`, `metasystem.steward.landing-ref` set to
`refs/remotes/origin/main` (test.go:919-934 requires it), a `testing.json`
with two `sh -c` command groups producing junit reports, of which one
declares `inputs: ["payload.txt", "scripts/**"]` so that a script change
is a declared input, the goal `fx` claimed under the fixture lineage
(:211-220), and the seed pushed WITHOUT `testing.json` on `main`.

The one seam the bed cannot reach is enrollment. With no contract on the
policy base, `trustedPolicyEngine` takes the first-transition branch
(test.go:377, `os.Executable()`); with a contract there it would require
`steward.OpenEnrolledBinary` (:378) and an enrollment record the bed does
not mint. So the legs prove the shell-to-engine wiring and every site on
the candidate's engine, and not the enrolled-engine resolution, which
internal/steward/rearm_test.go and internal/up/up_test.go cover on their
own and this chain's landing exercises live. The leg header states this
limit.

The peer's ledger move follows the seed's own recipe (:205-223): the peer
clone sets `goal.sync-remote local`, `goal.sync-branch refs/heads/metasystem/goals`,
`metasystem.goal.machine fixture-peer`, points that branch at `origin/main`,
runs one goal verb (`goal open --id peer-goal ...` under
`METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-peer`), then pushes
`refs/heads/metasystem/goals:refs/heads/main`. A..B is then one commit
touching only `plans/goals/peer-goal.md`.

### 8.2 The legs

- **Ledger-only move lands** (scenario `ledger-move-lands`). Clone one
  stages `payload.txt`, takes a receipt with
  `landing test-receipt --root . --tree $(git write-tree) --mode auto --goal fx --cap-min 1`
  at tip A, and asserts the receipt carries `workspace.tree`. The peer
  publishes B as above. Clone one runs
  `land.sh -m ... --chain <chain> --test-receipt <receipt> --staged-only --skip-transport`.
  Assert: exit 0; the chain log shows the receipt path; origin `main`
  equals clone one's HEAD; `HEAD^1` equals B; the pushed tree contains
  `plans/goals/peer-goal.md`. On the untouched tree this scenario must
  fail at site 8's first sentence (the fixture greps for it and reports
  "site 8 refused a ledger-only move" so the failure is legible).
- **Records-only move lands** (scenario `records-move-lands`). Same
  start; the peer instead commits a new file `records/misc/peer-note.md`
  plainly and pushes. W differs, no declared input moved. Assert the same
  as above, and additionally that `land.out` shows site 8 reaching step 5
  (`test verify` ran) and no `proof-input-moved-after-receipt` line.
- **Declared input moves, landing refuses** (scenario
  `input-move-refuses`). Same start; the peer appends a comment to
  `scripts/agents/coverage-delta.sh` (declared by the bed group's
  `scripts/**`; in the real contract by the `coverage-policy` surface at
  testing.json:12 and by every `section/*` group's `metasystem/scripts/**`),
  commits plainly and pushes. Clone one's landing exits non-zero,
  `land.out` contains `proof-input-moved-after-receipt`, the group id and
  `scripts/agents/coverage-delta.sh`, origin `main` still equals B, and
  clone one's HEAD is not on origin.
- **Receipt cutover** (scenario `receipt-cutover`): the older engine plus
  the new land.sh, with a real older engine. Revision 2's shim is
  withdrawn: it forwarded every verb but `landing workspace` to the new
  engine, so a receipt carrying `workspace` would have been accepted by
  the new reader where a real older engine refuses it
  (`DisallowUnknownFields`, receipt.go:458-462 and testing.go:148-149),
  and a shim that reproduces the old decoder by hand is exactly the
  claim this leg exists to test. The scenario builds the engine of the
  last `main` tip before this design, 2fbd77535, once: `git archive` of
  that commit's `metasystem/` prefix into the leg's temporary directory,
  then `METASYSTEM_BUILD_STAMP=cutover-2fbd77535 bash <that tree>/scripts/agents/go-build.sh --out <leg>/old-engine`.
  `--out` skips the gate fence and leaves `bin/metasystem` alone
  (go-build.sh:13-24 and :29), the stamp override skips the `git
  rev-parse` an archive cannot answer (:44-46), and `-buildvcs=false`
  needs no repository (:73). The pin is a commit id in the bed with this
  reason beside it; a clone that lacks the object fails the scenario
  naming the pin. Cost: one `go build` of that commit per bed run, inside
  this scenario only; the first build on a host compiles the tree (the
  Go gate's build time), later runs hit the build cache and pay the link.
  The leg installs the old engine as clone one's `bin/metasystem`, so
  every `$ms` call of land.sh and of the commit stub (land.sh:14;
  land-fixtures.sh:128) is the old engine, and it proves three things:
  1. An old receipt lands exactly when the tip did not move. Clone one
     takes a receipt with the old engine (`landing test-receipt --mode
     auto`, which writes no `workspace` field); `land.sh` exits 0, origin
     `main` equals clone one's HEAD, and `land.out` carries no
     `proof-input-moved-after-receipt` line.
  2. The same landing refuses at site 8's first sentence after the
     peer's ledger move. A second leg in the same scenario: clone one
     takes a receipt at A; the peer publishes B; clone one fetches and
     fast-forwards (`git merge --ff-only origin/main` keeps the staged
     `payload.txt`, which B does not touch), so its index is B plus the
     change; `land.sh` refuses byte for byte with today's first sentence
     (the text land-fixtures.sh:834 greps), `land.out` contains no
     unknown-verb text, and HEAD is not on origin. That proves land.sh's
     step 3 never calls `landing workspace` for a receipt without the
     field: the old engine has no such verb, and a call would have
     changed the text.
  3. A receipt carrying the field is refused by the old engine. In the
     first leg, before the landing, clone one takes a receipt with the
     candidate's engine by explicit path (`"$source_engine" landing
     test-receipt ...`, which writes the field); the old engine's `landing
     observe --root . --tree <tree> --chain <chain> --test-receipt
     <receipt> --actor <actor>` prints `verdictTrailer` equal to
     `would-refuse code=chain-test-receipt-refused` (observe.go:179-183).
     The control: the same receipt rewritten without the field, at the
     same path, is observed (`mode` `observe`) by the same old engine, so
     the refusal is the field and nothing else. The attempt records the
     new engine wrote are readable by the old one because `TestResult`
     does not change (2.3).
- **Drift inside the checkout still refuses** (inside `ledger-move-lands`,
  before the peer moves). After the receipt, `printf x >>payload.txt`
  unstaged: `land.sh` refuses with the existing "unstaged changes remain
  after staging" (land.sh:325). Then stage that change: `land.sh` refuses
  at site 8 with the existing first sentence, and `land.out` shows the
  verify refusal naming `payload.txt`. Both leave HEAD at A. This proves
  the exclusion did not widen.

### 8.3 The ceiling, the budget and the process group

Revision 1 promised a two-minute ceiling per leg that nothing enforces:
`run_fixture_bed_scenarios` (scripts/agents/fixture-bed-scenarios.sh:47-53)
starts each child and waits without a deadline, and the proof engine only
sums `targetMs` into the declared cost (test_build.go:196-216). What
bounds the section today is the proof watchdog: `suite.section-cap-min`
(metasystem.conf:49, 45 minutes) refuses a section that keeps producing
output past the cap and `suite.progress-silence-min` (:48, 30 minutes) a
silent one (internal/proofrun/watchdog.go:71-72), both for the whole
`section/land-fixtures` group, not per leg.

Revision 2 enforces the per-leg bound and withdraws the two-minute
promise as a promise: two minutes is the expected duration. Revision 3
adds the two things a shared ceiling needs to be usable and safe: the
harness initialises its own budget, and a timed-out scenario is reaped
as a process group.

The ceiling. `run_fixture_bed_scenarios` gains a per-scenario wall-clock
ceiling: a new named cap `bed-scenario` in `harness_fixture_base_cap`
(scripts/agents/fixture-budget.sh:266-302) with base
`${METASYSTEM_BED_SCENARIO_FIXTURE_TIMEOUT_SEC:-120}`, an integer from 1
through 120 (the override goes below the default, unlike the other caps'
overrides at :270-281, because the harness's own negative fixture must
wait seconds, not sixteen minutes), scaled by `harness_fixture_scaled_cap`
(:431-438) under the measured factor (8 to 48, so the enforced ceiling
is 16 to 96 minutes, a hang detector under the 2026-08-17 ruling quoted
at :379-392). The plain `wait` at fixture-bed-scenarios.sh:50 becomes a
poll in the shape of the calibration loop (:359-362): while the child is
alive and the deadline in `SECONDS` has not passed, sleep a quarter
second. A child that outlives the ceiling is reaped as below, recorded as
failed with `rc=124`, and the log line names the bed, the scenario, the
elapsed seconds and the scaled cap. A child that exits in time is waited
for and its status recorded as today.

The budget. `harness_fixture_scaled_cap` refuses when
`METASYSTEM_FIXTURE_CAP_SCALE_MILLI` is unset (:435-436). Of the six beds
that source the harness (brain, land, mission, return-schema,
second-session, witness-gate) only mission-fixtures.sh:17 calls
`harness_fixture_budget_init`, and the validation suite calls it once
above every bed it runs (validate-metasystem.sh:868). So the harness
initialises itself: at the top of `run_fixture_bed_scenarios`, when
`METASYSTEM_FIXTURE_CAP_SCALE_MILLI` is not a positive integer, it calls
`harness_fixture_budget_init` with the root the harness derives from its
own location at source time (`fixture_bed_harness_root`, the way
land-fixtures.sh:8 derives `root`). Init already handles both starts: an
operator's `METASYSTEM_FIXTURE_CAP_SCALE` is validated and its milli form
derived without a probe (:333-347), and nothing set runs the census
probe once (:349-394); it exports both variables (:400), so every
scenario child and every nested harness inherits them and skips the
probe. mission-fixtures.sh:17 keeps its call; the harness then finds the
scale set and does nothing. Init never runs twice in one process.

The process group. `fixture_bed_parent_cleanup` (fixture-bed-scenarios.sh:20-29)
and the calibration loop (fixture-budget.sh:359-369) signal the direct
child pid only; a timed-out scenario's descendants survive, keep locks
and state, and contaminate the next scenario, and this repository has
learned that leaked fixtures compound into self-worsening flakes.
`setsid` is not on macOS (probed on this host: absent from PATH,
`/usr/bin` and `/opt/homebrew/bin`), and the engine has no light
bounded-exec verb (`util` holds small helpers such as `token-hex`,
`sha256` and `now-ns`, and `proc` holds `started-at`, main.go:82-85 and
:409-416; the proof launcher's group handling at launcher.go:208 and
:226 is bound to attempts and stop fences). So the harness uses bash job control, which macOS's bash 3.2.57
and every later bash have: `set -m` immediately before the `&` at :47
and `set +m` right after it, with stdin from `/dev/null` because job
control leaves a background job's stdin attached. Probed on this host:
under `set -m` the child's process group id equals its pid (`kill -0 --
-$pid` succeeds; without `set -m` it fails), a group TERM leaves a
TERM-ignoring grandchild alive, and a group KILL empties the group.
Reaping is one function, `fixture_bed_reap_group <pid>`: TERM to `-$pid`;
poll `kill -0 -- -$pid` for up to five seconds (the launcher's own
`TermGrace`, test.go:636; the poll is the watchdog's `waitForGroup`
shape, watchdog.go:253-260); KILL to `-$pid` when members remain; `wait
"$pid"` for the direct child; then poll the group again for up to five
seconds and, when a member survives KILL, fail the bed loudly naming the
scenario and the group id. `fixture_bed_parent_cleanup` calls the same
function on a live child, so the EXIT, HUP, INT, QUIT and TERM traps
(:38-42) leave nothing behind either. The log lines are `<bed> fixture
scenario exceeded its ceiling: <scenario> (elapsed <n>s, scaled cap
<n>s)`, then `group <pgid>: TERM sent`, `group <pgid>: alive after 5s
grace; KILL sent` when it was, and `group <pgid>: empty`.

The fixture. A new bed, scripts/agents/fixture-bed-scenarios-fixtures.sh,
sources the harness and runs four scenarios through it. Each scenario
writes a throwaway inner bed into its temporary directory that sources
fixture-budget.sh and fixture-bed-scenarios.sh with the detection idiom
(fixture-bed-scenarios.sh:81-86) and runs one scenario of its own:

- `budget-standalone`: the inner bed runs with both scale variables
  removed from the environment; its scenario prints
  `METASYSTEM_FIXTURE_CAP_SCALE_MILLI`; the bed exits 0 and the printed
  value is an integer from 8000 through 48000 (the probe's floor and
  ceiling, fixture-budget.sh:378 and :393).
- `budget-inherited`: the inner bed runs twice, once with
  `METASYSTEM_FIXTURE_CAP_SCALE=3` alone (an operator override) and once
  with both variables set to 3 and 3000 (a parent bed); both runs print
  3000, which no probe can produce.
- `ceiling-reaps-group`: the inner bed runs with
  `METASYSTEM_BED_SCENARIO_FIXTURE_TIMEOUT_SEC=1`; its scenario `hang`
  writes its own pid, starts `bash -c 'trap "" TERM; sleep 600'` in the
  background, writes that pid, and sleeps; the inner bed exits 1, its
  output carries the ceiling line naming `hang`, the elapsed seconds and
  the scaled cap, then the TERM, KILL and empty lines; afterwards `kill
  -0` on the grandchild pid fails and `kill -0 -- -<child pid>` fails.
- `signal-reaps-group`: the same inner bed without the small cap; the
  outer scenario waits for the grandchild's pid file, sends TERM to the
  inner bed's parent process, and asserts exit 143, the same three group
  lines, and no survivor.

The bed is registered as `section/fixture-bed-scenarios-fixtures` in
testing.json beside `section/land-fixtures` (testing.json:54), selected
by a surface over the harness scripts and the bed the way `land-fixture`
selects its bed (testing.json:15), and run by validate-metasystem.sh
through `run_section` beside land-fixtures (:2910-2912), with the syntax
check beside :1103 and the fixture-script list at :1000-1012. The bed
and the two harness scripts join the files list (section 11).

### 8.4 Unit tests

- internal/gittree: `FilterTreePrefixes` removes a directory prefix and
  leaves siblings; an absent prefix is a no-op that returns a tree; a
  file path and a prefix in one call (`plans/goals.md` beside
  `plans/goals`); the result equals the tree from `git rm -r --cached` on
  the same input (the probe's equality).
- internal/landing: `WorkspaceExclusions()` names the seven paths of 1.1;
  `ProjectWorkspaceTree` joins the installation prefix in a nested
  checkout and not at the toplevel; `readTestReceipt` accepts a schema-2
  receipt on the exact path, on the W path after the fixture rewrites
  `plans/goals/x.md` and `plans/goals.md`, and on the key path after the
  fixture adds `records/misc/x.md` with a `VerifyTesting` callback that
  returns a result whose identities equal the receipt's; refuses when a
  selected identity differs, naming the group; refuses BEFORE the exact
  path, on a receipt whose tree equals the candidate, when the callback
  returns an error, an insufficient result, or a result for another
  tree, with the detail naming which of the three (4.4 step 0); refuses
  a receipt whose `workspace.tree` disagrees with the recomputed W;
  refuses a receipt whose `workspace.excludes` differs from
  `WorkspaceExclusions()`; a schema-2 receipt without the field keeps
  the exact rule and the callback is never called (the test's callback
  fails the test when invoked); `PublishCommittedReceipt` projects when
  the index equals the accepted tree and refuses when it moved again.
- internal/proofrun: `ExactReusableTestResult` returns the original result
  when `CandidateTree`, `BaseCommit`, `PolicyBaseCommit` and `PlanDigest`
  all differ and every selected identity is equal; refuses when one
  identity differs.
- cmd/metasystem: `test verify` prints `proof-input-moved-after-receipt`
  with the group and `scripts/x.sh` after a declared input moves, and
  the environment variant of the line when only a group environment
  variable moves; exits 0 after a records-only move; `landing workspace`
  prints the same id for two trees that differ only under `plans/goals`
  and `records/counselor`; `landing observe` computes the verify core
  only when the receipt carries `workspace`; the observer does not fail
  open: in the contract-bearing repository of 8.1
  (landing_verbs_test.go:1340-1375) take a receipt, run `test verify
  --tree $(git write-tree)` and require exit 0 (commit.sh's run at
  :272), delete the retained attempt's file (`AttemptPath`,
  attempt.go:204-208) and run `landing observe --test-receipt` on the
  unchanged index; the observation is `would-refuse
  code=chain-test-receipt-refused` with the sufficiency detail naming
  both groups, although the receipt's tree equals the candidate and the
  exact path would have accepted it.

## 9. What must not change, as checks the build can make

- T is still recorded: `receipt.tree`, `receipt.provedTree`,
  `TestResult.CandidateTree`, the attempt's `IdentityInputs` provenance.
- A receipt for T is unlandable against a staged candidate the key does
  not cover: sites 7, 8, 9 (section 4.5, last paragraph).
- The drift check inside one checkout stays exact (2.1; the drift leg).
- No group leaves testing.json; the `goal-records` surface is untouched.
- commit.sh's postcondition stays: :573-580 still compares the landed
  tree with `proved_tree`.
- `Snapshot`, `StagedTree`, `SnapshotRelevant`, `FilterTree` keep their
  contracts; the new code is `FilterTreePrefixes` and the two landing
  helpers.
- Every refusal the contract makes for a relevant change is still made;
  the one refusal revision 1 added for irrelevant changes is withdrawn,
  and the refusal for a moved declared input gains a name and its paths.
- The observer never accepts a receipt that carries `workspace` without
  the verify core's verdict on the very index it observes (4.4 step 0);
  no fast path runs before that verdict.

## 10. Facts the code could not answer

- Which process moves a seat's checkout under a running battery. No
  fast-forward or pull of the checkout was found in internal/steward or
  the goal verbs (grep for `ff-only`, `fast-forward`, `pull`). The design
  does not depend on the answer: a ledger-only move takes the W fast path
  at site 5, and a non-ledger move fails the attempt and costs the
  battery again, as it does today (section 3), whether the move happened
  during or after the battery.
- Whether enrollment can be minted inside a fixture bed without the
  steward. Not found; 8.1 names the tests that cover that seam instead.

## 11. Files this design touches

internal/gittree/gittree.go (`FilterTreePrefixes`) and its test;
internal/landing/registers.go (`ledgerPaths`, `WorkspaceExclusions`),
receipt.go (`TestReceiptProjection`, `workspace`, `readTestReceipt` with
step 0 of 4.4, `PublishCommittedReceipt`), testing.go (site 5),
observe.go (`ObserveParams.VerifyTesting`, the step-0 detail) and their
tests; internal/proofrun/test_result.go (`ExactReusableTestResult`) and
its test; cmd/metasystem/test.go (`verifyRetainedTesting`, the refusal
line), landing_verbs.go (`landing workspace`, `landing observe` passing
the verify core as the callback, the accepted tree for
`PublishCommittedReceipt`), main.go (the verb row) and their tests;
internal/refusal/register.go (one row); scripts/agents/land.sh (site 8);
scripts/agents/land-fixtures.sh (four scenarios, the drift checks, the
pinned old-engine build); scripts/agents/fixture-bed-scenarios.sh (the
per-scenario ceiling, the self-initialised budget, the process-group
reap); scripts/agents/fixture-budget.sh (the `bed-scenario` cap and its
override); scripts/agents/fixture-bed-scenarios-fixtures.sh (new, the
four harness scenarios of 8.3); testing.json (one section group and one
surface for that bed); scripts/validate-metasystem.sh (one `run_section`
line, the syntax check, the fixture-script list);
docs/project-rules.md:44 (section 7).

Not touched, and named so nobody looks for it: cmd/metasystem/proof_run.go.
`PrepareTestingReceiptPayload` keeps its signature; its contract widens
at the index posture only, and the proof launcher's callback
(proof_run.go:160-175) and terminal commit (:628-640) inherit that
without an edit.

## Revision record

- Revision 1, 2026-09-10, job dcwl-design1-20260910 (Claude Fable 5.1,
  design lane under R-89-m1d and R-25). First draft. Read at 2fbd77535.
  Probed `update-index --force-remove` with a directory name, `git rm -r
  --cached` against a seeded isolated index, and the `ls-files` plus
  `--force-remove --stdin` enumeration on git 2.50.1; the third is the
  primitive. Deviates from the brief's literal "same declaration widened"
  with one file and two declarations (1.2) because the carry chain's
  drift and advance verbs give the register slice append-only semantics a
  goal file does not have. Corrects the brief's site-2 claim against the
  code: at 2fbd77535 the component decision already returns reusable
  success after a ledger-only move; the new-attempt failure appears under
  the engine-bed chain's whole-tree stamp (section 5).
- Revision 2, 2026-09-10, job dcwl-design1-20260910-r2 (same lane), after
  the first critique (dcwl-crit1-20260910, codex gpt-5.6-sol; seven
  findings, six material; dispositions by m1d binding). Decisions 1.2,
  1.3, 2.1, 6 and the twelve-site inventory stand.
  - DCW-01 (high, accepted): the cross-tip key is the selected component
    execution identities; W is the recorded floor and a fast path, never
    the sole refusal. Sections 0, 2 (sites 3, 5, 6, 7, 8, 9), 3, 4, 7, 8
    and 9 rewritten. Site 2's move to W and `TestResult.workspaceTree`
    withdrawn; `PlanDigest` stays where it is. The refusal is issued only
    when a selected group's identity moved and names the group and the
    paths; the code is now `proof-input-moved-after-receipt`. A
    records-only positive leg added; the negative leg moves a declared
    input. The residual recovery is stated as today's steps in 4.5, and
    `land-resumes-a-refused-push` stays a one-line follow-up.
  - DCW-02 (high, accepted): site 9 needs no flag; `test verify` is the
    key and is called as today. Sites 7 and 8 gate every new call on the
    receipt carrying `workspace`; a receipt without it takes today's path
    and today's calls. The `receipt-cutover` leg runs the older-engine
    plus new-land.sh pair with an old receipt (8.2).
  - DCW-03 (high, accepted): section 5 constraint 2 rewritten: retained
    candidate-engine evidence is located by the engine build identity,
    never by T or W.
  - DCW-04 (high, accepted, narrowed): section 5 constraint 1 rewritten
    with the tree as it is (no `vendor/`, `-mod=readonly` in go-gate.sh
    only): the build forces `-mod=readonly`, the identity binds the ENGINE
    projection, toolchain, platform and build context, byte-identity is
    claimed only for an identical context, and one fixture fails when an
    input outside the key changes the bytes or an input inside it does
    not change the identity.
  - DCW-05 (high, accepted in part): 8.1 states that every leg runs the
    production shell-to-engine path on the candidate's own engine with a
    receipt carrying the field, and names the one seam (enrollment) the
    bed cannot reach and the tests that cover it; the cutover leg exists;
    8.3 replaces the unenforced two-minute promise with a per-scenario
    ceiling enforced in `run_fixture_bed_scenarios`, and both fixture
    scripts join the files list (section 11, new).
  - DCW-06 (medium, accepted): `plans/goals.md` and
    `plans/goals-accepted.json` join the exclusions (1.1, 1.2, 2.3, 7,
    8.4); `ledgerPrefixes` is renamed `ledgerPaths` because it now holds
    files too; the sentence that omitted them is removed.
  - DCW-07 (low, accepted): the counselor row in 1.1 now says the
    register is appended locally after the goal transaction has published
    and carried by later landings; the exclusion stands on the other
    reasons.
- Revision 3, 2026-09-10, job dcwl-design2-20260910 (same lane), after
  the second critique (dcwl-crit2-20260910, codex gpt-5.6-sol; six
  findings, all material; dispositions by m1d binding). The last design
  round. The key, W as the floor, site 9 as today's call, exact
  recertification and the exclusion set of 1.1 stand; every other
  decision of revision 2 outside the six items stands.
  - DCW-08 (high, accepted): 4.4 rewritten. The observer passes the
    verify core as a callback, `ObserveParams.VerifyTesting`, replacing
    the value field `VerifiedTesting`; `readTestReceipt` calls it once
    for a field-bearing receipt before every fast path and before owner
    validation, and refuses with `chain-test-receipt-refused` and a
    detail naming which of the three conditions held (core error,
    insufficient result, other tree). A refusal rather than a command
    error because commit.sh maps a failed observe to
    `evaluator-unavailable` with the wrong repair text. One test in 8.4
    (cmd/metasystem) removes a retained attempt between commit.sh's run
    and the observer's. Section 9 gains the check; the third run of the
    core on one landing is named as the cost.
  - DCW-09 (high, accepted): the "success without a committed receipt"
    lifecycle at site 5 is withdrawn. Section 3, the site-5 row of
    section 2 and section 10 state the residual as the launcher makes
    it: a `PrepareSuccess` error is a failed attempt, a failed attempt
    owns nothing, and the battery runs again at today's cost; the W fast
    path is all this design adds at site 5. proof_run.go is named as the
    second caller in the site-5 row; the signature does not move, so
    section 11 names it as untouched.
  - DCW-10 (high, accepted): the cutover leg builds the engine of
    2fbd77535 once, from a `git archive` of that commit through
    `go-build.sh --out`, and uses it as the old engine; the shim is
    withdrawn with the reason; the cost is one build per bed run inside
    that scenario. The leg proves the three things named in 8.2: an old
    receipt lands on an unmoved tip, the same landing refuses at site 8's
    first sentence after the ledger move, and a field-bearing receipt is
    refused by the old engine, with a stripped control.
  - DCW-11 (medium, accepted): 8.3: `run_fixture_bed_scenarios`
    initialises the fixture budget itself when the scale is unset,
    idempotently, through the root it derives from its own location, and
    inherits a set scale; the new bed's `budget-standalone` and
    `budget-inherited` scenarios prove both starts.
  - DCW-12 (high, accepted): 8.3: every scenario runs in its own process
    group through bash job control (`set -m`; `setsid` is absent on
    macOS; both probed on this host), timeout and parent signals TERM
    the group, wait five seconds, KILL it, wait for the direct child and
    verify the group is empty; the new bed's `ceiling-reaps-group` and
    `signal-reaps-group` scenarios prove it with a TERM-ignoring
    grandchild. The `bed-scenario` cap gains a 1-through-120 override so
    the negative fixture waits seconds. The bed is
    scripts/agents/fixture-bed-scenarios-fixtures.sh, registered in
    testing.json and validate-metasystem.sh.
  - DCW-13 (high, accepted, narrowed): section 5 constraint 1 binds the
    toolchain closure (the selected `go` executable's bytes and its
    `GOROOT/VERSION`, resolved through the build's own environment under
    `GOTOOLCHAIN=local`) into the engine identity through a new
    `ToolchainClosureIdentity`, with the reason for choosing the closure
    over a proven-once claim. The byte-identity claim stands for an
    identical context. The same-report, different-bytes case is a unit
    test with a substituted toolchain path, not a two-compiler bed.
    `CompleteToolchainIdentityAtWithEnvironment` is unchanged; its
    widening is the one-line follow-up
    `toolchain-identity-binds-the-compiler-bytes`.
- Coordinator note, m1d, 2026-09-10 17:50 local, after revision 3 was
  written: the engine-bed chain landed on `main` as 016b83e2f ("Build
  receipt section engines from the candidate tree") while the second read
  was running, with the whole-tree synthetic-commit stamp
  (`bindMaterializedCandidateCommit`, cmd/metasystem/test.go:584-625 at
  that commit) and the `retainedCandidateEngineDigest` lookup keyed by
  `CandidateTree` (:965-1012). Section 5's two constraints are therefore
  no longer constraints on a neighbouring chain: they are decisions this
  chain builds, exactly as section 5 states them, on a base that includes
  016b83e2f. No design decision moves; only who builds it.
- Coordinator note, m1d, 2026-09-10 22:50 local, after build rounds 2 and
  3: section 8.1's first-transition seed cannot mint a receipt on this
  tree (`RequireFirstTransition`, internal/testpolicy/select.go:54-75,
  hands the seed to the protected probe, which enrolls the running engine
  as if built from the candidate commit and refuses a dirty stamp), and
  the cutover leg's control cannot isolate one field (the old engine's
  strict decoder also refuses the engine-bed chain's fields, and this
  chain's identity version 2). The legs are built on the round-3 and
  round-4 build briefs' decisions instead: the seed carries the contract
  on `main`; the candidate engine is stamped with the seed's first commit
  through `METASYSTEM_BUILD_STAMP` and `go-build.sh --out`; clone one is
  enrolled with `steward arm` in fixture mode (`metasystem.runtimes=fake`)
  and stopped before the leg returns; the old engine is built from
  6bc19ba1c, the last `main` before this chain; the control is the old
  engine's own receipt at the same path, no field stripped by hand. The
  five legs and the leg count of 13 stand. No design decision moves.
