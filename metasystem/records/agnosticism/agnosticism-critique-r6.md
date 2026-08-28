REVISE — structural findings remain

Eleven material findings remain: ten structural and one mechanical-grain. This exhausts the second three-round budget; the loop must now escalate or take a human-ratified split/exit, not proceed to r7.

### Fold verification

| r5 finding | r6 result |
| --- | --- |
| 1. Fixture authorization | Open — Finding 1 |
| 2. Registration row | Open — Finding 2 |
| 3. Duplicate destinations | Open — Finding 3 |
| 4. Collision roots | Partial — current behavior pinned, future-runtime path still open in Finding 4 |
| 5. Installer escape | Open — Finding 6 |
| 6. Config-identity filter | Resolved |
| 7. Checkout configuration | Partial — default scaffold fixed, seam incomplete in Finding 7 |
| 8. Enforcement enum | Enum resolved; generic transport missing in Finding 8 |
| 9. Framing/idempotency | Open — Finding 10 |

### Findings

1. **HIGH — STRUCTURAL: `FixtureAuthorization` has neither executable transport nor complete consumer coverage.**

   R6 says the shell constructs a Go authorization value before fixture-capable reads ([ruling:483](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:483), [ruling:487](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:487)). In reality, arming calls rootless `proc alive` ([arm-supervision.sh:125](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/arm-supervision.sh:125), [arm-supervision.sh:359](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/arm-supervision.sh:359)); that verb accepts only PID and start time ([census.go:81](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/census.go:81)), and the reader remains rootless ([fixture.go:25](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/identity/fixture.go:25)). A shell process cannot pass an in-memory Go value across CLI invocations.

   The proposed consumer list also misses fixture-aware `census.Alive` decisions in arming verification ([verifyarmed.go:22](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/verifyarmed.go:22)), watchdog reporting ([watchdog.go:108](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/supervise/watchdog.go:108)), and mission preflight ([contract.go:1370](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/contract/contract.go:1370)).

   The ruling should name a cycle-safe, config-owning factory; require fixture-capable CLI verbs to reconstruct authorization from a canonical `--root`; thread it through every `FixtureEntryFor`, `census.Alive`, and `identity.Custodian` path; and pin unauthorized, absent-fixture, and unreadable-root outcomes. `identity` itself cannot read configuration under the declared layering ([architecture.md:36](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:36)).

2. **HIGH — STRUCTURAL: the tagged registration row still has contradictory requiredness and artifact ownership.**

   Every row carries `required|optional` ([ruling:143](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:143)), while `skill-profiles` separately carries `template-required/adopted-optional` ([ruling:150](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:150)). Those are materially different current states: template profiles are mandatory ([validate-metasystem.sh:525](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:525)); adopted profiles are required only when their source exists ([validate-metasystem.sh:580](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:580)). R6 also makes `shippedEnforcementConfig` an artifact-role reference ([ruling:157](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:157)) but later calls it a filename again ([ruling:195](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:195)).

   The ruling should define one contextual requiredness enum; make `(runtime, artifactRole)` unique; reject dangling or wrong-operation references; and remove the filename form entirely.

3. **HIGH — STRUCTURAL: union/dedup can discard artifact identity or weaken requiredness.**

   Codex and Devin intentionally share `.agents/skills` ([adopt.sh:323](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:323), [adopt.sh:334](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:334)). R6 compares operation, source, payload, and mode ([ruling:159](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:159)), but omits artifact IDs, requiredness, and which logical aliases survive execution deduplication.

   The ruling should plan by expanded concrete destination, preserve every `(runtime,id)` alias, define whether requirements must match or merge to the stricter state, and execute each compatible output once. Codex-plus-Devin must be tested together in both link and copy modes; the existing multi-runtime copy fixture covers Claude-plus-Codex instead ([adopt-fixtures.sh:413](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:413)).

4. **HIGH — STRUCTURAL: literal collision roots make a future runtime require another core edit.**

   R6 pins only `.claude`, `.devin`, and `.agents` ([ruling:163](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:163)). That preserves today’s scan ([adopt.sh:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:129)) and correctly excludes today’s later `.codex` write ([adopt.sh:335](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:335)), but it leaves no declaration point for a future runtime’s instruction root.

   The ruling should place contributed collision roots in the runtime registry, allow shared duplicates such as `.agents`, and scan the deduplicated full registry population. Current declarations must reproduce the exact three-root set, with Codex deliberately contributing no `.codex`.

5. **HIGH — MECHANICAL-GRAIN: `tree` does not pin per-skill installation grain.**

   The row says only `{source, mode}` ([ruling:146](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:146)). Current adoption expands `skills/*` and links or copies each child separately ([adopt.sh:293](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:293), [adopt.sh:302](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:302)); fixtures require child symlinks and exact targets ([adopt-fixtures.sh:237](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:237), [adopt-fixtures.sh:280](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:280)). Whole-tree linking would change later skill discovery.

   The ruling should bind `tree` acceptance to the existing exact-entry and symlink-target fixtures, with sources resolved from the post-`--enable` staged tree and destinations from the final target.

6. **HIGH — STRUCTURAL: the typed installer table remains ownerless and uncallable.**

   R6 names an “installation layer” table ([ruling:178](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:178)), but the sanctioned seams omit such an owner ([ruling:50](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:50)), the runtime verb list has no installer verb ([ruling:233](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:233)), and Class 3 instead sends behavior to adapter scripts ([ruling:303](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:303)).

   The ruling must name the sanctioned package/seam, typed request and result, registration key and conformance join, exact generic invocation, source/target/mode arguments, mutation phase, and failure contract.

7. **HIGH — STRUCTURAL: the checkout-configuration seam covers values but not runtime-bearing key positions.**

   Class 15 sanctions runtime-name “VALUES” ([ruling:553](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:553)), while it itself generates `role.default.model.<runtime>` keys ([ruling:557](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:557)). Live configuration also carries runtimes in model-key suffixes, capability-floor key segments, and model-tier prefixes ([validate.go:130](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/config/validate.go:130), [validate.go:177](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/config/validate.go:177), [validate.go:219](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/config/validate.go:219)). Moreover, checkout configuration remains absent from the global seam list, stated sweep, and doctrine ([ruling:50](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:50), [ruling:80](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:80), [ruling:562](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:562)).

   The ruling should sanction only schema-defined runtime positions—both values and key segments—require registry validation for each, and add checkout configuration consistently to the seam list, sweep, and doctrine.

8. **HIGH — STRUCTURAL: the corrected enforcement map has no generic adapter transport.**

   The enum now correctly matches the live schema ([ruling:219](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:219)), but the runtime verb list exposes no registry enforcement-map verb ([ruling:233](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:233)). Actual declarations remain inline in adapter probes, and validation uses named source greps ([validate-metasystem.sh:488](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:488), [validate-metasystem.sh:502](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:502)). A future runtime therefore still needs a core validation edit.

   The ruling should define canonical JSON output for the registry expectation and a side-effect-free adapter declaration verb reused by probe production; validation then compares both generically for every declared adapter.

9. **HIGH — STRUCTURAL: generic byte-drift validation would forbid valid live hook customization.**

   R6 says installed-enforcement paths and byte-drift validation derive generically from registration rows ([ruling:176](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:176), [ruling:290](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:290)). Today strict byte drift applies to copied skills and profiles ([validate-metasystem.sh:559](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:559)); live hook validation instead requires the shipped lifecycle-hook subset while allowing extra hooks ([hooks.go:36](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/hooks/hooks.go:36)).

   The ruling should define validation per operation. Preserve exact drift checks for copied skills/profiles and exact symlink targets, but preserve structural/subset hook validation unless the human separately ratifies exact live-config equality.

10. **MEDIUM — STRUCTURAL: framing and same-SHA recognition remain under-specified.**

   “Versioned header,” “fixed field order,” and “flattened payloads” do not state the literal header, ordered columns, unused-field representation, or per-tag arity ([ruling:171](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:171)). There is no existing wire format from which an implementer can infer those bytes.

   “No-op regardless of newly requested options” ([ruling:168](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:168)) also overlooks that runtime syntax, unknown names, duplicates, and illegal `none` combinations currently refuse before same-SHA recognition ([adopt.sh:57](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:57), [adopt.sh:72](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:72)); optional-skill validation occurs after the early exit ([adopt.sh:213](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:213)).

   The ruling should give the literal wire bytes and pin recognition order: syntax and runtime-list validity first; then healthy same-SHA no-op without comparing valid runtime/copy/skill options; incomplete same-SHA refusal; optional-skill validation remains after recognition.

11. **MEDIUM — STRUCTURAL: dispatch help still publishes a core-owned runtime universe.**

   [dispatch.sh:8](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:8) advertises `--runtime claude|codex|devin|fake`. Class 3 does not classify that site ([ruling:271](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:271)), and the CLI-name exception applies only to `cmd/metasystem`, not this shell help.

   The ruling should add this surface to Class 3 and render `--runtime <name>` with a registry-list reference, or derive the displayed choices from `metasystem runtime list`. A future-runtime help fixture must pass without editing `dispatch.sh`.

Evidence was checked by static code reading and exhaustive runtime-name/path sweeps. No behavior tests were run, no concrete import cycle was found, and no files were modified. The installer owner is too unspecified to validate its future import direction.

Because this is the final round of the second budget, the required next action is human escalation or the already-described ratified split—not another prose critique round.

Proposed review-only receipt:

`type=design|outcome=reworked|skills=design-critique|verify=static-runtime-sweep-and-call-path-review|corrections=11|stop_loss=yes|note=round-6 exhausted the second critique budget with structural agnosticism defects remaining`
