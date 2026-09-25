# Run the test suite safely in parallel

Focused design, independently reviewed on 22 September 2026 with no material findings. Runtime acceptance and delivery are recorded in the goal receipt and retained validation results.

## Intent and boundary

Run the existing suite in the shortest practical wall time, with configurable parallelism and deterministic tests. Keep its tests, race checks, coverage floors and required native-platform checks. The same worker mechanism must work for another application's test command.

## Performance correction authorized on 24 September

The user stopped the previous validation loop and authorized three specific changes. The recoverable starting candidate is `58919d4e9dc51ac89b95f87beebdc3094115495f`, published on `backup/test-parallelism-repairs-20260924`. Its full runs were stopped and do not prove completion or a full-suite speedup.

1. The existing Go runner will execute overlapping tests once per equivalent configuration. Prepared discovery supplies the identities; there is no maintained inventory. A passing native result may satisfy another selected group's overlapping identities within the same immutable candidate. The consumer runs its remaining tests and retains source provenance. Its prerequisites still apply. Different inputs, environments, tools, tags, race/coverage flags or worker conditions remain separate. Missing, failed, cancelled or incomplete evidence cannot supply a passing result. Existing result validation and retained-proof admission must verify the relationship.
2. Decision and failure combinations move into the existing isolated test harnesses. Keep a small genuine process/Git/landing integration witness and every behavioral assertion. Each test owns its state; use controlled clocks and existing stubs. First target: the separate two-conflict owner scenario currently repeats a whole-system lifecycle that the retained success witness already exercises.
3. Eliminate measured duplicate work within proof reuse. Initial profiling found a prefix plan running both AUTO plans in DEEP mode, then requesting the same two DEEP plans again. Retain those already-deep outputs within the operation, including their honest requested mode. Preserve fresh input checks, prefix green guarantees, restart verification and changed-input rejection. Further changes require an operation-level measurement, not an assumed bottleneck.

Acceptance for this slice: actual native invocation counts demonstrate deduplication and separate configuration execution; smaller fixtures preserve the original transition assertions; the unchanged receipt consumer passes with measured duplicate planning removed and no extra application test execution. Compare matched focused before/after runs and label host load. Full-suite improvement remains unproven until the combined candidate completes the required validation. No automatic second full run merely for timing.

Implementation uses Claude Opus 5.5 (`claude-opus-5-5`) following the user's latest roster ruling; Astra coordinates and Sol reviews. Work remains in private checkouts until reviewed. No new goal, framework, global cache, test inventory or audit service is part of these changes.

The user's scope correction is binding: improve test execution now; broader improvements can come later. Use the existing runner, worker pool, test groups, command adapter and result store. No new proof framework, contract schema, remote execution platform, cache redesign or batch-budget model. The earlier broad draft is superseded. Begin with a simple CPU-and-available-memory worker default and ordinary suite metadata. Optimize only if the user asks after several observed runs; do not start a tuning campaign now.

## User correction: automatic association and maintenance

On 22 September, Wido clarified the first-version application contract: setup must be fast, parallel, reliable and as simple as possible. An application supplies its ordinary full-test command and may map the granted worker setting to its framework. Adding or changing application code and tests must not require maintained file-to-test maps or testcase identities. Existing Go automatic discovery already appends `Always.Standard` and remains unchanged. Other opaque frameworks use their configured full-suite command, with the whole tracked project (`*` and `*/**`) as inputs, `always` selection and the existing fallback. Their honest evidence is command plus exit status; it is not an individual testcase census.

The first version adds only command-and-exit-status unit/integration acceptance, which the current validator forbids. It does not add universal dependency analysis. Remove the per-operation isolation approval catalog and its dedicated scanner/approval machinery; do not replace it with another registry. Preserve actual `testenv.Main` runtime isolation, worker limits, deterministic fixture fixes, failure/race/coverage checks, process cleanup, and the accepted scheduler and worker-propagation corrections. Iterate from observed use later.

## What is slowing us down

The retained C2 selected run took 75.10 minutes. Its largest groups took approximately 29 minutes (`go-batchtest`), 27 minutes (`section/land-fixtures`) and 24 minutes (the generated command-package group). Six workers were configured. These timings identify long groups; they do not establish a full-suite speedup or prove how much of each duration was useful execution rather than waiting.

Source inspection found three concrete execution gaps:

- Some nested Go commands—vet, build, staticcheck and cross-build—do not receive the worker limit. Conversely, a whole gate launched as a section can give its native tests only one worker by default.
- A non-Go group takes a scheduler slot before obtaining workers. Several large waiting groups can block a small ready group while workers sit idle.
- Static Go shards can repeat package startup and invoke packages with no matching tests. Large fixture sections also contain sequential work that the outer scheduler cannot distribute.

Owners: `metasystem/internal/proofrun/test_build.go:767–845,1127–1323`, `test_workers.go:11–84`, `test_go.go:551–647`, and `metasystem/scripts/agents/go-gate.sh:705–765`. Full source audits and retained timings are in `/Users/wido/LocalStorage/agentic-tools-evidence/test-parallelism-20260921/redesign/execution-facts.md`.

## Changes to make

### 1. Make the worker limit apply to the whole run

Keep the existing `testing.workers` setting and explicit override. For the initial automatic allowance, combine the existing CPU default with a provisional available-memory estimate: `min(existing CPU default, max(1, floor(max(0, available RAM - 1 GiB) / 2 GiB)))`. Preserve inherited ceilings. Resolve this allowance when the run starts; child groups and tools share or subdivide it, and none independently expands it. Show the configured value, resolved allowance and actual peak usage in the existing result. The 1 GiB reserve and 2 GiB-per-worker estimate are provisional defaults, not a host guarantee or invitation to tune before several real runs.

Pass the allowance through every nested Go phase, using `GOMAXPROCS` and supported `-p` controls. Give the complete Go gate an explicit width so its native tests do not accidentally run with a one-worker allowance. The command adapter supplies the same allowance to another application's runner; application configuration maps it to that framework's worker option.

Use existing host admission to limit concurrent suites. Do not introduce a new host scheduler. The CPU side of the default remains based on available execution CPUs divided by admitted concurrent runs, then the provisional memory allowance caps it. Explicit overrides and inherited parent limits remain ceilings. Reject a group requesting more workers than its run owns, with a useful error.

**Proof:** a recording fake Go executable checks every tool phase's arguments and environment; a small real workload checks maximum simultaneous work and nested limits at one worker and the default.

### 2. Keep available workers busy

Change admission inside the existing scheduler so a non-Go group obtains both a group slot and its requested workers before launch. A group waiting for workers must not consume a running slot. Go groups retain their existing per-shard worker grants; their parent must not reserve the same workers again.

Consider ready groups in deterministic order. Permit fitting work to pass a group that temporarily cannot fit. To prevent starvation, after one pass over the current ready queue, let existing work drain enough to admit the oldest waiting group before launching further bypass work. Cancelled waiters release their place. Preserve prerequisite ordering and exclusive performance tests.

**Proof:** with four workers, one running one-worker group and three waiting four-worker groups must not prevent another ready one-worker group from using spare capacity. The large groups must subsequently run. Test cancellation and resource release on failure.

### 3. Use the existing parallel fixture jobs

The landing fixture group already has 45 isolated scenario processes and a bounded parallel runner. Its contract currently gives the whole group one worker, so those scenarios run serially. Declare `resources.workers: 0` for this group: the existing meaning is to reserve the run's available allowance, which the fixture runner subdivides into one-worker children. Preserve its existing private directories, repositories, endpoints and process cleanup. This activation is the first change; no new fixture splitter is needed.

Measure this existing runner with its normal allowance. Only split another long group if its retained timings show independent work that still dominates the suite. Keep an individual test atomic when its cases share state. Existing scenario names and result collection must make omissions or duplicates fail.

Preserve the existing Go partitioner, package-qualified test identities, build-only packages, `TestMain`, race settings and complete coverage merging. Avoiding empty package invocations or changing partitions is a follow-up only if measured remaining costs justify it. Do not add a timing-prediction service or rebalance partitions on every run.

**Proof:** the existing fixture tests check scenario census, bounded overlap, isolated state, failed-child cleanup and cancellation. Add a contract regression proving the group receives the whole allowance, then measure its actual runtime. Retain existing Go census and coverage checks.

### 4. Remove shared-state and timing flakiness

Tests may run serially inside an isolated process while other processes run in parallel. Converting every test to `t.Parallel()` is unnecessary. Isolate mutable environment, cwd, HOME, files, repositories, sockets and fixture authorities using the existing `internal/testenv` facilities. Keep shared caches only where their concurrency behavior is supported.

Use controlled clocks for expiry/retry logic and explicit readiness/completion signals for concurrent work. Remove correctness assertions that require a slow machine to finish within a small elapsed-time window. Real subprocess tests retain an outer safety timeout; reaching it means incomplete infrastructure execution, never a passing test. An arbitrarily overloaded machine cannot be guaranteed to finish, but host speed must not decide a semantic assertion.

Do not gate applications on a maintained per-operation isolation catalog or dedicated scanner. Prove isolation through actual `testenv.Main` behavior, deterministic fixture controls, race/failure checks and descendant cleanup.

The earlier fixture-configuration defect is already fixed: `internal/fixtureauth/fixtureauth.go:319–335` uses layered configuration, and synthetic local-override/malformed-config tests exist. Preserve those tests; do not reopen that implementation.

**Proof:** focused tests for controlled time and readiness, concurrent isolated fixtures, and cancellation/descendant cleanup. Use synthetic configuration only; never inspect the real secret local configuration.

## Run and land this work without another expanding project

1. Preserve the current reviewed candidate. Its corrected selected run finished successfully: 139 groups passed, 27 reused matching results, and verification passed in 55.7 minutes. Full native race/coverage/platform obligations still remain.
2. Implement only the four execution changes above, in small independently reviewed units. Run focused tests during development. Compose the candidate once the focused checks pass.
3. Run cheap composition checks first: syntax/build and platform compile canaries. Then run the required suite, collecting independent failures in the same round. A failed prerequisite blocks its dependents, not unrelated checks.
4. For this goal, run the required native macOS and Linux checks concurrently where existing host/VM admission permits: the runner and process-handling changes require both. Use isolated checkouts and the existing launch/transport commands. Keep race tests, coverage floors and the complete expected test/stage census. Future landings use Mac checks by default, targeted Linux tests for a concrete portability risk identified during review, and a full Linux suite only for broad or inseparable impact or Wido's request. This is a review decision under `development/project-rules-local.md`, not a new automatic classifier or a universal two-platform gate. Do not add another platform runner.
5. Repair observed failures together, test those repairs narrowly, and consume existing exact matching passing results through the current reuse machinery. Do not modify cache keys or manually declare evidence equivalent.
6. Verify required evidence for the final candidate, publish once, then perform the already-required installed public-consumer check and cleanup verification. Finish the goal only when its original acceptance conditions are met.

Use ordinary `scripts/validate-metasystem.sh` or governed cadence for the required final full native proof. Its explicit `--delivery-contract` mode deliberately consumes an adopted application's selected tests; that result does not discharge this goal's full native race and coverage obligations. Source inspection confirmed that ordinary full validation cannot enter that selected-proof branch. Preserve the branch and the distinction between application delivery proof and full validation; no validator change is required.

Keep current batch prefix proof and failed-member isolation unchanged. Every integrated commit must still have its required matching proof. Cache granularity, production build stamps, automatic batch recovery, advanced scheduling history and batch sizing are recorded follow-ups; they are not prerequisites for this parallel-execution goal. The separate proving-state crash gap must be addressed before relying on automatic recovery in that state; this plan does not certify it as fixed.

## Implementation owners and acceptance

| Work | Existing owner | Required result |
| --- | --- | --- |
| Worker propagation | `cmd/metasystem/proof_run.go`, `proofrun`, Go gate/build scripts | All nested phases obey their grant; another application's command receives the same limit |
| Scheduler admission | `proofrun/test_build.go`, `test_workers.go` | Ready work uses spare workers; waiting and cancelled groups do not leak slots; large groups eventually run |
| Parallel fixture execution | Existing landing fixture runner and `testing.json` | Independent scenarios use the granted workers; every original case and cleanup obligation remains represented |
| Determinism | `internal/testenv`, fixture/test owners | Isolated state and semantic clocks; slow scheduling does not change expected behavior |
| Complete final gate | Ordinary validator or governed cadence | Retain actual required native race/coverage proof; selected delivery evidence is a separate verdict |

### Expected behavior claims

These claims are checked in every implementation and testing round. A claim is `satisfied`, `failed` or `inconclusive`; absent or non-comparable evidence is inconclusive.

| Claim | Expected observation | Contradiction |
| --- | --- | --- |
| `PAR-1 overlap` | Independent eligible native tests and fixture processes actually overlap. | Eligible work remains serial. |
| `PAR-2 capacity` | Configured worker grants reach nested runners, and blocked work does not occupy capacity that fitting ready work can use. | A nested runner loses its grant, exceeds it, or a blocked group leaves useful capacity idle. |
| `PAR-3 elapsed` | Equivalent work finishes sooner, and the complete suite records elapsed duration with platform, census and cache conditions. | Comparable work does not improve, or no comparable baseline exists and a speedup is still claimed. |
| `PAR-4 semantics` | Concurrency and slow scheduling leave semantic verdicts deterministic. | Scheduling order or host speed changes a pass, failure or expected result. |
| `PAR-5 completeness` | The complete required test census, race checks and coverage floors run; any required failure blocks integration. | A required item is absent, silently skipped or red while integration proceeds. |
| `PAR-6 cleanup` | No owned process remains after success, failure or cancellation. | The post-run census finds an owned survivor. |
| `PAR-7 generic control` | The same worker and cancellation controls govern another application's command adapter. | Correct bounds or cleanup depend on metasystem-specific command behavior. |
| `PAR-8 application setup` | A supported application supplies its normal full-test command and optional worker setting; new or changed tests are observed without per-file maps, testcase identities or a contract edit. A nonzero command blocks success, and changed tracked inputs invalidate old evidence. Remove the per-operation isolation approval catalog and its dedicated machinery without replacing it with another registry; retain runtime isolation and the required proof safeguards. | Application changes require maintained mappings or testcase identities; a red command succeeds; changed tracked input reuses stale evidence; command-level evidence is misreported as an individual testcase census; or catalog machinery remains/reappears while runtime safeguards regress. |

Report the final full-suite elapsed time, worker count, slowest groups and cleanup result. Compare with an earlier run only when suite, platform, inputs and cache conditions match. Otherwise state the measured duration without claiming a percentage improvement. Do not schedule repeated full runs merely to produce a benchmark number.

Verification must observe the behavior this work is intended to change, beginning at meaningful checkpoints in the first actual runs rather than waiting for every group to turn green. Record native test and fixture-process overlap, useful worker capacity and comparable elapsed duration. Configured concurrency or a high process count alone does not establish a speedup. If independent work is eligible but the real run stays serial, or equivalent work does not finish faster, investigate the bottleneck before claiming delivery.

Elapsed comparisons bind the work, platform, process census and cache conditions. Name a missing comparable baseline instead of guessing a speedup, and do not introduce a hard host-speed threshold or a separate large benchmarking framework. The completion test is practical: the required suite passes with configurable parallel execution, demonstrates useful overlap within its grants, preserves every required test and floor, leaves no owned processes behind and improves elapsed duration against comparable evidence. Further optimization follows the observed timings after this work lands.

Review record: Sol launch `20260922t063452-5c72eca131`; findings and dispositions are retained under the evidence directory cited above. The broad first review was cancelled when the user narrowed scope. The focused review found zero material issues; its output-placement failure was repaired by preserving the exact authored return from its transcript. No implementation or runtime completeness is claimed by this design review.

## Binding scope correction — 22 September 2026

Wido forbids any further scope expansion. Deliver only (1) parallel testing that reduces wall-clock duration and (2) further reduction through running the required test scope and reliably reusing matching passing evidence. Use the existing mechanisms. If achieving either outcome requires additional machinery or a broader design, explain the concrete need and obtain Wido's approval before designing or building it. Previously approved removal of the unnecessary audit catalog remains a reduction of machinery, not permission to introduce a replacement. Ordinary Git preservation, staging, proof and publication operations do not justify a new orchestration layer. Keep the already requested CPU/RAM default bounded; no tuning campaign, automatic optimizer, care implementation, new audit system or unrelated feature is part of delivery.


## Git-free behavior tests — user ruling, 23 September 2026

Wido has stopped the maintenance/cleanup patch route and explicitly authorized replacing real Git dependencies throughout the tests now. The existing goal remains active. Its delivery now includes this correction; the earlier estimate does not cover it. The private maintenance unit and obsolete Mac validation were cancelled with their evidence retained. No maintenance patch was applied to Main.

### Small dependency boundaries

A test of an application decision receives repository facts and operation results through an instance-owned dependency. Production binds the existing Git implementation. A test binds its own fake; never swap a process-global function or register a root-to-fake map. Keep real filesystem journals where their behavior is under test. No Git command emulator, new dependency, remote service, audit catalog, or maintained test inventory.

Share a small strict command stub in `internal/testgit` where callers already accept command runners. It records directory, argument vector, environment and input bytes, returns declared stdout/stderr/error, and rejects an unexpected call. Each test owns an instance; access is synchronized and recorded slices/bytes are copied. Expected calls must be consumed at cleanup; tests can assert invocation order from the call log when order matters. It never falls through to real Git. Adapters to existing runner signatures live in their test package. The first real consumer is `launch.UnitRunner.Git`, whose current test recorder falls back to `OSGitRunner` and whose fixture creates a repository unnecessarily (`unit_test.go:21–48,112–141`). Do not build unused general-purpose matching or replay features.

Stateful goal tests need a higher boundary than command answers. Add an optional, per-Endpoint `Repository` dependency for capture, accepted-ref read/CAS (read returns tip, present, error: absent is false/nil, valid is true/nil, corrupt or unreadable is an error), immutable file reads, commit creation, publish CAS, ancestry, transaction trailer presence, commit time, and temporary-ref release. The default Git adapter preserves today's commands and outcomes. Keep existing exported root-only functions as Git-backed compatibility entrypoints; endpoint-aware paths must stay on the injected instance all the way through transaction, projection, mutation and validation. In particular, `ValidateCommit` also validates channel files; channel validation must read the same injected snapshot rather than returning to Git.

The goal test fake stores immutable file snapshots, parent/trailer/time facts, one shared canonical tip and per-client accepted tips, with atomic comparisons under a mutex. Its identifiers are deterministic opaque valid IDs, not simulated Git object encodings. The fake must model stale-compare refusal and unknown publication outcomes explicitly. Missing commits and unsupported operations return errors. A fixture carries its endpoint explicitly; verb helpers accept that endpoint. No hidden lookup by directory. Keep the production journal/retry/validation logic under test; do not stub the entire operation being asserted.

Other packages use their current receiver or function dependency where available (`launch.GitRunner`, `testselect.commandRunner`, `proofrun.judgeGitReader`, `branch.PushTransport`). Extract a small dependency at the existing Git-call owner only where none exists. Higher-level landing, dispatch, and mission tests receive typed repository facts/effects, not a fake object database. Shell scenarios that assert policy decisions move those assertions to the same Go owner with injected dependencies; preserve a minimal public-command smoke test. Merely renaming broad fixture beds as integration tests is not conversion.

### Real-Git exceptions

Keep only focused tests whose claim requires our adapter to work with actual Git: atomic ref/push compare-and-swap and transport reconciliation; isolated-index and checkout preservation including modes, symlinks, binary bytes, nested/linked worktrees and steering; actual hook or configured merge-driver invocation. Each retained test names its adapter claim and why a stub cannot establish it. Argument construction, parsing, failure classification, orchestration, goal lifecycle and recovery decisions use fakes. Do not retain all variants of an application scenario merely because it currently creates a repository.

These adapter checks remain explicit integration checks. Run them when the changed code affects the adapter, and in a requested complete validation. Preserve the existing selection/reuse machinery; do not add a new classifier or metadata inventory. This implementation changes the adapters and therefore must pass their checks before integration.

### Execution and acceptance

Build and independently review bounded package slices: shared strict stub plus existing launch seam; goal repository boundary and an initial fake-backed Publish/Project/stop case; remaining goal behavior fixtures; dispatch/steward and leaf readers; landing/mission consumers; command and shell consumers. Start with consumers that already have injection seams while the repository slice is built. Every subsequent slice preserves the original behavioral assertions, including errors and races; deletion of redundant integration setup does not permit deletion of the assertion.

For each migrated slice run with an owned PATH executable that records and refuses Git invocation, and inspect the converted dependency path for absolute-binary forwarding. This is an early focused check, not complete denial. Before final acceptance, run the compiled ordinary tests in a private Linux mount namespace masking installed Git executables and helper paths, including absolute invocations, while retaining the normal test UID, PID namespace and procfs. This operational verification wrapper does not ship as product machinery and leaves host Git unchanged. Do not treat skipped tests as converted, allow a fallback to the real binary, or bless green tests whose fake simply reports success for every command. The build/checkout used to prepare tests may use Git; the test bodies and their fixture setup may not, except the named adapter checks. Exercise concurrent tests under race detection and verify unexpected calls fail. Keep command-based and stateful doubles test-only.

Before claiming completion: the ordinary behavior suite must pass with Git denied; all narrowly retained integration exceptions must be listed with their concrete reason and pass separately; original required behavior/race/coverage checks must remain; observe real worker overlap and elapsed duration; then use matching evidence to integrate once. Report remaining real-Git consumers honestly until that condition is met. No new goal, cache redesign, Git maintenance workaround, or automatic tuning campaign is authorized by this correction.
