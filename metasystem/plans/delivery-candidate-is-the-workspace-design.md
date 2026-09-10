# delivery-candidate-is-the-workspace — design: the delivery candidate is the workspace, not the ledger (revision 1)

Goal: plans/goals/delivery-candidate-is-the-workspace-not-the-ledger.md
(sixth member of plans/goals/the-metasystem-validates-itself-with-itself.md).
Ruling: Wido, 2026-09-10 16:40, verbatim: "We should never have to wait
until it gets quiet." Every cite below is read at main 2fbd77535 (whole
tree 8cb35ec3); paths are repository-relative under `metasystem/` unless
they name the two unlanded patches.

## 0. What is wrong, and what the contract already promises

The goal ledger lives on `main`. Every `goal open`, `claim`, `edit`,
`park`, `done` on any seat is a commit that rewrites one file under
`plans/goals/` or moves it to `records/goals/` and pushes. The last forty
goal-verb commits on `main` touch `plans/goals/**` and `records/goals/**`
and nothing else (read with `git log --name-only`; the one `done`,
eaf73ff97, moves `plans/goals/breach-stop-wedges-seat.md` to
`records/goals/`). The shared testing contract binds a delivery candidate
to the whole-project index tree, so a receipt for tree T is refused once
the checkout has rebased onto a tip whose only difference is under the
ledger.

The contract's own design already promises the fix.
plans/application-testing-contract-design.md line 231: "A changed
candidate consisting only of proven irrelevant coordination records can
consume the original receipt: schema-2 acceptance compares its component
input identities against the current candidate and rechecks current
index/worktree and provenance, rather than requiring irrelevant whole-tree
object-id equality. Record the old proved tree and current landing tree
distinctly." The receipt has both fields (`tree` and `provedTree`,
internal/landing/receipt.go:35 and :42), and every reader requires them
equal to the live index. This design makes the tree do what that sentence
says. It adds no new principle.

Two words used throughout:

- **T** is the exact whole-project tree the proof ran on. It stays
  recorded everywhere it is recorded today.
- **W** is the workspace projection of a tree: the same tree with the
  ledger paths removed. W is a real tree object (a `write-tree` id), so it
  can be named, diffed with `ChangedPaths`, and checked out with
  `NewDetachedWorktree`.

The rule in one sentence: within one checkout, comparisons stay exact;
across a tip move, comparisons use W.

## 1. Decision 1: the workspace projection W

### 1.1 The excluded set

W removes these workspace-relative paths from a tree:

| Path | Kind | Who writes it | Why it is ledger, not workspace |
| --- | --- | --- | --- |
| `plans/goals/` | directory prefix | every goal verb (the forty commits above) | the goal file is rewritten on every verb, on every seat |
| `records/goals/` | directory prefix | `goal done` (internal/goal/validate.go:38 names the records root; eaf73ff97 shows the move) | same writer, same cadence |
| `records/counselor/` | directory prefix | `goal accept-risk` (cmd/metasystem/goalsync_mutations.go:350) and a tier raise on `goal edit` (:1460), through internal/counselor/register.go:53 and compute.go:14 | appended by goal verbs in the same ledger commit; tracked (`git ls-files` lists both registers); read by the counselor report and by no testing group in testing.json |
| `memory/receipts.log` | file | the append-only register the carriage classifier names (internal/landing/observe.go:793) | already the carry chain's register |
| `records/narrator-digest.log` | file | same (observe.go:793) | same |

The counselor register is OUT of W (excluded) for the reason in the
table: it moves with goal verbs from other seats, and no group declares it
as an input. `plans/goals.md` and `plans/goals-accepted.json`, which
internal/behaviorsurface/policy.v2.json:33-34 still lists as coordination
paths, are not in the tree at 2fbd77535 (`git ls-files` names neither; the
migration commit 10ea45bce deleted both), so the exclusion does
not name them; if a verb brings them back, widening is one line.
`memory/rulings.md` stays IN W: it is a declared input
of `fast-static-build` (testing.json:27) and is written by landings, not
by goal verbs.

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

The brief asks for "the same declaration widened". Widening that slice
with `plans/goals` is wrong, because a goal file is rewritten, never
appended: `landing drift` would classify a dirty goal file as
`register-not-append` and `landing advance` would refuse a rebase that
changes it with `advance-register-contended`. So the fold is one FILE, not
one slice. `internal/landing/registers.go` holds:

```go
// appendOnlyRegisters: unchanged from the carry chain (files, append-only).
var appendOnlyRegisters = []string{"memory/receipts.log", "records/narrator-digest.log"}

// ledgerPrefixes: directories that goal verbs rewrite on every seat.
var ledgerPrefixes = []string{"plans/goals", "records/goals", "records/counselor"}

// WorkspaceExclusions returns appendOnlyRegisters followed by ledgerPrefixes,
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
| 2 | admission identity, test.go:551 | `IdentityInputs[0] = T`; manifest digest over T (:522, proof_run.go:439) | **W**: `IdentityInputs[0] = W`, `testingCandidateManifest` taken over a detached worktree of W | the whole-identity digest (attempt.go:701-743) |
| 3 | `ExactReusableTestResult`, internal/proofrun/test_result.go:267 | `CandidateTree`, `BaseCommit`, `PolicyBaseCommit`, `PlanDigest` (over T) must equal | **W**: compares `WorkspaceTree`; `BaseCommit` and `PolicyBaseCommit` become recorded provenance, not compared; `PlanDigest` is computed over W | the exact committed payload is returned after a ledger move |
| 4 | `groupExecutionIdentity`, test_build.go:641 | hashes group, inputs, env, tools, discovery, platform, four digests, section engine digest; never the tree | **unchanged**; the section engine digest must be W-stable (section 5) | group identities already are |
| 5 | `PrepareTestingReceiptPayload`, internal/landing/testing.go:29-45 | `result.CandidateTree == tree`; index posture and working posture both `== tree`, before and after owner validation | `result.CandidateTree == tree` stays exact; **index posture compares W**; working posture stays exact against T | the index moved under a running battery |
| 6 | `PublishCommittedReceipt`, receipt.go:199-201 | index and working posture `== receipt.Tree` | **index compares W**; working posture exact against `receipt.Tree`; payload published unchanged | the index moved between commit of the payload and its projection |
| 7 | `readTestReceipt` schema-2, receipt.go:480-503 | `TreeOf(receipt.Tree) == params.CandidateTree`; four bindings `== receipt.Tree`; live posture `== receipt.Tree` | **`InstallationWorkspaceTree(TreeOf(receipt.Tree)) == InstallationWorkspaceTree(params.CandidateTree)`**; four bindings stay exact against `receipt.Tree`; live index posture compares W; working posture exact against `receipt.Tree`; plus the new field checks of 2.3 | the staged candidate after a rebase |
| 8 | `check_supplied_test_receipt`, scripts/agents/land.sh:385 | `receipt.tree == staged_project_tree` | **W**: `landing workspace --tree` on both sides; refusal keeps its first sentence and adds the changed workspace paths | same |
| 9 | chain path, land.sh:596-597 and :614-616 | `test verify --tree HEAD^{tree}` | **`test verify --tree HEAD^{tree} --receipt <path>`**: W of the receipt must equal W of the tree, refusal code of section 4; then composed reuse as today | the rebased HEAD |
| 10 | commit.sh:262, :272, :435-447, :462, :573-580 | whole index proved, settled, observed, landed | **unchanged**; the commit happens before the rebase | nothing |
| 11 | `landing test-receipt --mode`, cmd/metasystem/landing_verbs.go:49-107 | run, then publish committed payload or compose | **unchanged in structure**; the exact path is now reached after a ledger move because site 3 matches on W | none of its own |
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
is about this check, and this design keeps it: W is compared between two
INDEX trees taken at different tips, never between the index and the
worktree.

### 2.2 Why the group identities survive already

`groupExecutionIdentity` (test_build.go:641-665) hashes the group
definition, the input digest, the environment digest, tool identities,
discovery, platform, the four contract and policy digests, and for
`section` groups the candidate engine digest. It does not hash the tree.
`RevalidateRetainedGroupExecutionIdentities` (:482-545) re-hashes declared
inputs and executables in a detached worktree of the candidate; ledger
paths are declared by no group, so the input digests are equal before and
after a ledger move. This is why the receipt breaks today and the proof
does not, and why the composed reuse (`reusedTestResult`, test_result.go:296)
already serves `test verify` after a rebase.

Two other inputs to reuse deserve a sentence. `PolicyEngineDigest` is the
enrolled engine's bytes (test.go:371-402), stable across a ledger move;
its source check `VerifySourceAtDestination` compares ENGINE projections
of the two commits, not the commits (internal/steward/rearm_resolver.go:287-303),
so a ledger-only tip passes it. `AccountingRevision` is the landing goal's
own claim-time revision (test.go:889); a raise after claim on THIS goal
still voids reuse (test_result.go:262, :321), and that is right: it is a
change to the goal, not a bystander's publish.

### 2.3 What the receipt records

Schema-2 testing receipts gain one field and keep their version:

```json
"tree": "<T, the exact whole-project tree the proof ran on>",
"provedTree": "<T, as today>",
"workspace": {"excludes": ["memory/receipts.log", "plans/goals", "records/counselor", "records/goals", "records/narrator-digest.log"], "tree": "<W = ProjectWorkspaceTree(T)>"}
```

The type is the carry chain's `TestReceiptProjection{Excludes []string; Tree string}`
(patch lines 1391-1394), declared once under that name so the second chain
to land drops its copy. The field name is `workspace`, distinct from the
carry chain's `worktreeProjection` (a version-3 command-receipt field with
a different meaning), and the carry chain's "mixes schema versions"
refusal for `worktreeProjection` on schema 2 stays.

Version rule. Readers use `DisallowUnknownFields` (receipt.go:459 and
testing.go:149), so an engine older than this design refuses a receipt
with the new field as malformed rather than misreading it; that is the
fail-closed reading the brief asks for. A new engine reading a schema-2
receipt WITHOUT the field applies the exact rule at every site, as today
(line 231: "Legacy receipts keep their stricter original rules"). A new
engine reading a receipt WITH the field recomputes
`ProjectWorkspaceTree(receipt.tree)` with its own `WorkspaceExclusions()`
and refuses when `workspace.tree` or `workspace.excludes` differs. No
reader ever trusts the field without recomputing it.

`TestResult` gains `workspaceTree` (json, omitempty) set by `NewTestResult`
from `TestRunRequest.WorkspaceTree`; `ValidateTestResult` requires a valid
tree id when present. `PlanDigest` (`TestPlanDigest`, test_result.go:229)
takes W in place of T. Attempt records keep T in `TestResult.CandidateTree`
as provenance.

## 3. Decision 3: the proof side

`test run` keeps the exact rule at site 1: the battery runs on exactly the
index, and the receipt binds the index posture. Nothing at site 1 crosses
a tip. What changes is what retained evidence is compared against.

Admission (site 2). At 2fbd77535 a repeat after a ledger-only move does
NOT open a new attempt by itself: `repeatDecisionLocked` (attempt.go:546-552)
asks the component decision first, and `componentDecisionLocked`
(:558-643) keys on goal, accounting revision and per-group identity,
never on the tree; with every group already successful it returns
reusable success (76). The whole-identity path (`noChildDecisionLocked`,
:701-743) is reached only without component identities. The brief's
sentence "admitted as a NEW attempt" becomes true under the engine-bed
chain, where a whole-tree-stamped engine changes every section identity
(section 5). This design still moves `IdentityInputs[0]` to W and takes
the manifest digest over W, so the attempt's recorded identity names the
thing reuse compares and two concurrent `test run`s on one seat after a
ledger move are recognised as duplicates by the whole-identity path too.

Exact reuse (site 3). After 76 the CLI tries `ExactReusableTestResult`
(test.go:566) and today falls to the composed projection because
`CandidateTree`, `BaseCommit`, `PolicyBaseCommit` and `PlanDigest` all
moved. With W compared, the plan digest over W, and the two commits
demoted to provenance, the exact projection matches, `AttemptID` is kept,
and `landing test-receipt` takes `PublishCommittedReceipt` (site 11,
landing_verbs.go:95-96). Dropping the two commits from the comparison
loses nothing they protect: the policy base contributed
`BaseContractDigest` and `PolicyEngineDigest`, the merge base contributed
the changed paths and therefore the plan, and all of those are still
compared (`ContractDigest`, `BaseContractDigest`, `PolicyEngineDigest`,
`BehaviorPolicyDigest`, `PlanDigest`, `RequiredGroups`, `SelectedGroups`,
per-group identities).

Publication (sites 5 and 6). The committed payload is published
unchanged: `tree` stays T, `provedTree` stays T, `workspace.tree` is W.
It is never re-projected under the new tip; rewriting `tree` would forge
an observation that never happened, and line 229 makes the committed
payload immutable. The moved checkout is served by every consumer
comparing W. The composed `CreateTestingReceipt` path serves only what it
serves today: a selection that no single attempt owns; its `tree` and
`provedTree` name the tree the composition was verified against, and each
group's `reuseAttempt` names where it ran. That is pre-existing and not
changed here.

## 4. Decisions 4 and 5: the landing

Site 8 (land.sh:365-389). For a schema-2 receipt the check becomes
`landing workspace --root . --tree <receipt tree>` equals
`landing workspace --root . --tree $(git write-tree)`. `landing workspace`
is a new read-only verb that prints `ProjectWorkspaceTree(tree)`; a
schema-2 receipt without the `workspace` field keeps the exact comparison.
The refusal keeps its first sentence byte for byte ("names tree X but the
staged candidate is Y; make the receipt against this exact candidate"),
because land-fixtures.sh:834 greps it, and appends one line:
`changed workspace paths: <ChangedPaths(W_receipt, W_staged)>`.

Site 7 (receipt.go:480-503) is the second reading of the same receipt,
inside `landing observe` at commit time, with the rules in the table.
`readTestReceipt` and `test verify --receipt` share one function:

```go
// ReceiptWorkspaceBinding recomputes W for a schema-2 receipt and a tree and
// returns the workspace paths that differ (empty when they are equal). A
// receipt without the workspace field compares exact trees.
func ReceiptWorkspaceBinding(root, receiptPath, tree string) (changed []string, err error)
```

Site 9 (land.sh:596-618). After `rebase_origin`, `verify_current_testing_proof`
passes `--receipt "$landing_test_receipt"` when a receipt was supplied.
`test verify` (cmd/metasystem/test.go:777) first calls
`ReceiptWorkspaceBinding(root, receipt, --tree)`; when paths differ it
prints

```
workspace-changed-after-proof: the retained receipt proves workspace <W1>, the rebased tree has workspace <W2>; changed paths: internal/landing/receipt.go, scripts/agents/land.sh
```

and exits 1 before any reuse computation. When W is equal it proceeds
exactly as today: `RevalidateRetainedGroupExecutionIdentities` then
`ReusedTestResult`, sufficient or "missing required proof". What makes
the retained proof sufficient after a ledger-only rebase, in order: the
group identities are tree-independent (2.2); the section engine digest is
W-stable (section 5); the goal and accounting revision are unchanged
(2.2); exact reuse is not needed here because `verify` composes.

The code is registered in internal/refusal/register.go as
`{Code: "workspace-changed-after-proof", Owner: "cmd/metasystem", Site: "test.go:<line>", Shape: Agent, Override: "landing test-receipt --root . --tree <rebased tree> --mode auto, then land again", Commands: 2}`.
The override is cheap on purpose: when the upstream change touched no
declared input of any selected group, the new receipt is a reusable
success with zero execution (component reuse, 2.2), so refusing on any
workspace difference costs a re-receipt, not a re-run. This is stricter
than today for an upstream change outside every declared input (today
`verify` would pass it); the brief asks for the refusal, and stricter is
allowed under "nothing from 1b12f534 gets weaker".

Recovery after that refusal is today's flow and is not changed here: the
rebased commit exists locally and was not pushed; the seat re-receipts
against `HEAD^{tree}` and lands again. land.sh has no "push an existing
commit" path; that is a follow-up if it bites (one line: goal
`land-resumes-a-refused-push`, not opened by this design).

A receipt for T stays unlandable against a staged candidate whose W
differs: site 8 refuses it before commit, site 7 refuses it at commit,
site 9 refuses it after the rebase. The three walls are the same
comparison in three places.

## 5. Constraints on the two neighbouring chains

Carry chain (`lrsrd-carry-round2.patch`, goal
landing-receipt-survives-records-drift, land-ready). Fold rule in 1.2:
one file, `appendOnlyRegisters` unchanged, `ledgerPrefixes` and
`WorkspaceExclusions()` added beside it; `TestReceiptProjection` declared
once. Its `landing advance` replaces `git rebase` at land.sh:414 and is
compatible with site 9: advance moves the branch, then `test verify --receipt`
runs on the result. Its rule "index never filtered" is kept as 2.1 reads
it.

Engine-bed chain (`rbce-build1-r4.patch`, goal
receipt-beds-run-the-candidate-engine, parked until this goal lands). Two
constraints, stated as constraints on that chain, not as changes made
here:

1. **The stamp must be a function of the ENGINE projection, never of T or W.**
   `bindMaterializedCandidateCommit` (patch lines 179-221) makes a
   synthetic commit of `HEAD^{tree}` of the detached candidate, and
   `go-build.sh --trimpath --out` links that id into the binary
   (patch lines 1356-1358). A whole-tree stamp changes on every ledger
   move, the engine bytes change, `candidateEngine.Digest` changes, and
   every `section` group identity changes through
   `groupExecutionIdentity` (test_build.go:642-645). The stamp must be a
   deterministic function of the candidate's ENGINE projection: the
   `enginePaths` list at internal/behaviorsurface/policy.v2.json:3-10
   (`cmd/**`, `internal/**`, `scripts/agents/**`, `go.mod`, `go.sum`,
   `records/misc/goals-migration-manifest.md`), which
   `archivedEngineDigestAtCommit` (rearm_resolver.go:307-323) already
   computes for enrolled engines. Two candidates with equal ENGINE
   projections must produce byte-identical proof engines. Whether the
   stamp is the synthetic commit id of an ENGINE-only tree (forty hex,
   the `commitBuildStamp` shape at rearm_resolver.go:37) or the
   `witness-<12 hex>` form (:38, the form go-build.sh:41-42 describes as
   "the judged engine-input digest") is that chain's choice; the ENGINE
   projection is not W (W keeps `docs/**`, `testing.json`, the skills and
   the rest), so the stamp must not be W either.
2. **`retainedCandidateEngineDigest` must match on W, not T.** Patch lines
   314-360 look up the retained candidate engine digest by
   `result.CandidateTree == prepared.CandidateTree` and by a receipt at
   `TestReceiptPath(root, prepared.CandidateTree)`. After a ledger move
   neither matches and `test verify` refuses with "candidate engine digest
   is absent from retained evidence". The match must be
   `result.WorkspaceTree == prepared.WorkspaceTree`, and the receipt
   lookup must accept a receipt whose `workspace.tree` equals the
   prepared W.

## 6. Decision 6: the recertified path stays exact

Recertification binds `TargetCommit` (internal/validate/recertification.go:57)
and the verifier replays the certified merge onto `TargetTree` (:1044-1096:
resolve the target, replay `q` on `TargetTree`, require `MergedTree`,
require `MergedWholeTree` from a graft on `TargetCommit`). land.sh:471-479
and observe.go:86 and :213 park with `chain-recertification-target-moved`
when HEAD or origin differs from that commit, and :490 requires the
landed commit's parent to be it. Rebinding to W(target) means: on every
ledger-only move, re-derive `MergedTree` on the new target tree, re-verify
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
record binds W(target) and the verifier replays the certified merge onto
any target whose W equals it.

## 7. Decision 7: the documented rule

docs/project-rules.md:44 today: "The candidate index and every
delivery-relevant working-tree input must describe the same whole-project
bytes before selected testing evidence can authorize delivery; a mismatch
is repaired, never treated as permission to publish partial coverage."

As it will read: "The candidate index and every delivery-relevant
working-tree input must describe the same workspace bytes (the whole
project minus the goal ledger paths that goal verbs rewrite: `plans/goals`,
`records/goals`, `records/counselor`, `memory/receipts.log`,
`records/narrator-digest.log`) before selected testing evidence can
authorize delivery; the exact tree the proof ran on is still recorded, a
tip move that touches only the ledger never voids that proof, and a
mismatch outside the ledger is repaired, never treated as permission to
publish partial coverage."

## 8. Fixtures: the design names them, the build proves them

All three shell fixtures extend scripts/agents/land-fixtures.sh as two new
bed scenarios in the list at :30-32 (the success line's leg count moves
from 9 to 11). Each is one isolated leg under `make_leg` (:50), with a
two-minute ceiling under the fixture-budget cap.

The leg needs a contract-bearing repository, which the full-width-chain
leg does not have (it adds and removes `testing.contract` at :801-811 and
takes a legacy command receipt at :844-846). The recipe is the Go test's
(cmd/metasystem/landing_verbs_test.go:1352-1375): `metasystem.conf` with
`testing.contract=testing.json`, `metasystem.steward.landing-ref` set to
`refs/remotes/origin/main` (test.go:919-934 requires it), a `testing.json`
with two `sh -c` command groups producing junit reports, the goal `fx`
claimed under the fixture lineage (:211-220), and the seed pushed WITHOUT
`testing.json` on `main` so the policy base has no contract and
`trustedPolicyEngine` takes the first-transition path (test.go:377,
`os.Executable()`), which needs no enrolled engine in the bed. That
limitation is stated in the fixture header: the enrolled-engine path is
covered by the unit tests below and by this chain's own landing.

The peer's ledger move follows the seed's own recipe (:205-223): the peer
clone sets `goal.sync-remote local`, `goal.sync-branch refs/heads/metasystem/goals`,
`metasystem.goal.machine fixture-peer`, points that branch at `origin/main`,
runs one goal verb (`goal open --id peer-goal ...` under
`METASYSTEM_OWNER_LINEAGE=land-receipt-fixture-peer`), then pushes
`refs/heads/metasystem/goals:refs/heads/main`. A..B is then one commit
touching only `plans/goals/peer-goal.md`.

- **Ledger-only move lands** (scenario `ledger-move-lands`). Clone one
  stages `payload.txt`, takes a receipt with
  `landing test-receipt --root . --tree $(git write-tree) --mode auto --goal fx --cap-min 1`
  at tip A. The peer publishes B as above. Clone one runs
  `land.sh -m ... --chain <chain> --test-receipt <receipt> --staged-only --skip-transport`.
  Assert: exit 0; the chain log shows the receipt path; origin `main`
  equals clone one's HEAD; `HEAD^1` equals B; the pushed tree contains
  `plans/goals/peer-goal.md`. On the untouched tree this scenario must
  fail at site 8's first sentence (the fixture greps for it and reports
  "site 8 refused a ledger-only move" so the failure is legible).
- **Workspace change in A..B refuses** (scenario `workspace-move-refuses`).
  Same start; the peer instead appends a comment to
  `scripts/agents/coverage-delta.sh`, commits plainly and pushes. Clone
  one's landing exits non-zero, `land.out` contains
  `workspace-changed-after-proof` and `scripts/agents/coverage-delta.sh`,
  origin `main` still equals B, and clone one's HEAD is not on origin.
- **Drift inside the checkout still refuses** (inside `ledger-move-lands`,
  before the peer moves). After the receipt, `printf x >>payload.txt`
  unstaged: `land.sh` refuses with the existing "unstaged changes remain
  after staging" (land.sh:325). Then stage that change: `land.sh` refuses
  at site 8 with the existing first sentence. Both leave HEAD at A. This
  proves the exclusion did not widen.

Unit tests, each named by the file it lands in:

- internal/gittree: `FilterTreePrefixes` removes a directory prefix and
  leaves siblings; an absent prefix is a no-op that returns a tree; a
  file path and a prefix in one call; the result equals the tree from
  `git rm -r --cached` on the same input (the probe's equality).
- internal/landing: `ProjectWorkspaceTree` joins the installation prefix
  in a nested checkout and not at the toplevel; `readTestReceipt` accepts
  a schema-2 receipt whose W equals the staged candidate's W after the
  fixture rewrites `plans/goals/x.md` and refuses it after the fixture
  edits `scripts/x.sh`; refuses a receipt whose `workspace.tree` disagrees
  with the recomputed W; refuses a receipt whose `workspace.excludes`
  differs from `WorkspaceExclusions()`; a schema-2 receipt without the
  field keeps the exact rule; `PublishCommittedReceipt` projects after a
  ledger-only index move and refuses after a workspace index move.
- internal/proofrun: `ExactReusableTestResult` returns the original
  result when only `CandidateTree`, `BaseCommit` and `PolicyBaseCommit`
  differ and `WorkspaceTree` is equal; refuses when `WorkspaceTree`
  differs; `TestPlanDigest` is equal for two trees with equal W.
- cmd/metasystem: `test verify --receipt` exits 1 with
  `workspace-changed-after-proof` naming the path after a workspace move
  and exits 0 after a ledger-only move; `landing workspace` prints the
  same id for two trees that differ only under `plans/goals`.

## 9. What must not change, as checks the build can make

- T is still recorded: `receipt.tree`, `receipt.provedTree`,
  `TestResult.CandidateTree`, the attempt's `IdentityInputs` provenance.
- A receipt for T is unlandable against a staged candidate whose W
  differs: sites 7, 8, 9 (section 4, last paragraph).
- The drift check inside one checkout stays exact (2.1; third fixture).
- No group leaves testing.json; the `goal-records` surface is untouched.
- commit.sh's postcondition stays: :573-580 still compares the landed
  tree with `proved_tree`.
- `Snapshot`, `StagedTree`, `SnapshotRelevant`, `FilterTree` keep their
  contracts; the new code is `FilterTreePrefixes` and the two landing
  helpers.
- Every refusal the contract makes for a relevant change is still made,
  and one is added (section 4).

## 10. Facts the code could not answer

- Which process moves a seat's checkout under a running battery. No
  fast-forward or pull of the checkout was found in internal/steward or
  the goal verbs (grep for `ff-only`, `fast-forward`, `pull`). The design
  does not depend on the answer: sites 5 and 6 compare W whether the move
  happened during or after the battery.
- Whether `plans/goals.md` and `plans/goals-accepted.json` are written by
  any current verb. The forty-commit sample says no; they stay in W.

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
