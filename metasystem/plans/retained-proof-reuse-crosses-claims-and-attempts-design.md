# retained-proof-reuse-crosses-claims-and-attempts: design

Goal 10 of plans/delivery-efficiency-plan.md. One critique read of this page,
then the build lands in two slices under the one goal, each with its own
fixtures and its own critic read; the third clause is parked here with its
reason.

## 1. Why reuse fires for 3.6 percent of groups

Retained proof is reused only when a group's execution identity is
unchanged, and three things churn that identity for no reason that touches
the group's outcome (proof-attempts.md section 5 of the delivery deep dive):

1. **The Go closure is backwards.** `discoveryFromGoCatalog`
   (internal/proofrun/test_go.go) marks a package relevant when it is
   imported by a relevant package (forward, right) AND when it imports a
   relevant package (backward, wrong). For `internal/testpolicy`, the
   backward rule pulls in `cmd/metasystem`, and from there every package of
   the module: 985 files in a group whose tests read one package. Any edit
   anywhere in the module changes every Go group's identity.
2. **The judge's file digest is in every identity.** `groupExecutionIdentity`
   (test_build.go) binds `PolicyEngineDigest`, the sha256 of the enrolled
   engine binary. Every rebuild, that is every landing, changes it, so no
   identity survives a landing even when nothing the group runs has changed.
3. **Reuse is scoped to one goal, one accounting revision and a successful
   terminal.** `reusedTestResult` (test_result.go), `test verify` and the
   worker's composition all filter attempts by goal id and accounting
   revision, and until ee957a7b7 by a success terminal. A group proven green
   under goal A on the same bytes is invisible to goal B.

Section groups are different: they run the candidate engine, so their
identity rightly carries the candidate engine digest (46 identities in 47
adoption-fixtures runs is the candidate changing, not churn).

## 2. Slice 1: the identity

### 2.1 The Go closure is the test-aware forward dependency closure

- Roots are the group's declared packages. Their files, test files, embed
  files and testdata trees enter the closure, as today.
- From the roots, the walk follows `Imports`, `TestImports` and
  `XTestImports` of the roots, then `Imports` only of every package reached
  (a dependency's own tests do not run). Nothing is added for importing a
  root. This is what `go list -deps -test <roots>` names, computed from the
  catalog `go list -json ./...` already loads once per module and
  environment.
- A root contributes its package files, test files, embeds, test embeds and
  its `testdata/**`. A reached dependency contributes only what the root's
  test binary compiles from it: its package files and embeds; its own test
  files, test embeds and testdata belong to the groups that run its tests.
  `go.mod` and `go.sum` stay in every closure.
- Packages outside the module (standard library, module cache) stay outside
  the manifest, as today: the Go tool identity and `go.sum` cover them.
- The declared `inputs` of a group stay as the contract author wrote them
  and are still hashed; the closure is the implicit part of the manifest.
  A retained manifest is re-hashed by `RevalidateRetainedGroupExecutionIdentities`
  from the recorded manifest, so identities computed before this change
  simply never match after it, which is the intended invalidation.

### 2.2 A judge-compatibility key replaces the judge's file digest

- `PolicyEngineDigest` leaves `groupExecutionIdentity`. In its place the
  identity binds `JudgeKey`: `judge/v<N>` (a constant in internal/proofrun,
  bumped by hand when the runner, the identity composition or the result
  vocabulary changes meaning) and the git ids, at the policy base commit, of
  what the engine binary is built from: the `cmd` and `internal` trees,
  `go.mod` and `go.sum` (four `git rev-parse` reads, no archive). The judge
  is the whole binary: coverage verdicts live in internal/audit, the worker
  in cmd/metasystem, and a hand list of judge packages would miss one
  (design critique M1); enrollment's engine projection digest would say the
  same but costs an archive per run and timed out once under load in the
  frozen corpus. A rebuild of an unchanged commit keeps both parts; any
  engine source change at the policy base changes the second, so an
  unbumped semantic change still invalidates. The cost is that every landing
  of engine code invalidates once; a records, plans, docs or scripts landing
  does not.
- The key is computed once per `test run`, `test plan` and `test verify` in
  `prepareTesting` (cmd/metasystem/test.go) from the policy base commit the
  trusted engine already binds, and carried in `TestRunRequest` and in the
  result (`JudgeKey`), beside `PolicyEngineDigest`, which stays recorded
  for the receipt and the retained-engine authentication but no longer
  enters group identities.
- One composition per attempt: the launcher plans every group's identity
  and the worker records that planned identity for the groups it launches
  instead of recomputing it with its own composition, while still checking
  the inputs digest before and after the run. A launcher and a worker of
  different builds (an engine rebuilt but not re-armed) can no longer
  disagree inside one attempt (design critique M2). The worker's packet
  decoder refuses unknown fields, so a worker older than this landing
  refuses a packet that carries `JudgeKey`: every seat rebuilds and re-arms
  before its next test run, as the seats page records for this landing;
  identities retained before it never match again, which is the intended
  one-time invalidation.
- A source the commit does not carry reads `absent` (an adopted application
  without engine sources, a fixture repository), so its key is stable. A
  repository or commit git cannot read yields a key with a fresh token that
  matches nothing, never one equal to another unreadable run (code critique
  F-2): a key never fails a run, and two unknown judges never look alike.
- `ExactReusableTestResult` and the composition's result-level checks
  compare `JudgeKey` where they compare `PolicyEngineDigest` today, for the
  same reason. The contract digest, base-contract digest and
  behavior-policy digest stay.

### 2.3 Fixtures for slice 1

1. `TestGoClosureFollowsDependenciesNotDependents` (proofrun): a module with
   packages `a`, `b` (imports `a`), `c` (imports nothing, has a testdata file
   and an embed); the closure of a group on `a` holds `a`'s files and not
   `b`'s; the closure of a group on `b` holds `a` and `b`; the closure of a
   group on `c` holds its testdata and embed file.
2. `TestReStageOutsideAClosureKeepsTheIdentity` (proofrun): after
   `PrepareGroupExecutionIdentities` on tree T1, an edit to `b` gives tree
   T2 whose identities keep `a`'s group equal and change `b`'s; an edit to
   `c/testdata/x.txt` changes `c`'s group alone.
3. `TestJudgeKeyKeepsIdentityAcrossARebuild` (proofrun): two requests that
   differ only in `PolicyEngineDigest` give equal identities; two that
   differ in `JudgeKey` give different ones; section groups still differ
   when the candidate engine digest differs.
4. The existing identity tests (`TestCandidateEngineIsBuiltFromCandidateTreeAndBindsExecutionIdentity`
   and its siblings in cmd/metasystem) keep passing with `JudgeKey` in the
   request.

## 3. Slice 2: the reuse scope

### 3.1 The newest observation on the seat decides

- An observation of a group is a retained record of that group at an
  execution identity: a `TestResult` group with `NativeLaunched` or status
  `reused`, or a live attempt's plan for it, which a joined nested attempt
  keeps in `PendingTestGroups` and a top-level `test run` publishes as
  `group:<id>:<identity>` in its proof identity inputs (design critique
  M4; the composer already reads both). A `not-run` or `invalid` record is
  not an observation: the group was not judged there.
- For each selected group the composer scans every attempt in the seat's
  control root, keeps the observations whose identity equals the current
  one, and takes the newest by the group's own end time (`GroupResult.EndedAt`;
  a reused record inherits its source's), a live plan ranking newest of all.
  Attempt start is the wrong clock under overlap (design critique M3): an
  attempt that started later can have judged the group earlier. The group
  is reused only when that newest observation is a `passed` (or `reused`)
  collection-complete group of an attempt that has a terminal, whatever
  the terminal result (a cancelled attempt's complete pass is a pass, as
  the validators already hold). A newest observation that is live, or
  whose status is failed, invalid-by-execution, unavailable, cancelled or
  any other non-pass, blocks reuse: the group runs afresh. Goal id,
  accounting revision and the attempt's terminal result no longer scope
  the scan; `ReusableTerminal` from ee957a7b7 is retired with it, because
  the observation's own status is now the criterion. The price of the live
  rule: a delivery attempt beside a cadence run on one seat reuses nothing
  the cadence run is executing; the intent asks for it, and the cadence run
  is rare beside a landing.
- The composition's result-level checks stay: contract digest,
  base-contract digest, judge key and behavior-policy digest of the source
  result must equal the template's.
- `validateRetainedGroupReuse` (runner) and `validateTestingAttemptOwners`
  (receipt) keep reading the named attempt from the control root and
  requiring its record of the group to be passed and collection-complete at
  the same identity; the owner attempt must have a terminal (never live);
  its terminal result is no longer a criterion.
- `ExactReusableTestResult` keeps its goal and accounting-revision scope:
  it recovers a committed receipt, which belongs to one goal.
- The frozen protection corpus probe `forge-component-reuse`
  (cmd/metasystem/test_protection.go) keeps holding: a reuse naming an
  attempt the control root never retained, or one whose record of the
  group is not a complete pass at the identity, is still refused.
- Admission (`componentDecisionLocked`, the live-duplicate and
  retry-required decisions) keeps its goal scope: it governs one goal's
  attempts and budget, not evidence.

### 3.2 Cadence attempts execute every group afresh

- A cadence-purpose template composes no reuse: `reusedTestResult` returns
  every selected group `not-run` with reason `cadence-executes-afresh`, so
  the worker hands the runner an empty `Reused` map and every group launches.
  The result already records the attempt id and `LaunchCounts`, which is the
  executed-group count; the cadence battery is what establishes trust in a
  judge and it never inherits from an earlier judge or seat.

### 3.3 Fixtures for slice 2

1. `TestReuseCrossesGoalsAndAttempts` (proofrun): attempt A under goal
   `x` (revision 3) proves `first` green; a template under goal `y`
   (revision 1) with the same identity reuses `first` from A.
2. `TestGreenThenRedYieldsNoReuse` (proofrun): A green, then B (newer)
   fails `first` at the same identity: the projection leaves `first`
   not-run; C (newer still) passes it again: reused from C.
3. `TestALiveObservationBlocksReuse` (proofrun): a live attempt whose
   `PendingTestGroups` holds the identity blocks reuse.
4. `TestVerifyRefusesWithoutAMatchingRetainedResult` (cmd/metasystem):
   `test verify` on a tree whose required group has no observation at its
   identity reports missing groups and a non-zero status (the existing
   verify path; the fixture pins it).
5. `TestCadenceComposesNoReuse` (proofrun).
6. The receipt fixture from ee957a7b7 keeps holding with the owner rule
   relaxed to "terminal, and a complete pass of the group".

## 4. Parked: the nested witness gate (absorbed clause)

The absorbed text is truncated in the goal record ("... (the r1 cr") and
the source goal's page was never committed; the paraphrase there is the
only surviving text (design critique, record defect).

The clause absorbed from goal nested-gate-witness-reuse (the adopt bed's
witness gate reusing the outer gate's retained proof for identical bytes,
scripts/agents/witness-gate.sh, base-commit engine as sole judge) is a
shell-side mechanism over a different artifact (the go-gate witness), not
the proof-attempt records this goal's two slices change. It lands as its
own slice after slice 2, designed on its own page, so the Go identity and
scope changes are not held by it. Recorded here so the goal's DONE reads
it as pending, not forgotten.

## 5. Soundness

- Reusing across goals is sound because an execution identity binds
  everything the run reads (closure bytes, declared inputs, tools,
  environment, discovery, platform, contract, judge key, behavior policy,
  and for section and steward groups the candidate engine): two attempts
  with equal identities ran the same test on the same bytes with the same
  judge. The goal that paid for it is bookkeeping, not evidence.
- Newest-wins is sound because a later failure at the same identity is
  evidence that the pass does not hold on this seat (flake, environment,
  load), and the honest answer is to run again.
- The judge key is sound because the judge's meaning lives in its source;
  a rebuild of the same source is the same judge. The manual version
  covers changes that the three packages' bytes do not show (a runner
  semantic moved elsewhere); the tree ids cover an unbumped change.
- The forward closure is sound because a test's outcome depends on what it
  compiles and runs, which is what it imports, never on what imports it.
  Test binaries link the root's test imports, so those are followed at the
  root and not below.

## 6. What the field will show

Trust: three fresh green cadence runs after each slice (the goal's own
words). The measure of DONE's intent: the share of groups reused per
delivery attempt, read from the attempt records a week after slice 2.
