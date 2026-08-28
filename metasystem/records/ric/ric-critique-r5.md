DRAFT r5 does not converge. Static code-grounded review found 9 material defects: 5 STRUCTURAL and 4 MECHANICAL-GRAIN. The trajectory is now 14/15/14/10/9, with structural findings 12/11/5/5/5. No files were modified and no behavior tests were run.

## Fold verification

| R4 finding | Result |
|---|---|
| 1 — fixture capabilities | **Resolved at the capability split**, including all four requested values and refusal tests at [design:291–312](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:291). One existing consumer is still not assigned to a capability; Finding 8. |
| 2 — global installation owner/recovery | **Not resolved.** `internal/install` is named, but recovery contradicts the bootstrap sequence and its persisted record cannot support the promised resume/removal procedure; Findings 1–3. |
| 3 — Codex in-place profile | **Resolved at classification level** at [design:78–85](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:78). The reference still conflicts with the general output planner; Finding 5. |
| 4 — template hook policy | **Resolved at ownership level** at [design:112–118](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:112). Removing the old filename field also removes the executable source lookup; Finding 6. |
| 5 — collision metadata ownership/equality | **Resolved semantically** at [design:123–150](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:123). Its cycle-safe projection owner is missing; Finding 4. |
| 6 — final stamped engine | **Partial.** It is restored as a core output, but the stated command cannot produce or receive it; Finding 3. |
| 7 — signature-vector transport | **Resolved.** The verb, JSON frame, newline, and exits are pinned at [design:345–354](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:345). |
| 8 — common-lifecycle transport | **Resolved.** The filtered view, ordering, exclusion, and no-combination rule are pinned at [design:362–368](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:362). |
| 9 — shared snapshot schema | **Resolved at declaration level.** Writer, selector, and contract are tied to one constant, but rollout would invalidate every existing live snapshot and the shell has no transport to that constant; Finding 7. |
| 10 — Validate source/taxonomy | **Resolved at command-output level** at [design:210–218](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:210). Mode and typed handler outcomes remain unrepresentable; Finding 9. |

## Material findings

1. **HIGH — STRUCTURAL: the incomplete-installation record cannot implement the promised resume or recovery path.**

   R5 promises exact-plan resumption from `{sha, selection, mode, planDigest}` and says mismatch recovery removes the record’s “listed installed outputs” at [design:194–198](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:194), but that record contains no output list, application journal, generated values, or preimages. The bootstrap later still says an incomplete same-SHA target refuses at [design:227–234](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:227), directly contradicting resume.

   Recomputing the “exact” plan is insufficient: adoption generates a new timestamp in `plans/goals.md` on each attempt at [adopt.sh:205–212](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:205). Blind removal is also unsafe because adoption merges user-owned `.gitignore` and `.gitattributes` contents at [adopt.sh:256–262](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:256).

   The design should name the record path and persist a canonical plan manifest, generated values, per-output status, and pre-mutation state for merge outputs. Recognition must route an exact match into resume, not refusal. Mismatch recovery must restore recorded preimages or require reconciliation; it must never delete every destination generically.

2. **HIGH — STRUCTURAL: the supposedly total plan omits existing adoption mutations.**

   The plan names only the payload allowlist and final engine as core outputs at [design:184–203](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:184), while claiming every output is prepared before mutation and the completion marker lands last. Current adoption additionally creates the goal baseline and artifact state at [adopt.sh:280–291](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:280), installs the runtime-neutral workflow at [adopt.sh:346–348](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:346), and may write a pre-commit hook into the Git common directory at [adopt.sh:364–369](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:364). That hook may be outside `R`, contradicting the target-containment rule at [design:178–183](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:178).

   The design should enumerate every mutation: payload files and merge operations, generated goal baseline, artifact directory state, runtime-neutral workflow, final engine, completion marker, and Git hook. The external Git-hook write needs its own prepared/collision-checked boundary before completion, or an explicit ratified behavior change.

3. **HIGH — STRUCTURAL: `internal/install` has no executable transport for the final engine artifact.**

   The global command receives only `R`, `S`, and the runtime list at [design:185–193](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:185). The current build script derives its own checkout root, derives the stamp from that root’s Git metadata, and writes only to that root’s `bin/metasystem` at [go-build.sh:10–42](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-build.sh:10). `S` comes from `git archive` and has no repository metadata at [adopt.sh:139–155](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:139). Having the Go installer execute `go-build.sh` would also add a new exception to the rule that verbs do not invoke scripts at [architecture.md:89–93](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:89).

   The design should make the shell prepare a named final-engine file through a refactored `go-build.sh --output PATH --stamp SHA`, then pass that immutable file into the plan—or explicitly sanction and specify a Go-owned build executor. The installer should treat the resulting bytes as a prepared core source, not guess where a build occurred.

4. **HIGH — STRUCTURAL: handler-derived collision metadata lacks a cycle-safe resolved-registration owner.**

   Canonical rows live in the registry at [design:39–45](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:39), handlers self-register in `internal/install` at [design:86–97](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:86), and the registration encoder must consume handler-derived collision metadata at [design:143–150](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:143). Meanwhile `internal/install` must consume registry rows to construct plans. If `internal/runtimes` resolves the handler metadata literally, the imports cycle; today it is explicitly a pure-data dependency leaf at [runtimes.go:1–8](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:1).

   The design should make `internal/install` own a resolved-registration projection: it imports raw runtime rows, joins handler metadata, runs collision-root proof, and encodes `registration/v1`. `internal/runtimes` must never import `internal/install`; the CLI should delegate to the resolved projection.

5. **MEDIUM — MECHANICAL-GRAIN: the reference-only in-place row is not excluded from the general output-overlap algorithm.**

   R5 correctly says `in-place` has no Apply and no runtime collision output at [design:78–85](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:78). It later says every alias participates in concrete-destination compatibility and every built-in row has collision classification at [design:123–151](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:123). A Codex reference and its core payload output cannot have identical operation, mode, policy, or collision metadata, so a literal planner refuses the overlap. Current code confirms there is only one writer: payload copying installs the profile at [adopt.sh:247–255](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:247), while the Codex arm copies no profiles at [adopt.sh:334–339](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:334).

   The design should define an `in-place` expansion as a `ReferenceRequirement`, never a `ConcreteOutput`. It must resolve to exactly one planned core destination, participate only in requiredness/validation, and be excluded from output compatibility and collision fields/proofs.

6. **MEDIUM — MECHANICAL-GRAIN: `LiveSelfCheck` loses its shipped-hook source transport.**

   R5 removes `shippedEnforcementConfig` after rows land at [design:120–122](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:120), but the live template check currently obtains the shipped source through exactly that command at [validate-metasystem.sh:1387–1393](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:1387). `hooks check` still requires both live and shipped paths as positional arguments at [hooks.go:17–37](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/hooks.go:17).

   The design should either make `LiveSelfCheck` reference an enforcement artifact role and have `hooks check` resolve that row’s source through the resolved-registration projection, or pin a replacement `runtime row-source <runtime> <role>` transport and its exact invocation.

7. **HIGH — STRUCTURAL: adding the snapshot discriminator is an unplanned live-security cutover.**

   R5 requires the selector to reject missing schema versions at [design:371–378](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:371). Every snapshot written by current code lacks that field at [snapshot.go:67–78](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/adapter/snapshot.go:67), and the selector currently accepts those files after runtime/version checks at [select.go:208–224](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/capability/select.go:208). Landing the new selector therefore invalidates all retained snapshots and refuses dispatch until every real provider is authenticated and reprobed.

   Shell adapters also cannot directly “read” a Go constant; their only bridge is the engine path initialized at [runtime-common.sh:6–13](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/runtime-common.sh:6).

   The design should specify a rollout for existing snapshots—validated one-time migration, an explicitly bounded legacy-schema acceptance, or choosing r4’s deterministic-construction alternative. It must also pin a CLI transport such as `runtime snapshot-schema`, backed by the same leaf constant used by writer and selector, which each adapter’s `contract` verb calls.

8. **MEDIUM — MECHANICAL-GRAIN: `dispatch.ValidateMission` is enumerated but assigned to none of the seven capabilities.**

   R5 names the direct fixture reader at [design:276–277](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:276), but the capability-to-recipient list at [design:291–309](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:291) does not place it under `IdentityProbe`, `CommandProbe`, or another value. The real path uses fixture command and process-group facts to authorize joining a mission at [mission.go:62–73](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/mission.go:62) and [mission.go:97–124](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/dispatch/mission.go:97).

   The design should explicitly assign this call site to a least-authority capability—either a dedicated mission-holder probe or a narrowly defined `IdentityProbe` operation exposing command plus process group—and name its factory construction inside `ValidateMission`.

9. **MEDIUM — MECHANICAL-GRAIN: install mode and Validate’s typed outcome remain absent from the executable contract.**

   Tree installation is user-selectable link or copy at [design:53–64](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:53), and the incomplete record stores `mode`, but `install plan|prepare|apply` has no `--mode` at [design:185–196](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:185). Current adoption receives that decision through `--copy-skills` at [adopt.sh:57–62](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:57).

   The scalar policy list also assigns different judgments to copied trees and links at [design:103–119](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:103), while handler `Validate` has no typed return contract capable of distinguishing drift from the operational errors mapped to exit 4 at [design:210–218](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:210).

   The design should add `--mode link|copy` to every global planning/apply command, persist it in completed installation state, and define tree validation by installed mode. Handler validation should return a typed `clean|drift` result plus a separate error channel, mapped deterministically to exits 0/1/4.

Proposed review receipt: `type=review outcome=reworked skills=design-critique verify=static-code-grounded-r5-fold corrections=9 stop_loss=no note="DRAFT r5 resolves two r4 findings completely but leaves five structural and four mechanical-grain material defects; no files changed"`

REVISE — structural findings remain
