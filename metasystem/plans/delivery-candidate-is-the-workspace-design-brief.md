Working Mode: design
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Design brief: the delivery candidate is the workspace, not the ledger

## Your authority to author this design

You are dispatched to AUTHOR a design. The composed prompt's Working Modes
table forbids design authoring to every delegate; that row is scoped to the
mode, not the role, and goal `design-prohibition-is-role-scoped` exists to
fix it. Until it lands this paragraph is the override, and it is standing
authority: R-89-m1d (Wido, 2026-09-09: "switch back to Claude using Fable
5.1 for designs now") and R-25 put design authoring on this lane. Author the
design; do not stop to report an authority conflict. Dispositions,
certification, the ledger and the receipt stay with the orchestrator.

## The defect, exactly

Wido, 2026-09-10 16:40, verbatim: "We should never have to wait until it
gets quiet."

A landing on one seat is voided by any goal-ledger publish on any other
seat. The goal ledger lives on `main`: every `goal open`, `claim`, `edit`,
`park`, `done` on any machine is a commit that rewrites one file under
`plans/goals/` (or moves it to `records/goals/`) and pushes. The shared
testing contract binds a delivery candidate to the WHOLE-PROJECT index
tree, so a receipt taken on tree T is refused at landing once the checkout
has rebased onto a tip that moved only under `plans/goals/`. Measured on
m1b today: four receipts of one candidate, each about 50 minutes and green
on every group but a load timeout, each void before it finished, with other
seats publishing 15 times in 20 minutes; the only landing route left was
asking the fleet to hold ledger writes for an hour. On this seat yesterday:
three recertified landings parked on `chain-recertification-target-moved`
for the same reason.

The goal record is
`metasystem/plans/goals/delivery-candidate-is-the-workspace-not-the-ledger.md`.
Read its Intent: it names DONE. The umbrella that explains the family is
`metasystem/plans/goals/the-metasystem-validates-itself-with-itself.md`.

## The mechanism as it stands, read at 2fbd77535 (tree 8cb35ec3)

Paths are repository-relative. Line numbers are from that commit.

**The contract's own design already promises what this goal asks.**
`metasystem/plans/application-testing-contract-design.md` line 231: "A
changed candidate consisting only of proven irrelevant coordination records
can consume the original receipt: schema-2 acceptance compares its component
input identities against the current candidate and rechecks current
index/worktree and provenance, rather than requiring irrelevant whole-tree
object-id equality. Record the old proved tree and current landing tree
distinctly." The receipt has both fields (`tree` and `provedTree`,
`metasystem/internal/landing/receipt.go` lines 33-45), but every reader
requires them equal to the live index. This design makes the tree do what
that sentence says; do not re-derive the principle, cite it.

**Where the whole-project tree is bound today, twelve sites.**

1. `metasystem/cmd/metasystem/test.go:235`, in `prepareTesting` (`:176`):
   "delivery candidate must equal the real whole-project index". `test run`,
   `test verify` and `landing test-receipt --mode` all pass through it.
2. `metasystem/cmd/metasystem/test.go:551`: the proof-launch admission's
   identity inputs start with `prepared.CandidateTree`, so a repeat after a
   ledger-only move is admitted as a NEW attempt instead of returning
   reusable success (76). `:566-568`: `ExactReusableTestResult`, then the
   composed `ReusedTestResult` when no exact match exists.
3. `metasystem/internal/proofrun/test_result.go:258`
   `ExactReusableTestResult` refuses unless `CandidateTree`, `BaseCommit`,
   `PolicyBaseCommit`, `PlanDigest` and the group selection all equal the
   template's. After any tip move nothing is exact. The composed path
   (`reusedTestResult`, `:296`) reuses per group by `ExecutionIdentity`.
4. `metasystem/internal/proofrun/test_build.go:641`
   `groupExecutionIdentity` hashes the group definition, the INPUT digest,
   the environment, tool identities, discovery, platform, the four
   contract/policy digests, and for `section` groups the candidate ENGINE
   digest. It does not hash the candidate tree. `:482`
   `RevalidateRetainedGroupExecutionIdentities` recomputes the identities in
   a detached worktree of the candidate. This is why a ledger-only move
   leaves every group identity intact today, and why the receipt is the
   thing that breaks, not the proof.
5. `metasystem/internal/landing/testing.go:24` `PrepareTestingReceiptPayload`
   refuses unless `result.CandidateTree == tree` and both postures from
   `testingReceiptPosture` (`:160`: index = `StagedTree()`, the whole
   project; working = `SnapshotRelevant(candidate, inputs)`) equal `tree`
   before and after owner validation.
6. `metasystem/internal/landing/receipt.go:177` `PublishCommittedReceipt`:
   the schema-2 branch requires the same posture equality before it projects
   the committed payload to `artifacts/agents/landing/receipts/<tree>.json`.
7. `metasystem/internal/landing/receipt.go:438` `readTestReceipt`, schema-2
   branch: `:482` "schema-2 whole-project receipt does not project to
   landing candidate" (`TreeOf(receipt.Tree)` must equal the metasystem
   subtree being landed); the four bindings must equal `receipt.Tree`; then
   the live posture must equal `receipt.Tree` again.
8. `metasystem/scripts/agents/land.sh:365` `check_supplied_test_receipt`:
   for a schema-2 receipt the receipt's `tree` must equal
   `staged_project_tree` (`:361`, `git write-tree` of the real index). This
   is the first wall a rebased checkout hits, with the message "make the
   receipt against this exact candidate".
9. `metasystem/scripts/agents/land.sh:596-618`, the ordinary chain path:
   fetch, `rebase_origin` (`:413`), then `verify_current_testing_proof`
   (`:497`: `test verify --tree HEAD^{tree} --mode auto --purpose
   delivery`), then the push loop, which rebases and verifies again on a
   rejected push.
10. `metasystem/scripts/agents/commit.sh:262` `proved_tree` is the whole
    index; `:272` `test verify --tree $proved_tree`; `:435-447` the settled
    tree must still equal it; `:462` `landing observe --tree <metasystem
    subtree>`; `:573-580` the landed commit's tree must equal `proved_tree`.
    The commit happens BEFORE the rebase, so this postcondition is not the
    problem; it must stay.
11. `metasystem/cmd/metasystem/landing_verbs.go:49` `landing test-receipt
    --mode`: runs `test run` with `--tree`, then `PublishCommittedReceipt`
    when the result names an attempt, else `CreateTestingReceipt`.
12. The recertified chain path: `metasystem/scripts/agents/land.sh:471`
    `check_recertification_target` and
    `metasystem/internal/landing/observe.go:86` (`recertifiedTargetMoved`)
    and `:213` (`head != verified.Record.TargetCommit`) park the landing
    with `chain-recertification-target-moved` whenever origin moved at all.

**The primitives.** `metasystem/internal/gittree/gittree.go:284`
`FilterTree(tree, paths)` rewrites a tree with named paths removed through
`update-index --force-remove` run from the toplevel; its existing callers
pass FILE paths. Check what `--force-remove` does with a directory name
before you rely on it for `plans/goals/**`: if it removes nothing, the
projection needs `git ls-files --with-tree` enumeration or `rm -r --cached`
in the isolated index, and you say which. `:223` `TreeOf`, `:255`
`Snapshot`; `metasystem/internal/gittree/snapshotscope.go:246`
`StagedTree`, `:417` `SnapshotRelevant`.

**The ledger paths, as written by verbs.** The last forty goal-verb
commits on `main` touch `metasystem/plans/goals/` and `metasystem/records/goals/`
and nothing else. The two append-only registers are `memory/receipts.log`
and `records/narrator-digest.log` (the carriage classifier at
`metasystem/internal/landing/observe.go:774` names them). The counselor
appends `records/counselor/misclassification-register.jsonl`
(`metasystem/internal/counselor/register.go:53`) on a goal edit. The
contract's `goal-records` surface (`metasystem/testing.json:20`) owns
`plans/**`, `records/**`, `memory/rulings.md`, `memory/receipts.log` and
selects only the two static section groups; that surface stays.

**The documented rule you are changing.** `metasystem/docs/project-rules.md:44`
states the whole-project-bytes rule in one sentence. The design says what
the sentence becomes.

## Two chains that will land next to yours; do not fight them

Both patches are in this checkout for you to read; they are not landed.

- `metasystem/artifacts/agents/dcwl-context/lrsrd-carry-round2.patch` (m1e, goal
  landing-receipt-survives-records-drift, land-ready). It adds
  `internal/landing/registers.go` with one declaration,
  `appendOnlyRegisters`, filters the WORKING-TREE projection by it at the
  receipt's posture reads, adds a `worktreeProjection` field, and adds the
  `landing drift` and `landing advance` verbs. Its rule is "index never
  filtered". Your projection set must be the same declaration widened, in
  the same file, so whichever chain lands second folds by extending a
  slice, not by reconciling two policies. Say that in the page.
- `metasystem/artifacts/agents/dcwl-context/rbce-build1-r4.patch` (m1b, goal
  receipt-beds-run-the-candidate-engine, land-ready, parked until THIS goal
  lands). It builds the proof engine from the candidate tree and stamps it
  with a synthetic candidate commit (`bindMaterializedCandidateCommit`);
  section groups then hash that engine's digest into their execution
  identity (site 4 above). If that stamp is a function of the whole tree, a
  ledger-only move changes every section group's identity and your landing
  is void again through the back door. Decide what the stamp must be a
  function of (the workspace projection, or the ENGINE projection that
  `go-build.sh` already knows) and say it as a constraint on that chain,
  not as a change you make.

## What you must decide

1. **The workspace projection W.** Exactly which paths are excluded from the
   whole-project tree to make W, where the set is declared (one declaration,
   shared with the append-only registers above), and how a directory prefix
   is filtered. Say whether the counselor register is in or out and why.
   W must be a real tree object so it can be named, diffed and detached.
2. **Which identity each of the twelve sites compares.** Within one checkout
   the index-versus-worktree posture check protects against drift while the
   battery runs; that stays exact. The comparisons that cross a tip move
   (receipt versus staged candidate at landing, receipt versus HEAD after
   the rebase, retained result versus a re-run) compare W. Go through the
   twelve and say, per site, exact or W, and what the receipt records
   (`tree` stays the exact T the proof ran on; `provedTree` stays; W gets
   its own field; a version-1 reader must not misread it).
3. **The proof side.** `prepareTesting` (site 1): the rule becomes W(request
   tree) equals W(index)? Or does `test run` keep the exact rule and only
   `verify` and the landing relax it? Say which, and what the admission
   identity (site 2) uses so a repeat after a ledger-only move returns
   reusable success instead of admitting a new attempt. `ExactReusableTestResult`
   (site 3) compares W, and `PublishCommittedReceipt` (site 6) can no longer
   require posture equality with the ORIGINAL tree after a move; say whether
   the committed payload is re-projected under the new T with the old
   `provedTree`, or the composed `CreateTestingReceipt` path is the one that
   serves a moved checkout.
4. **The landing.** `check_supplied_test_receipt` (site 8) and
   `readTestReceipt` (site 7) accept a receipt whose W equals the staged
   candidate's W. After `rebase_origin` (site 9), `test verify` on the new
   HEAD must find the retained proof sufficient: say exactly what makes that
   true (group identities are tree-independent already; the section engine
   digest must be W-stable; exact reuse is W-based) and what
   `verify_current_testing_proof` compares.
5. **A workspace change in A..B.** Another seat lands an engine change
   between the receipt and the landing. After the rebase W(HEAD) differs
   from the proved W; the landing refuses with a named refusal code and the
   message names the changed paths. Say the code.
6. **The recertified path (site 12).** Today any origin move parks the
   chain. Decide whether recertification binds `TargetCommit` (stays parked
   on a ledger-only move, out of this goal's scope, with the reason written)
   or W(target) (lands after a ledger-only move). The goal's sentence "the
   landing rebases onto the moved tip and verifies the same W" is about the
   ordinary chain path at least; if you leave recertification exact, name
   the follow-up goal in one line.
7. **The documented rule.** The one sentence at
   `metasystem/docs/project-rules.md:44` as it will read.

## What must not change

- The exact tree T the proof ran on is still recorded, and a receipt for T
  is still unlandable against a staged candidate whose W differs.
- The index-versus-worktree drift check inside one checkout stays exact: a
  staged or unstaged change to any non-ledger path between the battery and
  the commit still refuses with the existing messages.
- No group leaves `testing.json`; the `goal-records` surface stays as it is.
- commit.sh's postcondition (site 10: the landed tree equals the proved
  tree at commit time) stays.
- `Snapshot`, `StagedTree`, `SnapshotRelevant` keep their contracts for
  every other caller; filter at the call sites or add a sibling, never
  change what those functions return by default.
- Nothing from 1b12f534 (the shared testing contract) gets weaker: every
  refusal the contract makes for a relevant change is still made.

## Fixtures: the design names them, the build proves them

Each with the smallest proving run and a two-minute ceiling.
`metasystem/scripts/agents/land-fixtures.sh` already builds a bare origin,
clones, runs goal verbs under a fixture lineage (`:211`) and lands receipted
chains (`full_chain_receipt`, `:847`); extend it rather than inventing a bed.

- **Ledger-only move lands.** A receipt is taken at tip A in clone one. A
  SECOND clone runs a goal verb (open, claim, or edit) under its own fixture
  lineage and pushes, so A..B holds only goal-verb commits from another
  checkout. Clone one lands the receipted chain: fetch, rebase, verify,
  push all succeed, and the pushed commit's parent is B. On the untouched
  tree this fixture must fail at site 8's message.
- **Workspace change in A..B refuses.** Same shape, but the second clone
  lands a change under `internal/` or `scripts/`. Clone one's landing
  refuses with the code from decision 5, naming the path.
- **Drift inside the checkout still refuses.** A staged change to a
  non-ledger path after the receipt still refuses with the existing
  message. This one exists to prove the exclusion did not widen.
- **Unit tests** for the projection of a directory prefix, for W-based exact
  reuse, for `readTestReceipt` accepting a W-equal receipt and refusing a
  W-different one, and for the receipt's version rule.

## Deliverable

Write this new file: `metasystem/plans/delivery-candidate-is-the-workspace-design.md`

Ground every claim in the tree; cite lines from the revision you read
(2fbd77535). State plainly anything the code cannot answer rather than
inventing it. End the page with a short revision record. Wall-clock budget:
60 minutes. Design only; you implement nothing and run no bed.
