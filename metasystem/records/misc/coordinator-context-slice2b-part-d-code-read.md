# Independent code read: coordinator-context slice 2b part D

Reader: Opus 5 (1M), 2026-09-14. Worktree
`/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s2b/metasystem`.
Base HEAD `cf882eb26959d3e3698e53a3986a24dc33df8684` (parts A, B, C).
Computed diff: `git diff HEAD` plus untracked `cmd/metasystem/context_cost_test.go`.
Note the local `origin/main` ref has moved to `811b37d9`; the merge base with HEAD is
HEAD itself, so the review base is the stated one.

Diff shape (`git diff --numstat HEAD`):

```
140   0   metasystem/cmd/metasystem/context_verbs_test.go
  2   2   metasystem/scripts/agents/health-fixtures.sh
 41   6   metasystem/scripts/agents/supervision-hook-fixtures.sh
  1   0   metasystem/scripts/validate-metasystem.sh
  3   0   metasystem/testing.json
```

plus `cmd/metasystem/context_cost_test.go` (982 lines, new).

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | critical | yes | The new `context-budget` surface is not mirrored into the `residual` fallback surface, so the pinned fallback-union contract test fails. This tree is red today. | `metasystem/testing.json:16` adds surface groups `context-standard`, `section/supervision-and-census-fixtures`, `context-stop-cost`; `metasystem/internal/testpolicy/contract_test.go:122` requires `residual.Standard`/`Deep` to equal the ordered union of every other surface. Ran `go test ./internal/testpolicy/ -run TestMetaSystemContractPinsFallbackDeclarationAndGroupUnions`: FAIL. |
| F-2 | high | yes | The new surface lowers selection for `internal/usage`, `internal/output` and `internal/runtimes`. Those paths previously had no surface and fell to the residual fallback; they now resolve to `context-budget` alone, and the only Go group selected runs a named-test filter, so most of their own tests are no longer selected by any group. | Before: `git show HEAD:metasystem/testing.json` has no surface matching `internal/usage/**`; `metasystem/internal/testpolicy/select.go:121-139` sends unmatched paths to the fallback. Measured with the real contract: `internal/usage/cursor.go` selects 44 groups at HEAD, 9 groups on this tree; lost groups include `governed-standard`, `runtime-owner-standard`, `proof-standard`, `policy-protection` and every critical-obligation provider. `metasystem/testing.json:51` declares `context-standard` with 34 named tests, which `metasystem/internal/proofrun/test_go.go:94-102` turns into `-run ^(...)$`: 42 of 49 `internal/usage` tests, 4 of 5 `internal/output` tests and 10 of 11 `internal/runtimes` tests are then never run. Contradicts the brief ("Do not lower any selection or protection", `artifacts/reports/codex-ccb-slice2b-brief-v2.md` part D) and the builder's claim "Existing overlapping surfaces and protections remain" (`artifacts/reports/codex-ccb-slice2b-result.md`, Part D, Testing contract). |
| F-3 | medium | no | The cost proof never drives the candidate engine's own `health --hook-preview`. The bed's wrapper intercepts it and runs the in-test-binary helper, which calls `steward.PreviewHealthAt` directly, so the "hook role equals captured role" cross-check is same-process and circular, and a green run does not prove `bin/metasystem health --hook-preview` emits the role. The production health evaluation itself is real, and the two beds in `section/supervision-and-census-fixtures` do exercise the engine's health line, so the required seat proof set is still complete. | `cmd/metasystem/context_cost_test.go:289-293` (wrapper branch), `:186-206` (helper calls `steward.PreviewHealthAt(repoRoot, installationRoot, ...)`), `:432-435` (the `hookRole != role.Line()` comparison). The hook calls `"$ms" health --hook-preview --repo "$repo" --metasystem-root "$world_installation"` at `scripts/agents/supervision-hook.sh:959`; the wrapper ignores both flags and substitutes `METASYSTEM_CONTEXT_COST_HEALTH_ROOT`, so `repoRoot` becomes the installation rather than the checkout. Builder states this openly in `artifacts/reports/codex-ccb-slice2b-result.md`, Part D. |
| F-4 | medium | yes | The concurrent cold-read case asserts exactly one live result and one busy result, but nothing forces the two reads to overlap at the cursor lock. The barrier proves both processes started, not that they contend. If the standalone reader finishes its cold parse before the hook reaches the health role, both come back live and the assertion fails for a machine-speed reason. | `cmd/metasystem/context_cost_test.go:620-658`: barriers release the Stop then the reader, then `if !((readerLive && stopBusy) || (readerBusy && stopLive))` fails. Both sides read non-blocking: `internal/steward/context.go:99` sets `NonBlocking: true` for the shared evaluation used by the health role and by `context status`, and `internal/usage/cursor.go:56-62` takes `LOCK_NB` in that mode. The hook does `up`, ancestor discovery and digest work before line 959, while the reader parses immediately, so the orderings are wall-clock dependent. This is the load-shaped false-red class this repository has hit before. |
| F-5 | low | no | The "context role carried no duration" check can never fire. | `cmd/metasystem/context_cost_test.go:455` tests `role.DurationMillis < 1`; `internal/steward/health.go:378-384` floors `elapsedRoleMillis` at 1. |
| F-6 | low | no | `requireLiveRole` demands an exact role string, which the production reason can legitimately extend. Any spill written under the fixture installation during a measured Stop turns every live assertion red. | `cmd/metasystem/context_cost_test.go:466-472` requires exactly `context-budget=alive (120 thousand tokens this call, bound 150, ceiling 200)`; `internal/steward/context.go:126-129` appends `; newest spill: <name> (<n> bytes)` when `output.NewestSince` finds one. |
| F-7 | medium | yes | `supervision_and_census_section` runs under `set -e`, so the first failing bed aborts the section. The change puts `health-fixtures.sh`, a bed that `validate-metasystem.sh` has never invoked before, ahead of the three established beds, and the new contract test pins that order. A health failure now hides the runtime-hook, supervision-hook and supervision results, including the CCB-2-14 assertions this slice depends on. | `scripts/validate-metasystem.sh:1047-1051` (new line 1048 is first); `scripts/validate-metasystem.sh:353` runs the section body as `( set -e; "$@" )`; `cmd/metasystem/context_verbs_test.go:555` pins `wantOrder` with health first. Before this change `health-fixtures.sh` appears nowhere else in `scripts/validate-metasystem.sh`. The brief asks only to "Run all four serially". |
| F-8 | low | no | The health bed's new row proves role presence and a non-unknown status, not any budget computation. It does still fail against an engine without part B. | `scripts/agents/health-fixtures.sh:286` greps for the literal `context-budget=alive`. The bed announces its main with `--runtime fake` (`scripts/agents/health-fixtures.sh:220-222`), so `ContextBudgetLine` returns `roleAlive` at `internal/steward/context.go:84` (unregistered runtime) or at `:75` (lease absent) before any cursor, transcript, bound or ceiling work. |
| F-9 | low | no | The four new supervision-hook assertions accept any status, including `unknown` and `dead`. This matches CCB-2-14's wording ("carries `context-budget=`"), so it is not a brief violation, but it is the reason a seat whose role answers unknown still passes them. | `scripts/agents/supervision-hook-fixtures.sh:406`, `:464`, `:1547`, `:1572` all use `grep -Fq 'context-budget='`. Both paths carry the line because `checkin_tail=$health_line` (`scripts/agents/supervision-hook.sh:981`) is appended to both the blocking and the allowed message (`:1074-1083`). |
| F-10 | low | no | `template_stop_output` and `template_stop_digest` stay pointed at the Codex leg after it finishes, and `template_stop_evidence_ready` reads them as globals. A future leg that forgets to reassign would silently assert against the Codex output. No current caller is affected. | `scripts/agents/supervision-hook-fixtures.sh:385-388`, `:401-402`, `:446-447`. |
| F-11 | low | no | `context-stop-cost` declares a target far below its real cost, and peak temp-disk use is roughly 600 MB. `targetMs` is advisory, not a timeout, so this cannot fail the group; it distorts scheduling and cost reporting. | `metasystem/testing.json:52` declares `targetMs: 120000`. The enabled test writes about 290 MB of Codex source and 49 MB of Claude source (`cmd/metasystem/context_cost_test.go:800-830`), copies the Codex source again for the inode restart (`:900-922`), builds the engine, and drives roughly 22 bounded Stops with a 15-second ceiling each. `targetMs` is consumed only for cost declaration and ordering at `internal/proofrun/test_build.go:284-287` and `:625`; the Go adapter passes `-timeout 0` (`internal/proofrun/test_go.go:76`). |
| F-12 | low | no | The brief's own verification recipe cannot catch F-1, which is why the builder and the seat both saw green. | `scripts/agents/go-gate.sh:10-13` says fast mode runs no Go test suites; the brief's race command covers `internal/usage`, `internal/runtimes`, `internal/output`, `internal/steward`, `cmd/metasystem` and never `internal/testpolicy`. No testing group runs `TestMetaSystemContractPinsFallbackDeclarationAndGroupUnions` either (`policy-canary`, `policy-protection`, `carry-plumbing-standard` and `contract-unknown-diagnostic` all use named-test filters that exclude it), so only the full gate reaches it: `scripts/agents/go-gate.sh:662` runs `go test -race -cover ./internal/...`. |

## Answers to the seven questions

1. **Do the new fixture assertions prove what their rows claim?** The twelve-role
   claim is true: `scripts/agents/health-fixtures.sh:286` now lists
   steward-runner, supervision-owner, repo-watcher, census-freshness,
   narrator-freshness, session-main, hook-freshness, stop-hook-duration,
   context-budget, claimed-goal-appetite, nonterminal-jobs, capability-snapshots,
   and the scenario description at `:21` was updated to match. The assertion would
   NOT pass against an engine without part B: the grep is for the literal
   `context-budget=alive`, and a pre-2b health line carries no such role. It
   proves only presence and a non-unknown status; see F-8.

2. **The Stop duration proof.** It measures the wall time of
   `bash scripts/agents/supervision-hook.sh <runtime> stop` end to end, from
   process start to exit, for about eleven Stops per runtime, and requires each to
   be strictly below 15 seconds (`cmd/metasystem/context_cost_test.go:26`,
   asserted at `:415-417`). That is the budget the design names, because the bed
   pins `steward.stop-slow-sec=15` in its own `metasystem.conf`
   (`cmd/metasystem/context_cost_test.go:267`), so `min(15, configured)` is 15.
   The test reads the constant rather than the config, so a lower fixture config
   would not tighten the bound. It also requires the durable worker evidence to
   report under 57 seconds (`:449-451`) and refuses any deadline-expiry text
   (`:418-420`). It cannot pass where the role is never evaluated: the hook output
   must contain `context-budget=` (`:421-423`), the health sidecar file must exist
   (`:424-427`), and the helper exits 3 without writing it when no context role is
   found (`:198-200`). The caveat is F-3: the health evaluation is the production
   function but is invoked from the test binary, not from the candidate engine's
   `health --hook-preview`.

3. **The testing.json change.** Three additions: surface `context-budget`
   (`testing.json:16`) owning `internal/usage/**`, `internal/output/**`,
   `internal/runtimes/**`, five named steward files, five named command files and
   the two edited fixture scripts, with standard groups `context-standard` and
   `section/supervision-and-census-fixtures` and deep group `context-stop-cost`;
   unit group `context-standard` (`:51`) over five packages with 34 named tests,
   `race:false`, `coverage:false`; performance group `context-stop-cost` (`:52`)
   over `cmd/metasystem` with one named test and
   `METASYSTEM_CONTEXT_COST_PROOF=1`. All 35 named tests exist in the declared
   packages; none is missing or misplaced. For `internal/steward` the answer to
   the question is no: `governed-cadence` still owns `internal/steward/**` with
   `governed-standard` at `tests: "all"`, so a steward change selects both
   surfaces and gains proof rather than losing it. For `internal/usage` the answer
   is yes, and that is F-2.

4. **The validate-metasystem.sh change.** It adds
   `scripts/agents/health-fixtures.sh` as the first of four serial beds in
   `supervision_and_census_section` (`:1048`). Before this change the script
   referenced that bed nowhere, so this is the first time the engine will run it,
   across its four scenarios (direct-verdicts, narrator-recovery, alert-episode,
   fixture-notification), up to three concurrently. It can fail for reasons
   unrelated to the context budget: it arms real stewards and runners in a
   temporary repository, asserts eleven other roles, alert-episode delivery, dedup,
   acknowledgment and healthy clear. Because the section body runs under `set -e`
   (`:353`), a health failure aborts before the three established beds, and the
   new contract test pins that order. See F-7.

5. **The new cost test.** It is a real bound, not a tautology: it generates
   production-scale sources with enforced floors
   (`cmd/metasystem/context_cost_test.go:101-107`), drives the real hook and the
   real reader with no mocking of `usage.LatestCall` or cursor persistence,
   requires exact committed call counts across append and inode restart
   (`:138-149`), requires the week report to deduplicate the replay
   (`:150`, `:553-564`), and refuses warm-only evidence by measuring the cold and
   restart legs individually. Two stability concerns: F-4, a genuine wall-clock
   race in the concurrent cold-read assertion that can produce a false red; and
   F-6, an exact role-string match that a spill would break. The lock-wait case is
   deterministic and well built: a Go-held flock plus pipe handshakes, no sleeps
   (`:565-612`). The duration assertion itself is inherently wall-clock, which is
   the point of the obligation, and it is fenced behind an opt-in environment
   variable and a separate performance group so it never runs in ordinary unit
   selection.

6. **Anything from parts A to C altered?** No. `context_verbs_test.go` is 140
   insertions and 0 deletions, a pure append of one new test function. The only
   deletions anywhere are two lines in `health-fixtures.sh` (the role loop and the
   scenario description, both replaced in place) and six lines in
   `supervision-hook-fixtures.sh` (parameterizing the existing evidence helper;
   the Claude leg's assertions are preserved verbatim through
   `template_stop_output` and `template_stop_digest` set immediately before it).
   No coverage floor changed: neither `scripts/agents/coverage-ratchet.json` nor
   `coverage-ratchet-linux.json` is in the diff. No role text changed:
   `internal/steward/health.go` and `internal/steward/context.go` are not in the
   diff. No receipt, record or plans file changed.

7. **A runtime with no per-call stream, or a role answering unknown.** The four
   new supervision-hook assertions pass in every case: they grep for
   `context-budget=` with no status (F-9). `LatestCall` returns
   `unknown (no per-call stream)` for a NoStream capability
   (`internal/usage/calls.go:103`), which `contextBenignUnknown` classifies as
   benign (`internal/steward/context.go:340-341`), so the role comes out alive and
   the health bed passes too. A role that genuinely answers unknown, however,
   makes the aggregate exit code 2, so `wait_for_healthy healthy` never returns 0
   and the health bed fails at its cap, independently of the new grep. In the
   health bed's own fixture that cannot happen, because the announced main uses
   `--runtime fake` and the role short-circuits to alive before any read.

## What a green engine run would and would not prove

Would prove: the context-budget role is present and alive on the armed health
line across all four health scenarios; the role reaches the Stop response on the
Claude and Codex template legs and on both the allowed and blocked payload paths,
through the unmodified production hook; the testing contract selects the context
groups for a context source change and for either fixture script, with the cost
proof only in deep mode; and, for the cost group, that a real Stop over a
264 MB Codex rollout and a 48 MB Claude transcript stays under 15 seconds cold,
warm, after an append, after an inode restart and under contention, with exact
committed call counts and a deduplicated week report.

Would not prove: that `bin/metasystem health --hook-preview` itself emits the
role under the timed measurement (F-3; the two beds cover it separately); that
any budget arithmetic, bound, ceiling or cursor behaviour works in the health bed
(F-8); that a change to `internal/usage`, `internal/output` or `internal/runtimes`
is still covered by the proof it needs (F-2); or Linux coverage floors, which
still require the Linux seat per CCB-2-18.

## Verdict

NOT FIT TO LAND as it stands. Four material findings.

Must change before landing:

1. F-1: update the `residual` fallback surface in `metasystem/testing.json` so its
   `standard` and `deep` lists equal the ordered union again. Insert
   `context-standard` and `section/supervision-and-census-fixtures` immediately
   after `governed-standard` in `residual.standard`, removing the now-duplicated
   later occurrence of `section/supervision-and-census-fixtures`, and insert
   `context-stop-cost` after `section/workflow-tooling-fixtures` in
   `residual.deep`. Then `go test ./internal/testpolicy/` must be green.
2. F-2: restore proof for `internal/usage`, `internal/output` and
   `internal/runtimes`. The narrow fix is a second Go group on the
   `context-budget` surface that runs those three packages with `tests: "all"`, so
   the surface adds the context proof instead of replacing the residual battery.
3. F-4: make the concurrent cold-read case deterministic rather than
   wall-clock-dependent, or accept both-live as a third lawful outcome with the
   contention claim moved entirely onto the already-deterministic held-lock case.
4. F-7: run `health-fixtures.sh` last in `supervision_and_census_section` and
   update `wantOrder` in `cmd/metasystem/context_verbs_test.go:555`, so a
   newly wired bed cannot mask the three beds that carry CCB-2-14.

## Engine groups the seat must run

```
section/supervision-and-census-fixtures
context-standard
context-stop-cost
section/go-engine-gate
```

`section/go-engine-gate` will reproduce F-1 as a hard red until it is fixed; it is
the only required proof that runs `internal/testpolicy`. Linux floor evidence for
`section/go-engine-gate` must come from the Linux seat or matching retained
evidence.
# Closing read, coordinator-context slice 2b part D, after the fold rounds

Worktree `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/ccb-s2b/metasystem`.
Base `cf882eb2` (= local HEAD). Change = `git diff cf882eb2` plus two untracked test files.
Computed diff, not the delegate's file list:

```
189  0  metasystem/cmd/metasystem/context_verbs_test.go
  2  2  metasystem/scripts/agents/health-fixtures.sh
 65 11  metasystem/scripts/agents/supervision-hook-fixtures.sh
  1  0  metasystem/scripts/validate-metasystem.sh
  5  1  metasystem/testing.json
 ??     metasystem/cmd/metasystem/context_cost_test.go
 ??     metasystem/internal/usage/context_cost_helper_test.go
```

All checks below were run on a scratch copy at
`/private/tmp/claude-501/.../scratchpad/probe`, never in the repository worktree.
No fixture bed was run.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| G-1 | medium | yes | The F-7 fold moved the health bed to last in a section that aborts at the first failing bed, so on the current tree the newly wired health bed never runs at all, and the section result says nothing about it. This change wires `health-fixtures.sh` into the validation suite for the first time in the repository's history, and its landed position has no green engine observation. | `scripts/validate-metasystem.sh:1047-1052` (health last in the section body); `scripts/validate-metasystem.sh:353` `( set -e; "$@" )` so the body aborts at the first non-zero bed; newest engine run of the section, `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem/artifacts/agents/suite-failures/20260914T001529Z-detached-section_supervision-and-census-fixtures-48763-1789344929953130000/source-001-suite-failures/20260914T001529Z-51859/supervision-and-census-fixtures.log` lines 1, 2 and 74: runtime-hook passed, supervision-hook passed, `supervision fixture scenario failed: nested-root`, and no health line anywhere in the 85-line log; `artifacts/reports/supervision-bed-failure.log:7-15` is the only run in which the health bed ever executed, and that was the pre-fold health-first layout; `git log -S "health-fixtures.sh" -- metasystem/scripts/validate-metasystem.sh` returns no commit. |
| G-2 | low | no | The twelve-role health assertion greps only `context-budget=alive`, and the context role is returned alive even when it carries no measurement, so the assertion passes on a role whose reason literally reads `unknown (no announced holder: ...)`. The neighbouring `stop-hook-duration` assertion pins its reason text; this one does not. | `scripts/agents/health-fixtures.sh:286` (role loop, `grep -Fq "$role=alive"`); `internal/steward/context.go:75` returns `roleAlive(RoleContext, noHolderReason)` where the reason begins `unknown (`; observed render in the section log above: `context-budget=alive (unknown (no announced holder: ...))`. |
| G-3 | low | no | The fold makes `context-budget`'s standard/deep/critical arrays a byte-identical, order-sensitive copy of `residual`'s, enforced by two separate `reflect.DeepEqual` guards, and neither guard is named by any group the ladder selects for a change to `metasystem/testing.json` alone. The next surface addition must hand-sync both mirrors in exact union order; a miss stays green in the ladder and only the full gate catches it. | `testing.json` surface `testing-policy` owns `metasystem/testing.json` and selects only `policy-canary`, `adapter-canary`, `command-interface-smoke`, `carry-plumbing-standard`; `internal/testpolicy/contract_test.go:82` and `:121` is the union guard and its name appears zero times in `testing.json`; `cmd/metasystem/context_verbs_test.go:453` is the second mirror, named only in `context-standard`, which a testing.json-only change does not select; `scripts/agents/go-gate.sh:540` shows `--fast` exits before the package tests, `:662` and `:704` show the full gate runs `./internal/...` and `./cmd/...`, so the guards do run at landing. |
| G-4 | low | no | `run_brain_stop` depends on `exec` replacing a background subshell so that `$$` inside equals `$!` outside. Both call sites use `&`; a future call without `&` would replace the fixture script itself and end the bed silently. | `scripts/agents/supervision-hook-fixtures.sh:259-273`. |
| G-5 | low | no | The four `context-budget=` assertions in the hook bed prove only that the role line is present in the Stop response. A role that degrades to `unknown`, or loses its measurement entirely, still passes. The failure messages say "omitted the context-budget role", so nothing is overclaimed. | `scripts/agents/supervision-hook-fixtures.sh:424`, `:469`, `:1566`, `:1591`. |
| G-6 | low | no | The Codex leg's evidence poll greps `$template_stop_output` before the inner runner has created that file (it first shells out for `proc started-at`), so the first polls write "No such file" noise into the section log. Cosmetic only; the `until` loop treats it as not-ready. | `scripts/agents/supervision-hook-fixtures.sh:259-273` and `:388-396`. |

Material findings: 1 (G-1).

## The seven checks

### 1. F-1, the residual surface

Resolved, and verified rather than asserted. The whole `internal/testpolicy` package passes on
the folded tree (`go test ./internal/testpolicy` -> ok, 0.316s), which includes
`TestMetaSystemContractPinsFallbackDeclarationAndGroupUnions`
(`internal/testpolicy/contract_test.go:82`). That test rebuilds the ordered union with
`appendUnique` over the surfaces in declaration order and compares it to the fallback with
`reflect.DeepEqual`, so "exactly the ordered union" is the literal property proved.
`residual.standard` now carries `context-standard`, `section/supervision-and-census-fixtures`,
`context-foundations-standard` directly after `governed-standard` (the move of
`section/supervision-and-census-fixtures` earlier in the list is required by the union order,
not cosmetic), and `residual.deep` carries `context-stop-cost` after
`section/workflow-tooling-fixtures`.

It is still hand-synced, and this change doubles the hand-sync: see G-3. Worth stating plainly
because the contract offers no way to add groups to the fallback selectively, so the byte
copy is forced by the design rather than chosen by the implementer.

### 2. F-2, selection breadth

Measured independently with a probe built against `internal/testpolicy.Select`, run over both
the base contract (`git show cf882eb2:metasystem/testing.json`) and the candidate. Breadth is
restored and slightly exceeded; nothing got narrower.

| Changed path | Base auto/deep | Base standard | Candidate auto/deep | Candidate standard |
| --- | ---: | ---: | ---: | ---: |
| `internal/usage/cursor.go` | 47 | 44 | 50 | 46 |
| `internal/output/output.go` | 47 | 44 | 50 | 46 |
| `internal/runtimes/runtimes.go` | 47 | 44 | 50 | 46 |
| `internal/steward/context.go` | 13 | 13 | 50 | 46 |
| `cmd/metasystem/context_verbs.go` | 47 | 44 | 50 | 46 |
| `scripts/agents/health-fixtures.sh` | 47 | 44 | 50 | 46 |

The packages' tests are genuinely run, not filtered out by a named-test pattern. For each of
the three paths, `context-foundations-standard` is selected and carries
`packages: [internal/usage, internal/output, internal/runtimes]` with `tests: "all"`;
`context-standard` adds its 35 named tests on top. At base, no selected group named any of
those three packages at all, so this is strictly more coverage than parts A to C had, not a
restoration to parity. `internal/steward` keeps its whole-package run through
`governed-standard` (`tests: "all"`), which is also selected.

The shipped guard is real: `TestContextTestingContractSelectsProof` asserts the
`internal/usage` plan equals the plan for a path that matches no surface
(`cmd/metasystem/context_verbs_test.go:551-557`), which is the structural version of "not
lowered". Running it logs `internal/usage auto selection: 47 surface-specific groups plus 3
always-run canaries`, matching the probe's 50.

### 3. F-4, contention determinism

Deterministic, not merely likelier.

The held-lock leg (`cmd/metasystem/context_cost_test.go:653`) takes the real production lock
in the test process itself, with `unix.Flock(..., LOCK_EX|LOCK_NB)` on
`usagepkg.CursorPath(...) + ".lock"`, before either child is started. That is the exact path
and the exact lock production uses (`internal/usage/cursor.go:56`, `:223`, `:592-596`:
`LOCK_EX` for the blocking reader, `LOCK_EX|LOCK_NB` for the Stop). With the lock held by the
test, the blocking `context report` child cannot proceed and the non-blocking Stop cannot
acquire, so both outcomes are forced. The lock is released explicitly and the reader is then
required to finish. The one weak step is the `select`/`default` immediately after the barrier
release, which can only produce a false pass, never a false failure.

The cold leg (`:704`) is coordinated rather than raced. The reader subprocess signals ready
from inside the `callBytesRead` hook, that is after `usage.LatestCall` already holds the
cursor lock and has read at least one byte, then blocks on an explicit release pipe while
still holding it (`internal/usage/context_cost_helper_test.go:37-51`). Only then does the Stop
run. No sleep and no scheduler ordering decides either assertion.

Residual wall-clock bounds exist and are load-shaped by nature, but with large headroom
against the implementer's recorded run (dominant Stop 2.1s; whole codex subtest 15.60s):
5s barrier readiness (bash fork plus printf), 20s per child process
(`contextCostProcessLimit`, `:27`), 15s per Stop (`contextCostStopLimit`, `:26`, checked at
`:477`). Roughly 7x on the Stop bound and 10x on the process cap. I did not run the proof.

### 4. F-7, bed order and honest reporting

Order is fixed and pinned. `supervision_and_census_section` now runs runtime-hook,
supervision-hook, supervision, health (`scripts/validate-metasystem.sh:1047-1052`), and the
new test drives the extracted section block through a recording stand-in and asserts exactly
that sequence (`cmd/metasystem/context_verbs_test.go:604`). A health-bed failure can no longer
abort the three beds that carry the section's `runtime-custody` and `governed-cadence`
obligations. That half of F-7 is genuinely fixed.

The converse was not considered, and it is live right now. Because the section body runs under
`( set -e; "$@" )`, a failure in any earlier bed aborts before health runs, and `run_section`
records one section result with a log tail. Nothing announces that a bed was skipped. The
newest engine run proves this is not hypothetical: runtime-hook and supervision-hook passed
(so the Codex template Stop repair from the bed-fix round is verified end to end, which is
good news), `supervision-fixtures.sh` then failed at `nested-root`, and the health bed
produced not one line. The seat's framing that "the only red is a standing trunk red, not this
candidate" is true about the cause and incomplete about the consequence: the same red is
currently swallowing this change's newly wired bed. That is G-1.

### 5. The new helper in `internal/usage`

Test-only, and the production surface is unchanged. `internal/usage/context_cost_helper_test.go`
is a `_test.go` file; the computed diff contains no change to any non-test file under
`internal/usage`. The only package symbol it touches is `callBytesRead`, declared unexported at
`internal/usage/calls.go:88`, present since part B and already used the same way by
`internal/usage/cursor_test.go` in seven places. It is consumed in production only behind a nil
check (`internal/usage/cursor.go:629-630`). The test body is inert unless
`METASYSTEM_CONTEXT_COST_READER_HELPER=1`, and the bed invokes the compiled package test binary
with `-test.run ^TestContextCostColdReaderHelper$`. Nit, not a finding: it restores
`callBytesRead = nil` rather than the previous value, unlike its siblings which save and
restore; harmless given the dedicated subprocess.

### 6. Parts A to C, floors, receipts

Nothing from parts A to C was altered. `context_verbs_test.go` is 189 insertions and zero
deletions. `supervision-hook-fixtures.sh`'s 11 deletions are all rewritten equivalents (two
comments, the identity printf, two grep lines, a function signature, a diagnostic echo, the
direct hook invocation, the wait call); its `exit 1` guard count rose from 186 to 192, so no
assertion was removed. `health-fixtures.sh` is 2 lines for 2 (the role list and the banner
noun) and its 55 `fail` guards are unchanged.

No coverage floor was lowered: `coverage-ratchet.json` and `coverage-ratchet-linux.json` are not
in the diff. No receipt row was left behind: `memory/receipts.log`, `records/` and `plans/` are
all outside the diff, and `git status` carries only the five modified files and the two new
test files. `artifacts/` is gitignored, so the fold's ledger and report pages are not part of
the change. `git diff --check` clean, `gofmt -l` clean on all three Go files, `go vet
./cmd/metasystem ./internal/usage` clean.

Note for the record, not a finding: all three new groups declare `"race": false`. That is the
contract-wide default (no other group declares `race` at all), so nothing was lowered; race
coverage for `internal/usage` still comes from the full gate's `go test -race ./internal/...`.

### 7. The twelve-role assertion and the Stop duration bound

The twelve-role assertion counts twelve and asserts each alive, so the banner noun is accurate.
Its weakness is G-2: `alive` here can wrap a reason that begins `unknown (`, so the bed proves
the role is listed and non-degraded, not that it measured anything. The measurement proof lives
in the Go cost bed, which does pin `120 thousand tokens this call` on every live leg
(`cmd/metasystem/context_cost_test.go:527-540`).

The Stop duration bound still proves what it claims. Fifteen seconds is checked on every Stop
leg including the two contention legs (`:477`), over asserted production-scale floors: codex at
least 264,233,967 bytes, 54,471 complete lines and exactly 6,638 ids; claude at least 47.8 MB
and 2,000 ids (`:103-108`). The proof is confined where it belongs: `context-stop-cost` is a
deep-only group carrying `METASYSTEM_CONTEXT_COST_PROOF=1` and exactly one test, and the
shipped contract test asserts both that `context-standard` excludes
`TestContextStopFitsDurationBudget` and that `context-stop-cost` names only it. Standard-mode
selection confirms this: `context-stop-cost` appears in the deep plan for all six probed paths
and in none of the standard plans.

## Verdict

Not fit to land as it stands, on one point. The code is right: F-1, F-2, F-4 and F-7 are each
genuinely folded and I verified all four by computation rather than by reading the fold report.
What is missing is proof of the one line that wires the health bed into the suite. The bed has
never run in its landed position, and the section it was added to cannot currently reach it,
while reporting nothing about the skip.

What must change, minimum: observe `section/supervision-and-census-fixtures` green once
end to end after the standing `nested-root` red is fixed, so `health-fixtures.sh` is actually
seen to run last and pass, and record that observation as this change's evidence. If landing
ahead of that red is preferred, then the honest alternative is to change
`scripts/validate-metasystem.sh` so a skipped bed is reported (give the health bed its own
section, or collect per-bed results instead of aborting), and say in the landing record that
the health bed's new position is unproven until the section is green. Either answer is small.
Landing today while describing the section red as "unrelated to this candidate" would be the
one thing to avoid.
