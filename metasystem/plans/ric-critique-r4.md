Draft r4 does not converge. Static code-grounded inspection found 10 material defects: 5 STRUCTURAL and 5 MECHANICAL-GRAIN. Eleven Round 3 findings are resolved; Findings 1, 2, and 5 remain partial. No files were modified and no behavior tests were run.

## Fold verification

| R3 finding | Result | Evidence |
|---|---|---|
| 1 — fixture authorization | **Partial** | Capability values are split at [design:260](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:260), but `ProcessProbe` still combines command lookup with signal-authorizing group ownership, and process-table enumeration has no capability; Finding 1. |
| 2 — concrete-output lifecycle | **Partial** | Concrete planning and core-first apply appear at [design:169](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:169), but the global planner has no executable owner and “refusal plus rerun” is not a recovery path; Finding 2. |
| 3 — collision schema | **Resolved as requested** | The twelve-column frame and total collision fields are present at [design:131](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:131) and [design:151](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:151). New contradictions remain; Findings 3 and 5. |
| 4 — common lifecycle selection | **Resolved at the declaration level** | `commonLifecycleAdapter` separates lifecycle shape from static-map presence at [design:313](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:313). Its shell transport is missing; Finding 8. |
| 5 — signature vectors | **Partial** | The vectors are declaration-owned and the fake branch is removed at [design:304](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:304), but no command transports them to the fixture; Finding 7. |
| 6 — live tree target | **Resolved** | The target is explicitly `R/source/child`, never the staged source, at [design:53](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:53). |
| 7 — hook preconditions | **Resolved** | Runtime grammar and both executable checks precede membership lookup at [design:328](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:328). |
| 8 — offline adapter contract | **Resolved at the transport surface** | The no-provider `contract` verb is specified at [design:321](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:321). Its advertised schema is not tied to live snapshots; Finding 9. |
| 9 — `none` and default resolution | **Resolved** | `none` bypass and deferred omitted-default lookup are explicit at [design:343](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:343). |
| 10 — per-skill requiredness | **Resolved** | Skills are enumerated independently before requiredness is evaluated at [design:70](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:70). |
| 11 — Validate invocation | **Resolved at the requested surface** | Aggregate command, framing, and primary exits exist at [design:183](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:183). Source binding and operational failures remain undefined; Finding 10. |
| 12 — staged-build cleanup | **Resolved** | One temporary directory owns output, execution, and cleanup at [design:209](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:209). |
| 13 — declaration-derived tests | **Resolved** | Only population expectations become declaration-derived; relational named policies remain at [design:365](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:365). |
| 14 — reconciliation inventory | **Resolved** | Phase 0 derives runtime-owned entries from instruction, registration, and collision views at [design:373](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:373). |

## Material findings

1. **HIGH — STRUCTURAL: the split fixture capabilities remain overbroad and process enumeration remains unauthored.**

   `ProcessProbe` combines command lookup and group ownership at [design:260–266](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:260). These are different authorities: fake command lookup participates in host-start verification at [host.go:301](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/host.go:301), while group ownership authorizes real `SIGTERM` and `SIGKILL` at [host.go:91](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/host.go:91). The census process table is merely said to use “the same authorization” at [design:268](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:268), despite two independent selectors at [census.go:41](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/census.go:41) and [supervise_watchercfg.go:45](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/supervise_watchercfg.go:45). The proposed tests at [design:279](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:279) omit command, signal ownership, publication, and enumeration refusals.

   The design should define separate `CommandProbe`, `GroupOwnershipGrant`, `PublicationGrant`, and `ProcessTableProbe` values; name every construction and recipient call site; and require fake-positive plus non-fake/unreadable-configuration tests for each authority, including proof that an unauthorized group is never signalled and an unauthorized fixture is never written.

2. **HIGH — STRUCTURAL: the global installation plan and its recovery state still have no executable owner.**

   Cross-runtime alias joining is global at [design:113](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:113), and the prepare barrier includes every runtime output plus the core payload at [design:169](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:169). The only command, however, accepts one runtime and one row at [design:162](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:162); it cannot own the selected-runtime union, core manifest, deduplication, or global ordering.

   Recovery is also not executable. The design says failure leaves outputs installed and recovery is “incomplete-same-SHA refusal plus rerun” at [design:179](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:179). Today the SHA marker is written before runtime outputs at [adopt.sh:291](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:291), and same-SHA recognition uses only the structural audit at [adopt.sh:117](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:117), whose required-file check does not inspect runtime registrations at [metasystem.go:73](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/audit/metasystem.go:73). A partial installation can therefore become either a permanent refusal or a false healthy no-op.

   The design should make `internal/install` own one immutable plan over the selected runtimes and explicit core outputs, exposed through a complete plan/prepare/apply interface. Before the first target write it should persist an incomplete record containing SHA, selection, mode, and plan digest; reruns must recognize and idempotently resume that exact plan. The completed adoption marker is written atomically last. If resumption is rejected, specify the exact manual recovery procedure instead of saying “rerun.”

3. **HIGH — STRUCTURAL: Codex’s in-place profile is classified as an output that declaration validation must reject.**

   Codex consumes `agents/openai.yaml` in place at [design:67](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:67), but the draft classifies every `skill-profiles` output as instruction-bearing at [design:131](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:131) and requires it to lie below `.claude`, `.devin`, or `.agents` at [design:124](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:124). `skills/{skill}/agents/openai.yaml` lies below none of them and is not the sole `.codex/hooks.json` exception.

   The real Codex arm performs no profile copy: the file arrives with the core payload at [adopt.sh:247](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:247), while Codex installs only the shared skill registration and hooks at [adopt.sh:334](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:334).

   The design should represent `in-place` as a validation/reference-only alias to an already installed core-payload output: it has requiredness and validation but no Apply operation, runtime collision output, or second writer. Core collision handling retains ownership of `skills/`.

4. **HIGH — STRUCTURAL: one scalar validation policy cannot preserve both template and adopted behavior.**

   Every row and every wire record carries one `policy` at [design:42](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:42) and [design:151](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:151). Yet the policy list requires `structural-hook-subset` in template context and `presence-only` in adopted context at [design:96](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:96), while aggregate Validate promises to use “the row’s policy” at [design:183](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:183).

   The template self-check is also not a normal row destination: it checks the parent repository’s live `.claude/settings.json` through explicit paths at [validate-metasystem.sh:1387](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:1387). Adopted configuration currently checks registration presence without live-hook drift at [validate.go:339](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/config/validate.go:339).

   The design should either keep template live-hook verification owned by `LiveSelfCheck`/`hooks check`, with the enforcement row supplying only its source reference and adopted policy, or make policy a context-indexed product with explicit template target binding and adopted-destination policy. Compatibility must compare both components.

5. **HIGH — STRUCTURAL: collision metadata has two owners and is omitted from overlap compatibility.**

   Installer handlers declare collision metadata at [design:83](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:83), while registry rows independently carry `collisionClass` and `uncoveredException` at [design:131](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:131). The bidirectional handler join does not require those facts to agree. Moreover, overlap compatibility at [design:113](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:113) compares operation, source, payload, mode, policy, and handler id—but not collision class or exception status.

   Two aliases can therefore deduplicate while one says `plain` and the other `instruction-bearing`, or while only one claims the uncovered exception. That changes whether the collision-root security proof runs.

   The design should appoint one owner. Prefer deriving installer collision metadata from the handler table and making the registration encoder consume that result. Otherwise require conformance equality. Compatible overlaps must also have identical `collisionClass` and `uncoveredException`; conflicting security metadata refuses.

6. **HIGH — MECHANICAL-GRAIN: the final stamped engine artifact disappears from the installation contract.**

   The only build specified by r4 is explicitly query-only, unstamped, and forbidden from replacing `bin/metasystem` at [design:208](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:208). Current adoption separately performs the production build and installs it into the target at [adopt.sh:268](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:268). That build pins `CGO_ENABLED=0`, disables VCS stamping, injects `BuildStamp`, stages, and atomically renames at [go-build.sh:25](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-build.sh:25).

   The design should retain a distinct final-engine output in the core plan, pin those build bytes and stamp, prepare it before target mutation, and atomically install it at `R/bin/metasystem`. The query binary remains temporary and is never shipped.

7. **MEDIUM — MECHANICAL-GRAIN: provider-owned signature vectors have no shell transport.**

   The fixture is told to consume declaration-owned vectors at [design:304](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:304), but no runtime verb or framing is named. The current shell-facing registry supports only list/lookups at [runtime_verbs.go:28](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_verbs.go:28), while `proc signature-check` requires literal positive and lookalike arguments at [census.go:94](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/census.go:94).

   The design should pin a command such as `runtime signature-vectors <runtime>`, preferably canonical JSON `{positive,lookalike}`, with field grammar, trailing-newline behavior, and exit codes 0/1/2. S4-7 then decodes that output for each `--with-adapter` runtime.

8. **MEDIUM — MECHANICAL-GRAIN: `commonLifecycleAdapter` has no purpose-filtered registry view.**

   The named views at [design:301](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:301) include adapters, hosts, collision roots, and row assets, but the later source assertions must iterate the new flag at [design:318](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:318). Current `runtime list` accepts only no flag or `--adoptable` at [runtime_verbs.go:28](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_verbs.go:28).

   The design should add and pin `runtime list --with-common-lifecycle`—including combination rules, ordering, newline framing, and fake’s exclusion—or expose the flag through one structured runtime descriptor command.

9. **MEDIUM — MECHANICAL-GRAIN: the offline contract advertises a snapshot schema that live snapshots do not carry.**

   The proposed output claims `snapshotSchema: v1` at [design:321](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:321). Production snapshots contain runtime, version, hashes, capabilities, permissions, and enforcement, but no schema version at [snapshot.go:67](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/snapshot.go:67). The production loader validates runtime and CLI version without checking any schema discriminator at [select.go:208](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:208). An adapter can therefore print `v1` from `contract` while its real probe writes unrelated bytes.

   The design should either add one shared snapshot-schema constant to production output and require the selector to reject missing/unknown versions, with `contract` reading that same constant, or make the offline verb exercise the actual snapshot construction path using deterministic dummy provider facts.

10. **MEDIUM — MECHANICAL-GRAIN: aggregate Validate still lacks source binding and a total failure taxonomy.**

   Prepare/apply bind row sources below staged root `S` at [design:162](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:162). `install validate` receives only `--root R` at [design:183](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:183), so exact-byte and transformed-byte handlers do not know whether their canonical source is `R/source`, a staged tree, or something else. Its exits cover only clean, drift, and usage, although the adjacent row command reserves exit 3 for unknown runtime, row, or handler at [design:178](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:178); handler I/O and invariant failures are also unclassified.

   The design should state that adopted validation resolves both canonical source and destination below `R`, or add an explicit source-root flag. Pin distinct handling for drift, usage, unknown declaration/handler, and operational failure, including whether stdout may contain partial `ok` lines when an unexpected failure occurs.

Proposed review receipt: `type=review outcome=reworked skills=design-critique verify=static-code-grounded-r4-fold corrections=10 stop_loss=no note="DRAFT r4 resolves eleven of fourteen r3 findings but leaves five structural and five mechanical-grain material defects; no files changed"`

REVISE — structural findings remain
