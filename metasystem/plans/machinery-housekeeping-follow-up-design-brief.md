# Follow-up design brief: engine identity, then remaining housekeeping

Working Mode: Design. Split from `plans/designs/machinery-housekeeping.md` on 2026-10-08 under the task's five-unit limit. This is a brief, not an opened ledger goal or an accepted design. Read the original housekeeping and engine-inputs briefs in full, the relevant findings and mechanisms, then recheck code sites on the tree that will be built.

## Order and size

The parent keeps source-flake repair first and the flake register next, in four units. This follow-up starts with the **complete engine stamp and inputs**, then takes findings 17, 18, 19 and 21 and any remaining read-output isolation from finding 14. Each unit remains at most 250 changed production lines, tests excluded; at most five units per goal. If engine work consumes that allocation, split the remaining repairs again before its units table. Each unit checks only new/changed tests and their mutations; the full suite runs once when that goal lands.

Preliminary engine allocation, to validate by code tracing rather than adopt as a build plan: input comparison and source verification 220 lines; session/rebuilt-engine re-arm and successful remedies 240; both dispatch admissions 200; host stamp publication and existing boundary/launch readers 240. This approximately 900-line feature does not fit the parent's one remaining slot. No unused comparator or unconsumed host record is an acceptable smaller unit.

## Engine facts read on the parent's tree

- `internal/behaviorsurface/policy.v2.json:3` already declares ENGINE inputs: `cmd/**`, `internal/**`, `scripts/agents/**`, `go.mod` and `go.sum`. Embedded assets therefore belong to the input closure; runtime settings, ledger and records are not new engine versions.
- `internal/steward/rearm_resolver.go:632` already compares ENGINE projections, including an archived-digest fallback for undecidable diffs. Reuse it, including mode, submodule and unknown-input behavior; do not add another path list.
- `internal/steward/rearm_resolver.go:475` resolves commit/witness stamps; `:503` requires the source to be landed. `:519` still uses ancestry plus an `internal`/`cmd` log filter for enrollment skew. `:543` has the witness reverse-ancestry exception. Equal inputs must not accidentally waive executable provenance.
- `internal/steward/runner.go:719` uses checkout HEAD as the rebuilt engine's landed commit and `:726` requires source-commit equality. Its minted record at `:745` uses that value. These are separate changes from merely sharing a comparator.
- `internal/delegation/infra.go:57` uses a merge base against local main; errors at `:58` and `:63` fail open. `cmd/metasystem/intent_work.go:529` skips witness stamps, and `:540` independently scans `stamp..origin/main` with an `internal`/`cmd` list. Both are production dispatch consumers.
- `cmd/metasystem/hook_entry.go:637` verifies the pinned source at the destination. Include session start, its hook, ordinary up and rebuilt-engine refresh; do not fix only one entrance.
- `cmd/devgate/build.go:163` stamps HEAD; `:176` checks dirty inputs through the ENGINE policy. Building an unlanded HEAD cannot repair a refusal requiring a landed source.
- `internal/enginebuild/record.go:25` owns linked stamp records and legacy compatibility. `internal/enginedeploy/enginedeploy.go:117` builds and validates a commit-stamped artifact, `:196` activates it and `:249` reports its identity. These are existing deployment consumers, not proof that a monotonic host stamp already exists.
- `internal/steward/rearm_checkout.go:59` owns the current unit-boundary busy check. Mechanisms 7 and 8 require host stamp adoption at admission/boundary, never halfway through a running round; mechanism 10 requires launch/read provenance.

## Decisions the engine design must close

Keep `work/ldfc-u3` and commit `119b72998` plus its corrections as failed-attempt evidence only; **do not merge them**. The prior review counts were 1, 2, 2 material, with a new defect in the last correction. Do not reset that history by renaming a helper.

Name one owner for the captured landed base `B = merge-base(HEAD, owned landing ref)` in session start, rebuilt-engine refresh and both dispatch paths. Capture the owned landing tip `L` separately: B is not necessarily L. Never hardcode origin/main where the enrollment names another endpoint. Name one owner for equal ENGINE inputs, keeping source provenance, dirty-input checks and in-flight ownership separate.

Resolve the executable's genuine source `S` from its commit or witness stamp against landed history. For an engine newer than an older checkout, equal inputs may be reused, but the minted enrollment must record S as its executable source, not the older HEAD/B. A witness can resolve to a later content-equivalent landed commit; do not turn that legal case into an ancestry loop. Missing/malformed provenance remains unknown; a commit hash is not an ordered version number.

The required matrix is **equal/different inputs × HEAD behind/equal/ahead/diverged from L × commit/witness stamp**: 16 explicit cells in the design, each with outcome and a remedy exercised through its real owner. Define equality as ENGINE(S) versus ENGINE(B), after S has independently been proven landed. Distinguish “equal to the desired host engine” from “equal to my branch's unlanded changes.” Include:

| HEAD versus L | Equal ENGINE inputs, either stamp kind | Different ENGINE inputs, either stamp kind |
| --- | --- | --- |
| Behind | Reuse proven bytes; retain S in enrollment, clear obsolete deferral | Defer adoption of different bytes; advance the owned checkout by the existing lawful refresh, then retry |
| Equal | Reuse, with source/enrollment verification | Build/adopt the proven landed source, then start and retry work |
| Ahead | Ignore unlanded HEAD for comparison; reuse proven bytes | Obtain the engine from the owned landed checkout/deployment; never prescribe `devgate build` at this unlanded HEAD |
| Diverged | Compare against B with captured endpoint ownership; equal inputs do not require rewriting the branch | Defer mismatched adoption; use the owned landed engine/checkout and existing isolated-work path; preserve branch changes |

This is the required outcome constraint, not permission to guess a command: the design must trace and name the existing successful refresh/deployment act for each different-input cell, including an engine ahead of HEAD. If today's public verbs cannot supply it automatically, explicitly design the smallest missing act and budget it. A read-only status command, reset of the branch, person re-enrollment or another build of the same unlanded HEAD is not a successful remedy. A legitimate dirty/conflicting checkout or unavailable transport remains a stated blocker; do not promise success by deleting work.

Dispatch's advisory skew check stays fail-open when comparison is unavailable and rebuilding cannot help (Git error/no merge base); it reports unknown and proceeds to normal work admission. Enrollment/source verification stays fail-closed on unknown executable identity. Both public work admission and delegation must use the same input comparison, including witness stamps. Test module, agent-script, embedded-asset and mode changes, not just Go files; records/settings-only commits must not defer or rebuild.

For the host stamp, extend the fleet's host record only once its actual writer and registered owner exist. Define stamp identity/order, publication after a successful lane push, unchanged-input pushes, failed build/deployment, rollback and missing/stale owner behavior. Bind stamp to the executable's source and bytes; never order commit hashes or content hashes. Pin the admitted engine for every running round, update each seat at its next boundary, and store that identity on actual build/read launches. Include the production status reader (`machine list`) and all preflight consumers in the same allocation. No second engine-deployment service beside `enginedeploy`.

Critique this design to convergence under the governing stop rule before any build. Give every function, record and act the five answers and every unit a public-verb proof that fails under its mutation. Carry the four recurring defect classes from the parent.

## Remaining findings, preserved rather than declared solved

| Finding | Required smallest repair | Existing home to trace / dependency |
| --- | --- | --- |
| 14: output collisions | Each launched read owns its output; one environment retry, then a recorded stop. The parent repairs fixture-store ownership, not every production read-output lifecycle. | Read sequence, standalone reads and review-chain outcome/stop owner; retain launch identity in each output path. |
| 17: census after rebuild | Boundary engine adoption re-arms census without a separate person act. | Existing steward boundary refresh and census owner; build on the proven source/stamp, never a parallel refresh timer. |
| 18: remaining flakes and proof deadlines | Trace the original ledger-fetch, cancel and frozen-corpus failures against current code. Reuse artificial clocks/fixture ledgers. Every proof uses the declared deadline. | Goal projection clock, current test runner and proof deadline owner from the lane/fleet goals; the parent's three source repairs are not proof of these other fixes. |
| 19: safe worktree removal | Repack before every worktree removal; failed repack keeps the worktree. | Current diskstore/worktree owners, plus direct removals such as `internal/landing/plain/replay.go:226`. Apply to all callers found, not just the sweeper. |
| 19: finding identity | Finding ids include the read identity, including split/carry paths. | Review chain's read record and disposition join; no rekey that loses the decision or stop history. |
| 19: ignored files and caches | A clean-worktree judgement ignores ignored build products; unit-owned caches end only when their owner is finished. | Existing worktree cleanliness and registered store lifetime owners. Respect alternates and active users; no global cache wipe. |
| 21: records publication | Validate status grammar and run the relevant message/instruction audits before a records commit. | Existing records push/landing owner; do not invent a second publisher or require these audits on every unrelated unit. |
| 21: test registration | A unit's normal check regenerates/diffs registration for its added/changed tests. | Existing generated testing contract and declared-check owner; no extra full suite. |
| 21: accept-risk first use | Read/register the exact critic-root findings needed by the existing accept-risk act, so a prior dispositions invocation is unnecessary. | Goal risk act, critic root and finding/disposition registration owner; preserve person authority and exact finding identity. |

Deletion units must specify empty-id refusal, error propagation, exact path scope, the protected set and a preceding dry run. Their mutation proof must demonstrate that a failed repack or unreadable owner never permits removal. No code, memory, record or ledger write was performed to enact any of these repairs in the parent's design task.

## Added 2026-10-08 16:25: a load-fragile landing test

cmd/metasystem TestIntentLandWholeOwnerGitAdapter/refused_atomic_publication_retries_the_prepared_landing failed once in goal 2's integration gate under load (185 s; publication retry outcome partial) and passed 4 of 4 alone (33-42 s). Convert its waits to injected clocks or bound its git work so load cannot change the outcome (Wido: artificial clocks, never load-fragile tests). Evidence: agentic-tools-evidence/gate-rcs2-20261008 and the gate-rcs3 cmd log.
