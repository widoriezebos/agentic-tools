Working Mode: implement
Orchestrator Identity: m1d (lineage main-1788941004-20871-6e7a43, coordinator under goal delivery-candidate-is-the-workspace-not-the-ledger, closing read of chain dcwl-build2-20260910, final work round 2)
Date: 2026-09-11

# Review brief: closing read of the workspace-candidate implementation

FINDING IDS: chain-unique, continue the register: DCW-19, DCW-20, ... never F-n.

## What you are reading

The implementation chain dcwl-build2-20260910 for goal
delivery-candidate-is-the-workspace-not-the-ledger
(metasystem/plans/goals/delivery-candidate-is-the-workspace-not-the-ledger.md),
final work round 2 (job dcwl-build2-20260910-r2), reviewed tree c27379772a9fefb83d2e2ab5b63a2c2d3aadf66d (the installation subtree the conformance review names as reviewedTree; its whole-project tree is 32c57bfd75329926b6312da3713b00dda014a010), rebased onto
the remote tip immediately before this dispatch. Round 1 carries
the two green rounds of the first chain (dcwl-build1c-20260910, cancelled
by the coordinator's own re-arm, not by a finding: the engine and shell
code with unit tests; the harness and its bed) and adds the five land
legs; round 2 ends fixture runners with `steward disarm`, re-seeds the
cutover leg so the old engine's stamp equals its tip, and makes one
engine change you must judge (mandate 2b). All thirteen land legs, the
harness bed and the fast gate are green in the delegate's own run. The
specification is metasystem/plans/delivery-candidate-is-the-workspace-design.md,
revision 3 (sha256 1622cf7cfc4fd10259e23eabb0fdae2d1ba7a59a67666cea679f2b951bcca83b),
read twice on the Sol lane (metasystem/records/misc/delivery-candidate-is-the-workspace-critique-r1.md,
metasystem/records/misc/delivery-candidate-is-the-workspace-critique-r2.md;
thirteen findings, all folded) and carrying two coordinator notes: the
engine-bed chain (016b83e2f) landed under it, so section 5's constraints
are this chain's own code; and the land legs take the seed and cutover
recipe of the round-3 and round-4 build briefs (metasystem/plans/delivery-candidate-is-the-workspace-build-brief-r3.md,
metasystem/plans/delivery-candidate-is-the-workspace-build-brief-r4.md).

This is the second closing read of the same round. The first
(dcwl-cc1-20260911) found no material finding and five low ones
(DCW-14 to DCW-18) but named the whole-project tree as its reviewed tree,
which the recertification's join refuses; its register is at
`metasystem/artifacts/agents/dcwl-context/code-read-r1.md`. Report your
reviewed tree as c27379772a9fefb83d2e2ab5b63a2c2d3aadf66d. Read the diff
fresh; agree or disagree with DCW-14 to DCW-18 in one line each, and add
anything they missed. Do not re-derive what they settled.

Write your register as this new file: `metasystem/records/misc/delivery-candidate-is-the-workspace-code-read-r2.md`
(if your runtime is read-only, return it in the job return; the
coordinator projects it).

This is the independent critique of the FINAL work round that a
DESTRUCTIVE-REACH chain needs to close, and it is the last read this goal
can afford; a material finding here is folded and re-read, so make each
one exact. Read the diff against its base and the design side by side. A
material finding is: a behaviour the design specifies that the code does
not have; a behaviour the code has that the design forbids (section 9);
a test the design names that is missing or does not test what it says;
a refusal or message that differs from the page's text where the page
says "byte for byte"; or a way for a receipt to land that the key does
not cover.

## Settled, do not re-derive

The design's decisions. The coordinator has run outside the sandbox the
one Go test the implementer's sandbox could not
(`TestGroupOwnedLiveNonOwnerExitsNotOwned`: passes) and reruns the
dispatch and land beds on the reviewed tree; do not repeat those runs,
read the code.

## Mandate, in order of consequence

1. **The key at the three walls.** Trace a schema-2 receipt carrying
   `workspace` through land.sh site 8 (steps 1 to 5), `landing observe`
   (`ObserveParams.VerifyTesting`, `readTestReceipt` step 0 before every
   fast path, then exact, W, key) and the post-rebase `test verify`. Find
   any order in which a fast path runs before step 0, any path where a
   receipt without the field reaches a new call, and any way the
   observer accepts a result that is not sufficient or is for another
   tree.
2. **The engine identity (design section 5, now this chain's).**
   `candidateEngineBuildIdentity`: the engine projection tree from
   `enginePaths`, the toolchain closure (`ToolchainClosureIdentity`: the
   selected `go` executable's bytes and `GOROOT/VERSION`), platform and
   build context, committed with fixed dates. Check determinism (two
   candidates equal under the projection and different in a records file
   yield the same commit), that `GOFLAGS=-mod=readonly` reaches the build,
   that `retainedCandidateEngineDigest` matches only on the recorded build
   identity and never on T or W, and that the worker's digest check and
   `validateTestingReceiptEngineIdentity` still hold. Say whether the
   section-group identity is now stable across a ledger-only move on the
   reviewed tree, by reading `groupExecutionIdentity` and what feeds
   `CandidateEngineDigest`.
2b. **Round 2's compatibility shim.** `metasystem/cmd/metasystem/test.go`
   (the worker's request check) now accepts a request without
   `CandidateEngineBuildIdentity` when `METASYSTEM_POLICY_PROBE_WORKER`
   (the `policyProbeWorkerEnvironment` variable) is `1`, and
   `metasystem/internal/proofrun/test_build.go` records identity version
   1 (`candidateEngineDigestIdentityVersion`) for such a result instead
   of version 2. The implementer added it because the frozen
   public-version-1 probe of an OLD engine sends digest-only requests.
   Decide whether this is a legacy-only door or a weakening: who sets
   that variable, whether a current engine can reach the digest-only
   path outside the probe, whether a version-1 result can be reused as
   delivery evidence by a current engine, and whether `test_result.go`'s
   validation still refuses a version-2 result without a build identity.
3. **Exact reuse and provenance (section 3).** `ExactReusableTestResult`
   compares the per-group identities and the digests and treats
   `CandidateTree`, `BaseCommit`, `PolicyBaseCommit`, `PlanDigest` as
   provenance; `PublishCommittedReceipt` takes the accepted index tree
   from every caller (three call sites) and never an empty string; site 5
   is exact-then-W on the index posture only and the working posture is
   exact.
4. **The projection (section 1).** `FilterTreePrefixes` removes a
   directory prefix and a file in one call, an absent path is a no-op,
   the result equals `git rm -r --cached`; `WorkspaceExclusions()` names
   the seven paths; `ProjectWorkspaceTree` joins the installation prefix;
   `InstallationWorkspaceTree` does not; the receipt's `workspace` field
   is recomputed by every reader and refused when it differs.
5. **The beds (section 8).** The five land legs do what 8.2 says,
   including the real old engine in `receipt-cutover` built from
   6bc19ba1c and its three proofs (the control is the old engine's own
   receipt at the same path); the harness ceiling, self-initialised
   budget and process-group reap; the new bed's four scenarios; the
   testing.json and validate-metasystem.sh rows. Name any assertion that
   would pass on the base tree (a leg that proves nothing).
6. **What must not change (section 9).** `Snapshot`, `StagedTree`,
   `SnapshotRelevant`, `FilterTree` unchanged; no working-tree posture
   filtered; commit.sh untouched; the recertified path exact; no group
   left `testing.json`; the `goal-records` surface untouched; every
   refusal 1b12f534 made is still made.
7. **The thirteen reopening triggers.** For DCW-01 to DCW-13 (the two
   registers), one line each: met by the code, or not, with the line.

## What is not in your mandate

Style, comment density, and re-deriving the design. Do not run beds.

## Return

Findings by id (DCW-19 onward) with the file and line in the reviewed
tree, the design section, the claim, the evidence, and what changes if it
stands; then the thirteen trigger lines; then a verdict line: closable as
is, or one fold naming which findings. Wall-clock budget: 35 minutes.
