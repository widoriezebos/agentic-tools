DRAFT r1 does not close the prior critique. Static inspection found 14 material defects: 12 structural and 2 mechanical-grain. No files were modified and no behavior tests were run.

### R6 fold verification

| R6 finding | Result |
|---|---|
| 1 — fixture authorization | Partial; Findings 4, 5, 14 |
| 2 — registration row | Partial; Finding 1 |
| 3 — destination deduplication | Partial; Finding 1 |
| 4 — collision roots | Narrowly resolved; Finding 8 is a new coverage defect |
| 5 — per-skill installation grain | Partial; Finding 11 |
| 6 — installer escape | Partial; Findings 2–3 |
| 8 — enforcement transport | Partial; Finding 6 |
| 9 — drift validation | Partial; Finding 12 |
| 10 — framing and recognition | Partial; Findings 3 and 10 |

### Findings

1. **HIGH — STRUCTURAL: contextual requiredness and destination ownership remain unrepresentable.**

   The design makes requiredness one mutually exclusive enum and requires `(runtime, artifactRole)` uniqueness ([design:35](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:35), [design:40](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:40)). But the same profile row is mandatory in a template ([validate-metasystem.sh:531](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:531)) and source-conditioned after adoption ([validate-metasystem.sh:594](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:594), [validate-metasystem.sh:600](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:600), [validate-metasystem.sh:614](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:614)). Consequently `template-required` and `adopted-optional` must coexist; they cannot be alternatives or participate in an undefined “stricter” merge.

   Every row also already owns `destination`, while `skill-profiles` introduces another `destination pattern` ([design:21](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:21), [design:32](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:32)). Live adoption computes one destination per profile ([adopt.sh:314](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:314), [adopt.sh:327](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:327)); `optional-profile-copy` has no separate live behavior.

   **The design should say instead:** requiredness is a context-indexed product, at minimum `{templateSource, adoptedDestination}`, with an explicit componentwise join. Each row has one destination expression, scalar or patterned. Remove the redundant profile-copy arm and the second destination field.

2. **HIGH — STRUCTURAL: the installer escape is still a closed-union extension point.**

   The only registration operations are centrally enumerated ([design:24](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:24)), while the installer table is keyed by `(runtime, operation)` ([design:54](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:54)). A genuinely new operation therefore still requires edits to the shared union, registration encoder/parser, planner, and validator—the core-touch the escape is meant to eliminate.

   **The design should say instead:** reserve a permanent `installer` union arm carrying a stable handler identifier and complete common request data. New implementations add a registry row and seam handler without adding another central tag. If novel payload shapes necessarily require shared types, declare that as an explicit exception instead of promising registry-plus-seam only.

3. **MEDIUM — MECHANICAL-GRAIN: neither the installer invocation nor `registration/v1` has executable bytes.**

   The supposedly exact command ends in `...` ([design:59](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:59)). There are no literal flags for source, destination, mode, or phase; no phase enum; and no exit, stdout, handler-error, or partial-mutation contract. The framing says “payload fields in the operation’s declared order” and “`-` for unused” without defining a global column set or per-tag arity ([design:62](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:62)).

   **The design should say instead:** pin the complete command grammar, phase values, path binding, exit codes, output framing, idempotency, and partial-failure behavior. Define one exact column list and the occupied/unused columns and total arity for every tag.

4. **HIGH — STRUCTURAL: fixture authorization omits live identity and authority decisions.**

   `dispatch.ValidateMission` directly reads fixture identity after its own local configuration check ([mission.go:97](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/mission.go:97), [mission.go:115](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/mission.go:115)); it is absent from the design’s consumer list and blast radius. Mission preflight has a second, unrelated fixture reader, `METASYSTEM_MISSION_PROCESS_IDENTITY_FILE` ([contract.go:1375](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/contract/contract.go:1375), [contract.go:1387](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/contract/contract.go:1387)). Ancestor inference trusts `METASYSTEM_FAKE_AGENT_ANCESTOR_PID` before any configuration authorization ([ancestor_production.go:54](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/ancestor_production.go:54)). None is protected by making `FixtureEntryFor` refuse.

   This is not merely liveness classification: fixture-backed `Live` feeds lease takeover authorization ([claim.go:73](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/claim.go:73)).

   **The design should say instead:** enumerate every fixture source and every downstream authority decision, including dispatch mission validation, mission-process fallback, synthetic ancestor inference, lease claim/renewal, and holder authorization. Either unify these sources behind the root-bound authorization or explicitly eliminate them. Add fake-positive and non-fake/unreadable-configuration refusal tests for each decision, including takeover.

5. **HIGH — STRUCTURAL: the `fixtureauth` package direction is incompatible with the declared layering.**

   The design simultaneously keeps `identity` configuration-free, makes `fixtureauth` construct the authorization from configuration, and requires the identity reader to validate a fixtureauth-owned value ([design:86](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:86), [design:97](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:97)). But `identity` is a foundation package with no metasystem imports ([architecture.md:79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:79)). Importing `fixtureauth` from `identity` points upward; exposing a freely constructible boolean/interface authorization makes the boundary forgeable.

   **The design should say instead:** choose the package dependency explicitly. A cycle-safe shape is for `fixtureauth` to own fixture parsing and root/config authorization above `identity`, while fixture-aware identity decisions inject a neutral probe into the foundation. No public raw fixture reader or caller-constructible “allowed” token may bypass that owner.

6. **HIGH — STRUCTURAL: enforcement-map population is impossible for `fake`.**

   The design compares every adapter-declaring runtime ([design:107](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:107)). Fake declares `HasAdapter: true` ([runtimes.go:157](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:157)) but deliberately has no static expected map because its result is profile-driven ([runtimes.go:79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:79), [runtimes_test.go:130](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes_test.go:130)). Its emitted network enforcement changes by profile ([fake.go:229](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/fake.go:229), [fake.go:257](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/fake.go:257)).

   **The design should say instead:** compare only runtimes explicitly declaring a static enforcement map, with pinned absent-map semantics, while retaining fake’s profile-driven behavioral test. Require both sides to emit canonical JSON, or decode and canonicalize both before semantic comparison.

7. **HIGH — STRUCTURAL: the generic validation population required by the split has no owner or interface.**

   The live suite hardcodes enforcement assets and configuration filters ([validate-metasystem.sh:306](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:306), [validate-metasystem.sh:379](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:379)), host syntax checks ([validate-metasystem.sh:433](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:433)), adapter rows ([validate-metasystem.sh:479](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:479)), and host rows ([validate-metasystem.sh:511](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:511)). The registry has independent `HasAdapter` and `HasHostLauncher` flags ([runtimes.go:42](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:42)), but shell-facing enumeration supports only all runtimes or adoptable runtimes ([runtime_verbs.go:28](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_verbs.go:28)). A blast-radius mention is not a derivation contract.

   **The design should say instead:** provide purpose-filtered registry views for adapter, host, required asset, syntax-check, and registration-row populations, or move these checks into one Go-owned generic validator. Explicitly replace each hardcoded shell population.

8. **HIGH — STRUCTURAL: instruction-bearing collision coverage has two security holes.**

   The phase-A registry already declares each runtime’s root instruction filename ([runtimes.go:62](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:62)), but adoption still hardcodes `CLAUDE.md` in collision detection ([adopt.sh:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:129)) and payload inclusion ([adopt.sh:158](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:158), [adopt.sh:166](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:166)). Contract 1 never carries those consumers, contrary to the split ruling ([rulings:451](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:451), [rulings:462](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:462)).

   Separately, rows and contributed collision roots are independent declarations ([design:44](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:44), [design:49](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:49)). A future runtime can write `.foo/skills/...` while forgetting to contribute `.foo`; all stated conformance still passes.

   **The design should say instead:** the deduplicated `InstructionFile` view augments both collision detection and payload inclusion. Declaration validation must also prove every instruction-bearing expanded destination lies beneath a contributed collision root, except for explicit human-adjudicated exclusions. The new-instruction-file fixture must span all five Class-8 consumers.

9. **HIGH — STRUCTURAL: `supervision-hook.sh` is in scope but has no generalization contract.**

   The script still rejects anything outside four named runtimes and uses cross-runtime Claude/Devin environment fallback ([supervision-hook.sh:4](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:4), [supervision-hook.sh:21](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:21)). The registry already owns `SessionEnv` ([runtimes.go:58](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:58)), but `runtime session-env` returns the same code for unknown runtime and absent optional capability ([runtime_verbs.go:10](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_verbs.go:10), [runtime_verbs.go:113](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_verbs.go:113)).

   **The design should say instead:** recognize the runtime separately through the registry; query its optional session environment; indirectly expand only that validated variable name; use payload `cwd`, then the declared nonempty environment value, then `PWD`; unknown runtimes refuse while a known runtime with no variable uses the fallback. Add a future-runtime fixture requiring no script edit.

10. **HIGH — STRUCTURAL: the recognition order contradicts its source-fresh query requirement and changes same-SHA behavior.**

   The design says registry validity refuses first but also says adoption rebuilds before any registry query ([design:68](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:68), [design:72](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:72)). Today invalid runtime syntax refuses before Go/preflight ([adopt.sh:57](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:57), [adopt.sh:90](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:90)); recognition additionally follows clean-source provenance ([adopt.sh:102](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:102)). A healthy same-SHA run exits without rebuilding ([adopt.sh:117](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:117)), whereas the shared build path can refuse behind a live gate ([go-build.sh:16](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-build.sh:16)). The proposed move therefore creates new no-op failures and changes error precedence.

   **The design should say instead:** pin one complete bootstrap and recognition sequence, including syntax, toolchain/preflight, source provenance, runtime lookup, build, recognition, and optional skills. Either use a source-fresh, non-overwriting registry query that preserves the existing no-op, or explicitly ratify and test the changed prerequisites and refusal precedence.

11. **MEDIUM — STRUCTURAL: the claimed symlink fixture does not arbitrate the proposed exact-target drift rule.**

   Installation currently creates `../../skills/<name>` links ([adopt.sh:293](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:293)), but the cited fixture asserts only that a link exists ([adopt-fixtures.sh:237](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:237)); its target comparison merely proves a rerun preserved the snapshot’s existing target ([adopt-fixtures.sh:282](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:282), [adopt-fixtures.sh:290](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:290)). Live validation accepts any non-dangling link ([validate-metasystem.sh:568](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:568), [validate-metasystem.sh:573](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:573)). Exact-target drift rejection is therefore a behavioral strengthening, not a preserving move.

   The draft also does not bind expansion to the post-`--enable` staged tree; the current optional-skill fixture proves only that the skill moved ([adopt-fixtures.sh:411](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:411)).

   **The design should say instead:** installer output uses the canonical relative target and resolves sources after optional-skill materialization, with direct fixtures for both. Preserve non-dangling-link validation unless exact-target enforcement is separately human-ratified as a hardening.

12. **HIGH — STRUCTURAL: installation operations do not determine valid drift semantics.**

   The draft names exact copied skill/profile bytes, exact symlink targets, and the hook subset exception, but assigns no policy to `copy-file`, `json-strip-key`, or individual artifact roles ([design:74](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:74)). Current hook validation deliberately allows live supersets while checking shipped lifecycle events and commands structurally ([hooks.go:40](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/hooks/hooks.go:40), [hooks.go:58](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/hooks/hooks.go:58)). Treating every copied or transformed file as exact would reject valid customization; skipping them leaves generic validation undefined.

   **The design should say instead:** every row carries or derives an explicit validation policy—exact bytes, transformed canonical bytes, exact link target, structural hook subset, or in-place source—and declaration validation rejects unsupported operation/policy combinations.

13. **MEDIUM — STRUCTURAL: the split’s operational-documentation contract was reduced to a weaker assertion.**

   The accepted ruling requires installation instructions to become registry pointers plus one generic repair procedure ([rulings:588](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:588), [rulings:595](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:595)). The draft carries only a no-exhaustive-universe assertion ([design:120](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:120)). Current core docs still prescribe runtime-specific layouts and hooks ([project-adaptation.md:10](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/project-adaptation.md:10), [project-adaptation.md:14](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/project-adaptation.md:14)), maintain a current-runtime mechanics table ([orchestration.md:226](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/orchestration.md:226)), and give a concrete runtime list and adoption command ([glossary.md:182](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/glossary.md:182), [README.md:217](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/README.md:217)).

   **The design should say instead:** carry Class 14 in full: replace prescribed runtime layouts with `runtime registration` and one generic repair procedure, move runtime-specific operational detail to seam-owned help/assets, and audit the known violating locations—not merely phrases claiming an “exhaustive universe.”

14. **MEDIUM — MECHANICAL-GRAIN: adding required `proc alive --root` leaves shipped callers broken.**

   `proc alive` currently accepts only PID and start time ([census.go:81](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/census.go:81)). The draft names only the arming caller, but rootless calls also remain in the fingerprint harness ([fingerprint-harness.sh:103](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/fingerprint-harness.sh:103)) and supervision fixtures ([supervision-fixtures.sh:23](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-fixtures.sh:23)).

   **The design should say instead:** enumerate and migrate every `proc alive` caller, pinning the canonical checkout root each passes, and include those harnesses in the interface-change verification.

Proposed review receipt: `type=review outcome=reworked skills=design-critique verify=caught corrections=0 stop_loss=no note="DRAFT r1 has 14 material findings; structural revision required"`.

REVISE — structural findings remain
