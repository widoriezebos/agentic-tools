REVISE — structural findings remain

Static code-grounded review found 15 material defects: 8 STRUCTURAL and 7 MECHANICAL-GRAIN. The second critique budget is exhausted; implementation should not start without the recorded split or a human-ratified exit.

## Round-5 fold verification

| R5 finding | R6 result |
|---|---|
| 1 — incomplete-install record | **Not resolved.** Resume still contradicts refusal, recovery trusts destructive journal data, and the transaction has no stable plan or completion transition. Findings 1–3 and 9. |
| 2 — total mutation plan | **Not resolved.** The mutation classes are named, but several have no operation, owner, recovery behavior, or preserving hook semantics. Findings 4–5. |
| 3 — final engine transport | **Not resolved.** The shell builds the artifact, but no installer argument carries it or the source SHA. Findings 6 and 10. |
| 4 — resolved-registration owner | **Resolved.** `internal/install → internal/runtimes` is cycle-safe at [design:155](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:155); `internal/runtimes` remains a dependency leaf at [runtimes.go:1](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:1). |
| 5 — in-place reference | **Partial.** `ReferenceRequirement` is introduced, but the wire and collision rules still classify it as an output. Finding 11. |
| 6 — `LiveSelfCheck` source | **Partial.** `row-source` replaces the shell lookup, but its root is undefined and deleting the old field misses `internal/audit`. Findings 13–14. |
| 7 — snapshot schema rollout | **Resolved by the alternative.** R6 keeps retained snapshots valid and uses deterministic construction instead of a discriminator at [design:408](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:408). A new security-path defect remains in Finding 7. |
| 8 — `ValidateMission` capability | **Partial.** The call-site paragraph adds `MissionHolderProbe`, but the authoritative capability inventory omits it. Finding 15. |
| 9 — mode and typed `Validate` | **Partial.** `--mode` and typed clean/drift/error outcomes are present at [design:204](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:204), but persisted-mode representation and row ownership remain broken. Findings 8 and 12. |

## Material findings

1. **HIGH — STRUCTURAL: mismatch recovery turns a target-owned record into a deletion and overwrite capability.**

   R6 restores record-supplied preimages and removes record-listed outputs at [design:213](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:213), including one outside-root path at [design:229](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:229). Current recognition does not reserve or authenticate `.metasystem` ([adopt.sh:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:129)), and current merge behavior preserves user content ([adopt.sh:256](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:256)). A forged record can name arbitrary paths; a legitimate stale record can erase edits made after interruption.

   The design should treat the record as untrusted data: version and validate it, reconstruct all permissible destinations independently, require ordinary paths beneath canonical `R`, freshly derive the sole Git-hook path, and restore/remove only when current type, mode, and bytes equal the recorded installer postimage. Otherwise require reconciliation. A different or unreconstructable source SHA must never trigger automatic cleanup.

2. **HIGH — STRUCTURAL: the “one immutable plan” has neither a stable carrier nor an enforceable mutation boundary.**

   `plan`, `prepare`, and `apply` are separate commands with no plan-file transport at [design:194](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:194). Their resolved sources contain per-attempt temporary paths, while adoption creates a new staging directory each run ([adopt.sh:139](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:139)). Worse, public `install row --phase apply` carries no plan digest and can bypass global preparation and journaling ([design:187](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:187)).

   The design should define one sealed plan artifact generated once. Its digest covers logical operations and destinations, generated values, effective modes, and prepared-content hashes—not ephemeral absolute staging paths, statuses, or preimages. Every later phase consumes that artifact and digest. Row-level apply must be internal or verify the active journal, digest, and prepared status.

3. **HIGH — STRUCTURAL: successful adoption has no crash-consistent audit and completion transition.**

   Only `plan|prepare|apply` exists at [design:204](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:204); the completion marker is an apply output written last at [design:225](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:225). The draft never names completed-state storage, retires the incomplete record, or orders the structural audit. Current adoption audits after installed outputs ([adopt.sh:346](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:346)), and same-SHA health recognition depends on that audit ([adopt.sh:120](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:120)).

   The design should define one versioned adoption-state path. Apply leaves it incomplete; the shell runs the existing structural audit; then `install complete` rechecks the digest and atomically promotes the state to completed as the final mutation. Completed state carries SHA, selection, plan identity, and effective modes. Healthy recognition requires it, with an explicit legacy-marker migration.

4. **HIGH — STRUCTURAL: “total” mutation enumeration names objects without defining executable core operations.**

   The plan is initially defined as runtime rows plus payload and final engine at [design:194](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:194), then later acquires the goal baseline, artifact state, relocated workflow, marker, and hook at [design:225](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:225). These lack Prepare/Apply/recovery contracts. `docs/project-rules.md` would also be both a payload copy and a later transform—two incompatible writers under [design:130](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:130)—matching today’s copy-then-rewrite sequence at [adopt.sh:247](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:247) and [adopt.sh:291](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:291). Goal-baseline encoding still belongs solely to `internal/goal` ([goalverbs.go:197](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/goal/goalverbs.go:197)).

   Define a closed `CoreOutput` operation set covering copy, append-merge, generated goal baseline, ensure-directory, relocated workflow, stamped project-rules bytes, engine, and optional hook. Each needs prepare/apply/validate/recovery behavior. `internal/goal` should expose a pure canonical genesis-baseline constructor. Track created directories and remove them deepest-first only while empty. Install the pre-stamped `docs/project-rules.md` once.

5. **HIGH — STRUCTURAL: the sanctioned outside-`R` hook does not preserve the current live decision.**

   “Prepared, collision-checked” at [design:229](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:229) does not specify path derivation or collision outcome. Today Git’s canonical common directory is queried, non-Git targets succeed with a warning, existing hooks are preserved, and only an absent hook is written and made executable ([adopt.sh:358](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:358)).

   Pin the owner and exact query, authorize only `<derived-common-dir>/hooks/pre-commit`, omit with the current warning for non-Git targets or existing hooks, and plan exact bytes plus mode `0755` only when absent. Refusal or overwrite would require explicit ratification.

6. **HIGH — STRUCTURAL: the prepared engine and source SHA still cannot enter the plan.**

   R6 says the shell passes prepared engine bytes at [design:197](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:197), but the command at [design:204](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:204) accepts neither an engine source nor the SHA needed by the journal and stamped project-rules output. The staged archive has no Git metadata; current shell code obtains the SHA separately ([adopt.sh:102](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:102)).

   Add `--source-sha SHA` and `--engine-source PATH`, or carry both in the sealed plan input. Hash the engine bytes and executable mode into the plan and recheck them before apply.

7. **HIGH — STRUCTURAL: enforcement-map validation is detached from the production snapshot that controls permissions.**

   R6 compares the registry with a separate adapter `enforcement-map` verb at [design:365](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:365), while its real-constructor contract check asserts only identity and shape at [design:408](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:408). Today the checked literals are in the actual probe path ([claude.sh:65](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/claude.sh:65)), are persisted by the snapshot writer ([snapshot.go:67](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/snapshot.go:67)), and drive live restrictive-permission decisions ([select.go:92](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:92)). R6 could validate one value while dispatch trusts another.

   Compare the registry map with `envelopeEnforcement` extracted from the deterministic contract snapshot. Production probe and contract must share one pure constructor, varying only provider-derived facts. Remove the detached adapter map or make it a strict extraction from that snapshot; retain fake’s behavioral leg.

8. **HIGH — STRUCTURAL: one persisted scalar mode cannot preserve currently accepted installations.**

   R6 persists one requested mode and validates by it at [design:205](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:205), but names no completed-state schema or rollout. Existing installations record only the SHA ([adopt.sh:117](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:117)), while current validation judges every registration independently—any non-dangling link is accepted, and each copy is compared exactly ([validate-metasystem.sh:565](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:565)). The documented repair procedure likewise permits copy or symlink per skill ([project-adaptation.md:10](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/project-adaptation.md:10)).

   Persist `effectiveMode` per `ConcreteOutput`; keep the invocation-wide requested mode only as provenance. Define legacy migration by classifying each existing destination under current rules, or seek ratification to reject legacy and mixed layouts.

9. **HIGH — MECHANICAL-GRAIN: exact-plan resume still contradicts bootstrap refusal.**

   Exact `planDigest` is routed to resume at [design:220](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:220), but bootstrap still says “incomplete same-SHA refuses” at [design:267](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:267), matching today’s refusal at [adopt.sh:125](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:125).

   Replace step 4: incomplete state bypasses healthy no-op, enters staged plan reconstruction using recorded generated values, and resumes only after exact SHA/selection/mode/digest agreement.

10. **MEDIUM — MECHANICAL-GRAIN: the `go-build.sh` refactor can break every gate caller.**

    R6 specifies `go-build.sh --output PATH --stamp SHA` at [design:199](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:199). Existing gates invoke it without arguments and pass witness stamps through `METASYSTEM_BUILD_STAMP` ([go-gate.sh:141](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-gate.sh:141), [go-gate.sh:252](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-gate.sh:252)); current default and atomic publication behavior lives at [go-build.sh:25](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-build.sh:25).

    Specify backward-compatible optional flags and precedence, or migrate every caller. No arguments must retain the fence, environment/Git stamp resolution, and `root/bin/metasystem`; non-default output must stage beside its destination before atomic rename.

11. **MEDIUM — MECHANICAL-GRAIN: `ReferenceRequirement` still receives collision fields in the registration wire.**

    In-place rows are excluded from every collision field at [design:78](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:78), but collision class is later total over rows and all `skill-profiles` are called instruction-bearing at [design:150](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:150); every row carries those columns at [design:176](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:176).

    Make collision metadata total only over `ConcreteOutput`s. For `skill-profiles/in-place`, encode `-` for `collisionClass` and `uncoveredException`, require those absences, and exclude the reference from compatibility and collision-root proof.

12. **MEDIUM — MECHANICAL-GRAIN: the registry’s tree mode conflicts with the install-time `--mode`.**

    The canonical tree row carries `mode` at [design:53](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:53), and the registration wire requires tree rows to fill it at [design:176](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:176). Yet link versus copy is chosen later by global `--mode` at [design:204](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:204), matching today’s invocation-wide choice ([adopt.sh:54](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:54)).

    Raw tree rows should encode no fixed mode—or a distinct `user-selected` source—and expansion should inject the effective mode into each concrete output. Static `copy|in-place` remains specific to profile rows.

13. **MEDIUM — MECHANICAL-GRAIN: deleting `ShippedEnforcementConfig` misses a live audit consumer.**

    R6 calls the hook lookup the field’s one remaining consumer at [design:121](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:121). `internal/audit` still uses it both to select delivery-contract rows and locate shipped files at [metasystem.go:334](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/audit/metasystem.go:334), with a regression test at [coverage_test.go:335](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/audit/coverage_test.go:335).

    Migrate that audit downstream to enforcement-config rows through the resolved projection, add `internal/audit` to the blast radius, and test that a new enforcement row enters the audit population.

14. **MEDIUM — MECHANICAL-GRAIN: `install row-source` has no source-root contract.**

    It accepts only runtime and row while promising a “resolved source path” at [design:126](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:126). Elsewhere, source resolution requires `S` during installation and `R` during adopted validation ([design:188](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:188)); the current nested-template check needs the explicit template root ([validate-metasystem.sh:1387](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:1387)).

    Either emit exactly the validated clean-relative scalar source and pin the caller’s `$root/…` join, or add `--root R` and return an absolute containment-checked path. Assign each exit code explicitly.

15. **MEDIUM — MECHANICAL-GRAIN: `MissionHolderProbe` is absent from the authoritative capability inventory.**

    The call-site paragraph adds and constructs it at [design:310](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:310), but the supposedly complete capability list at [design:328](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:328) omits it. Current `ValidateMission` still directly reads fixture command and process-group facts ([mission.go:97](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/mission.go:97)).

    Add `MissionHolderProbe` to that inventory, name its root-checked factory and sole `ValidateMission` recipient, and require fake-positive plus non-fake/unreadable-configuration refusal tests.

Evidence level: checked by reading the draft and cited code. No behavior tests were run and no files were modified.

Proposed review receipt:

```text
type=review outcome=reworked skills=design-critique verify=static-code-grounded-r6-fold corrections=15 stop_loss=no note="DRAFT r6 leaves eight structural and seven mechanical-grain material defects; the final prose budget is exhausted, so the design must split or take a human-ratified exit; no files changed"
```
