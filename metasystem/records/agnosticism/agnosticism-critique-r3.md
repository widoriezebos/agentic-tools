REVISE — nine material findings remain. Evidence: repository inspection, exhaustive `rg` sweeps, and successful Go import-graph inspection. No tests were run and no files were modified.

## Fold verification

| Round-2 finding | Result |
| --- | --- |
| R2-1, role-owned permission waivers | **Partial.** Role ownership is restored, but the residual identifier lacks a security-safe field mapping. Finding 1. |
| R2-2, independent hook capabilities | **Resolved as a field split.** New downstream and path-resolution defects remain. Finding 5. |
| R2-3, adoption population/default | **Resolved for runtimes.** The non-runtime `none` selection was omitted. Finding 4. |
| R2-4, incomplete fake list | **The four named r2 omissions were added, but the claimed full enumeration is still false.** Finding 7. |
| R2-5, instruction-file consumers | **Resolved.** All five named consumers are covered. |
| R2-6, registration contract | **Partial.** The new row shape still cannot reproduce current installation semantics. Finding 3. |
| R2-7, recovery outcome and owner | **Partial.** States and malformed-tail behavior are added, but discovery and accounting invariants remain open. Findings 2 and 6. |

## Findings

1. **HIGH — Permission-residual identifiers are not bound to the permission field they waive.**

   R3 declares only “unique names for enforcement gaps” and says selection matches an identifier generically at [plans/agnosticism-audit-rulings.md:264](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:264). Today the security decision is explicitly `(field, runtime)`: selection iterates each unverified field at [internal/capability/select.go:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:129), and `waived` looks inside that field’s role-owned list at [internal/capability/select.go:300](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:300).

   A flat residual list does not tell the selector which identifier corresponds to `readRoots`, `writeRoots`, or `network`, nor what to do when a snapshot reports an unverified field with no declared residual. An implementation can therefore accidentally let one identifier waive multiple boundaries or ignore an undeclared gap.

   The ruling should declare `permissionResiduals` as an exact `permission field → globally unique residual id` map. Role files should waive that identifier under the same field, and selection must fail closed when an unverified restrictive field has no matching declaration, when an identifier belongs to another field/runtime, or when identifiers are duplicated.

2. **HIGH — The pure-data registry cannot register the behavioral capabilities that Classes 5–7 require.**

   The registry is required to remain pure data and a leaf at [plans/agnosticism-audit-rulings.md:54](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:54), but runtime-specific delivery recollection is discovered “by lookup” at [plans/agnosticism-audit-rulings.md:171](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:171), and recovery is “declared per runtime” at [plans/agnosticism-audit-rulings.md:195](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:195). No registration mechanism connects those functions to runtime names.

   Today these are direct named calls at [internal/missionrunner/turnio.go:63](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/turnio.go:63) and [internal/mission/fence.go:786](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/mission/fence.go:786). Storing function values in the leaf registry violates its dependency rule; maintaining central switches or maps in shared host/usage files means adding a runtime still edits shared core.

   The ruling should define seam-local behavioral registration: each per-runtime host/usage seam file registers its own capability, while the pure-data registry declares whether that capability is expected. Conformance must join those sets. Core receives only neutral lookup results.

   It must also add the per-runtime `internal/usage` files to the sanctioned-seam list. “Everything else is core” at [plans/agnosticism-audit-rulings.md:40](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:40) contradicts their later sanction at [plans/agnosticism-audit-rulings.md:186](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:186). Go imports packages, not individual files, so the proposed prohibition on importing “per-runtime files” is not implementable.

3. **HIGH — The registration rows still cannot preserve installation, collision, and verification behavior.**

   R3 reduces registration to `{source pattern, destination, kind, required|optional, copy|link}` at [plans/agnosticism-audit-rulings.md:70](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:70). Current behavior includes:

   - User-selected link versus recursive copy at [scripts/adopt.sh:293](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:293).
   - A JSON transformation rather than copying Claude’s enforcement file at [scripts/adopt.sh:317](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:317).
   - Refusal on an existing runtime-owned directory, not merely an exact destination collision, at [scripts/adopt.sh:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:129).
   - Directory-presence validation at [internal/config/validate.go:339](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/config/validate.go:339).

   The separate `dirs`, registration, enforcement-config, and live-settings declarations also give the same paths multiple owners. They can diverge so adoption installs one layout while configuration or hook validation checks another. The unspecified “NUL- or tab-delimited” output at ruling line 94 is likewise not a pinned shell contract.

   The ruling should define explicit operation and verification types—such as selectable skill-tree link/copy, file copy, optional profile copy, and JSON transform—plus collision roots and exact output framing. Directory requirements and installed enforcement paths should be derived from registration rows rather than redeclared. Every escape operation must be enumerated; Claude’s JSON transform is the existing required case.

   R3 must also order a source-fresh binary build before these pre-mutation queries. Adoption currently resolves `$root/bin/metasystem` at [scripts/adopt.sh:50](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:50), invokes it before copying at [scripts/adopt.sh:225](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:225), but only rebuilds it after target mutation at [scripts/adopt.sh:268](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:268).

4. **HIGH — The adoptable/default design drops the supported `none` selection.**

   `none` does not appear anywhere in r3. Yet adoption accepts it, rejects mixed forms, and treats it specially at [scripts/adopt.sh:72](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:72). Configuration tailoring independently accepts the same sentinel at [cmd/metasystem/config_verbs.go:34](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/config_verbs.go:34) and turns it into an empty runtime set at [internal/validate/conftailor.go:29](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/conftailor.go:29).

   Because `none` is intentionally not a runtime, neither `runtime list --adoptable` nor `adoption-default` can supply it. Replacing the current validation with the declared runtime list will reject `--runtimes none`, contradicting the pinned behavior at [scripts/adopt-fixtures.sh:387](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:387).

   The ruling should retain `none` as a named non-runtime sentinel in adoption and `config tailor`, before registry validation, with its exclusivity and empty-roster behavior preserved.

5. **HIGH — The independent hook fields still leave hook verification with multiple hardcoded consumers and no path-base contract.**

   The two optional fields at [plans/agnosticism-audit-rulings.md:82](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:82) correctly separate shipped enforcement from live self-check. But shipped hook files remain independently hardcoded as required assets at [scripts/validate-metasystem.sh:315](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:315) and as the JSON-shape validator’s inputs at [scripts/validate-metasystem.sh:464](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:464). A future declaration can therefore pass the generalized audit while never receiving this structural verification.

   The live side is also underspecified. Today the command receives explicit live and shipped paths at [cmd/metasystem/hooks.go:12](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/hooks.go:12), while template validation deliberately finds live settings in the parent repository at [scripts/validate-metasystem.sh:1382](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:1382). A clean-relative `settingsPath` cannot express that without a defined project-root base.

   The ruling should make every shipped-config consumer iterate `shippedEnforcementConfig`, and define live-path resolution explicitly: either retain explicit `--live`/`--shipped` paths and use `--runtime` only for capability/marker selection, or require separate project-root and metasystem-root arguments. Pin both nested-template and adopted-root cases. Session-environment declarations also need an environment-variable-name grammar and indirect expansion without `eval`.

6. **HIGH — `RecoveryOutcome.recovered` can still silently produce no metered usage.**

   R3 defines the three states and typed usage at [plans/agnosticism-audit-rulings.md:197](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:197), but gives no invariant connecting `recovered` to an actually measured token, cost, or provider unit.

   The current parser can return a typed `"native"` object with every value null at [internal/usage/usage.go:171](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/usage.go:171). The fence deliberately counts fields and converts that result to unavailable when nothing measured at [internal/mission/fence.go:786](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/mission/fence.go:786). Losing that check can mark a job recovered while adding no units and omitting it from `unavailableJobs`.

   The ruling should make `recovered` valid only when the shared aggregator returns `measured=true`; otherwise it becomes `unavailable` with today’s source/detail. It must also define the outward mapping of `unsupported`, including exact provenance, source, and detail. Devin and generic no-recoverer cases need field-for-field preservation tests, not only Claude and Codex.

7. **HIGH — The “full” fake enumeration omits the shared identity-fixture authority path.**

   Class 10 claims a full enumeration at [plans/agnosticism-audit-rulings.md:280](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:280), but omits the central fake identity reader at [internal/identity/fixture.go:25](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/fixture.go:25). That reader can replace authentication identity at [internal/census/verbs.go:28](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/verbs.go:28), which feeds lease identity at [internal/lease/identity.go:36](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/identity.go:36), and can replace custodian liveness at [internal/identity/custodian.go:24](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/custodian.go:24).

   It also omits the mission-runner helper branches that accept fixture commands, group ownership, and publish fixture identity at [internal/missionrunner/proc.go:32](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/proc.go:32), [internal/missionrunner/proc.go:76](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/proc.go:76), and [internal/missionrunner/proc.go:102](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/proc.go:102).

   The ruling should enumerate the identity reader and every authority consumer, state whether the existing arm-time fence or a local checked capability is the owner, and pin that exact gate. A textual `== "fake"` sweep alone cannot find helper-hidden authority paths.

8. **HIGH — The expanded production sweep still misses fixed runtime validation and security rows.**

   R3 classifies only selected locations in `validate-metasystem.sh` at [plans/agnosticism-audit-rulings.md:135](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:135). The completion gate separately hardcodes:

   - Host and adapter/config-filter assets at [scripts/validate-metasystem.sh:368](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:368).
   - Per-host syntax checks at [scripts/validate-metasystem.sh:427](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:427).
   - Runtime-specific expected envelope-enforcement declarations at [scripts/validate-metasystem.sh:488](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:488).

   These are core validation decisions, not seam files or test data. The enforcement rows protect live permission claims, especially Devin’s known unenforced boundaries. Leaving them open-coded contradicts the doctrine and makes a future runtime either evade equivalent validation or require another core edit.

   The ruling should route required seam assets, syntax validation, optional config-filter assets, and expected enforcement declarations through registry fields. Otherwise the enforcement rows must become a third handwritten exception, which conflicts with the draft’s declared limit of two.

9. **MEDIUM — The generic self-test CLI still embeds a Devin-only behavioral switch.**

   The only sanctioned runtime names in `cmd/metasystem` are supposed to be thin seam verb names at [plans/agnosticism-audit-rulings.md:35](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:35). Instead, the otherwise generic self-test verbs expose `--devin-checks` at [cmd/metasystem/adapter_selftest_verbs.go:91](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/adapter_selftest_verbs.go:91) and [cmd/metasystem/adapter_selftest_verbs.go:146](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/adapter_selftest_verbs.go:146). It controls scratch registration and evidence requirements in shared self-test code at [internal/adapter/selftestrun.go:270](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:270) and [internal/adapter/selftestrun.go:343](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/selftestrun.go:343).

   The ruling should replace this with a capability-named probe—such as symlinked skill discovery with a declared path and marker—selected by the Devin adapter seam. Shared command and self-test code should not name the runtime.

Proposed receipt:

`type=design|outcome=reworked|skills=design-critique|verify=skipped|corrections=9|stop_loss=no|note=read-only round-3 agnosticism critique found nine material defects; no files changed`

**REVISE**
