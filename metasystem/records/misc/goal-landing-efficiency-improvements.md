# Goal landing efficiency improvements

Design proposal, 20 September 2026. Applies to MetaSystem and adopted applications with their own test suites. This document specifies changes; it does not implement them, change standing policy, or approve new ledger goals.

Evidence baseline: `92743c95110f8682d84599fb88f9cdf156b9d3b0`. The [analysis report](goal-landing-efficiency-analysis-2026-09-20.md) contains measurements, source traces and limitations. Its twelve diagnostic development proofs occupied 3.12 hours; the longest group ran twelve times without reuse. That motivates the design, but is not a current batch-landing benchmark.

## 1. The change in design

Make landing a composition of sufficient evidence. Each stage asks the same testing owner what remains to be proved for its candidate. It consumes compatible retained results and executes only missing work. Reviews, batch joins, seal, prefix receipts and commit checks must stop owning separate overlapping test pipelines.

The target is to minimize intent-to-landed latency and repeated work while preserving individually green commits, correct selection, independent review, human authority and trustworthy failure diagnosis. It is not possible to avoid re-executing a test whose actual inputs changed, nor to reuse an old observation to measure current performance or service health.

The six decisions are:

1. Keep one intent and acceptance contract through design, build, critique and delivery; correction rounds carry deltas and evidence references.
2. Use the existing testing planner, executor and retained-result store at every proof boundary.
3. Separate execution identity from the policy decision that makes the evidence sufficient for a particular candidate.
4. Make real boundary canaries prerequisites for expensive dependent checks; collect independent failures together after those prerequisites pass.
5. Plan proof for every exact prefix and the final union, then execute the distinct missing identities once wherever freshness permits.
6. Admit resource-consuming work through one host owner, and keep waiting and routine transitions outside model contexts.

No new workflow framework, distributed cache, test language or goal-specific optimization is required. Existing owners gain the missing decisions and callers surrender duplicate implementations.

## 2. Intent, design and review become a short feedback loop

### One acceptance contract

The goal owns the user-visible outcome, non-goals and reserved decisions. Its existing design/obligation records link each risky behavior to its owner, affected consumers and decisive test. The builder brief references that contract rather than copying and gradually rewriting it. The testing plan supplies the executable coverage selection; the model does not maintain a second list of required suite commands.

Before implementation, resolve uncertain call chains with source evidence and name changed execution boundaries. A new or moved boundary gets a small end-to-end canary using the real caller and consumer. This puts interface facts into an executable witness before a long suite is spent discovering them.

Units follow coherent behavior and independently valid commit boundaries. Line estimates support budgeting and review; they do not force fragmentation. If two changes require each other to work, they form one unit or a dependency-ordered sequence whose intermediate behavior is valid. A later fix must not be needed to make an earlier landed commit green.

### Default sequence

| Stage | Work performed | Evidence handed forward |
| --- | --- | --- |
| Design | Acceptance, owner/consumer trace, failure behavior, focused proof targets | One versioned design and obligation map |
| Design critique | Material behavioral risks at the applicable tier; bounded disposition/fold | Findings with stable identifiers and the changed obligations |
| Builder loop | Regression for the defect, relevant unit checks, real boundary canaries | Exact candidate/diff and structured results from runnable checks |
| Code critique | First read covers brief and complete diff; corrections cover findings and affected interactions | Independent review bound to the resulting change |
| Join | Authority/read/witness checks and missing inexpensive admission checks | Member obligation set and references to retained proof |
| Batch acceptance | Missing selected prefix/tip proof, under shared admission | Sufficient evidence for each exact prefix and tip |
| Commit and publish | Revalidate trees, selection, authority and receipts | Verified commit range and one publication |

The builder runs focused checks before a paid read. A broad delivery suite is not an unconditional prerequisite for that read. A critic does not run the entire suite merely to repeat the builder's result. Its new behavioral finding names a targeted reproduction and returns to the builder with the existing failure artifacts.

The first code read examines the complete implementation. After a correction, conformance still checks the whole recomputed diff for scope drift; the substantive confirmation read concentrates on the changed behavior, its consumers and prior findings. A correction that changes the design or unrelated behavior expands that boundary explicitly. Exhausting review rounds never certifies a severe unresolved defect.

Use the existing tier and round limits. Reconcile the stale full-battery wording in the code-critique skill with the shared selector through the normal instruction change process. A field named `gateWidth` must not trigger a second selection policy.

Run waiting and routine state transitions in the existing launcher/wait/landing owners. Wake the model for new evidence requiring judgment, a material conflict, a design gap or a reserved decision. Store full command output in artifacts and return a compact result, changed identities, failures and artifact paths. A restart resumes the durable stage instead of reconstructing it from a long transcript. Context-window tuning is separate from this design.

## 3. One proof path, with clear ownership

| Decision | Owner | Change |
| --- | --- | --- |
| Required checks and protected policy | `internal/testpolicy` | Compute candidate-specific requirements and prerequisites; validate pattern grammar |
| Test identity, execution, collection and reuse | `internal/proofrun` and its adapters | Retain compatible group evidence across every entrypoint; explain misses |
| Candidate/build preparation | Existing testing preparation in `cmd/metasystem/test.go` | Reuse verified immutable artifacts and discovery; avoid build-before-lookup |
| Capacity, cancellation and child lifetime | Existing proof admission and process custody | Cover heavy join/build/proof entrypoints; account nested work once |
| Review sequence and handoff | `internal/launch`, dispatch and existing review records | Replace certifying raw command lists with shared-plan results |
| Ordered composition and publication | `internal/landing/batch` | Plan prefix obligations; consume evidence; retain authority and crash recovery |
| Current delivery sufficiency | Landing receipt/verification owner | Recompute coverage and check referenced evidence without launching work |

The batch owner supplies candidate trees, ordered member obligations, frozen authority and destination facts. It must not inspect Go imports, select `go-gate.sh`, recognize `cmd/metasystem`, or map fixture filenames to commands. Those facts belong in application contracts and language adapters. Start by moving existing behavior, including mandatory coverage, into those owners; remove the superseded direct gate executors in the same cutover.

### Plan, ensure, verify

These are responsibilities in existing APIs, not three new services:

- **Plan:** resolve the exact project tree, destination/base policy, acceptance obligations, environment and required coverage. Produce an immutable plan identity, selection reasons and prerequisite relationships. Cheap static planning does not launch native tests or builds.
- **Ensure:** validate that plan against current authority and inputs; prepare only necessary metadata/artifacts; consume eligible retained results; reserve and execute missing work; publish complete observations. It can return sufficient evidence without creating an execution attempt when nothing runs.
- **Verify:** re-evaluate current required coverage and compare evidence identities, freshness, completeness and provenance. It launches no tests or builds. A missing prerequisite, missing evidence or changed policy produces a precise unsatisfied requirement for `ensure`.

Existing `test plan`, `test run` and `test verify` remain the public entrypoints. A stored plan is a reproducible decision, never a bearer token granting delivery. Consumers recompute mandatory selection or validate its exact frozen inputs; a caller cannot omit a required group by supplying an edited plan.

Explicit `--groups` retains its diagnostic meaning. Batch receipt and rebased verification stop constructing delivery commands with that flag. They request a candidate's complete policy-selected delivery plan and carry additional sealed obligations through a typed internal requirement set, which can only add mandatory coverage. If a transport requires a serialized plan reference, its reader verifies the stored input binding and recomputes the coverage floor before execution.

This repairs the [current prefix/selector mismatch](../metasystem/cmd/metasystem/landing_batch_land.go#L393) without permitting diagnostic selection to authorize delivery.

## 4. Execution evidence can survive policy re-evaluation

### Separate two identities

**Execution identity** answers whether the same check ran against the same relevant inputs. It binds:

- The effective command/test inventory, adapter, working-directory semantics, runtime flags and result collection contract.
- The complete input closure: source and test code, transitive dependencies, lockfiles, fixtures, generated inputs, configuration, file modes/symlinks, additions/deletions and declared external snapshots.
- Actual tool identities, consumed build-artifact digests, platform and effective executed environment.
- Execution/collection semantics and any settings that change behavior, including enforced timeouts or coverage instrumentation.

**Delivery decision identity** binds candidate tree, protected base and candidate policy, selected obligations, current authority, required verdict criteria and the evidence references satisfying them. It changes when these facts change, even if no test must rerun.

Group names are lookup labels; they are not proof identities. An equal manifest path list is insufficient: file contents, dependencies and execution semantics must match. Goal IDs, receipt IDs, output-log locations and unrelated contract rows are provenance, not execution inputs, unless a check actually reads them.

The first identity change removes whole-contract coupling from group reuse. Canonicalize the **effective per-group execution definition** after protected base/candidate composition; preserve ordered fields such as argv and normalize only genuinely unordered sets. Keep full policy digests on the delivery decision. Adding group B can then leave group A's execution valid while policy requires new work for B. Changing A's required test inventory changes A's identity.

Change all three consumers together: [group hashing](../metasystem/internal/proofrun/test_build.go#L1225), retained metadata reconstruction at `test_build.go:1082`, and [newest-observation composition](../metasystem/internal/proofrun/test_result.go#L352). Changing the hash alone leaves global contract-equality refusals in place. Preserve the existing whole-receipt compatibility checks when recovering an already committed receipt: policy, judge, engine, purpose, coverage and execution identities must still match. This recovery returns the original bytes and original tree provenance; it does not newly require raw tree/base equality when all those checks hold. The consumer binds sufficient evidence to its current decision separately. Reuse across different policy decisions composes a new result from compatible group observations instead.

Scheduling estimates and display text are outside the execution key only if they cannot alter the launched command or its outcome. The current `targetMs` can affect adapter behavior; it cannot simply be classified as descriptive metadata and dropped.

### Preserve the policy floor

Extend [protected contract composition](../metasystem/internal/testpolicy/protection.go#L51), rather than bypassing it. Candidate policy cannot remove a protected provider, lower a floor, erase a consumer edge or rewrite the tests judging its own change. Each prefix is evaluated using its own effective protected contract. The final tip must also cover the batch's admitted union of obligations.

A changed verdict-only criterion can reuse retained raw reports only when those reports contain all data the current trusted evaluator requires and the execution producer remains trusted. Otherwise the check runs again. A discovered defect in a producer invalidates affected evidence even when the apparent input hash is unchanged. Reusing evidence is a correctness decision, not an entitlement of the cache.

### Do not remove the judge identity indiscriminately

Today the judge key covers broad engine source trees. Preserve its conservative executor compatibility check in the first cut. Removing it together with contract coupling would mix two independent trust changes.

The target boundary distinguishes the **tool executing and collecting tests** from the **application artifact being tested**. An adopted application's ordinary code change does not inherently change its installed test executor. Candidate application binaries are inputs only to checks that consume them. Replace special cases such as the MetaSystem steward package with adapter-declared consumed artifacts.

When the test tool itself changes, distinguish native execution/collection changes from pure delivery evaluation changes. The latter may re-evaluate sufficiently complete retained reports; the former invalidate affected executions. Derive producer identities from actual immutable artifacts. If the current monolithic binary prevents that distinction, retain broad invalidation until a measured follow-up physically separates that responsibility. Do not substitute a manually asserted “compatible version” or an incomplete source-path allowlist. This design's initial speed gains do not depend on that extraction.

### Environment, observations and concurrency

Use an explicit environment assembled by the adapter. Hash the semantic values actually supplied; additional ambient variables are not silently inherited in this mode. External configuration and services must be declared as inputs or force fresh execution. Missing declarations fail preparation with a useful diagnostic, not an unexplained native error.

Existing installations initially retain their current environment mode; adoption of an explicit environment is a named contract migration with a real harness canary. Correction to the analysis: the current [environment hash](../metasystem/internal/proofrun/test_build.go#L1665) already excludes standard cache/temp locations and custody identifiers. Those specific variables are not an additional source of churn to fix. Other inherited variables still need measurement and deliberate treatment.

Retain the existing newest-observation rule. A newer failure, incomplete authoritative execution or live reservation cannot be hidden by finding an older green. A complete pass from a failed overall attempt remains eligible when its siblings' failures do not invalidate its prerequisites. A canceled or incompletely collected check is never promoted to passed.

Coalesce equivalent work under the existing attempt/component reservation lock. Freeze an attempt's complete group and prerequisite set before admission. In one lock transaction, recheck retained observations and live producers, allocate **all** newly missing identities, and persist execution ownership separately from complete plan membership. Assign a durable increasing admission sequence; a consumer may wait only on an earlier admitted producer. Do not add late reservations or wait edges to a frozen attempt: expanded coverage needs another admission. This makes the wait graph acyclic even while evidence remains consumable only after its owning attempt terminates. No new independently committed group-result protocol is required.

Only actual producer reservations count as live execution observations. A plan mentioning a reused or awaited group must not supersede that group's producer. Composed receipts preserve original observation identity and time; copying evidence does not make it newer. Terminal retention checks the complete admitted inventory and all referenced owners, while duplicate admission checks only execution ownership. Change both readers together; merely adding execute/reuse/wait labels to today's full-plan scan would still permit deadlock.

The reservation lookup must widen from its current goal/revision scope to `(installation namespace, group ID, identity version, execution identity, freshness episode)`. Authority and accounting remain on the owning attempt. A consumer's cancellation detaches that consumer; it does not cancel another goal's producer. Keep separate attempts for distinct prefix variants of the same group, because today's `PendingTestGroups` map has one identity per group ID. Share equal variants through retained references and live-producer lookup rather than overloading that map. Reuse is initially confined to the existing trusted installation/evidence namespace. Cross-machine sharing is a separate problem.

Wait through existing durable notifications without holding execution capacity. A producer's terminal failure is returned to every waiter with the original evidence; it is not an automatic cache miss that launches the same work again. Independent admitted checks may finish, but dependent work and delivery remain blocked. Any retry needs the existing bounded diagnosis/retry decision and budget. Reconcile a lost producer through custody/recovery before replacement; elapsed time alone does not authorize duplication. Restart reloads producer references and admission order rather than electing new owners independently.

Freshness is explicit. Ordinary deterministic checks use compatible retained evidence. Cadence, forced diagnostics and latency benchmarks require native observations within their current episode; mutable external checks declare the same rule plus any required expiry. The existing testing owner creates and persists an episode in the plan/attempt records before admission, bound to the purpose, diagnostic target or cadence occurrence, exact candidate/base, selected plan and relevant external-state scope. Each newly requested diagnosis or certification gets a new episode; a changed candidate, base or plan also starts one. A crash/resume of the same unchanged episode may consume its completed native observation from a terminal producer, provided its declared expiry has not passed. Restart alone must not create a new measurement; a native success lost before durable collection is still incomplete evidence. Receipt-only edits follow the existing normalized candidate projection when that projection and all bound inputs remain equal. Different fresh episodes never share observations merely because content hashes match.

Batch certification records an episode per prefix decision requiring fresh work. Receipt composition and repeated `ensure` calls for that same pending decision retain the episode; `verify` validates it and never creates one. Reassembly preserves an unaffected prefix's pending episode only when all its bound facts remain equal. A changed prefix gets a new episode. A changed destination base renews every affected prefix's episode, including an input-disjoint rebase. Fresh work is consequently not shared across distinct prefix decisions in the initial design; deterministic work remains shareable. This conservative cost is explicit and does not require a broader freshness-sharing protocol.

Carry episode IDs, producer references, native-launch evidence and expiry into retained results and receipt verification. An ordinary cached pass cannot satisfy a fresh episode; an expired or mismatched observation returns missing work. Implement the documented `--no-reuse` diagnostic intent by forcing component execution in a new episode as well as a new attempt, and verify it with native launch counters.

## 5. Make targeted tests and canaries effective

### One path grammar

Use one matcher for surface ownership, declared input expansion and diagnostic attribution. Support exact relative paths, `*` and `?` within one path component, and a trailing `/**` subtree. Reject other wildcard syntax explicitly. Normalize separators and `./`; reject path escape. Additions, deletion, renames and an absent optional input participate in the manifest. A pattern matching no current file is allowed for future/optional paths and is reported; unsupported syntax is not silently treated as a literal filename.

Keep conservative selection for genuinely unknown paths. Explain why a path reached fallback. Fix the existing `landing_batch*.go`, `launch*.go` and `handoff*.go` declarations through the shared grammar, then measure actual selection. Do not remove true dependency edges to manufacture a smaller plan.

### Admission versus acceptance

The application contract identifies inexpensive admission checks and complete selected acceptance coverage. A check's phase is distinct from its test kind or resource demand. Admission usually includes parse/build feasibility, focused changed-behavior regressions, and boundary canaries. Expensive affected-consumer or whole-package checks remain required acceptance obligations; they need not all run synchronously before the unit can enter the batch queue.

This is an intentional change to the current whole reverse-dependent package join gate. Its obligations move into the owner's prefix/tip proof; they are not deleted. Join means “ready for batch proof,” not “already safe to publish.” Authority, independent read and mutation/witness requirements remain in force.

For the first contract migration, preserve all existing coverage obligations and mark their intended phase explicitly. A review of the host contract can later narrow broad groups only when actual readers and consumers justify it. Named Go test subsets still compile their whole package; hand-narrowing `inputs` cannot erase that dependency. Split genuinely independent test/package owners where repeated expensive compilation or fixture startup merits it.

### Prerequisites and failure collection

Add only the group prerequisite relation needed by real canary/build consumers to the testing contract. It is a directed acyclic relation between existing groups, validated for missing references and cycles. Selecting a group includes its prerequisite closure. A prerequisite providing a runtime artifact also declares that artifact as an input to its consumer; an ordering edge alone does not describe data dependencies.

The executor schedules ready independent groups with bounded capacity. A failed or invalid prerequisite marks its dependent groups `blocked`, naming the prerequisite; it does not launch them. Independent groups continue within the admitted budget. Delivery sufficiency requires all required groups to be passed or validly reused, so blocked groups keep delivery red. Reused results do not bypass a currently unsatisfied prerequisite.

Canaries must exercise the real boundary: actual CLI arguments, policy selection, custody, child startup, result collection and cleanup where relevant. For this mechanism, a tiny command-adapter project exercises prefix receipts through the actual public command and receipt consumer. Mocks still serve unit tests, but cannot certify that interface.

After a valid expensive acceptance run starts, collect independent failures instead of spending a separate broad run on each one. On repair, first reproduce and clear the specific failure, then ensure only failed, blocked, newly selected or invalidated identities. Stop collection on a shared environment failure, canceled budget or unsafe resource condition. Classify product failure separately from invalid infrastructure evidence.

## 6. Batch landing algorithm

Let `B` be the frozen destination base and `T1 ... Tk` the exact cumulative trees in original join order. For each prefix, select `Qi` from the protected policies at `B` and `Ti`, the cumulative changed paths, and the accepted obligations of members `1 ... i`. The tip also covers the sealed union of admitted member obligations. Selection refers to effective check definitions, not just strings naming groups.

This replaces the current request to run every tip group on every earlier prefix. A test introduced only by member three is not demanded of prefix one merely because it exists at the tip. Prefix one still receives its complete applicable proof. If an earlier member requires that test or a later implementation to be correct, the proposed unit boundary is invalid and must be repaired before landing.

### Join and seal

1. Prepare the candidate from the reviewed change and frozen base. Validate authority, review provenance and required witnesses. Ensure only missing admission checks on the exact candidate; store structured result references.
2. Check composition/conflicts and record the member's acceptance obligations. Return after the durable queue/handover record exists. Heavy acceptance work is owned by the batch owner.
3. Seal a recoverable ordered member list and all prefix trees. Recompute `Qi` and the final union on those actual trees. Record the policy and authority revisions that justified the selection.
4. Remove seal's unconditional fast-gate and changed-package command loop. Its responsibility is composition validation and proof planning; missing checks go through the common executor.

### Execute distinct missing evidence

Form the required requests `(effective group, execution identity, freshness requirement)` over all `Qi`. Resolve complete identities and prerequisite closure before admission, then apply Section 4's atomic missing-set allocation and producer lookup. Coalesce equal compatible requests. Fresh requests carry their prefix decision's episode and cannot be satisfied by an old observation.

Run prerequisites first. Prefer the tip's checks when they provide early integration feedback and can cover matching prefixes; schedule genuinely distinct prefix checks through the same bounded pool when their prerequisites are ready. Current batch budget attribution remains authoritative: shared/tip work charges the recorded authority member; prefix-only work charges that prefix's member. Reserve total missing work before starting it, charge a native execution once, and charge no execution attempt for receipt-only composition. A refusal does not silently transfer budget to another goal.

For independent components A, B and C, a stable harness canary plus checks `A1`, `B1`, `C1` may supply:

| Prefix | Required component evidence | New component execution beyond the tip's evidence |
| --- | --- | --- |
| A | Harness, A1 | None |
| A + B | Harness, A1, B1 | None |
| A + B + C | Harness, A1, B1, C1 | Tip executes each missing identity once |

This example assumes genuinely independent closures and an unchanged harness. If integration check I consumes the whole application, the prefixes may require `I1`, `I2` and `I3`. All three are real work. The system must show their differing inputs rather than promise that batching removes them.

### Red, green and moved trunk

- **Red:** persist every observed failure and complete pass. Diagnose only the failing groups and prerequisites needed for valid fresh diagnosis. Test base and relevant combinations under the existing bounded diagnosis rules; preserve join order and explicit handling of interactions. No blind retry of the same red batch.
- **Reassembled batch:** recompute exact prefix trees and requirements. Keep retained observations in the common store; invalidate only mismatching evidence. A prior attempt's red overall status does not discard its eligible green groups.
- **Green:** compose a sufficient receipt for every exact prefix and the tip, each naming its decision context and original observations. Revalidate review and authority bindings. Construct the commit series locally, run the existing held-range verification and publish once. Receipt/record changes cannot themselves force native execution when selected identities remain valid.
- **Moved trunk:** rebase/recompose through the existing transport. Recompute every affected prefix's selection against the new destination policy as well as the previously admitted protection floor. Content-disjoint transport is not enough to skip this check. Verify reusable evidence; ensure only newly missing work. A conflict, policy incompatibility or changed reviewed behavior returns to its existing owner.
- **Restart:** load durable members, candidate trees, terminal observations and publication state. Resume missing work and receipt composition. Resolve ambiguous publication through existing origin trailers before publishing again. Never infer success from a log tail or an absent worker.

The existing held-range check walks commit ownership; it is not proof verification for every prefix ([held.go](../metasystem/internal/landing/held.go#L43)). The batch receipt owner must verify each final commit's corresponding prefix evidence before the held-range/publication step, including **every rebased prefix**. Current moved-base recovery explicitly verifies only the tip at `landing_batch_land.go:212`; replace that assumption with the prefix loop. A passing held check cannot substitute for it.

### When to seal

Keep original order and existing maximum-wait rules. Add a cost view based on distinct missing identities, observed duration and host capacity. Close admission when the projected missing work no longer fits the remaining budgets of the goals that would actually pay for it. Tip work charges its authority goal, prefix-only work its prefix owner, and compatible reuse requires no new native reservation. Budgets are not pooled. Missing duration estimates use a conservative declared allowance, separately labeled from measured work or predicted wall time; they are not treated as zero.

Do not wait indefinitely for an ideal batch or reorder work solely for a cache hit. The first implementation uses the cost view to enforce the existing budget and report poor batches; automatic tuning of batch size waits for matched measurements. This avoids adding an unproven scheduler to the critical path.

The forecast is a timestamped snapshot, not an execution reservation. Join checks it before handover and seal recomputes it for the fetched base. Stored status always labels it historical and shows live headroom separately. Final locked admission remains authoritative before test groups start. A read-only cold candidate-build screen avoids preparation when the goal is already definitely exhausted and retained evidence cannot cover the selection; it does not reserve against a later spending race or cover earlier trusted-policy rearm.

### Split a failing batch safely

Wido explicitly approved this bounded behavior as part of the current goal. After bounded diagnosis identifies a failing member, or the accepted ledger confirms a fence against its exact handed-over claim, return that member through the existing owner. If removing it makes another member's patch inapplicable, return that dependent member with an explicit composition reason. Do not falsely label it as a failed test. Preserve original order and bound dependency closure by the original number of members; unknown attribution, policy errors, and infrastructure failures hold the batch.

The survivor-reassembly owner records all return requests and the new candidate/state atomically against the original membership, custody and proof bindings. A contributes a file, B changes it, and independent C changes another file: if A fails, A and B leave together and C can proceed. Recompute the surviving prefixes on the actual accepted base and consume only matching evidence. Missing proof runs through the common executor, and every surviving prefix must be sufficient before publication. A moved base also requires the private candidate branch to match the newly sealed tree.

The returned goals remain repairable and can join a later batch. This does not create an unbounded parallel repair lane, an automatic optimal-partition search, or a semantic dependency graph. Shared budget contributions and adaptive batch sizing remain separate follow-up designs.

## 7. Preparation and resource admission

Before building, compute a candidate-engine/build input identity from tracked content, toolchain and explicit build context. Reuse an immutable artifact only when its content digest and producer identity validate. Publish new artifacts atomically under an identity reservation. A corrupt or absent artifact is rebuilt; a failed build cannot occupy the successful-cache entry. Existing compiler caches remain useful beneath this layer.

Cache discovery and input manifests by complete dependency/tool/environment identity. Enumerating a changed tree and checking tool state may still cost work; the guarantee is no unnecessary native build or test when retained facts establish compatibility. Do not promise constant-time verification or trust stale metadata to achieve it. Preserve isolated writable outputs and cleanup for executing groups; share immutable source preparation where safe.

Use the existing host admission owner for expensive native execution from unit proof, join, batch proof, diagnostics and cadence. Application configuration declares cost/resource classes and exclusive resources; bounded cheap checks remain concurrent. Retain the current host safety envelope until replacement admission is proved. A Go test that expands to 41 packages cannot remain exempt solely because its caller names it “focused.”

Nested workers consume their parent's reservation instead of acquiring the same capacity again. Capacity is held only during actual resource-consuming work, not human review or waiting for an existing producer. Cancellation propagates through owned child processes, incomplete evidence remains non-green, and capacity returns only after cleanup/reconciliation proves the resource is free. Queue time and execution time are recorded separately. This is one local admission mechanism, not a cross-machine scheduling service.

## 8. Application contract and migration

Preserve the existing Go, section and command/JUnit adapters. The proposed contract additions are group admission/acceptance placement, prerequisite references, consumed artifacts, explicit environment mode, freshness and resource requirements. Reuse existing fields where they already express those facts. The schema revision and validation must distinguish these proposed additions from fields today's strict decoder accepts.

The command adapter remains the portability test. A Java service might declare a pricing unit group, checkout consumers, a migration integration group and an HTTP canary. A JavaScript application can declare equivalent groups around its own commands and reports. Neither requires a Go-specific branch in batch landing. The MetaSystem executable may still have its own toolchain requirement; reuse its unchanged artifact rather than rebuilding it for every application receipt.

Version the new evidence identity and contract schema explicitly. Old records remain readable as history; never relabel an old hash as the new identity. Old evidence is consumable only through its exact existing compatibility rules, otherwise selected checks execute once under the new identity. Do not make migration require every application test merely because the cache format changed.

Cut over one owner at a time with its consumers and tests. Do not maintain a second writable proof database or parallel certifying path. Finish active old-format batches before changing their owner, or reopen them through the existing return/reassembly mechanism with authority intact. Interrupted batches require explicit format detection and re-planning; no in-place reinterpretation of their receipts. Rollback restores a compatible engine/contract pair and revalidates evidence before publishing.

## 9. Implementation map

These are proposed work units, not new approved goals. Reuse the existing efficiency and boundary-canary goals where their scope fits. Estimates include implementation and tests and are revisable when the coherent boundary is clearer; they are not line caps or reasons to omit proof.

| Unit | Observable result and existing owners | Changed-line allocation | Required evidence |
| --- | --- | --- | --- |
| 1. Make boundaries honest | Repair delivery selection and fresh diagnostic semantics in `testpolicy`, testing CLI and batch receipt callers; unify path grammar and declarations | 1,200 | GLE-1–3: real argument-to-selector canary; real diagnostic launch count; exact/wildcard/add/delete fixtures |
| 2. Carry one proof through delivery | Shared execution/evidence in `proofrun`, `launch` and batch join/seal; exact-prefix planning and receipt consumption | 1,500 | GLE-2, 3, 7, 8: three-unit external application lands locally with per-prefix receipts and no duplicate compatible launches; changed shared dependency still reruns |
| 3. Remove unrelated invalidation | Versioned effective-group identity and current-policy evaluation in `proofrun`/`testpolicy`; verified preparation-artifact reuse | 1,400 | GLE-1, 3, 5: unrelated contract edit reuses A; weakened policy refuses; true input/tool/environment change invalidates; unchanged proof has zero native build/test launches |
| 4. Shorten failed iterations | Prerequisite closure, independent failure collection, structured follow-up evidence, resource admission across entrypoints | 1,500 | GLE-4, 6: two independent failures found together; prerequisite failure suppresses dependent launches; repair retains unaffected passes; concurrent joins plus owner remain within capacity |
| 5. Prove and cut over the workflow | Application contract migration, obsolete gate deletion, review-guidance reconciliation and cost reporting at existing owners | 1,000 | GLE-7–9: matched before/after cohort, crash/recovery, moved-trunk and non-Go application acceptance; no stale second proof path |

Unit 1 provides the earliest useful improvement and the regression harness for the rest. Unit 2 first preserves current conservative identities. Unit 3 changes reuse semantics with dedicated invalidation tests. Unit 4 uses the same executor instead of adding another queue. Every unit carries its own behavior tests and reviewer context; later stages consume prior compatible results. Broad validation follows the existing selected/milestone policy and any applicable judge-change requirements, not a blanket full suite after each correction.

## 10. Obligations and acceptance

The rows track implementation obligations. The [implementation evidence report](goal-landing-efficiency-implementation-evidence.md) distinguishes focused and public runtime evidence from outstanding final acceptance. None of these rows authorizes publication while a critical or high obligation remains open.

Runtime log names below are retained under `/Users/wido/LocalStorage/agentic-tools-evidence/goal-landing-efficiency-20260920`; source and test paths name the uncommitted integration tree.

| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GLE-1 | CRITICAL | Sections 3–4 | Reuse cannot weaken required coverage or trust | `internal/testpolicy`, `internal/proofrun` | `metasystem/internal/testpolicy/protection.go:ProtectedContract`; `metasystem/internal/proofrun/test_build.go:groupExecutionIdentity`; `metasystem/cmd/metasystem/test.go:checkDeliveryInputParity` | `metasystem/internal/proofrun/test_identity_v2_test.go:TestEffectiveGroupIdentitySeparatesPolicyFromExecution`; `metasystem/cmd/metasystem/landing_batch_prefix_gle_test.go:TestGLEBatchDeliverySupplementKeepsPolicyFloor` | `portable-cohort-7-integration.log`: unrelated B addition reused A; declared input change reran A; sufficient final candidate `390357e8e1675048ad5bceceefdc1b479e775ace`, attempt `proof-muamxtj4-3c765243d2af4133`, exact-tree verification exit 0 | DONE | Capability proof complete; release publication/install recorded in the handoff |
| GLE-2 | CRITICAL | Section 6 | Every published prefix has sufficient exact-tree evidence | `internal/landing/batch`, `cmd/metasystem` | `metasystem/internal/landing/batch/receipts.go:ComposePrefixReceipts`; `metasystem/cmd/metasystem/landing_batch_prefix.go:verifyBatchSeries`; `metasystem/internal/landing/batch/land.go:LandSeries` | `metasystem/cmd/metasystem/landing_batch_prefix_gle_test.go:TestGLEBatchEveryPrefixHasApplicablePolicyProof`; `metasystem/internal/landing/batch/receipts_gle_test.go:TestGLEBatchTipGreenCannotHideRedEarlierPrefix`; `metasystem/cmd/metasystem/goal_landing_portable_test.go:TestCommandApplicationRedEarlierPrefixBlocksReceipt` | `portable-acceptance-final.log` and `focused-final/batch-result.md`: exact prefixes and red-prefix refusal; owner publication fixture passed; sufficient final candidate `390357e8e1675048ad5bceceefdc1b479e775ace`, attempt `proof-muamxtj4-3c765243d2af4133`, exact-tree verification exit 0 | DONE | Capability proof complete; release publication/install recorded in the handoff |
| GLE-3 | HIGH | Sections 3–4 | Unchanged checks execute once without wait cycles or implicit failure retries; fresh diagnostics really execute | `internal/proofrun`, `cmd/metasystem` | `metasystem/internal/proofrun/test_ownership.go:allocateTestOwnershipLocked`; `metasystem/internal/proofrun/test_ownership.go:newestOwnedObservation`; `metasystem/cmd/metasystem/test.go:testingFreshGroups` | `metasystem/internal/proofrun/test_ownership_test.go:TestOverlappingReservationsTerminateOnce`; `metasystem/internal/proofrun/test_ownership_test.go:TestSharedFailureReachesAllWaitersWithoutRetry`; `metasystem/internal/proofrun/test_ownership_review_test.go:TestGLEOwnershipFreshExpiryIsBoundAtWriteReadAndReuse` | `portable-fresh-final.log`: same episode reused A; new episode reran A; sufficient final candidate `390357e8e1675048ad5bceceefdc1b479e775ace`, attempt `proof-muamxtj4-3c765243d2af4133`, exact-tree verification exit 0 | DONE | Capability proof complete; release publication/install recorded in the handoff |
| GLE-4 | HIGH | Section 5 | Real canaries block invalid dependent work while independent defects are collected | `internal/testpolicy`, `internal/proofrun` | `metasystem/internal/testpolicy/select.go:WithPrerequisiteClosure`; `metasystem/internal/proofrun/test_build.go:runExecutionContractPlan`; `metasystem/internal/proofrun/test_ownership.go:blockedWithoutNativeProducer` | `metasystem/internal/proofrun/test_execution_contract_test.go:TestGLEPrerequisiteFailureBlocksOnlyDependents`; `metasystem/internal/proofrun/attempt_test.go:TestGLEBlockedReservationDoesNotBecomeFailedProducer`; `metasystem/cmd/metasystem/goal_landing_portable_test.go:TestCommandApplicationPrerequisiteAndIndependentFailures` | `portable-gle4-final.log`: H/B failed, A blocked, C passed; repaired checks retained independent C; sufficient final candidate `390357e8e1675048ad5bceceefdc1b479e775ace`, attempt `proof-muamxtj4-3c765243d2af4133`, exact-tree verification exit 0 | DONE | Capability proof complete; release publication/install recorded in the handoff |
| GLE-5 | HIGH | Sections 4, 7 | Cached preparation cannot omit real inputs or substitute artifacts | `internal/proofrun`, `cmd/metasystem` | `metasystem/internal/proofrun/test_build.go:PrepareGroupExecutionIdentities`; `metasystem/internal/proofrun/test_build.go:groupTestEnvironment`; `metasystem/cmd/metasystem/test.go:checkDeliveryInputParity` | `metasystem/internal/proofrun/test_identity_v2_test.go:TestEffectiveGroupIdentitySeparatesPolicyFromExecution`; `metasystem/internal/proofrun/test_execution_contract_test.go:TestGLEExplicitEnvironmentAndDeclaredToolInputs`; `metasystem/cmd/metasystem/test_contract_tools_test.go:TestGLEExplicitToolReadinessUsesExecutedEnvironment` | `portable-cohort-7-integration.log` and `portable-result.md`: warm reuse and declared shared-input reruns; sufficient final candidate `390357e8e1675048ad5bceceefdc1b479e775ace`, attempt `proof-muamxtj4-3c765243d2af4133`, exact-tree verification exit 0 | DONE | Capability proof complete; release publication/install recorded in the handoff |
| GLE-6 | HIGH | Section 7 | Heavy work respects capacity and crashes do not leak it | `internal/proofrun` | `metasystem/internal/proofrun/host_resources.go:AcquireHostResources`; `metasystem/internal/proofrun/resource_custody.go:RunResourceCustodian`; `metasystem/internal/proofrun/resource_custody.go:RunResourceCommand` | `metasystem/internal/proofrun/host_resources_test.go:TestHostResourceNestedLeaseRequiresHeldSubset`; `metasystem/internal/proofrun/host_resource_custody_test.go:TestGLEHostResourceKilledLauncherAndWorkerKeepOrdinaryGrandchildInCustody`; `metasystem/internal/proofrun/resource_custody_cancel_test.go:TestGLEResourceCommandCancellationDrainsClosedFDDescendantBeforeRelease` | `resource-r2-corrections/manifest.json`: native live-owner regression fails before and passes after; `cmd-r2-public-final-green.log`: all public routes, nested charge and launcher loss pass; `focused-final/resource-r2-admission-final.log`: forged exclusive descriptor and nested outer-custody loss pass; sufficient final candidate `390357e8e1675048ad5bceceefdc1b479e775ace`, attempt `proof-muamxtj4-3c765243d2af4133`, exact-tree verification exit 0 | DONE | Capability proof complete; release publication/install recorded in the handoff |
| GLE-7 | HIGH | Sections 4, 6, 8 | Rebase, restart and format change cannot publish stale or duplicate work; fresh episodes survive identical recovery and renew on changed certification | `internal/landing/batch`, `cmd/metasystem` | `metasystem/internal/landing/batch/receipts.go:ensurePrefixEpisode`; `metasystem/cmd/metasystem/landing_batch_land.go:reopenBatchBeforeReceiptsWhenProofBaseMoved`; `metasystem/internal/landing/batch/land.go:ReopenMovedTrunk` | `metasystem/internal/landing/batch/receipts_gle_test.go:TestGLEBatchFreshEpisodeSurvivesRestartAndRenewsOnExpiryOrBaseMove`; `metasystem/cmd/metasystem/landing_batch_retry_authority_test.go:TestGLEBatchMovedRetryRejectsRevisedMemberBeforePublication`; `metasystem/cmd/metasystem/goal_landing_portable_lifecycle_test.go:TestGLEBatchPortableOwnerLandsRealCommandApplication` | `portable-gle7-final.log`, `focused-final/batch-result.md`, and `cross-version-probe/result.md`: moved-base, owner succession and schema-1 migration canaries passed; sufficient final candidate `390357e8e1675048ad5bceceefdc1b479e775ace`, attempt `proof-muamxtj4-3c765243d2af4133`, exact-tree verification exit 0 | DONE | Capability proof complete; release publication/install recorded in the handoff |
| GLE-8 | HIGH | Sections 3, 8 | Another application uses the same landing implementation | `internal/proofrun`, `cmd/metasystem` | `metasystem/internal/proofrun/test_build.go:runTestGroup`; `metasystem/cmd/metasystem/test.go:runTestRun`; `metasystem/internal/landing/batch/receipts.go:ComposePrefixReceipts` | `metasystem/cmd/metasystem/goal_landing_portable_test.go:TestCommandApplicationThreePrefixReceiptConsumer`; `metasystem/cmd/metasystem/goal_landing_portable_test.go:TestCommandApplicationGreenTipCannotHideRedPrefix`; `metasystem/cmd/metasystem/goal_landing_portable_lifecycle_test.go:TestGLEBatchPortableOwnerLandsRealCommandApplication` | `portable-acceptance-final.log` and `focused-final/batch-result.md`: command/JUnit app joined, recovered, re-proved moved B and atomically published A1/B2/C1; sufficient final candidate `390357e8e1675048ad5bceceefdc1b479e775ace`, attempt `proof-muamxtj4-3c765243d2af4133`, exact-tree verification exit 0 | DONE | Capability proof complete; release publication/install recorded in the handoff |
| GLE-9 | MEDIUM | Sections 2, 9 | Review and model waiting do not repeat settled work | `internal/launch`, `docs/orchestration.md` | `metasystem/internal/launch/declared_outputs.go:Manager.prepareDeclaredOutputs`; `metasystem/internal/launch/declared_outputs.go:Manager.checkEarlierOutputOwners`; `metasystem/docs/orchestration.md` | `metasystem/internal/launch/launch_test.go:TestDeclaredOutputPathIsLockedWhileItsLaunchOwnsIt`; `metasystem/internal/launch/launch_test.go:TestUnprovenOutputWriterBlocksSuccessorAcrossManagerRestart`; `metasystem/internal/launch/launch_test.go:TestUnrecordedOutputWriterKeepsItsPathClaim` | Focused launch-output cases cover stale output and live writer; bounded review records remain in `plans/goal-landing-efficiency-implementation-evidence.md` | PARTIAL | Retain final output-ownership and workflow evidence at release |
| GLE-10 | MEDIUM | Section 11 | Matched cohort reports actual landing latency and work without conflating engine changes, cache counters or overlapping test durations | Release measurement owner | Existing portable cohort and proof result accounting | Seven-case matched v1-compatible command/JUnit cohort | `rollout/measured-release-cost-and-limits.md`; `rollout/efficiency-acceptance-coverage-read.md`: full batch before/after and p50/p95 remain unmeasured | PARTIAL | Retain this explicit measurement gap; no overall percentage speedup claim; carry matched comparison into the authorized performance follow-through |

GLE-6 now has the accepted RC-3 correction and decisive public capacity, nested-lease and guardian-loss process evidence; its exact-candidate delivery proof is now sufficient, as recorded in the closed rows above. The resource, generic and Go review rounds are closed as reads; accepted corrections require decisive focused and public evidence, without a third read. The nested-installation F-1 failed the project-root case before correction and passed after it; the second and final scoped read at `reviews/nested-installation-r2-fulltree/source/review-findings.md` returned `VERDICT: land` on tree `134529cd6cddcb287c359367726868f6db74946d`. Final selected delivery proof is now sufficient; publication and installation are recorded separately in the handoff.

Use a tiny application and disposable local Git remote for the first end-to-end proof. Exercise the production planner, executor, evidence store, receipt reader and range publication path; do not inject synthetic green result JSON across the very boundary under test. Native commands emit complete reports and increment counters so an extra execution is observable.

Guard cases include a failing earlier prefix repaired only at the tip; a changed shared dependency; an unrelated contract addition; a removed required test; a latest failed observation after an older pass; a killed collector; an environment/tool mutation; a fresh base diagnosis with cached green evidence; and a changed destination policy. Each must produce the expected refusal or minimal additional work.

## 11. Efficiency acceptance and stop conditions

Record the same fixed cohort before and after: docs-only, local behavior, shared dependency, testing-policy change, disjoint batch, interacting batch, failed harness and moved trunk. Separate cold/warm conditions and host load. Keep child/work totals separate from critical-path wall time.

The deterministic acceptance criteria are:

- Zero duplicate native executions of a reusable required identity across a sequential join/seal/prefix/commit path, absent a newer invalidating observation.
- Zero native build/test launches for an unchanged sufficient proof when retained preparation facts are valid.
- Zero expensive dependent launches after a failed prerequisite.
- Every independent seeded failure reported in the admitted collection pass.
- No loss of required coverage, prefix safety, fresh-diagnostic execution or publication/authority checks.
- The external application scenario requires no batch implementation changes.

Measure p50/p95 intent-to-landed latency, lock waiting, preparation, native test time, expensive-group reuse, review rounds and post-landing defects. An aggregate cache-hit percentage is insufficient when only cheap groups hit. Report selection and reuse misses by changed content, definition, producer, environment, policy/freshness, newer observation or absent evidence.

No percentage speedup is promised before that comparison. Prefer the next change that removes measured critical-path work. Keep the broad judge invalidation until a measured and safely separable executor/evaluator boundary justifies further work. Do not add distributed execution, speculative retries, automatic batch optimization or a new daemon to improve an unmeasured secondary cost.

Before an expensive acceptance run, preserve the exact candidate and state its question, changed facts, expected signal, budget and stop condition. A failed run returns a reusable failure set and eligible passes. A subsequent run must establish a new fact; repeating unchanged red work is not progress.

## 12. Design verification and scope

This proposal names the current consumers, existing owners, cutovers, failure semantics, evidence rules and focused acceptance targets. Source references are anchored to the baseline and the earlier analysis; no performance improvement or runtime correctness is claimed as achieved. Product code, runtime settings, ledger authority and the active mission are unchanged by writing this document.

An independent complete-design read returned two material findings. Both are accepted below and each has a bounded implementation fixture. The design-critique loop therefore ends at round 1 under its fixture-expressible early-exit rule, within the declared two-round limit. This is review of the proposed design, not implementation certification. The [verbatim review](goal-landing-efficiency-improvements-review-r1.md) records the counterexamples and source evidence.

| Finding id | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- |
| GLE-R1-01 | accepted | Current terminal-only observation scanning treats complete-plan membership as production. Two overlapping plans can each own one group and wait indefinitely for the other's outer termination. | Section 4 separates producer ownership from required inventory, atomically reserves the complete missing set, permits only waits on earlier owners, preserves original observation provenance and relays failure without automatic retry. Section 6 uses that rule. GLE-3 requires interleaved termination and shared-failure fixtures. |
| GLE-R1-02 | accepted | Without a durable episode lifetime, recovery can duplicate fresh work or a rebase can consume a previous certification's mutable-state observation. | Sections 4 and 6 define episode creation, exact bindings, unchanged recovery, unaffected-prefix retention, changed-base renewal and verify-only consumption. GLE-3 and GLE-7 require launch-count and recovery/rebase fixtures. |

Document verification at design delivery covered local links, source line anchors, whitespace and the finding/disposition join. At that point GLE-1–8 were `MISSING`. The implementation status above is a later progress record; it does not retroactively turn the design review into code or runtime certification.

Rejected alternatives: dropping prefix proof sacrifices per-commit correctness; a larger context window does not remove native work; blanket parallelism worsens contention; full suites after every fold repeat low-information work; a batch-specific cache duplicates an existing owner; blindly narrowing hash inputs admits stale evidence; and replacing all review with tests loses independent judgment.

The remaining implementation work is tracked in the matrix and [handoff](handoff-goal-landing-efficiency.md). The final selected proof runs after the critical and high rows are ready for runtime acceptance.

The 13:23 CEST runtime-readiness update means the implemented boundaries and focused regressions pass. Final scoped review remains in progress; any material finding reopens its row before the delivery run. The remaining common runtime proof is the actual destination-selected A/B release and verification of the published result. It is not a completion declaration.
