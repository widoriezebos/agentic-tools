REVISE — Round 5 has nine material findings: six structural and three mechanical-grain. The per-owner capability tables, typed probe lifecycle, and documentation scope are materially resolved; fixture authorization, registration/assets, and configuration ownership are not.

## Fold verification

| Round 4 finding | Round 5 result |
| --- | --- |
| R4-1, enforcement versus residuals | Core separation resolved; new enum defect remains in Finding 8. |
| R4-2, fixture authority gate | Not resolved. Finding 1. |
| R4-3, registration contract | Partial: derived directories, source-fresh build, and the stale Class-3 schema are fixed. Findings 2–5 and 9 remain. |
| R4-4, behavioral table ownership | Resolved. `go list` confirmed the proposed owner-local import directions do not introduce a current cycle. |
| R4-5, config filters and skill profiles | Partial. Findings 2 and 6. |
| R4-6, typed probe lifecycle | Resolved. All four existing stages and both earned labels are covered. |
| R4-7, operational documentation | Resolved. The class now covers shipped exhaustive claims and installation instructions. |

## Findings

1. **HIGH — STANDING R4-2 — STRUCTURAL: “armed fixture mode” is not a viable or defined authorization contract.**

   R5 permits fixture reads only when the checkout is in “armed fixture mode” at [agnosticism-audit-rulings.md:429](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:429). In repository terminology, “armed” means supervision is already running after announcement, lease work, component launch, and a healthy census at [glossary.md:74](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/glossary.md:74). Fixture-backed liveness is needed earlier, at [arm-supervision.sh:359](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/arm-supervision.sh:359).

   The promised root-bearing choke point also does not exist: `FixtureEntryFor` accepts only a PID at [fixture.go:25](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/fixture.go:25), while census authentication, lease identity, census liveness, and custodian liveness all call through rootless interfaces at [verbs.go:28](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/verbs.go:28), [identity.go:36](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/lease/identity.go:36), [run.go:453](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/run.go:453), and [custodian.go:24](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/custodian.go:24). Making foundational `identity` read configuration itself would also violate the layering recorded at [architecture.md:36](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:36).

   The ruling should define a pre-arming, root-bound fixture authorization with the exact predicate—presumably preserving today’s exact `metasystem.runtimes=fake` rule at [reservedcap.go:38](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/reservedcap.go:38)—and thread that checked capability through every authority consumer. The first check must precede the fixture-capable read at arm-supervision line 361. Tests need both positive fake-checkout bootstrap and the four non-fake refusals.

2. **HIGH — STANDING R4-3/R4-5 — STRUCTURAL: the “canonical” registration row is neither executable nor single-owner.**

   The five-field row at [agnosticism-audit-rulings.md:123](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:123) has nowhere to carry `json-strip-key`’s `key`, even though current behavior passes `_comment` at [adopt.sh:317](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:317). It also cannot specify Claude’s `<skill>.md` profile projection, Devin’s `<skill>/AGENT.md` projection, or Codex’s in-place `agents/openai.yaml` consumption; those distinct behaviors are visible at [adopt.sh:308](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:308) and [orchestration.md:226](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/orchestration.md:226).

   Requiredness is context-dependent: built-in template profiles are mandatory at [validate-metasystem.sh:525](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:525), while profiles for adopted or project-added skills are optional unless their source exists at [validate-metasystem.sh:580](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:580). Finally, `shippedEnforcementConfig` at [agnosticism-audit-rulings.md:155](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:155) repeats the enforcement source already required in the registration row.

   The ruling should define a tagged union with operation-specific payloads, including the JSON key and a typed profile contract containing source pattern, `copy|in-place` delivery, destination pattern, and template-required/adopted-optional semantics. Rows need stable IDs or artifact roles so `shippedEnforcementConfig`, installed paths, validation, and drift checks derive from the same row instead of repeating a path.

3. **HIGH — NEW IN R5 — MECHANICAL-GRAIN: rejecting duplicate destinations breaks compatible Codex-plus-Devin adoption.**

   R5 declares every duplicate destination erroneous at [agnosticism-audit-rulings.md:136](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:136). Devin and Codex intentionally share `.agents/skills` at [adopt.sh:323](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:323) and [adopt.sh:334](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:334); validation likewise treats the destination as shared at [validate-metasystem.sh:601](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:601).

   The ruling should compute the union of selected rows before mutation, deduplicate compatible overlaps with identical operation, source, parameters, mode, and output, and reject only incompatible overlaps. Codex-plus-Devin fixtures in both link and copy modes can arbitrate this mechanically.

4. **HIGH — STANDING R4-3 — STRUCTURAL/SECURITY-BOUNDARY: collision roots remain unspecified and cannot safely be derived.**

   R5 calls each collision root “runtime-owned” at [agnosticism-audit-rulings.md:134](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:134), although `.agents` is shared. Current fresh-target detection checks `.claude`, `.devin`, and `.agents` regardless of selected runtimes at [adopt.sh:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:129), but does not check `.codex`, which Codex later creates and writes at [adopt.sh:334](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:334).

   Querying only selected runtimes would weaken foreign-instruction detection; deriving roots from destinations would newly reject existing `.codex` directories. The ruling should pin the exact current collision-root values and require a deduplicated full-population pre-mutation scan. Adding `.codex` must be called out as a human-adjudicated behavior/security change, not preservation.

5. **HIGH — STANDING R4-3 — STRUCTURAL: the installer escape still creates a forbidden third core edit.**

   R5 sends unrepresentable installation behavior to “the adapter’s typed table” at [agnosticism-audit-rulings.md:131](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:131), but the only defined adapter table owns self-test probes at [agnosticism-audit-rulings.md:88](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:88). Class 3 then requires every escape to be listed in doctrine at [agnosticism-audit-rulings.md:259](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:259), contradicting both “never a doctrine edit” and the two declared exceptions.

   The ruling should either make the row vocabulary complete or define a separate typed installer owner with duplicate rejection, lookup/list conformance, and one generic shell invocation. New handlers must require only registry plus seam edits; doctrine must not enumerate uses.

6. **HIGH — STANDING R4-5 — STRUCTURAL: `configIdentityFilter` is validation-only, not the live identity source.**

   R5 declares the path at [agnosticism-audit-rulings.md:109](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:109), but the runtime-verb list at [agnosticism-audit-rulings.md:190](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:190) exposes no filter lookup. Live identity still independently constructs `$runtime-config-filter.v1.json` at [runtime-common.sh:413](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:413), and that identity controls snapshot selection at [dispatch.sh:485](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:485).

   The ruling should add `runtime config-identity-filter <runtime>` with explicit absent semantics and require live identity hashing to use exactly that declared path. A test must prove the validated bytes are the live hashed bytes.

7. **HIGH — NEW/MISSED SCOPE — STRUCTURAL: checkout configuration has no sanctioned class, and its shipped model scaffold still enumerates runtimes.**

   The sweep remains limited to `cmd/`, `internal/`, and `scripts/` at [agnosticism-audit-rulings.md:70](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:70). Yet `metasystem.conf` contains live runtime selections at [metasystem.conf:5](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/metasystem.conf:5) and one default-model row per real runtime at [metasystem.conf:30](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/metasystem.conf:30). Those live values legitimately drive dispatch at [roster.go:117](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/roster.go:117).

   For a future runtime, tailoring drops unselected model rows and expands only the special code-critic placeholder at [conftailor.go:125](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/validate/conftailor.go:125), while validation requires the selected runtime’s default model at [validate.go:275](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/config/validate.go:275). Adding a runtime therefore still requires editing the core template scaffold.

   The ruling should sanction runtime-name values in `metasystem.conf` as a narrow checkout-configuration seam: they express operator selection, are validated against the registry, and never define the supported universe. Replace per-runtime model boilerplate with generic tailoring that materializes `role.default.model.<runtime>=<model>` for every selected non-synthesized runtime.

8. **MEDIUM — NEW IN R5 — MECHANICAL-GRAIN: the enforcement map admits a value rejected by the live schema.**

   R5 permits `enforced | mapped | notEnforced` at [agnosticism-audit-rulings.md:179](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:179). Snapshot production and live selection accept exactly `mapped | notEnforced` at [snapshot.go:101](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/snapshot.go:101) and [select.go:260](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:260).

   The ruling should require exactly `{writeRoots, readRoots, network}` with values `mapped | notEnforced`, and a conformance fixture should pin registry and snapshot enums as identical.

9. **MEDIUM — STANDING R4-3 — MECHANICAL-GRAIN: framing and outer idempotency remain contradictory or underspecified.**

   Registration is pinned to tab-separated lines at [agnosticism-audit-rulings.md:138](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:138), but the verb contract still allows NUL or tab at [agnosticism-audit-rulings.md:193](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:193).

   R5’s “same rows, same bytes” idempotency also misses current top-level behavior: a healthy same-SHA installation exits successfully before comparing newly requested runtimes, copy mode, or optional skills at [adopt.sh:117](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:117), while an incomplete same-SHA installation refuses at [adopt.sh:125](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:125).

   The ruling should pin one versioned tab encoding, including field order, payload fields, zero-row output, and trailing newline. Separately pin current outer recognition: healthy same-SHA means no-op regardless of changed options; incomplete same-SHA refuses. Focused fixtures can arbitrate both.

Evidence: static code reading, exhaustive runtime-name/path sweeps, and a successful Go import-graph query. No behavior tests were run and no files were modified.

Proposed review-only receipt:

`type=design|outcome=reworked|skills=design-critique|verify=static-runtime-sweep-and-go-import-graph|corrections=9|stop_loss=no|note=read-only round-5 agnosticism critique found six structural and three mechanical-grain material defects`

**REVISE**
