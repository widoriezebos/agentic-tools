# Candidate engine for landing receipts

Status: Withdrawn before implementation. On 2026-09-10 the coordinator found that m1b already owned and was implementing the candidate-engine correction under receipt-beds-run-the-candidate-engine, chain rbce-build1. This record preserves the abandoned proposal and is not an implementation contract. Its four accepted review findings remain diagnostic evidence; the actual m1b implementation requires its own verification.

Date: 2026-09-10. Goal: gate-fence-returns-to-per-landing-proof. Design round 2.

**Settled decision:** prepare and retain one exact candidate engine during test-run preparation. Pass its existing path/digest fields and owner-pinned Git metadata to the unchanged enrolled worker. Keep source/build binding local to preparation and verification. No packet migration, enrollment change or candidate policy-worker substitution.

The human's collect-all ruling resolves the stage question. Preserve RunTestPlan and TestStageCollectsIndependentFailuresAndNativePrerequisiteResults byte-for-byte. Development canaries pass before the host elects to start expensive proof; once selected proof starts, collect every independent group across all selected stages. The stale dispatcher instruction is outside this goal.

## Implementation contract

1. **One commit-metadata owner.** Add CandidateCommitEnvironment in metasystem/internal/gittree/detached.go. Return six fixed entries: GIT_AUTHOR_NAME and GIT_COMMITTER_NAME are MetaSystem; both corresponding EMAIL variables are metasystem@invalid; GIT_AUTHOR_DATE and GIT_COMMITTER_DATE are 2000-01-01T00:00:00Z. Use these explicitly on NewDetachedWorktree's existing commit-tree invocation. Preserve its candidate tree, captured parent, message, subtree graft, HEAD/index checks and cleanup. Do not change ScrubbedEnviron or mutate process-wide environment.

   After prepareTesting sanitizes environment, overlay the same owner-generated entries for a MetaSystem source candidate in both TestRunRequest.Environment and LaunchSuite.Environment. The launch environment matters because the old worker's Git owner reads os.Environ. Reject conflicting selected group.Env values for these six names; equal values are allowed. New preparation and the unchanged worker then produce the same synthetic commit for the same candidate and parent. Check the caller's HEAD still equals captured BaseCommit before worker launch and before publishing success; a changed base refuses. The execution checkout remains exclusively owned and stable during proof.

2. **Small local preparation owner.** Add metasystem/internal/proofrun/candidate_engine.go and call it from runTestRun and runTestVerify in metasystem/cmd/metasystem/test.go. No new public flag, configuration key, worker request/result field or artifact framework.

   Detect the MetaSystem module at the candidate installation: github.com/widoriezebos/agentic-tools/metasystem. If present, require its command source and existing build script. A base containing that module followed by candidate source removal/corruption refuses; do not call that an adopted-app fallback. If neither base nor candidate has the module, preserve the current supplied-executable/digest path, native adapters and command-only support without requiring Go or adding the Git metadata overlay.

   Create one detached whole-project candidate; compile from its installation directory with the existing scripts/agents/go-build.sh --out and the actual full synthetic HEAD as METASYSTEM_BUILD_STAMP. Override ambient stamp values. Use the explicit environment owner with GOFLAGS=-mod=readonly, GOWORK=off, GOTOOLCHAIN=local and CGO_ENABLED=0 for this proof build; reject ambient modfile/overlay redirection before applying the overlay, consistent with the existing gate. The source candidate and enrolled installation are never build output destinations.

3. **Retained input/executable binding.** Store builds in the existing control evidence hierarchy, installation-relative artifacts/agents/proof-runs/candidate-engines/<input-key>/. Do not hardcode the installation prefix. The input key is SHA-256 of canonical version-1 metadata: whole-project candidate tree, captured base, installation prefix, computed synthetic commit, six metadata pins, exact build argv/effective environment, platform, and resolved Bash/Go executable identities using existing helpers. Also bind the candidate installation's pre-build FullDigest; the whole-project tree binds candidate files outside that installation.

   Pure lookup resolves and hashes tools without version commands. Preparation may retain existing tool-version metadata and count those actual helper launches. Stored tool paths/digests must still match on lookup; GOTOOLCHAIN=local prevents an implicit downloaded compiler switch.

   Use the existing file-lock pattern with nonblocking acquisition bounded by the preparation deadline, rechecking the binding after acquisition. Each build uses a unique child directory with a retained log and output. Atomically publish binding.json through atomicfile only after all checks. It contains the input metadata, executable relative path, SHA-256/mode/stamp, log identity and actual preparation cost. Treat uncertain durability according to atomicfile's existing outcome, without calling it durable. Never replace a valid published binding. Retain incomplete build directories; a missing binding is a cache miss, a corrupt published binding/executable is a loud refusal.

   Check HEAD, index and FullDigest before/after compilation. Read the finished Go executable's build information without executing it; require the linked stamp to equal full synthetic HEAD, then hash it, set mode 0500, and rehash before publication. Use private directories and regular-file/no-symlink checks. One valid retained build serves all selected section/steward consumers and future matching runs. No per-group rebuild.

4. **Bounds and diagnostics.** Build, lock wait and metadata preparation share the existing limits.sectionCap ceiling. Run the build through boundedexec with the remaining ceiling so its owned descendants cannot survive expiry. Count the actual build and helper launches in PreparationLaunches, and all lookup/wait/build/metadata time in PreparationDurationMS. This follows the existing pre-admission preparation owner; add no admission stage or reservation protocol.

   Open retained stdout/stderr before launch. Nonzero build, timeout, invalid stamp, source or executable mutation, and publication/retention failure return TEST_CANDIDATE_ENGINE_BUILD_FAILED or TEST_CANDIDATE_ENGINE_INVALID with candidate identity, native/timeout cause, elapsed/ceiling and diagnostics location. Use the existing preparation refusal outcome and start no worker after failure. Preserve compiler diagnostics outside the disposable source bed before cleanup; if retention fails, keep the bed and name it. No enrolled-engine fallback on failed candidate build, no pruning of failed evidence.

5. **Existing worker identity remains compatible.** Every request construction in runTestRun, including pre-admission identity calculation and reuse projection, receives the prepared CandidateEngine and CandidateEngineDigest. Carry the normalized metadata environment consistently. Keep SharedEngine tied to its existing launcher owner; it is distinct from the section executable.

   Preserve groupExecutionIdentity's computation and TestRunRequest's schema. The old worker already includes candidate executable digest for sections and the selected MetaSystem steward consumer, and hashes the six metadata values through its existing environment digest. Source binding stays in the retained descriptor plus the existing candidate tree and checked commit stamp; add no per-tree environment token that would invalidate unrelated groups.

   Keep prepareSectionEngine's digest/type/mode/no-overwrite/private-copy checks, and the worker's packet, policy-digest, custody and deadline checks. Revalidate the retained binding immediately before launch and before accepting/publishing success. Preserve ordinary workerEngine = prepared.PolicyEngine, trusted destination policy, admission, frozen protection, terminal ownership and receipt checks. Candidate stamp equality is never enrollment authority.

6. **Read-only verification and reuse.** runTestVerify reconstructs the same metadata overlay and exact candidate-build lookup, validates source/key/stamp/tool/executable bytes, then supplies its digest to existing retained group revalidation. Never substitute os.Executable for a MetaSystem source candidate. Lookup must launch no compilation, version helper or discovery. A missing binding reports insufficient delivery proof and names test run; corruption refuses. A build alone is not test evidence: existing matching successful terminal outer ownership remains mandatory.

   A different candidate tree or base changes the synthetic commit/key, even for a record-only change. Such a checkout cannot consume a differently stamped executable; engine-consuming groups require matching new build/evidence. Unchanged non-engine groups retain the existing component-reuse rules, since the six metadata pins are constant rather than per-tree.

## Selection and exact code/test boundary

At this base metasystem/testing.json already gives section/gate-fence-fixtures its runtime-custody obligation and includes it in runtime-custody deep selection. Preserve both and the cadence entry. Add TestGateFenceRemainsRequiredForRuntimeCustody in metasystem/internal/testpolicy/select_test.go: load the actual contract, select an engine change affecting runtime custody under the accepted goal risk, and require gate-fence as delivery evidence. Also assert that missing its successful result makes delivery insufficient using the existing result owner. If the demotion arrives during integration, restore only these declarations before final proof.

Product targets are metasystem/cmd/metasystem/test.go, metasystem/internal/gittree/detached.go and the new metasystem/internal/proofrun/candidate_engine.go. Tests are metasystem/cmd/metasystem/test_test.go, metasystem/internal/gittree/detached_test.go, new metasystem/internal/proofrun/candidate_engine_test.go, and metasystem/internal/testpolicy/select_test.go. Existing build script, stage loop/test, identity payload, worker schema, enrollment and fixture bodies remain unchanged. The separately owned Mac command-test failures are outside scope.

## Compatibility and rollout evidence

The bounded temporary Go overlay below drives unchanged NewDetachedWorktree and proofChildEnvironment. For both root and nested candidates it compares preparation against two fresh subprocess worktrees, checks exact author/committer metadata, injects foreign Git steering/config variables, and proves that a changed committer date changes the commit. Only temporary files/repositories and a private Go cache are used. This establishes the existing environment channel; it does not authenticate a production worker or prove a receipt.

Observed here: both cases passed, each matching preparation and two child worktrees at fixture commit 221ea80d5815f162a0300a63964ee67a9a0a0a6a; the differing-date negative also passed. The test body took 1.54 seconds (excluding compilation). Production source and the worker packet schema were unchanged. The actual engine-changing receipt remains the host's required proof.

The host separately reports baseline skew at candidate tree 675d8c257eaba9b8f40407ce642398e0b3606101, synthetic commit 32dd39d73115766830b54f5626f29b72c9f4315c, enrolled stamp d1a47c35: exit 1 named older engine/changed scripts. This author did not independently inspect that external log.

For first landing, use a private corrected command frontend for preparation and descriptor-aware verification. Existing trustedPolicyEngine selection still retains the enrolled policy engine, and the actual ordinary worker remains that enrolled executable. Existing METASYSTEM_BIN support in commit/land plumbing selects the frontend without modifying enrollment. Keep the existing trusted-base plan/admission/verification composition; introduce no candidate-only policy or first-transition exemption. The unchanged worker receives only existing fields and the metadata environment. No enrollment verb runs.

## Obligations

These rows specify implementation and runtime proof still to be performed; this design does not certify delivery.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| ENGINE-01 | CRITICAL | Exact candidate and enrolled authority | Same stamped commit across preparation and unchanged worker beds | gittree and testing composition | metasystem/internal/gittree/detached.go; metasystem/cmd/metasystem/test.go | TestCandidateCommitMetadataMatchesLegacyWorker; TestCandidateEngineUsesEnrolledWorker | Engine-changing receipt; unchanged enrolled bytes/enrollment | PARTIAL | Implement, then host receipt |
| ENGINE-02 | CRITICAL | Immutable preparation/reuse | Source, key, stamp and bytes checked; one shared build; verify never compiles | proofrun preparation | metasystem/internal/proofrun/candidate_engine.go | TestCandidateEngineBuildAndReuse; TestCandidateEngineRejectsMutation; TestCandidateEngineVerifyDoesNotLaunch | Sufficient repeated verify with zero builds/tests | PARTIAL | Implement binding and negatives |
| ENGINE-03 | HIGH | Bounded diagnostic failure | Failed/expired build prevents worker and preserves output | candidate preparation and boundedexec | metasystem/internal/proofrun/candidate_engine.go | TestCandidateEngineFailureRetainsDiagnostics including timeout/descendants and retention failure | Explicit refusal and readable diagnostics | PARTIAL | Run bounded failure canary |
| ENGINE-04 | HIGH | Source-free applications | Command app proof still works without Go | existing fallback/adapters | metasystem/internal/proofrun/candidate_engine.go | TestCandidateEngineSourceFreeApplication; existing command-app regression | Existing disposable app receipt bed | PARTIAL | Preserve fallback |
| ENGINE-05 | HIGH | Runtime custody at landing | Gate-fence required and successful inside receipt | contract/testpolicy | metasystem/testing.json | TestGateFenceRemainsRequiredForRuntimeCustody | Complete gate-fence result in sufficient receipt | PARTIAL | Add regression; reconcile demotion before proof |

## Bounded canaries, custody, then selected delivery

Place the new metadata test in the existing gittree test file, candidate-engine tests in the new proofrun test file, and the worker-composition test in the existing command test file. The build/reuse test uses a real small stamped Go executable and two detached beds, counting builds; separately mutate source, descriptor, stamp and binary. Do not fabricate successful terminal evidence.

Run these development commands from the repository root. Collect every command's result even after another fails; all must pass before expensive proof begins:

~~~sh
(cd metasystem && go test ./internal/gittree -run '^Test(CandidateCommitMetadataMatchesLegacyWorker|DetachedWorktreeHeadArchivesCandidate|DetachedWorktreeWorkspaceMatchesPhysicalCWD)$' -count=1 -timeout=2m)
(cd metasystem && go test ./internal/proofrun -run '^Test(CandidateEngine.*|SectionEnginePreparationPreservesBytesAndNestedSelector|CommandApplicationRunsWithoutGoAndSelectedGoRefuses|RunTestPlanRefusesForgedComponentReuseWithoutTerminalOuterOwner|StageCollectsIndependentFailuresAndNativePrerequisiteResults)$' -count=1 -timeout=5m)
(cd metasystem && go test ./internal/testpolicy -run '^Test(GateFenceRemainsRequiredForRuntimeCustody|ModesDoNotLowerRequiredRisk)$' -count=1 -timeout=2m)
~~~

Then the host runs these custody/worker canaries with process visibility:

~~~sh
(cd metasystem && go test ./internal/proofrun -run '^Test(ProofParentCustody|JoinedTestingResultAttachesWithoutClosingParent|JoinedSharedLaunchUsesEngineIdentityForShellCommand|LegacyWorkerRequiresLiveProcessRecord|LaunchSuiteAuthenticatesLegacyWorker)$' -count=1 -timeout=5m)
(cd metasystem && go test ./cmd/metasystem -run '^Test(CandidateEngineUsesEnrolledWorker|TrustedPolicyEngineIsRequiredWithoutBuildingDuringReadOnlySelection|AmbientTrustedPolicyDecisionCannotBypassRetainedEngine|TestListCheckPlanAndVerifyWithoutLaunching)$' -count=1 -timeout=5m)
~~~

After canaries and independent review, the host stages the actual whole-project candidate and uses its private corrected frontend through METASYSTEM_BIN. Preserve the goal's recorded risk and all selected groups; no fixed battery:

~~~sh
set -eu
test -n "$METASYSTEM_BIN"
receipt_tree=$(git write-tree)
"$METASYSTEM_BIN" test plan --root metasystem --goal gate-fence-returns-to-per-landing-proof --tree "$receipt_tree" --mode auto --json
"$METASYSTEM_BIN" test run --root metasystem --goal gate-fence-returns-to-per-landing-proof --tree "$receipt_tree" --mode auto
"$METASYSTEM_BIN" test verify --root metasystem --goal gate-fence-returns-to-per-landing-proof --tree "$receipt_tree" --mode auto --json
~~~

Once selected execution starts, retain every independent result across all stages. Require the actual gate-fence selector and checkout-guard fixture to pass inside the receipt, candidate stamp/HEAD equality, unchanged enrolled executable/enrollment hashes, matching successful outer terminal ownership and delivery.sufficient=true. Retain request, binding/logs, group results and verify output. No fake readiness, unlanded enrollment or candidate policy-worker substitution. The host owns this expensive proof; this design job runs only the compatibility probe.
