Draft r3 does not converge. Static inspection found 14 material defects: 5 STRUCTURAL and 9 MECHANICAL-GRAIN. Six Round 2 findings are fully resolved; nine are only partially resolved. No files were modified and no behavior tests were run.

## Fold verification

| R2 finding | Fold result | Evidence |
|---|---|---|
| 1 — fixture authority | **Partial** | r3 adds the missed consumers at [design:209–237](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:209), but omits fixture publication and the census process-table source; Finding 1. |
| 2 — bootstrap sequence | **Resolved** | The ratified no-compilation same-SHA path is explicit at [design:165–189](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:165), matching the current recognition boundary at [adopt.sh:90–127](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:90). |
| 3 — permanent installer lifecycle | **Resolved at the ownership level** | `internal/install`, self-registration, the bidirectional join, and Prepare/Apply/Validate are present at [design:72–88](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:72). The new lifecycle itself remains defective; Findings 2 and 11. |
| 4 — collision transport/exception | **Partial** | The verb and `.codex/hooks.json` marker exist at [design:117–131](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:117), but the canonical schema cannot classify or transport the exception; Finding 3. |
| 5 — adapter validation population | **Partial** | r3 replaces `--with-adapter` with an unrelated static-map filter at [design:272–279](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:272); Findings 4 and 8. |
| 6 — overlap compatibility | **Resolved** | Policy, handler, operation, mode, source, and payload equality are required at [design:106–116](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:106), covering the live shared destination at [adopt.sh:323–339](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:323). |
| 7 — live-hook drift policy | **Resolved** | Template structural validation and adopted presence-only validation are separated at [design:89–102](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:89), matching [validate-metasystem.sh:565–622](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:565) and [validate-metasystem.sh:1387–1394](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:1387). |
| 8 — supervision-hook precedence | **Partial** | Registry/missing-engine precedence is fixed at [design:280–292](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:280), but runtime syntax and the missing-arm-script path are dropped; Finding 7. |
| 9 — adoption default/help/population | **Partial** | Generic lookup is specified at [design:294–300](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:294), but it would validate `none` against an adoptable list that deliberately excludes it; Finding 9. |
| 10 — computed tree target | **Partial** | The relative formula is added at [design:53–60](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:53), but its source is later bound under the temporary staged root; Finding 6. |
| 11 — `{skill}` grammar | **Partial** | The placeholder and ordering are pinned at [design:63–71](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:63), but valid zero matches contradict template-requiredness; Finding 10. |
| 12 — installer invocation | **Partial** | Roots, phases, exits, and no rollback appear at [design:148–164](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:148), but global mutation order, expansion grain, and Validate transport remain undefined; Findings 2 and 11. |
| 13 — adapter enforcement-map verb | **Resolved** | The literal adapter verb and outcomes are pinned at [design:250–262](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:250). |
| 14 — staged query build | **Partial** | Most build bytes are pinned at [design:183–189](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:183), but the temporary output cannot be cleaned as written; Finding 12. |
| 15 — root instruction-file seam | **Resolved** | The sanctioned seam amendment is explicit at [design:308–311](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:308). |

## Material findings

1. **HIGH — STRUCTURAL: fixture authorization is still neither complete nor purpose-safe.**

   r3 exposes one probe containing `identity`, `command`, `groupOwnership`, `ancestor`, and `missionProcess` at [design:209–237](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:209). Giving a consumer that broad interface also gives it `groupOwnership`, whose result authorizes real signals at [proc.go:73–99](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/proc.go:73) and [host.go:91–106](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/host.go:91). No method authorizes fixture publication at [proc.go:102–129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/proc.go:102).

   The “complete” source list also omits `METASYSTEM_CENSUS_PROCESS_FILE`, selected independently at [census.go:41–44](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/census.go:41) and [supervise_watchercfg.go:45–52](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/supervise_watchercfg.go:45), then parsed and authorized at [run.go:355–372](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/run.go:355).

   The design should define consumer-local, one-purpose capabilities—including publication and process enumeration—and map every consumer to exactly one capability. Alternatively, it must explicitly exclude the already-root-guarded census table from `fixtureauth`’s claimed ownership.

2. **HIGH — STRUCTURAL: the prepare/apply/no-rollback lifecycle is specified at the wrong grain.**

   r3 orders row prepares before row applies and says only “earlier rows” survive failure at [design:148–164](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:148). Current adoption writes the entire core payload at [adopt.sh:247–278](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:247) before runtime registration begins at [adopt.sh:293](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:293). An implementer can therefore legally leave most of the metasystem installed when a supposedly pre-mutation handler Prepare refuses.

   A tree or profile row also expands into zero or many concrete destinations at [design:53–71](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:53), while one row invocation emits one destination. Failure after the third child is neither an “earlier row” nor atomic stage-and-rename behavior.

   The design should construct one canonical concrete-output plan, prepare every output before the first target write, and define the apply order between core payload and runtime outputs. No rollback must cover prior outputs within the same expanded row, temporary-residue cleanup, and the manual recovery path after an incomplete same-SHA installation.

3. **HIGH — STRUCTURAL: the uncovered-destination exception lacks a total collision-classification schema.**

   Canonical rows do not say whether built-in outputs are instruction-bearing at [design:42–45](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:42); only installer handlers declare that fact at [design:76–80](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:76). Nevertheless, declaration validation must classify every expanded destination at [design:124–131](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:124). Neither the collision class nor `uncoveredDestinationException` exists in the ten-column schema at [design:138–147](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:138), despite built-in operations writing live instructions at [adopt.sh:317–339](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:317).

   The design should define a total built-in-operation-to-collision-class mapping or carry an explicit collision class per row. It should define the exception as a concrete enum or boolean field, reserve `-` exclusively for absent wire fields, and permit the sole current exception only when the expanded destination equals `.codex/hooks.json`.

4. **HIGH — STRUCTURAL: a static enforcement map does not imply the common adapter source shape.**

   r3 selects the `adapter_common_init` and snapshot-writer source assertions using static-map presence at [design:272–279](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:272). The registry defines `ExpectedEnvelopeEnforcement` only as a snapshot expectation at [runtimes.go:79–83](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:79); common lifecycle plumbing is a different responsibility at [runtime-common.sh:3–13](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:3). A valid standalone adapter can have a static map, and a common-lifecycle adapter can be profile-driven.

   The design should delete source-text shape assertions in favor of behavioral conformance, or declare the exact common-lifecycle capability separately. Static-map presence should select only enforcement-map comparison.

5. **HIGH — STRUCTURAL: signature conformance still requires a shared runtime list and provider-specific branch.**

   r3 says every hardcoded shell population becomes registry-derived at [design:264–279](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:264), but the security-relevant S4-7 fixture still loops over four names and special-cases fake’s positive vector at [supervision-fixtures.sh:325–343](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-fixtures.sh:325). Production compiles and consumes every configured adapter’s signature at [run.go:595–620](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/run.go:595). A future runtime’s valid positive process name cannot be inferred from its registry name; fake already disproves that premise.

   The design should make positive and lookalike signature vectors provider-owned—through declaration data or a side-effect-free adapter self-test—and iterate `runtime list --with-adapter`, with no shared runtime branch.

6. **HIGH — MECHANICAL-GRAIN: the computed tree target is bound to the temporary source instead of the installed source.**

   r3 computes the link relative to the “expanded source child” at [design:53–60](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:53), then says the source expression binds under the staged root at [design:148–154](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:148). A literal implementation therefore targets the temporary staging directory, which is deleted. Current links instead target the payload already copied under the target root at [adopt.sh:293–299](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:293).

   The design should distinguish the staged read source `S/source/child` from the live link target `R/source/child`, require the latter to be installed before link creation, and compute the relative target from the destination parent to the live target-root path.

7. **HIGH — MECHANICAL-GRAIN: the pinned hook precedence drops two current preconditions.**

   r3 checks only event syntax before engine resolution at [design:280–292](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:280). Current code refuses an invalid runtime argument before executable lookup at [supervision-hook.sh:4–7](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:4), and benign fallback requires both the engine and `arm-supervision.sh` to be executable at [supervision-hook.sh:25–26](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:25). With the engine present but the arm script absent, r3 can run Stop decisions or surface arming failure where today it exits 0.

   The design should shell-validate event syntax and the exact registry name grammar first, resolve both required executables next, preserve exit 0 if either is absent, and only then perform registry membership and session-environment lookup.

8. **MEDIUM — MECHANICAL-GRAIN: all-adapter decoded snapshot validation has no offline transport.**

   r3 requires decoded identity/schema assertions for every adapter at [design:272–279](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:272). Real `probe` commands require installed and authenticated providers—for example [codex.sh:51–79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/codex.sh:51) and [devin.sh:63–89](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/devin.sh:63)—while the offline suite explicitly limits real adapters to static inspection at [validate-metasystem.sh:409–410](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:409).

   The design should pin a no-auth, no-provider, side-effect-free adapter contract verb for every adapter, with canonical framing for runtime identity and snapshot schema facts, or withdraw the decoded-all-adapters assertion.

9. **MEDIUM — MECHANICAL-GRAIN: the ratified no-op leaves runtime grammar and `none` behavior contradictory.**

   The early no-op now depends entirely on an unspecified shell “name-shape check” at [design:165–175](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:165); the current cited shell code is a closed-name union, not a grammar, at [adopt.sh:72–79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:72). The actual registry grammar is `^[a-z][a-z0-9-]{0,31}$` at [runtimes.go:102–107](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:102).

   Separately, r3 says every explicit value is validated against `--adoptable` at [design:294–300](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:294), but `none` is deliberately not a runtime at [runtimes.go:194–211](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:194) and its empty-selection behavior is live at [adopt-fixtures.sh:387–395](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:387).

   The design should pin the exact name regex, treat `none` as a separate shell sentinel that bypasses membership/adoptability checks, and defer omitted-default lookup until the mutation-path query binary exists.

10. **MEDIUM — MECHANICAL-GRAIN: valid zero matches defeats template-required profile validation.**

   r3 says the profile row is template-required at [design:46–50](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:46), but later makes zero pattern matches valid at [design:63–71](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:63). Expanding only from existing profile files means deleting every required profile produces no rows and therefore no failure. Current template validation directly requires those profiles at [validate-metasystem.sh:531–561](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:531).

   The design should enumerate staged skill directories independently, substitute `{skill}` exactly once into source and destination per skill, and then apply source requiredness. Zero rows should be valid only when there are no staged skills or the source is template-optional.

11. **MEDIUM — MECHANICAL-GRAIN: installer-owned Validate has no executable invocation.**

   Handlers must implement `Validate` at [design:76–80](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:76), but the only command permits `--phase prepare|apply` at [design:148–160](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:148). Saying drift validation “walks the rows” at [design:190–192](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:190) does not tell the shell how to reach handler validation.

   The design should add a pinned validate phase with root binding, framing, and exit semantics, or name one separate Go-owned aggregate validator command that invokes every selected handler’s `Validate`.

12. **MEDIUM — MECHANICAL-GRAIN: the staged build’s temporary pathname cannot be cleaned as written.**

   r3 literally builds to `-o "$(mktemp)"` while promising a cleanup trap at [design:183–189](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:183). Command substitution discards the pathname, so the trap cannot remove the query binary. The existing build binds one staging path and uses the same variable for output and cleanup at [go-build.sh:32–40](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-build.sh:32).

   The design should bind a temporary directory and a named child binary, use that variable for `-o` and execution, and remove the directory through the trap.

13. **MEDIUM — MECHANICAL-GRAIN: closed-universe tests still make a new runtime edit shared core tests.**

   The runtime CLI test pins exact full and adoptable populations at [runtime_verbs_test.go:32–35](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_verbs_test.go:32). Registry tests also pin the exact universe, exact hook population, and exact instruction-file set at [runtimes_test.go:43–95](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes_test.go:43). A valid new declaration fails these tests before any behavior is exercised.

   The design should require population assertions to derive from declarations or registry overrides, while retaining only relational current policies such as one default, priority ordering, and fake’s named guards.

14. **MEDIUM — MECHANICAL-GRAIN: Class 14 still omits the destructive reconciliation inventory.**

   r3 names project adaptation, orchestration, glossary, and README as its complete documentation rewrite at [design:312–320](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:312). The reconciliation manual instructs humans to inventory a static minimum containing current runtime files and directories at [metasystem-reconciliation.md:14–21](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/metasystem-reconciliation.md:14). A new runtime with another instruction filename or registration root leaves that destructive cutover inventory incomplete unless shared prose is edited.

   The design should derive the runtime-owned part of Phase 0 from instruction-file and registration/collision views, while retaining fixed examples of foreign, unregistered assets.

Because this is the third focused round, any successor critique must enumerate Findings 1–14 as its opening worklist under the next three-round budget.

Proposed review receipt: `type=review outcome=reworked skills=design-critique verify=static-code-grounded-r3-fold corrections=14 stop_loss=no note="DRAFT r3 leaves five structural and nine mechanical-grain material defects; no files changed"`

REVISE — structural findings remain
