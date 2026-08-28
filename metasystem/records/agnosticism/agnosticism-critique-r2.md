REVISE — seven material findings remain. Evidence: static inspection of the ruling, production code, expanded `rg` sweep, and Go import metadata. No tests were run and no files were modified.

## Fold verification

| Round-1 finding | Result |
| --- | --- |
| R1-1, incomplete sweep/host seam | **Partial.** Host launchers and scripts are now in scope, but adoption, instruction delivery, registration, and several fake branches remain uncovered by implementable rulings. |
| R1-2, tailoring precedence/synthesis | **Resolved.** Unique priority and synthesized-model declarations preserve both policies. |
| R1-3, resume import cycle | **Resolved.** Only recollection moves; runner validation and fallback remain in `missionrunner`. |
| R1-4, usage behavior in core | **Partial.** Runtime parsing moves out, but no single owner is selected for logic shared by adapter and host. |
| R1-5, false Claude baseline | **Resolved.** Claude recovery and field-preservation tests are required. |
| R1-6, insufficient recovery input/sink | **Partial.** The context and generic aggregation improve, but the recovery outcome and failure contract remain unspecified. |
| R1-7, live permission waivers | **Not safely resolved.** The move changes the owner of live security policy. |
| R1-8, instruction no-waiver boundary | **Resolved narrowly.** Conformance is covered, but other instruction-file consumers are not. |
| R1-9, omitted fake behavior | **Not resolved.** The exception list is incomplete. |
| R1-10, hookless runtimes | **Not resolved.** The registry still bundles two capabilities that the prose calls independent. |

## Findings

1. **HIGH — Moving permission waivers into the compiled registry removes the role-owned, live security control.**

   Capability selection currently reads the checkout’s role requirements on every dispatch at [internal/capability/select.go:81](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:81), obtains its waivers at [internal/capability/select.go:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:129), and refuses unless that live file grants the exact runtime and field at [internal/capability/select.go:132](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:132). The standing architecture assigns role behavior and capability needs to those files at [docs/orchestration.md:58](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/orchestration.md:58).

   R2 instead moves the matrix into `internal/runtimes` at [plans/agnosticism-audit-rulings.md:194](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:194). Removing a waiver from an adopted repository’s role file would then no longer revoke it; the compiled default would continue authorizing the dispatch. A golden of today’s decisions cannot detect that ownership regression.

   The ruling should keep role approval live and role-owned. Runtime declarations may expose unique permission-residual identifiers, but a role must explicitly waive those identifiers. A new under-enforced runtime fails closed until the human edits role policy; that security decision is an explicit exception to “one registry edit.”

2. **HIGH — Hook enforcement and live self-check remain conflated, so the proposed audit drops existing Codex and Devin coverage.**

   The registry defines one optional object containing enforcement config, live settings, and vendored marker, declares it only for Claude, then says those capabilities are independent at [plans/agnosticism-audit-rulings.md:64](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:64). They are not represented independently.

   Today the audit verifies shipped enforcement for Claude, Codex, and Devin at [internal/audit/metasystem.go:332](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/audit/metasystem.go:332), and Codex and Devin genuinely ship configs at [scripts/enforcement/codex-hooks.json:1](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/enforcement/codex-hooks.json:1) and [scripts/enforcement/devin-hooks.json:1](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/enforcement/devin-hooks.json:1). Only the live repository self-check is Claude-specific, as shown by [internal/hooks/hooks.go:16](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/hooks/hooks.go:16) and its project-directory assertion at [internal/hooks/hooks.go:62](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/hooks/hooks.go:62).

   The ruling should define two optional fields: `shippedEnforcementConfig` for all three current real runtimes, and `liveSelfCheck {settingsPath, vendoredMarker}` for Claude only. Audit iterates the first; `hooks check` requires the second.

3. **HIGH — A single `runtime list` cannot preserve the different runtime populations or adoption default.**

   The registry shape has no `adoptable` or adoption-default property at [plans/agnosticism-audit-rulings.md:56](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:56). Yet configuration supports all four runtimes, including fake, at [internal/config/validate.go:118](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/config/validate.go:118), while adoption accepts only Claude, Devin, and Codex at [scripts/adopt.sh:77](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:77). Adoption also defaults specifically to Claude at [scripts/adopt.sh:53](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:53), an outcome pinned by [scripts/adopt-fixtures.sh:245](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt-fixtures.sh:245).

   Consuming the proposed unqualified list either admits fake into adoption or leaves the Claude default open-coded. “Registered real runtimes” at ruling line 72 is also undefined, while a generic fixture classification is expressly refused.

   The ruling should add explicit `adoptable` and unique `adoptionDefault` declarations and purpose-filtered verbs such as `runtime list --adoptable` and `runtime adoption-default`. Runtime names and every emitted relative path also need validated shell-safe grammars.

4. **HIGH — The named fake-exception list omits multiple production authority and fault-injection branches.**

   R2 lists only mission host behavior, the synthetic ancestor, and supervision’s reserved-cap check at [plans/agnosticism-audit-rulings.md:205](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:205). The expanded sweep also finds:

   - Fake-only census process enumeration at [internal/census/run.go:355](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/run.go:355).
   - Mission lease identity substitution at [internal/dispatch/mission.go:100](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/mission.go:100).
   - Delegate-writable mirror fault injection at [internal/dispatch/mirror.go:44](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/mirror.go:44).
   - Fake supervisor and process-group ownership fallbacks at [scripts/agents/dispatch.sh:238](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:238) and [scripts/agents/dispatch.sh:265](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/dispatch.sh:265).

   These materially alter liveness, signaling authority, evidence mirroring, or process identity. The ruling should enumerate every one in Class 10 and the doctrine, preserving a local named-fake guard at each security boundary. The refusal of a generic `IsFixture` bypass should remain.

5. **HIGH — Registry-declared instruction filenames still do not reach every production consumer.**

   R2 feeds new filenames only to the audit allowlist and conformance protection at [plans/agnosticism-audit-rulings.md:177](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:177). Today `CLAUDE.md` separately appears in the audit’s outside-reference scan roots at [internal/audit/metasystem.go:29](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/audit/metasystem.go:29), its instruction inventory at [internal/audit/metasystem.go:107](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/audit/metasystem.go:107), adoption’s collision detection at [scripts/adopt.sh:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:129), and the payload allowlist at [scripts/adopt.sh:166](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:166).

   A newly declared instruction file can therefore be protected from waivers yet fail to ship, evade adoption conflict detection, or escape audit scanning.

   The ruling should make the declared instruction-file set feed inventory, outside-reference scanning, conformance protection, adoption collision detection, and payload inclusion. Registry-declared instruction files themselves must also be sanctioned seam entries.

6. **HIGH — The shell verb and registration schema cannot express the behavior being moved.**

   The declared shell API contains only `list`, `dirs`, `enforcement-config`, and `instruction-file` at [plans/agnosticism-audit-rulings.md:51](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:51), but Class 3 additionally needs session-environment names and an installation operation at [plans/agnosticism-audit-rulings.md:110](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:110). Neither has a verb contract.

   A flat directory list also cannot preserve the current optional profile mappings and link/copy rules in [scripts/adopt.sh:311](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:311), or their byte-drift validation at [scripts/validate-metasystem.sh:591](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:591). An implementer would have to leave those branches in core or silently weaken validation.

   The ruling should either declare a structured registration contract—source pattern, destination pattern, kind, required/optional, copy/link semantics—or define explicit seam-owned `install` and `verify-registration` operations. It must also specify the session-environment query, output encoding, validation, exit codes, and pre-mutation adoption checks.

7. **HIGH — Recovery has no outcome, failure, provenance, or single-owner contract.**

   R2 specifies only a loose context and typed-usage return at [plans/agnosticism-audit-rulings.md:151](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:151). Current recovery deliberately:

   - Runs only after group and custodian death are proved at [internal/mission/fence.go:746](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/mission/fence.go:746).
   - Skips malformed JSONL lines and can still recover an earlier valid usage block at [internal/usage/usage.go:236](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/usage.go:236).
   - Converts unusable evidence to per-round `unavailable`, with exact source/detail provenance, rather than failing the whole aggregate at [internal/mission/fence.go:794](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/mission/fence.go:794).

   A seam recoverer that returns an error for a truncated final line would instead trigger the aggregate-wide failure path, leaving the previous mission aggregate standing as described at [internal/missionrunner/usageprojection.go:13](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/usageprojection.go:13). Killed streams are exactly where truncation is expected.

   The ruling should define a read-only `RecoveryOutcome` with `recovered`, `unavailable`, and `unsupported` states; typed usage; exact source paths; and detail. Malformed provider evidence must normalize to unavailable or be skipped according to today’s parser semantics, not become an aggregate error. Group-death gates remain in `mission`. It should also name one seam owner for parsing shared by adapter and host, preserving the single-owner purpose documented at [internal/usage/usage.go:1](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/usage/usage.go:1).

Proposed receipt: `type=design|outcome=reworked|skills=design-critique|verify=skipped|corrections=7|stop_loss=no|note=read-only round-2 agnosticism critique found seven material defects; no files changed`

**REVISE**
