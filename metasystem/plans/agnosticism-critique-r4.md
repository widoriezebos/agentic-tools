REVISE — seven material findings remain. Round 4 fully resolves three of the nine Round 3 findings; one is textually resolved but creates a new security inconsistency, and five remain incomplete. Evidence: code inspection, exhaustive runtime-name/path sweeps, and a successful Go import-graph query. No behavior tests were run and no files were modified.

## Fold verification

| Round 3 finding | Round 4 result |
| --- | --- |
| R3-1, field-bound permission residuals | Original gap resolved, but deriving enforcement rows from residuals is invalid against current Codex behavior. Finding 1. |
| R3-2, seam-local behavioral registration | Partial. The table has no typed owner and permits a central explicit wire-up. Finding 4. |
| R3-3, registration/install contract | Not resolved. Parallel declarations, stale vocabulary, unspecified framing, and an untyped transform remain. Finding 3. |
| R3-4, `none` sentinel | Resolved. |
| R3-5, hook consumers/path base/environment grammar | Resolved. |
| R3-6, measured recovery and outward mapping | Resolved. |
| R3-7, full fake identity-fixture path | Not resolved. The selected arm-time gate is not a gate on fixture use, and one direct authority consumer remains omitted. Finding 2. |
| R3-8, fixed validation/security rows | Partial. Config-filter validation still has no declaration, and the residual-derived enforcement rule is unsound. Findings 1 and 5. |
| R3-9, `--devin-checks` replacement | Partial. `--probe` does not express all behavior and evidence controlled by the old switch. Finding 6. |

## Findings

1. **HIGH — NEW: `permissionResiduals` cannot be the derived source of expected envelope enforcement.**

   Round 4 says residuals are field-bound waiver identifiers at [plans/agnosticism-audit-rulings.md:138](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:138), then equates them with every expected `notEnforced` declaration at [plans/agnosticism-audit-rulings.md:211](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:211).

   Current Codex disproves that equivalence: its snapshot reports no unverified permission fields at [scripts/agents/adapters/codex.sh:77](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/codex.sh:77) while declaring `readRoots:"notEnforced"` at [scripts/agents/adapters/codex.sh:78](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/codex.sh:78). Live selection gates only fields in `permissions.unverified` at [internal/capability/select.go:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:129).

   Consequently, either:

   - Codex gains a read-root residual and selection starts requiring a new role waiver, changing live security decisions; or
   - the residual is not declared, and the derived validation view falsely expects Codex to report `mapped`.

   The ruling should declare a separate complete `expectedEnvelopeEnforcement[field]` map for static adapter validation. `permissionResiduals` should remain only the field-bound namespace used when live evidence lists a field as unverified. If `notEnforced ⇒ unverified` is desired, that is an explicit security-policy change requiring adjudication of Codex and the role waivers, not a preserving derivation.

2. **HIGH — STANDING R3-7: the arm-time fence neither precedes nor governs shared identity-fixture use.**

   Round 4 assigns ownership to the existing arm-time fence and promises fixture identity is refused outside armed-fixture mode at [plans/agnosticism-audit-rulings.md:382](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:382). In reality, `FixtureEntryFor` trusts the environment variable without a root, runtime, or armed-state input at [internal/identity/fixture.go:30](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/fixture.go:30).

   The supposed fence exists only inside the reserved-cap scan at [internal/supervise/reservedcap.go:38](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reservedcap.go:38). During actual arming, the script writes the announcement and invokes lease authorization first at [scripts/agents/arm-supervision.sh:378](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/arm-supervision.sh:378); the fence is reached later at [scripts/agents/arm-supervision.sh:390](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/arm-supervision.sh:390). It also cannot protect independently invoked lease, census, or reaper decisions.

   The claimed full enumeration still omits the direct census-liveness consumer at [internal/census/run.go:453](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/run.go:453).

   The ruling should require an explicit, root-checked fixture capability at every authority consumer—or make the central reader require such a checked capability. The arming check must occur before announcement, lease authorization, locks, or other mutation, but must not be the sole gate. Tests must exercise census authentication, census supervision liveness, lease classification, and custodian verdicts directly in a non-fake checkout.

3. **HIGH — STANDING R3-3 plus NEW exception: the registration contract still has multiple owners and no executable transform contract.**

   Round 4 says directories are derived from registration rows at [plans/agnosticism-audit-rulings.md:110](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:110), but separately declares skills/agents directories at [plans/agnosticism-audit-rulings.md:118](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:118). Class 3 also reverts to the obsolete `{kind, copy|link}` schema at [plans/agnosticism-audit-rulings.md:203](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:203), while the registry section declares the new operation vocabulary.

   The shell interface still chooses between NUL and tab framing at [plans/agnosticism-audit-rulings.md:146](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:146). More importantly, `json-transform` carries no transform identifier or parameters, although current behavior specifically strips `_comment` at [scripts/adopt.sh:317](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:317). Listing every future escape in doctrine at [plans/agnosticism-audit-rulings.md:215](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:215) also creates a third “adding a runtime touches core” exception.

   The ruling should retain one canonical row schema and delete the separate directory declaration and stale Class 3 schema. It should pin one framing, row expansion, shared-destination/idempotency semantics, and collision-root population. The transform must either be a typed generic operation such as `json-strip-key {key}`, or a seam-local installer capability registered like other behavior. New handlers must not require doctrine edits.

   The framing and shared-destination portions are mechanical-grain refinements that adoption fixtures can arbitrate once the schema is selected.

4. **MEDIUM — NEW: the behavioral capability table has no type owner and permits a core-edited wire-up list.**

   Round 4 proposes one neutral table for delivery recollection, usage recovery, and self-test probes, and permits either package initialization or “an explicit seam wire-up call” at [plans/agnosticism-audit-rulings.md:76](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:76).

   These capabilities do not share a contract. Delivery uses host-owned `HostCollectParams` and `HostCollectVerdict` at [internal/host/hostcollect.go:30](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/host/hostcollect.go:30), consumed by a package that already imports `host` at [internal/missionrunner/turnio.go:8](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/turnio.go:8). Self-test probes operate on adapter-owned state such as [internal/adapter/selftestrun.go:34](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:34). A single table therefore requires type erasure, moving all contracts into a new lower package, or imports back into registering packages. An explicit central wire-up also makes adding a runtime edit shared core.

   The ruling should define one typed table per behavioral owner: recollectors in `host`, recoverers in `usage`, and probes in `adapter`. Registration must reject duplicate `(runtime, capability)` keys, expose read-only lookup/list views for conformance, and require no central runtime enumeration.

5. **HIGH — STANDING R3-8 / MISSED SCOPE: config-identity filters and other declaration-addressed runtime assets are outside the registry and sanctioned seams.**

   The registry has only generic adapter/host flags at [plans/agnosticism-audit-rulings.md:91](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:91). Yet configuration identity constructs a runtime-specific filter path at [scripts/agents/adapters/runtime-common.sh:413](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:413), and validation hardcodes the three existing filter assets at [scripts/validate-metasystem.sh:373](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:373). `has adapter` cannot derive this requirement because the fake adapter exists without a filter.

   The sanctioned shell seam covers only `scripts/agents/adapters/*.sh` at [plans/agnosticism-audit-rulings.md:41](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:41), excluding those JSON files. Per-skill runtime profiles are likewise production inputs copied by adoption at [scripts/adopt.sh:314](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:314) and hardcoded in template validation at [scripts/validate-metasystem.sh:525](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:525), but `skills/` and `optional-skills/` are absent from the sweep and seam list.

   The ruling should add an optional `configIdentityFilter` path and sanction every runtime-owned asset addressed by a declaration, including config filters and skill-profile templates. Their existence and template/adopted validation must derive from those declarations.

6. **MEDIUM — STANDING R3-9: the proposed probe represents only the marker, not the behavior or evidence controlled by `--devin-checks`.**

   Round 4 specifies a probe name, path, and marker at [plans/agnosticism-audit-rulings.md:407](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:407). Today the switch controls four distinct stages:

   - Scratch-repository construction at [internal/adapter/selftestrun.go:193](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:193).
   - Prompt augmentation at [internal/adapter/selftestrun.go:270](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:270).
   - Returned-evidence validation at [internal/adapter/selftestrun.go:343](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:343).
   - Pass-record construction at [internal/adapter/selftestrun.go:354](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:354), including both `documented-exit-status-observation` and `symlinked-skill-discovery` at [internal/adapter/selftest.go:123](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftest.go:123).

   A path and marker cannot reproduce that contract, while a generic caller-selected `--probe` can request a probe not declared for its runtime.

   The ruling should define a typed probe lifecycle: prepare scratch state, contribute prompt text, verify evidence, and return the exact behavior labels earned. The runner should select the runtime’s declared probes or reject any cross-runtime/undeclared name. Preservation tests should pin the current Devin pass-record fields. This is principally a mechanical-grain refinement suitable for focused implementation tests.

7. **HIGH — MISSED SCOPE: shipped operational documentation still requires core edits for every new runtime.**

   The sweep is limited to `cmd/`, `internal/`, and `scripts/` at [plans/agnosticism-audit-rulings.md:58](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:58), while the doctrine allows no operational-documentation exception at [plans/agnosticism-audit-rulings.md:416](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:416).

   Shipped core guidance contains exhaustive runtime contracts:

   - Adoption’s specification and manual fallback enumerate runtimes and installation layouts at [docs/project-adaptation.md:5](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/project-adaptation.md:5) and [docs/project-adaptation.md:10](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/project-adaptation.md:10).
   - The orchestration manual has a fixed runtime mechanics table at [docs/orchestration.md:226](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/orchestration.md:226).
   - The canonical glossary enumerates the runtime universe at [docs/glossary.md:182](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/glossary.md:182).
   - The README claims the docs are runtime-neutral while enumerating three profile formats at [README.md:169](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/README.md:169).

   Adding a runtime therefore still requires edits outside the two allowed exceptions, or ships a false manual.

   The ruling should include shipped operational documentation in the scope. Exhaustive runtime lists and installation instructions should be replaced by the registry’s runtime/registration views and generic manual-repair procedure; runtime-specific operational detail belongs in seam-owned help or declared assets. Non-exhaustive examples may remain only where they do not claim the supported universe or prescribe the installation contract.

Proposed review-only receipt:

`type=design|outcome=reworked|skills=design-critique|verify=static-rg-and-go-import-graph|corrections=7|stop_loss=no|note=read-only round-4 agnosticism critique found seven material defects; no files changed`

**REVISE**
