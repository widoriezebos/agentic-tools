R2 does not converge. Static inspection found 15 material defects: 11 structural and 4 mechanical-grain. No files were modified and no behavior tests were run.

### Fold verification

| R1 finding | Result | Evidence |
|---|---|---|
| 1 — contextual requiredness/destination ownership | **Resolved** | The product, componentwise join, single destination, and alias-preserving deduplication are specified at [design:33](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:33), matching template and adopted behavior at [validate-metasystem.sh:531](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:531) and [validate-metasystem.sh:594](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:594). |
| 2 — closed-union installer escape | **Partial** | The permanent arm exists at [design:56](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:56), but validation and collision semantics remain closed and `internal/install` is not a sanctioned seam. Findings 3 and 4. |
| 3 — executable registration/install bytes | **Partial** | The ten-column frame is pinned at [design:95](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:95), but phase, path, pattern, and adapter-verb bytes remain incomplete. Findings 11–14. |
| 4 — incomplete fixture authority | **Partial** | New sources are included at [design:148](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:148), but mission-runner signal authority and purpose separation are missing. Finding 1. |
| 5 — fixtureauth layering | **Resolved** | The neutral interface remains in foundational `identity`, with `fixtureauth` above it at [design:138](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:138), consistent with [architecture.md:79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:79). |
| 6 — fake enforcement-map population | **Resolved substantively** | Static-map filtering and fake’s absent semantics now match [runtimes.go:79](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:79). The adapter interface itself remains mechanically incomplete. Finding 13. |
| 7 — validator population ownership | **Partial** | Purpose views are named at [design:191](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:191), but `--with-adapter` selects fake for assertions it cannot satisfy. Finding 5. |
| 8 — instruction/collision coverage | **Partial** | The five instruction-file consumers are covered at [design:81](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:81), but collision roots and their exception have no executable transport. Finding 4. |
| 9 — supervision-hook contract | **Partial** | Runtime/session-environment resolution is specified, but registry unavailability makes the stated exit behavior impossible to preserve. Finding 8. |
| 10 — recognition order | **Unresolved** | The staged lookup changes failure precedence, adds compilation before a healthy no-op, and moves optional-skill refusal after installation. Finding 2. |
| 11 — tree grain/link validation | **Partial** | Post-enable child expansion and current target bytes are pinned, but the hardcoded relative target is not generic. Finding 10. |
| 12 — drift semantics | **Partial** | Built-in policies are listed, but installer validation and existing unvalidated live hook destinations remain unrepresentable. Findings 3 and 7. |
| 13 — operational documentation | **Resolved** | The full named Class-14 rewrite is restored at [design:214](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:214). |
| 14 — `proc alive --root` callers | **Resolved** | All containing scripts are named at [design:164](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:164); direct calls are at [arm-supervision.sh:127](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/arm-supervision.sh:127), [fingerprint-harness.sh:105](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/fingerprint-harness.sh:105), and [supervision-fixtures.sh:24](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-fixtures.sh:24). |

### Material findings

1. **HIGH — STRUCTURAL: the fixture authority matrix remains incomplete and purpose-unsafe.**

   The claimed complete enumeration at [design:148](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:148) omits mission-runner fixture reads, writes, and signal authority. `processCommand` and `groupOwned` read the fixture at [proc.go:32](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/proc.go:32) and [proc.go:76](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/proc.go:76); the latter authorizes real signals at [host.go:91](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/host.go:91). The runner also writes the fixture directly at [proc.go:102](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/proc.go:102). Fixture-backed custody additionally controls drain and usage derivation at [drain.go:262](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/missionrunner/drain.go:262) and [fence.go:746](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/mission/fence.go:746).

   “The same probe” also fails to preserve source scope: the mission identity file is currently consulted only after unreadable kernel argv at [contract.go:1378](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/contract/contract.go:1378), while the synthetic ancestor additionally requires `runtime == "fake"` at [ancestor_production.go:55](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/census/ancestor_production.go:55).

   The design should enumerate mission-runner command lookup, ownership proof, publication, drain custody, and usage custody; include `internal/missionrunner` and `internal/mission` in the blast radius; and define purpose-specific probe methods. Root authorization must be necessary but not sufficient: retain current `allowFake`, runtime-name, kernel-death, and fallback-order guards.

2. **HIGH — STRUCTURAL: the bootstrap sequence cannot preserve the behavior it claims.**

   R2 calls [design:115](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:115) today’s sequence, but:

   - Unknown runtimes currently refuse before toolchain and provenance checks at [adopt.sh:72](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:72), not afterward.
   - Healthy same-SHA adoption exits before compilation at [adopt.sh:117](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:117); a temporary build can fail even when the installed target is healthy.
   - Optional skills currently refuse before the first target write at [adopt.sh:213](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:213) and [adopt.sh:247](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:247), whereas r2 places validation after “install” at [design:126](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:126).

   The design must explicitly choose and ratify either compilation before same-SHA recognition or ignoring registry-validity checks on the no-op path; both current properties cannot coexist with a source-fresh compiled query. Optional-skill materialization and validation must remain after same-SHA recognition but before every target mutation.

3. **HIGH — STRUCTURAL: the permanent installer is not a permanent lifecycle seam.**

   Installer rows promise registry-plus-handler extension at [design:56](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:56), but must still select from the closed validation-policy set at [design:62](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:62). Nothing declares whether handler output is instruction-bearing. Nor does r2 require seam-local registration or add `internal/install` to the sanctioned seams in [architecture.md:45](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:45).

   The design should make `internal/install` own neutral `Prepare`, `Apply`, and `Validate` contracts plus collision metadata. Sanctioned per-runtime handler files must self-register without a central wire-up, with a both-ways registry/table join. New validation or collision semantics must either remain handler-owned or be named as a human-reserved contract amendment.

4. **HIGH — STRUCTURAL: collision roots have neither shell transport nor a representable exception.**

   R2 declares contributed roots at [design:81](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:81), but the ten-column frame at [design:95](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:95) and derived views at [design:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:129) expose none. Adoption’s scan remains shell-owned at [adopt.sh:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:129).

   Current behavior also accepts an existing `.codex` directory before writing `.codex/hooks.json` at [adopt.sh:335](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:335), but r2’s “explicit human-adjudicated exclusions” have no schema.

   The design should define `runtime collision-roots`, emitting the sorted, deduplicated full population with pinned framing. It should name the current uncovered destination exactly—`.codex/hooks.json`—and state that every other uncovered instruction-bearing destination refuses. Any additional exclusion or adding `.codex` to the scan is a human-reserved security-contract change.

5. **HIGH — STRUCTURAL: `--with-adapter` selects the wrong validation population.**

   R2 uses it to replace the adapter contract loop at [design:191](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:191). Fake has `HasAdapter: true` at [runtimes.go:158](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:158), but the current loop deliberately covers only Claude, Codex, and Devin and requires literal common-initializer/writer calls at [validate-metasystem.sh:479](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:479). Fake has its own implementation at [fake.sh:30](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/fake.sh:30).

   The design should use `--with-adapter` only for universal behavioral contracts and replace the source-text assertions with decoded snapshot identity/schema checks that fake can satisfy. Otherwise it needs a separately declared population for the common real-adapter implementation shape.

6. **HIGH — STRUCTURAL: destination-overlap compatibility ignores validation policy.**

   Validation policy is row state at [design:33](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:33), but compatibility compares only operation, source, payload, and mode at [design:75](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:75). Two aliases can therefore deduplicate one output while prescribing different drift judgments.

   The design should require identical validation policy—and identical installer handler validation—for compatible overlaps, or define an explicit policy join. Otherwise the overlap is incompatible and adoption refuses. It should also pin the requiredness orders and evaluate `source-conditioned` against the post-`--enable` staged source.

7. **HIGH — STRUCTURAL: no listed validation policy preserves today’s live-hook behavior.**

   R2 makes installed-enforcement validation row-derived at [design:129](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:129). Today adopted validation checks copied skill/profile drift at [validate-metasystem.sh:565](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:565), while live hook checking is template-only at [validate-metasystem.sh:1387](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/validate-metasystem.sh:1387). The structural checker also requires a nonblank self-check marker at [hooks.go:21](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/hooks/hooks.go:21), which Codex and Devin do not declare at [runtimes.go:112](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:112) and [runtimes.go:124](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/internal/runtimes/runtimes.go:124).

   Exact bytes or structural-subset validation would both strengthen current adopted behavior. The design should add a preserving presence-only/no-live-drift policy for those destinations, or explicitly seek human ratification for the strengthening and define the missing runtime-specific structural contract.

8. **HIGH — STRUCTURAL: supervision-hook registry lookup has impossible failure precedence.**

   R2 requires unknown runtimes to exit 2 through registry membership at [design:197](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:197). Today runtime/event syntax is checked before binary lookup, while a missing binary is benign at [supervision-hook.sh:4](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:4) and [supervision-hook.sh:26](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/supervision-hook.sh:26). Without the binary, registry membership cannot be known.

   The design should pin the unavoidable precedence: validate event syntax, resolve the engine, and preserve missing-engine exit 0 before membership lookup; when the engine exists, unknown runtime exits 2. If unknown runtime must always exit 2, the missing-engine behavior is a ratified change, not preservation.

9. **HIGH — STRUCTURAL: adoption’s default, population, and help remain core-owned.**

   `adopt.sh` still publishes a fixed universe at [adopt.sh:7](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:7), prescribes runtime layouts at [adopt.sh:20](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:20), hardcodes Claude as default at [adopt.sh:53](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:53), and recognizes fixed names at [adopt.sh:77](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:77). The generic adoptable/default views already exist at [runtime_verbs.go:28](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/cmd/metasystem/runtime_verbs.go:28).

   The design should distinguish omitted `--runtimes` from an explicit value, resolve omission with `runtime adoption-default`, validate explicit names specifically against `runtime list --adoptable`, and make help use `<comma-separated names>|none` with registry pointers and no runtime layouts.

10. **MEDIUM — STRUCTURAL: `tree` hardcodes a target valid only for today’s directory depth.**

   The operation globally emits `../../skills/<name>` at [design:44](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:44). That matches today’s two-level registration roots at [adopt.sh:293](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:293), but ignores the row’s source and destination depth.

   The design should compute a clean relative link from each expanded destination’s parent to its expanded source child. Current rows must still produce exactly `../../skills/<name>`; that literal is a current-row fixture, not the general algorithm.

11. **MEDIUM — MECHANICAL-GRAIN: patterned registration fields have no grammar.**

   R2 permits patterned source/destination fields at [design:35](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:35) and transports them opaquely at [design:95](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:95). Current Claude, Devin, and Codex projections differ materially at [adopt.sh:312](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:312), [adopt.sh:324](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:324), and [adopt.sh:334](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:334).

   The design should pin one placeholder grammar, permitted variables, post-enable expansion source, sorted result order, clean-relative validation after substitution, zero-match behavior, and the exact `mode` bytes for `tree` and `skill-profiles`.

12. **MEDIUM — MECHANICAL-GRAIN: the installer invocation still lacks phase and path binding.**

   [Design:105](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:105) does not say whether handlers run once or twice, what read-only pre-mutation success emits, or how `R`, `S`, and `D` bind to the staged source and target. Current adoption distinctly stages at [adopt.sh:139](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:139), checks collisions at [adopt.sh:227](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:227), and mutates at [adopt.sh:247](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/adopt.sh:247).

   The design should define an explicit staged-source root, canonical target root, containment checks, two-phase call order, read-only `ready <destination>` output for pre-mutation, mutate output, and whether a later failure rolls back earlier successful rows.

13. **MEDIUM — MECHANICAL-GRAIN: the adapter-side enforcement-map interface is unnamed.**

   R2 pins the registry verb but calls the adapter side only a “side-effect-free declaration verb” at [design:182](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:182). Adapter public interfaces are literal usage/case contracts, for example [codex.sh:4](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/adapters/codex.sh:4).

   The design should pin `<adapter> enforcement-map`, no arguments, exit 0 plus one canonical JSON line for static-map adapters, exit 2 for usage, and no required verb for fake.

14. **MEDIUM — MECHANICAL-GRAIN: the staged registry build lacks executable bytes.**

   R2 gives only `go build -o <tmp>` at [design:120](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:120). The canonical build pins root, `CGO_ENABLED=0`, VCS behavior, package, stamping, cleanup, and error handling at [go-build.sh:30](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/scripts/agents/go-build.sh:30).

   The design should pin the working directory, `./cmd/metasystem` package, required build flags and stamp, `mktemp` lifecycle, and adoption failure mapping, while forbidding rename/copy over `bin/metasystem`.

15. **MEDIUM — MECHANICAL-GRAIN: runtime-owned root instruction files remain outside the sanctioned seam list.**

   The accepted ruling explicitly sanctioned them at [agnosticism-audit-rulings.md:462](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/agnosticism-audit-rulings.md:462). The canonical seam list at [architecture.md:40](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:40) still omits them, and r2’s five-consumer work at [design:89](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/plans/runtime-integration-contracts-design.md:89) does not require that correction.

   The design should add registry-addressed root instruction files to the sanctioned runtime seam assets.

The two standing exceptions are otherwise preserved correctly: role-owned residual waivers remain fail-closed human policy at [architecture.md:67](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/architecture.md:67), and the handwritten delivery-evidence row remains the second declared exception at [turn-verdict-delivery-contract.md:41](/Users/wido/LocalStorage/GitHub/agentic-tools/metasystem/docs/design/turn-verdict-delivery-contract.md:41).

Proposed review receipt: `type=review outcome=reworked skills=design-critique verify=static-code-grounded-r2-fold corrections=15 stop_loss=no note="DRAFT r2 leaves eleven structural and four mechanical-grain material defects; no files changed"`

REVISE — structural findings remain
