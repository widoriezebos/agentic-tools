# Goal landing efficiency: evidence, causes, and a generic solution

Date: 20 September 2026. Scope: analysis and recommendations, from intent through integration into main. No implementation, configuration change, goal mutation, test run, or publication was performed for this report.

Source baseline: `92743c95110f8682d84599fb88f9cdf156b9d3b0`, committed 19 September at 08:55 Amsterdam time, in the `agentic-tools-m1e` checkout. Existing uncommitted receipts, narrator records, retro material, and plans belong to other work and were preserved. Source conclusions below are about this baseline, not an assertion that Claude's running executable or another seat has identical code.

## Verdict

The system has most of the right primitives already: risk-selected groups, canary stages, dependency closure, retained per-group proof, verify-only delivery, bounded critique, and periodic broad validation. Its costly paths do not consistently use those primitives. The main opportunity is to make every step consume the same sufficient evidence and execute only the missing evidence, while finding integration defects before entering expensive proof.

Four changes should lead:

1. Repair and canary-test the real batch-to-testing interface. The checked-in prefix receipt command currently supplies a flag combination the selector rejects. Mocked tests miss it.
2. Bring join, seal, round proof, prefix receipts, and final delivery onto the existing testing/evidence owners. Join and seal currently run overlapping direct commands without reusable group evidence.
3. Explain and reduce unnecessary invalidation. Entire contract digests, broad engine identity, broad declared inputs, and ambient environment all influence reuse. Some invalidations protect real dependencies; others result from combining policy validity with execution identity.
4. After cheap prerequisites pass, collect independent failures in one bounded run, repair them together, and reuse unaffected passes. Preserve genuinely fresh diagnostics and cadence runs.

The generic objective is **one successful execution per required, unchanged test identity, consumed at every applicable workflow boundary**. Freshness requirements are explicit exceptions. Every landed commit still receives sufficient proof for its own tree. There is no general way to promise one execution of every test for an arbitrary batch whose intermediate trees change those tests' actual inputs.

## 1. Evidence and where the time goes

Evidence labels used here:

- **Measured here:** calculations from existing local structured artifacts or static contract data. No new test workload.
- **Recorded historical measurement:** an existing report describes measured results; its underlying transcripts were not independently reconstructed here.
- **Read:** a conclusion traced through source at the baseline above. Runtime behavior was not executed.
- **Proposed/model:** a recommendation or conditional cost model, not an achieved improvement.

### Local retained proof data

The local `metasystem/artifacts/agents/proof-runs/attempts/` directory contains 172 direct attempt JSON records, with starts from 10–17 September. All are terminal: 86 success, 80 failed, and six with terminal result `unknown`. These retained records are a local sample, not a complete fleet accounting ledger.

| Cohort, measured here | Attempts and outcomes | Sum of attempt durations | Group evidence |
| --- | --- | --- | --- |
| Starts on 16–17 September | 51: 30 success, 20 failed, one unknown | 44,199 seconds, or 12.28 attempt-hours; median 930 seconds | 2,180 passed, 56 failed, three invalid, 209 reused; reused share 8.54% of 2,448 observations |
| Goal `units-land-in-batches-under-one-proof`, 17 September local time | 12: seven success, five failed | 11,244 seconds, or 3.12 attempt-hours; median 923 seconds; range 839–1,085 seconds | 563 passed, 14 failed, two invalid, 91 reused; reused share 13.58% of 670 observations |

All 12 attempts in the second row were **diagnostic deep proofs while developing the batch mechanism**, selecting 54–57 groups. They are not measurements of twelve batch landings, nor of the user's current round 11. The newest retained attempt ends on 17 September. Summed attempt-hours must not be presented as elapsed fleet time: concurrent attempts overlap.

For this particular twelve-attempt cohort, merging the start/end intervals finds **no overlap**: active attempt intervals occupy 3.12 hours of an 18.70-hour span, from 01:25 to 20:07 Amsterdam time on 17 September. The other 15.58 calendar hours are unclassified here; they cannot be assigned to critique, waiting or implementation without the linked job/landing records.

For those twelve attempts, four expensive groups executed twelve times each with no reuse:

| Group | Median native group duration | Interpretation |
| --- | --- | --- |
| `section/land-fixtures` | 817 seconds, 13.62 minutes | The longest of these repeated groups; 9,174 total child-seconds |
| Dispatcher fixtures | 486 seconds, 8.10 minutes | Runs overlap other groups; do not add medians into batch elapsed time |
| Supervision fixtures | 464 seconds, 7.74 minutes | Same accounting qualification |
| Adoption fixtures | 410 seconds, 6.83 minutes | Same accounting qualification |

These are the first groups whose actual input changes and failure yield should be examined. Zero reuse is an observation; it does not prove that reuse would have been safe. The first bad group in the five failed attempts completed 198–493 seconds into the run; the remaining diagnostic run took another 363–887 seconds. That continuation collected other results by design. Its value depends on additional defects found and passes retained, not just on when the first failure appeared.

Identity churn is also **measured here**. Six of eleven adjacent attempts change the judge key and have no matching group execution identities. In two of those transitions, 34 of 55 group input digests stay equal, yet every execution identity changes and the later run reuses nothing. The candidate-engine digest changes in all eleven transitions. One same-judge transition also has 34 matching input digests but no matching execution identities. This supports investigating multiple invalidation components; it does not identify the judge as the sole cause or establish that those inputs capture all consumed dependencies.

### Historical evidence that explains the larger cycle

| Source and period | Recorded result | What it supports |
| --- | --- | --- |
| [Delivery audit summary](../metasystem/plans/delivery-efficiency-plan.md#L11), 6–11 September | 302 of 360 coordinator session-hours waiting; 45 of 146 delegate active-hours in fold/critique; 3.6% group reuse | Testing, waiting, and review relays all matter. These are aggregate session-hours, not one person's elapsed week. |
| [Underlying fleet proof audit](../metasystem/artifacts/reports/delivery-deep-dive-2026-09-11/proof-attempts.md#L39), retained attempts on 9–11 September | 97 attempts, 119.5 aggregate record-hours, 49.2 active hours; 44 successful attempts across fourteen goals reaching success, or 3.14 successful attempts per such goal | Corrects the summary's compressed “3.1 attempts per green goal”: its numerator counts successful attempts, not all attempts. Hung/idle time and concurrent execution prevent interpreting record-hours as fleet elapsed time. |
| [Suite measurements](../metasystem/plans/suite-speed-plan.md#L17), 10 September | Wide deep: 49–56 minutes, six runs. Narrow deep: about 21 minutes, four runs. Canary plus standard: 52–58 seconds, three runs. | Selection has far more potential than shaving seconds from an individual assertion. These are older suite shapes. |
| [Warm round proof experiment](../metasystem/plans/delegate-rounds-reuse-a-warm-gate-design.md#L155), 12 September | Docs round: 5.2 seconds, one group executed and eleven reused. Package round: 28 seconds, ten executed and two reused. A different docs round: thirteen minutes, no reuse, after judge identity changed. | Reuse already works; broad invalidation can erase its benefit. The source's failed-round table says 46 groups, while its prose says 47; that discrepancy is not resolved here. |
| [Delivery process reset](../metasystem/records/misc/delivery-process-reset-2026-09-17.md#L44), 17 September | A roughly 3,000-line goal became 24 units under a 300-line slicing rule; first eleven units consumed sixteen hours. | More units can multiply build/read/proof/landing relays. Prefer coherent, independently testable behavior slices. |
| [Reverse-dependent gate record](../metasystem/records/goals/unit-gate-runs-reverse-dependents.md#L11), 18 September | A gate covering 41 dependent packages took 1,550 seconds, about 26 minutes. | A targeted package gate can still be expensive. “Focused” is not a resource estimate. |
| User-supplied Claude transcript | 179 compactions, about seven hours; current join about thirty minutes; round 11 still running | A relevant operational symptom. The original transcript, current binary identity, and current rehearsal artifacts were not available as a matched dataset here. |

The context-cap pricing model in the supplied transcript is not validated by this report. Context removed at compaction, billed input, cached input, output, and elapsed time are different measures. Reducing command-output volume and model polling is worthwhile; changing a context cap cannot repair repeated test execution. Seven hours inside compaction also does not establish seven hours on the delivery critical path if background work continued.

### What we cannot yet price

There is no matched current trace here connecting one intent's design jobs, critique jobs, build rounds, join commands, owner lock waits, tip proof, prefix receipts, and push. Consequently, neither “three full suites per three-unit batch” nor a percentage speedup is a measured conclusion of this audit. A trustworthy latency report must join those phases and distinguish elapsed critical-path time, summed worker time, and host waiting.

## 2. The actual intent-to-main path

| Boundary | Current approach, read | Efficient obligation at that boundary |
| --- | --- | --- |
| Intent and intake | Human intent enters the goal ledger; severity and novelty derive the tier. Exposure/accumulation affect proof obligations and cadence weight. | Record observable acceptance, scope, dependencies, and the cheapest decisive checks once. Avoid turning every local correction into another independent delivery program. |
| Design | Trace existing owners, write the design and risky obligations, divide work into coherent units. | Resolve uncertain source facts before critique; name cross-component consumers and testable failure cases. A design needs enough detail to build, not a second implementation in prose. |
| Design critique | Tier-three design has a two-round ceiling, materiality test, dispositions, and fixture obligations for bounded residual findings. | One substantive read; a scoped second round when the fold changes a rule. Severe unresolved behavior remains unresolved when the budget ends. |
| Build and correction | Builder runs focused tests. `unit run` executes its configured proof command list before the read. Each follow-up initializes a new list. Older chain flow uses `job prove-round`, which calls the shared plan diagnostically and can reuse groups. | Put focused regressions and real boundary canaries before expensive proof. Both execution routes should retain consumable evidence. |
| Code critique | Conformance against brief/diff, then defect review; fresh independent reader. | Review the full initial build, then confirmed fixes and affected interactions. Recompute the full diff to detect drift without restarting an unrelated full review. |
| Batch join | Fast gate, changed packages, reverse dependents, tagged command tests where applicable, changed fixture groups, witness/read requirements. Selected tip groups are planned separately. | Admit a proved unit; carry precise evidence and identities forward. New combined dependencies still need proof. |
| Batch seal | Reassemble in join order, then run fast gate and union's changed packages again. | Re-evaluate obligations on the combined tree and run only newly required or invalidated checks. |
| Batch tip | Shared selected delivery plan, covering the sealed union. Default delivery stops launching after a failure. | Cheap prerequisites first; missing selected acceptance checks next; keep all independent failure information worth the bounded run. |
| Prefix receipts | For every non-last member, request the tip's selected groups on that prefix; shared runner is capable of reuse. | Validate each exact prefix against sufficient applicable coverage, using retained matching evidence. Never substitute a green tip for an unproved prefix. |
| Landing and moved trunk | Build commits locally, verify the held range, publish the series once; disjoint changes can use verify-only evidence checks. | Verify authority, policy, provenance, current tree and proof. A bookkeeping or record change should not itself launch tests. |
| Goal completion and cadence | Human-governed completion; broad fresh validation at required cadence. | Report the user-visible result and residual risk. Periodic broad proof remains a backstop for selection mistakes and environmental drift. |

Sources: [orchestration loop](../metasystem/docs/orchestration.md#L38), [design critique](../metasystem/skills/design-critique/SKILL.md), [code critique](../metasystem/skills/code-critique/SKILL.md), [unit execution](../metasystem/internal/launch/unit_run.go#L225), [round proof](../metasystem/cmd/metasystem/prove_round.go#L99), [selection](../metasystem/internal/testpolicy/select.go#L162), and [batch proof](../metasystem/internal/landing/batch/prove.go#L37).

There is instruction drift: the code-critique skill still describes accumulation of two or more as requiring a full battery, while the current shared selector says goal risk does not choose per-landing depth. The migrated landing path requires sufficient schema-two proof, with the legacy full-battery rule only in its fallback ([landing observation](../metasystem/internal/landing/observe.go#L389)). Reconcile the prose with the executable policy through the normal instruction change process. Do not infer an extra suite from an old field called `gateWidth`.

## 3. Concrete inefficiencies and integration gaps

### A. Small real boundary tests are missing where they would prevent expensive discovery

**Read, high priority:** [prefix arguments](../metasystem/cmd/metasystem/landing_batch_land.go#L393) construct `test run --mode auto --purpose delivery --groups ...`. [Selection](../metasystem/internal/testpolicy/select.go#L108) rejects explicit groups unless mode is canary, then treats them as diagnostic. [Preparation](../metasystem/cmd/metasystem/test.go#L411) forwards these fields directly. Rebased verification constructs the same incompatible combination at `landing_batch_land.go:308`.

The test at [landing_batch_status_test.go:274](../metasystem/cmd/metasystem/landing_batch_status_test.go#L274) asserts those arguments; the following test replaces the executable with a script that writes successful reused JSON. The advertised disjoint-unit fixture runs seam tests ([land-fixtures.sh:106](../metasystem/scripts/agents/land-fixtures.sh#L106)). These tests prove local plumbing but do not cross the real selector boundary.

**Fix:** retain public protection against manually narrowing delivery. Have the batch owner consume a validated plan through the existing testing owner, with the sealed obligations as constraints. Prove that operation through the actual CLI and retained evidence store. Merely switching a receipt to diagnostic canary mode would not supply delivery authority.

A related **read finding requiring a runtime witness** concerns diagnosis: [batch diagnostic arguments](../metasystem/cmd/metasystem/landing_batch_red.go#L74) pass `--no-reuse`, but [admission](../metasystem/cmd/metasystem/test.go#L159) maps that to a fresh attempt; [component composition](../metasystem/cmd/metasystem/test.go#L1028) uses the separate `ForceGroups` switch. A new attempt can therefore retain a previous group's pass. A base-tree diagnostic intended to determine whether a failure exists now must force the relevant native executions. Keep attempt freshness and execution freshness distinct in the interface and test both.

The queued [boundary-canary goal](../metasystem/plans/goals/boundary-canaries-before-expensive-proof.md#L6) records the same larger failure pattern: one attempt found 21 invalid sections after 297 seconds; a later attempt found eight failed groups after 1,064 seconds. A route-only probe had not proved the downstream child could start and finish under real custody. The generic lesson is a short representative journey through the actual production boundary, including cleanup, before expensive acceptance.

### B. Join and seal cannot consume the same proof today

**Read:** [join execution](../metasystem/cmd/metasystem/landing_batch_join.go#L254) creates a new detached worktree for each step, runs a command directly, and returns a hash of tree, step name and arguments as a run identifier. That identifier is not the shared runner's structured test evidence. [Seal](../metasystem/internal/landing/batch/seal.go#L190) then unconditionally runs the fast gate and every changed package in the union.

This is genuine overlapping execution. Some seal runs are necessary because composition changes dependencies, but the implementation does not distinguish them from unchanged joins. Join's whole reverse-dependent package tests and an extra tagged command-package run can be much broader than the phrase “cheap gate” suggests ([package steps](../metasystem/internal/landing/batch/unitgate.go#L462)).

**Fix:** plan and execute admission checks through `testpolicy` and `proofrun`, retaining per-group results that seal and delivery can consume. A package result authorizes later coverage only if the adapter proves matching test inventory, execution flags, dependencies, environment, and complete collection. Do not treat the current synthetic join run ID as a reusable certificate. Reuse immutable preparation within a candidate where safe; retain isolated writable outputs and process cleanup.

The batch owner currently knows about Go, `go-gate.sh`, the nested `metasystem` module and fixture-bed mappings. Those are host application facts. Move their selection into the project's contract/adapters; keep batch ownership concerned with trees, obligations, evidence and publication.

The build/read loop has a similar boundary: [unit plans](../metasystem/internal/launch/unit_plan.go#L14) carry arbitrary proof commands, and [each successful build round](../metasystem/internal/launch/unit_run.go#L247) runs that list before the reader. The launcher itself does not compose group evidence across follow-ups; a command can do so only if it delegates to the shared testing owner. A plan that names a broad shell suite can therefore repeat it after every correction. Make focused shared-plan evidence the normal round proof and reserve broad acceptance for its named obligation. Keep the reader's input tied to the exact resulting diff.

### C. Prefix receipt requests are not automatically full reruns

**Read, correction to the supplied diagnosis:** [receipt composition](../metasystem/internal/landing/batch/receipts.go#L68) invokes a runner per non-last prefix, but [its arguments](../metasystem/cmd/metasystem/landing_batch_land.go#L393) do not force re-execution. [The shared composer](../metasystem/internal/proofrun/test_result.go#L278) already searches across goals and terminal attempts. Successful groups from a failed overall attempt can be retained; a newer failure or live conflicting observation prevents resurrecting an older green.

Thus the source does not establish the claimed fixed multiplier of three full native runs. The interface mismatch above also means source/executable parity must be checked before explaining a successful live prefix path.

Nevertheless, requesting the entire tip selection at each prefix can be expensive. A prefix has fewer changes; some tip obligations may belong only to later units. More fundamentally, even identical group-local content can lose reuse through the wider identity inputs below. The earlier [batch design](../metasystem/plans/units-land-in-batches-under-one-proof-design-r3.md#L204) explicitly anticipated broad-input prefix re-execution, while the later [build brief](../metasystem/plans/units-land-in-batches-under-one-proof-brief-b.md#L77) requires per-prefix identity reuse. This is a known design tension, not evidence that wiring alone will solve it.

**Fix:** first count actual native executions per prefix and explain each miss. Then compose exact-prefix certificates from sufficient evidence. Preserve the tip's sealed union and each prefix's protected policy floor; explicitly define the applicable prefix obligations rather than silently dropping the union. Tests introduced only in a later unit cannot be assumed to exist on an earlier tree. Prefix selection changes need a reviewed contract, not a shortcut at receipt creation.

### D. Proof identity is broader than many tests' actual execution dependencies

**Read:** [group identity](../metasystem/internal/proofrun/test_build.go#L1225) includes the full group definition, content inputs, environment, tools, discovered tests, platform, complete current and base contract digests, judge key, behavior-policy digest, and candidate engine digest for engine-consuming groups. The whole candidate tree and absolute worktree directory are not directly in this key. Reuse separately compares those result-level policy digests ([test_result.go:352](../metasystem/internal/proofrun/test_result.go#L352)).

Consequences:

- An unrelated contract addition can invalidate all groups through the global contract digest.
- A main-branch engine-source change can invalidate every group through the [judge key](../metasystem/internal/proofrun/judge.go#L21), which covers all `cmd`, `internal`, `go.mod`, and `go.sum` at the policy base.
- Broad groups genuinely read broad trees. `fast-static-build` covers command, internal and script trees plus configuration. A changed engine genuinely changes an engine-consuming section test.
- The [environment digest](../metasystem/internal/proofrun/test_build.go#L1665) hashes ambient variables apart from a small explicit exclusion list. Changed temporary/cache/context variables can prevent otherwise useful reuse. Hashing their effects away without controlling what children receive would be unsafe.

**Fix:** separate three questions within existing owners: what ran, what current policy requires, and whether that execution is trusted here. Re-evaluate policy at consumption, but key execution evidence on the effective check and its complete execution dependencies. Keep a conservative invalidation when the runner/verdict semantics change. Do not replace a real runner dependency with an unverified hand-maintained compatibility label.

Narrow the *executed environment* before narrowing its hash. Declare external inputs and tool identities. Tests depending on mutable services, current time, random samples, secrets/configuration or shared state need a freshness rule or no reuse. Secret values must not appear in reports; identity calculations can use protected digests.

### E. Some intended targeted selection silently becomes broad selection

**Measured static inventory:** the baseline contract has 23 surfaces, 78 groups, three always-canary groups, eight always-standard groups and 44 cadence groups. These categories overlap; they are not counts of executions for an individual landing. Seven groups declare a whole `metasystem/internal/**` input. Both `context-budget` and fallback `residual` list 41 standard and eighteen deep entries.

**Read and checked against the contract:** [path matching](../metasystem/internal/testpolicy/select.go#L273) supports exact names and trailing `/**`. Yet [testing.json](../metasystem/testing.json) contains `landing_batch*.go`, `launch*.go` and `handoff*.go` surface patterns. Applying that exact matcher to `metasystem/cmd/metasystem/landing_batch_land.go` and `.../launch_unit.go` finds no explicit owner and selects `residual`. This is a static reproduction of the matcher, not a production `test plan` run. The handoff example has another owning surface, so its effect differs.

The input-manifest walker also treats internal wildcard-looking filenames as literal paths ([input hashing](../metasystem/internal/proofrun/test_build.go#L1453)). Go's implicit source closure and other broad declarations can still bind their actual content; this report does not infer unsafe reuse solely from the wildcard mismatch.

**Fix:** use one declared path-pattern grammar for selection, manifest expansion and diagnosis; validate unsupported syntax and test representative files. Make unmatched paths visible in the plan. Preserve broad coverage for genuinely unknown impact, while correcting declarations that accidentally create it. Then inspect reverse dependency edges and broad mirrored surfaces: removing a true consumer edge for speed is not safe.

### F. Expensive preparation happens before an all-reused result is known

**Read:** [test run](../metasystem/cmd/metasystem/test.go#L927) materializes and builds a candidate engine before preparing group identities and composing evidence. [The builder](../metasystem/cmd/metasystem/test.go#L714) invokes `go-build.sh` into a temporary output. A warm Go compiler cache helps but does not make checkout, discovery, hashing, linking and publication free. A proof with zero native tests can still do substantial preparation.

**Fix:** reuse a verified build artifact by the existing [candidate-engine build identity](../metasystem/cmd/metasystem/test.go#L769), and cache immutable discovery/materialization facts by their proper input keys. Keep artifact integrity, toolchain changes, cancellation and corrupt-cache recovery explicit. Measure this separately from test-result reuse. Optimize the longest measured preparation owner first; do not build a distributed cache without a demonstrated need.

### G. Failure collection and canary admission need different rules

**Read:** [execution](../metasystem/internal/proofrun/test_build.go#L100) stops delivery after a first failure by default. Diagnostic and cadence runs collect later groups; `--all-groups` already enables collection for delivery. Batch tip uses the default. Conversely, diagnostic execution can continue into later expensive stages even after a failed canary.

**Fix:** inexpensive contract/setup and real-boundary canaries guard expensive dependent execution. Once prerequisites are valid, collect independent selected failures within the admitted budget. Continue independent checks when useful; mark checks blocked by a failed prerequisite explicitly. Repair the discovered set together, run focused regressions, then execute only invalidated, failed or previously unrun required groups. Do not turn every iteration into a full battery or treat a broken harness as dozens of product failures.

This combines the useful existing collection and reuse behavior. It does not require replacing tests with canaries or accepting incomplete delivery evidence.

### H. Resource admission should follow actual workload

**Read, correction to the supplied safety claim:** the [recorded design](../metasystem/plans/units-land-in-batches-under-one-proof-design-r3.md#L8) explicitly exempts focused/diagnostic runs and places join outside the host lock at lines 88–97. [The owner](../metasystem/internal/landing/batch/owner.go#L144) takes the lock for its proof/diagnosis/landing work. Absence of a join lock alone therefore does not establish a violation of the cited ruling.

The practical concern remains: a 41-package reverse-dependent run is substantial, and several such joins can contend with owner proof. A timeout of forty minutes is not a runtime measurement, but the historical 26-minute gate is direct evidence that this category can grow large.

**Fix:** use the shared execution admission owner for heavy workloads from every entrypoint, including joins and proof commands. Classify resource demand from declared or observed cost, not the verb name. Preserve concurrent cheap checks. Hold capacity only while resource-consuming work runs, avoid nested reservation deadlock, record queue time separately, and prove cleanup before returning capacity. Test concurrent joining seats alongside the owner; a sequential rehearsal cannot establish that property. Do not prescribe one global serial lock for every application.

## 4. A generic operating model

### Keep the existing owners

| Responsibility | Existing owner to extend | Host application supplies |
| --- | --- | --- |
| Intent, acceptance, authority and budgets | Goal/intake and existing obligation records | Product outcomes, risk and scope |
| Select required coverage | `internal/testpolicy` and the testing contract | Surfaces, dependencies, group definitions, obligations and conservative fallback |
| Identify, execute and retain checks | `internal/proofrun`, adapters and evidence store | Commands, inputs, outputs, tools, environment, external dependencies and result format |
| Resource admission | Existing proof admission/process custody | Resource classes/costs and legitimate concurrency constraints |
| Assemble and publish | `internal/landing/batch` and landing verification | Destination policy; no hardcoded test framework |
| Review material changes | Existing design/code critique machinery | Brief, diff, findings, dispositions and focused proof |

The existing [command adapter contract](../metasystem/internal/testpolicy/contract.go#L374) already supports arbitrary command arguments and JUnit reports with expected tests; exit status alone is limited to build/static checks. A Java service or JavaScript application can use that seam. Its contracts would name, for example, a pricing unit group, checkout consumer checks, a database migration integration group and an HTTP smoke journey. The batch owner should not know Maven, npm, pytest, package naming conventions, or today's MetaSystem goal names.

The present public runner still prepares a Go-built MetaSystem engine even for other application test commands. Distinguish that orchestration-tool dependency from the application's test dependency; reuse the tool artifact when unchanged rather than requiring another engine build for each receipt.

### Evidence and policy remain separate but both mandatory

Conceptually, for group `g` on candidate tree `T`:

```text
execution identity = hash(effective check definition, full source/data closure,
                          tools, executed environment, platform,
                          consumed build artifacts, execution/verdict semantics)

required coverage  = current protected selection policy(base, T, obligations)

delivery allowed   = authority valid AND tree/provenance valid
                     AND every required check has compatible complete evidence
                     AND freshness and newest-observation rules hold
```

This is a separation of responsibilities, not a proposal for another proof database or policy language. Retain full policy digests for audit and re-evaluation. Policy changes must not erase the protected base contract, required consumers, new tests, or known failures. Cross-seat evidence is a later extension requiring explicit environment equivalence and trusted artifact transport; local reuse is the first deliverable.

### Batch cost is the number of distinct required identities

Let `T1 … Tk` be the exact prefixes in original join order. Let `Q(Ti)` be the required checks for prefix `Ti`, including the applicable protected policy and unit obligations. The final tip must cover the sealed union. Form the distinct pairs:

```text
needed = union over prefixes Ti of {(group, identity(group, Ti)) in Q(Ti)}
missing = needed minus compatible retained successful evidence
```

Plan those pairs before launching native work. Prove the tip and required prefix differences, assemble a receipt for each prefix, then verify the final range and publish once. Execution order may minimize waste; receipt obligations and commit order remain intact. If evidence already covers every pair, create certificates without inventing a new execution attempt.

For three units and a selected suite costing `S`, the pessimistic repeated-proof part is roughly `3S`. If unchanged identities cover earlier prefixes and their remaining costs are `d1` and `d2`, that part becomes roughly `S + d1 + d2`, plus preparation and verification. This is a serial cost model, not a measured wall-clock saving. If every prefix changes a broad integration group's real inputs, both differences can approach `S`.

Use this estimate, resource pressure and a bounded queue age to decide when to seal. Group count alone is a poor batch-size limit: one group can take fifteen minutes. Forecast reuse and missing work for coherent independent units; keep dependency order. A conflicting or interacting unit must be proved in its actual combination, not declared independent from file disjointness alone.

### Failure routing should shorten the repair loop

| Observation | Next action |
| --- | --- |
| Contract/CLI/setup canary invalid | Repair the boundary; skip its dependent expensive work |
| Product assertion fails | Retain complete failures and unaffected passes; builder reproduces and fixes the affected behavior |
| Host/custody failure | Preserve evidence; correct environment/admission validity before attributing a product bug |
| Combined batch fails | Run bounded fresh diagnosis of failed groups on base and relevant combinations; preserve interaction evidence |
| Prefix fails although tip passed | Refuse that prefix; correct or reassemble the series under existing ownership rules |
| Unchanged failure repeats | Require a new explanatory fact or stop; do not keep buying the same broad run |
| Trunk moves without relevant input changes | Re-evaluate selection and verify existing evidence; execute only genuinely new obligations |

Cadence and performance measurements remain fresh. Reuse cannot establish current latency, current service health or the absence of a timing-sensitive failure. A diagnostic that intentionally needs fresh observations must visibly record actual native execution.

## 5. Implementation order and decisive acceptance checks

These are proposed coherent changes, not newly opened or approved ledger goals. Existing overlapping goals should absorb the work where appropriate, particularly boundary canaries, warm round proof and batch landing. Do not create a second efficiency program with duplicate owners.

| Order | Change | Cheap decisive proof before a broad run |
| --- | --- | --- |
| 1 | Repair prefix/rebased delivery selection and diagnostic freshness at their current owners | A tiny real command-adapter project completes two prefixes through the real CLI; a fresh base diagnosis increments a native execution counter despite retained green evidence. No forged result JSON. |
| 2 | Correct path grammar/declarations and expose selection reasons | Representative batch/launcher paths select intended surfaces; unsupported syntax is caught; unknown paths retain conservative coverage; newly added/deleted/renamed files affect manifests correctly. |
| 3 | Retain join/seal/round evidence through the shared executor | Three disjoint units produce three exact prefix receipts; unchanged required identities execute once; a shared dependency change invalidates its consumers; normal and race/tagged executions remain distinct. |
| 4 | Separate execution identity from policy authorization; reuse engine preparation | An unrelated contract addition preserves an unaffected execution, while changing that check, its tools, consumed binary, environment or dependencies invalidates it. A policy weakening is refused. An all-reused receipt launches no build/test; corrupt cached artifacts are rejected or safely rebuilt. |
| 5 | Canary prerequisites, independent failure collection and repair reuse | Two independent seeded faults appear in one admitted diagnostic pass; a failed prerequisite prevents its expensive dependent launches; repairing one group preserves other eligible passes; missing reports never become green. |
| 6 | Resource admission across expensive entrypoints | Two concurrent joins plus owner proof respect configured capacity; cheap work proceeds; nested children cannot deadlock on a parent's reservation; killed workers release capacity only after cleanup. |

The first acceptance fixture must cross the real entrypoint, planner, retained store, receipt consumer and local Git range check, using tiny application commands. It can use disposable local repositories/remotes; no external publication is needed. Separate fast deterministic state-machine fixtures cover crash/restart and authority edges. Reserve expensive real-world suites for the behaviors these small fixtures cannot establish.

The portability acceptance case is a small non-Go application contract using the existing command/JUnit adapter: alter one domain component, verify its consumer selection, combine two independent units, inject an integration failure, repair it, and complete prefix and tip verification. No changes to batch code, goal-name exceptions, or MetaSystem-specific fixture paths are allowed to make that application pass.

Review should first attack invalidation soundness, policy weakening, prefix completeness, diagnostic freshness and resource cleanup. After fixes, confirm those findings and the affected behavior. A naming or bookkeeping preference should not trigger another full implementation/read/proof relay. Conversely, exhausting critique rounds is not evidence of correctness.

## 6. Measurement and stop conditions

Extend current records and reports rather than introducing another telemetry subsystem. Existing results already carry native launches, reuse, identities, durations, cost phases and attempt references; unit records carry step times. Add only the missing links and reasons needed to explain a goal's critical path.

Record per goal/unit/batch:

- Intent-ready, design-ready, build-ready, proof-ready, joined, lock-requested/acquired, sealed, tip-proved, prefixes-proved, published and completion timestamps.
- Selected, native-executed, reused, failed, invalid, prerequisite-blocked and not-run groups at each boundary.
- A reuse-miss explanation: content, definition, policy, tool, engine, environment, freshness, newer failure, incomplete evidence, or no retained artifact. Name changed input paths without logging secrets.
- Planning/discovery, hashing/materialization, build, native test, publication and queue time separately. Sum worker time separately from interval-union elapsed time.
- Review rounds and material findings, separating old unfixed defects, new defects introduced by a fold, and newly discovered interactions.
- Bytes returned to model context and repeated reads after compaction; retain full logs in files and return concise structured summaries plus paths.

Evaluate on a fixed representative cohort: docs-only change, local behavior change, shared-library change, contract change, batch of disjoint units, interacting units, failed harness and moved trunk. Preserve cold/warm-cache labels and machine/load conditions. Compare p50/p95 intent-to-landing latency, active test time, duplicate executions per identity, invalidation reasons and post-landing regressions. Do not optimize the reuse percentage by silently narrowing required coverage.

Stop broad reruns when there is no new decision-relevant question. Before each expensive attempt, record what it is intended to establish, the changed facts since the previous attempt, cost budget, expected signal, stop condition and recoverable candidate. The useful first milestone is a real small batch completing with honest per-prefix proof and native execution counts. Only then spend one required selected acceptance run on the implementation; retain its eligible evidence at commit/landing. Subsequent fresh broad runs must answer cadence, compatibility or another named question.

## 7. Audit completion and limitations

This report meets the requested analysis outcome: it traces the lifecycle, identifies code-grounded inefficiencies and boundary defects, quantifies the available retained data, and proposes application-independent corrections with owners and acceptance checks. Three bounded read-only explorations informed the analysis; the main investigation verified the central call chains and contract facts.

Investigation passes: (1) workflow and history established that the existing contract already supports selection/reuse; (2) source tracing distinguished actual duplication from receipt calls and exposed integration/selection gaps; (3) retained artifact analysis identified repeated costly groups and the limits of the sample. Each pass produced new facts. No performance experiment or implementation claim was made.

Verification for this document: source anchors and local cohort calculations were checked; Markdown paths and whitespace were checked before delivery. No unit, integration, full-suite, benchmark or live concurrency test was run. The prefix-interface and freshness findings remain source-level findings pending the specified focused runtime witnesses. Current Claude run parity, its seven-hour compaction breakdown, current batch prefix launch counts and fleet-wide elapsed-time attribution remain unverified.

No new behavior or failure path was introduced. The proposed owners above are implementation recommendations. The report is not an approval to weaken proof, remove prefix receipts, change human authority or alter runtime settings. Existing product and record changes were left intact.

Proposed review-only receipt content, not appended to the shared receipt ledger: `analysis: intent-to-landing efficiency; traced selection, batch gates and reuse; measured retained local attempts; identified prefix CLI mismatch, broad invalidation and unretained join/seal proof; generic corrections and focused acceptance cases in this report; no runtime tests or implementation.`

## Appendix: reproducible local cohort

Read only direct `*.json` children of `metasystem/artifacts/agents/proof-runs/attempts/`. The twelve-attempt cohort is the exact `goalId == "units-land-in-batches-under-one-proof"` filter in the retained index inspected for this report. The broader 51-attempt row uses stored `startedAt >= "2026-09-16"`; in this snapshot its first start is 16 September at 02:45 Amsterdam time. Record timestamps are machine UTC values; times below are Amsterdam local time, all on 17 September.

| Attempt filename without `.json` | Started | Duration, seconds | Terminal result | Reused groups |
| --- | --- | --- | --- | --- |
| `proof-mu4qazfk-a76d8b712b5de725` | 01:25 | 907.6 | success | 21 |
| `proof-mu4vkedj-141be63bfd1b4687` | 03:52 | 961.6 | success | 0 |
| `proof-mu4zge1b-3be59f9c3f0a62d3` | 05:41 | 996.5 | success | 0 |
| `proof-mu53bjy2-050819c669e9597c` | 07:29 | 1011.7 | success | 0 |
| `proof-mu58xz3g-469445665d0aff52` | 10:07 | 926.5 | failed | 0 |
| `proof-mu5bbrhm-61f8ba8fda5365e7` | 11:13 | 850.2 | success | 20 |
| `proof-mu5mtbga-c28b9a78172ce5c3` | 16:35 | 1084.8 | failed | 0 |
| `proof-mu5oq28s-90320c0c3fef0aac` | 17:28 | 976.2 | failed | 10 |
| `proof-mu5phhmb-72bf534216305871` | 17:50 | 920.2 | success | 26 |
| `proof-mu5s49mv-824543bafb3df849` | 19:03 | 838.6 | failed | 0 |
| `proof-mu5t1gju-6ecc56f8bdb1fd90` | 19:29 | 855.6 | failed | 0 |
| `proof-mu5tulzx-02ba06a96ddfd826` | 19:52 | 914.3 | success | 14 |

Calculations: attempt duration is `endedAt - startedAt`, outcome is `terminal.result`, group counts come from `testResult.groups[].status`, and native execution is checked with `nativeLaunched`. Group medians use `durationMs` for native executions only. Identity comparisons join adjacent attempts' groups by `id`, then compare `inputDigest` and `executionIdentity` alongside `testResult.judgeKey` and `candidateEngineDigest`. Reused-group durations refer to their original observations and must not be charged again. The corresponding expensive group identifiers are `section/land-fixtures`, `section/dispatcher-adapter-and-mission-runner-fixtures`, `section/supervision-and-census-fixtures`, and `section/adoption-fixtures`.

The twelve original files have aggregate SHA-256 `97b2b0be29c54c515961c5c208a19c841d1e6a6f2a2d426be9e54bad5f10c745`, computed over the table's chronological sequence of UTF-8 `filename + NUL + sha256(raw file bytes) + newline`. This identifies the retained inputs without copying the large records into `plans`. Local artifacts are not shipped repository policy; their absence in another checkout does not invalidate the source-level findings, but that checkout cannot independently reproduce these measurements without the evidence files.
