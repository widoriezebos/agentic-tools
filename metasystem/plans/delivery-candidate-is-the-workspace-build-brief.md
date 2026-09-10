Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, dispatch delegate under goal delivery-candidate-is-the-workspace-not-the-ledger)
Date: 2026-09-10

# Goal

Goal delivery-candidate-is-the-workspace-not-the-ledger (approved by Wido
2026-09-10, tier 3). Its record,
metasystem/plans/goals/delivery-candidate-is-the-workspace-not-the-ledger.md,
is the contract. Wido's sentence: "We should never have to wait until it
gets quiet." Today a landing receipt is bound to the whole-project index
tree, so any goal-ledger publish by another seat between the receipt and
the push voids the landing. DONE, from the record: a retained proof stays
sufficient when only paths no selected test group reads have moved; the
landing rebases onto the moved tip and verifies the same proof; a fixture
proves a receipt taken at tip A lands at tip B where A..B holds only
goal-verb commits from another checkout, and another proves a moved
declared input still refuses.

# The design is the specification

metasystem/plans/delivery-candidate-is-the-workspace-design.md, revision 3
(sha256 1622cf7cfc4fd10259e23eabb0fdae2d1ba7a59a67666cea679f2b951bcca83b, with the coordinator note at the end of its revision record),
is in your worktree. Build exactly what it says; where it names a
function, a field, a code, a message or a fixture, use that name. It was
read twice by an independent critic
(metasystem/artifacts/agents/dcwl-context/critique-r1.md and
metasystem/artifacts/agents/dcwl-context/critique-r2.md, thirteen findings,
all folded); the decisions are settled and revision 3 is the last design
round.
If the tree contradicts the page at a cited line, build what the tree
requires, say so in your return under `deviations`, and do not stop.

Read these sections in this order: 1 (W and the primitives), 2 (the
twelve sites, the table), 4 (the landing: one engine function, site 8's
five steps, the observer's step 0 that must not fail open), 3 (the proof
side; site 5 is the W fast path and nothing more), 2.3 (the receipt
field), 8 (fixtures: the bed, the five legs including the real
old-engine cutover, the harness ceiling with self-initialised budget and
process-group reaping, the new harness bed), 11 (files), 9 (what must
not change).

# Landed since the page was read: the engine-bed chain is in your base

Your worktree's base includes 016b83e2f, "Build receipt section engines
from the candidate tree" (m1b, goal receipt-beds-run-the-candidate-engine),
which the page read as an UNLANDED patch. Read the page's section 5 as
decisions you build, not constraints on someone else. What that commit
did, at the lines in your worktree:

- `buildCandidateEngine` (metasystem/cmd/metasystem/test.go:529) builds
  the proof engine from the candidate tree before admission and
  `bindMaterializedCandidateCommit` (:584-625) stamps it with a synthetic
  commit of `HEAD^{tree}`, the WHOLE candidate tree, so every ledger-only
  move changes the stamp, the engine bytes, `CandidateEngineDigest`, and
  every `section` group's execution identity
  (metasystem/internal/proofrun/test_build.go:641-648). That defeats the
  goal's ledger-only fixture until you build section 5's constraint 1:
  the stamp is a deterministic function of the ENGINE projection of the
  candidate's installation subtree (`enginePaths`,
  metasystem/internal/behaviorsurface/policy.v2.json:3-10; build an
  engine-only tree from the candidate with the same enumerate-and-write
  idiom as `FilterTreePrefixes`, kept paths instead of removed ones, and
  `commit-tree` THAT), the build forces `GOFLAGS=-mod=readonly`
  (`candidateEngineBuildEnvironment`, test.go:512 sets `GOFLAGS=`), and
  the toolchain closure identity `ToolchainClosureIdentity` of section 5
  exists in metasystem/internal/proofrun beside
  metasystem/internal/proofrun/execution_context.go:91.
- `retainedCandidateEngineDigest` (test.go:965-1012) locates the retained
  candidate-engine digest by `result.CandidateTree == prepared.CandidateTree`
  and by a receipt at `TestReceiptPath(prepared.CandidateTree)`; after any
  tip move it refuses "candidate engine digest is absent from retained
  evidence". Build section 5's constraint 2: the result and the receipt
  record the engine BUILD identity beside the digest (a new field on
  `TestResult` and on the receipt, versioned the way
  `CandidateEngineIdentityVersion` is, metasystem/internal/proofrun/test_result.go:178-179),
  and the lookup matches on that identity, never on T or W. `test verify`
  launches no build (the contract: identity and evidence checks only), so
  the identity is computed from the tree and the toolchain, never by
  rebuilding.
- Keep what that commit protects: the worker's digest check on the engine
  it runs (test.go:858-870), `validateTestingReceiptEngineIdentity`
  (metasystem/internal/landing/testing.go:166), and the receipt's
  `policyEngineDigest` and `candidateEngineDigest` fields
  (metasystem/internal/landing/receipt.go:44-45).
- The two fixtures of section 5 are yours: the engine built twice from
  two candidates equal under the ENGINE projection and different in a
  records file yields equal digests, and a `go.sum` change changes the
  identity; and the `ToolchainClosureIdentity` unit test with a
  substituted toolchain path.

# Facts the page assumes, verified at 2fbd77535

- Before 016b83e2f the candidate engine was the running engine
  (`os.Executable()` into `testingRunRequest`, test.go:485-492) and group
  identities were tree-independent; the section above is what changed.
- Readers of `readTestReceipt`: metasystem/internal/landing/observe.go:180,
  :185, :224 and metasystem/internal/landing/tierone.go:83. The tier-one
  reader gets the key path for free through `ObserveParams.VerifiedTesting`;
  it needs no change of its own, but its test must keep passing.
- Callers of `PublishCommittedReceipt`:
  metasystem/cmd/metasystem/landing_verbs.go:96, :154, :191. The page
  gives it the accepted index tree from `landing test-receipt`'s
  `--tree`; the other two callers must pass a tree too (the page's site 6:
  the tree the reuse decision was judged on), never an empty string.
- Callers of `ExactReusableTestResult`: metasystem/cmd/metasystem/test.go:566
  only. `CreateTestingReceipt`: landing_verbs.go:101 only.
- The carry chain's `internal/landing/registers.go` (goal
  landing-receipt-survives-records-drift, m1e) may or may not have landed
  when you start. Check `git log --oneline -3 -- '*/internal/landing/registers.go'`
  in your worktree. If it is there, ADD `ledgerPaths` and
  `WorkspaceExclusions()` beside `appendOnlyRegisters` and reuse its
  `TestReceiptProjection`. If it is not, CREATE the file with
  `appendOnlyRegisters`, `ledgerPaths`, `WorkspaceExclusions()` and the
  `TestReceiptProjection` type exactly as section 1.2 and 2.3 name them,
  so that chain folds by deleting its duplicate.
- `landing` verbs register in metasystem/cmd/metasystem/main.go:130-136;
  `landing workspace` is one more row there.
- The refusal register is metasystem/internal/refusal/register.go; the
  fast gate (metasystem/scripts/agents/go-gate.sh --fast) re-proves its
  rows, so a malformed row fails the gate.
- The shared fixture scripts metasystem/scripts/agents/fixture-bed-scenarios.sh
  and metasystem/scripts/agents/fixture-budget.sh are sourced by other
  beds (brain, mission, second-session, return-schema, witness-gate). The
  per-scenario ceiling
  of section 8.3 reaches them all: keep it a hang bound (base 120 s
  scaled 8 to 48), never a duration assertion, and run one other bed's
  section once to prove nothing healthy is cut.

# Decisions (the orchestrator's; decided, not open)

1. Build in this order, each step green before the next: gittree
   (`FilterTreePrefixes` and tests); landing (registers, the two helpers,
   the receipt field, site 5, site 6, `readTestReceipt` with step 0 and
   `ObserveParams.VerifyTesting`, tests); proofrun
   (`ExactReusableTestResult` and tests); cmd/metasystem
   (`verifyRetainedTesting`, the refusal line and row, `landing workspace`,
   `landing observe` passing the verify core as the callback, the accepted
   tree for `PublishCommittedReceipt`, tests); land.sh site 8; the docs
   sentence; then section 5's engine identity (the engine-projection
   stamp, `-mod=readonly`, `ToolchainClosureIdentity`, the recorded build
   identity and the lookup keyed by it, with their tests). That is this
   round. The beds are the NEXT round of this same chain: the harness
   (ceiling, budget, process group) and its new bed with testing.json and
   validate-metasystem.sh rows, then the five land-fixtures legs. Do not
   start them; if the cap arrives before this round's list is green,
   return with what is green and say what is red.
2. Do not change `Snapshot`, `StagedTree`, `SnapshotRelevant` or
   `FilterTree`; do not filter any working-tree posture; do not touch the
   recertified path, commit.sh, or the two constraints on the engine-bed
   chain (section 5 is for that chain's owner, not you).
3. Messages and codes are the page's, byte for byte where the page says
   "byte for byte" (land.sh site 8's first sentence, which
   land-fixtures.sh:834 greps).
4. Every new test name is one of section 8.4's; every new shell scenario
   is one of section 8.2's or 8.3's, in the lists the page names, with
   land-fixtures' success line moved to the leg count the page states.
   The cutover leg pins the old engine to commit 2fbd77535 and builds it
   through `go-build.sh --out` from a `git archive`, as 8.2 says; no shim.
5. No new flag on `test verify`. Site 9 (`verify_current_testing_proof`,
   land.sh:497-505) does not change.

# Proof before you return

- `go build ./... && go vet ./...` in metasystem/.
- The unit tests of every package you touched, plus
  `go test ./internal/landing/... ./internal/proofrun/... ./internal/gittree/... ./cmd/metasystem/...`.
- `bash scripts/agents/go-gate.sh --fast` green.
- `bash scripts/agents/land-fixtures.sh` still green with today's nine
  legs (the new legs are the next round's); `bash scripts/agents/dispatch-fixtures.sh`
  still green (it asserts the candidate engine stamp, and the stamp
  changes in this round).
- `metasystem test plan --root . --mode auto --purpose delivery --goal delivery-candidate-is-the-workspace-not-the-ledger`
  on your staged candidate, and the plan's selected groups in your return.
  Do not take a receipt; the orchestrator does.

# Return

`diffBoundary` and `files` are repository-root paths (`metasystem/...`).
List under `evidence` every command above with its observed result. List
under `deviations` every place the tree made you depart from the page,
with the line. Wall-clock expectation: 120 minutes; if a step will not go
green within it, stop, return what is green, and say what is red and why.
