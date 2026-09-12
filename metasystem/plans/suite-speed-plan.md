# suite-speed

- Owner: unassigned (handoff from Wido's 2026-09-10 investigation session, branch main)
- Goal and current status: a goal delivery's required proof completes in under 2 minutes wall on the 18-core Mac, and the deep battery completes in under 10 minutes, without any group leaving the test law. Status: investigated and measured, no code changed.
- In flight right now: nothing.
- Decisions made (and who made them): none yet. Section 6 lists the decisions Wido has to make before slice 1 can land.
- Waiting on the human: the per-landing depth law (section 6, decision 1). Slices 2 to 5 do not wait on it.
- Dead ends (do not retry without new evidence): none.
- Next step: Wido answers section 6; the assigned agent lands slice 1, then slices 2 to 5 in order.

All paths below are relative to the repository root and carry the `metasystem/` prefix. Every number in this plan was measured on Wido's Mac (18 cores, macOS, Go 1.27.1) on 2026-09-10 unless it says otherwise. The Lima guest `metasystem-debian` has 4 vCPUs and 8 GiB; parallel gains there are capped and the plan says so where it matters.

## 1. What the problem is

Every goal delivery pays about an hour of testing. The retained attempt records in `metasystem/artifacts/agents/proof-runs/attempts/` show three run shapes:

| Run shape | Groups | Wall time | Runs seen |
|---|---|---|---|
| deep, wide surface | 42 to 43 | 49 to 56 min | 6 |
| deep, narrow surface | 22 to 23 | about 21 min | 4 |
| canary plus standard only | 11 | 52 to 58 s | 3 |

The bottom row is the shape ruling R-3 in `metasystem/memory/rulings.md` prescribes for a landing ("per-landing verification is fast tests; full batteries at ~6h of accumulated work"). The top row is what almost every landing actually runs. Section 3 explains why, section 4 shows where the minutes go, section 5 is the work.

## 2. How it works today

Read this section before touching anything. It is the map an agent needs to change the machinery without breaking its laws.

### 2.1 The testing contract

`metasystem/testing.json` is the single contract. It has 19 surfaces, 63 groups, an `always` block, an `unknown` list, and a `cadence` list.

- A surface maps path globs to group lists: `standard`, `deep`, `critical` (obligation names), `crossCutting` (obligation names), and `dependsOn` (other surface ids). Example: surface `testing-policy` covers `internal/testpolicy/**`, `testing.json`, `cmd/metasystem/test.go` and friends; surface `proof-and-landing` depends on it; surface `dispatch-goal-mission` depends on `proof-and-landing`.
- A group has an `adapter`: `go` (a `go test` invocation over `packages` with either a named `tests` list or `"all"`), `command` (an argv, used by `fast-static-build` which runs `scripts/agents/go-gate.sh --fast`), or `section` (one section of the shell suite, see 2.3). Each group declares `inputs` globs, `obligations`, `targetMs`, and for `go` groups optionally `race` and `coverage`. No group in the file sets `race`. Only `goal-full-coverage` and `missionrunner-full-coverage` set `coverage`, each with `targetMs` 1800000 (30 minutes).
- `always.canary` is three small go groups; `always.standard` is `fast-static-build` plus six static sections.
- `cadence` lists 45 groups: the two coverage groups plus 43 `section/*` groups. This is the whole battery.

### 2.2 Selection: which groups a landing runs

Owner: `metasystem/internal/testpolicy` (pure Go, no processes). Entry from `metasystem/cmd/metasystem/test.go` (`test plan`, `test run`, `test verify`).

1. Changed paths between base commit and candidate tree are matched to surfaces (`metasystem/internal/testpolicy/select.go:251`, prefix match on `/**` globs). Consumers are added transitively through `dependsOn` (`select.go:134-144`). Because `dispatch-goal-mission` depends on `proof-and-landing` which depends on `testing-policy`, a touch in the policy surface reaches almost every surface.
2. A risk assessment is composed from `projectRisk` in the contract (severity 1, exposure 1), the accepted goal's own four risk answers (`--risk severity=,novelty=,exposure=,accumulation=` at `goal claim`, stored in `plans/goals/<goal>.md` as the `- Risk:` line), and per-surface raises (`metasystem/internal/testpolicy/risk.go`).
3. The required mode starts at `standard` and becomes `deep` when `requiresDeep` says so (`risk.go:106-109`):

   ```go
   return protected || risk.Severity >= 2 || risk.Exposure >= 2 || risk.Novelty >= 2 ||
       risk.Accumulation >= 2 || risk.Reversibility != "revert" || risk.Detection == "delayed" || risk.Recovery == "unbounded"
   ```

   `protected` means the diff touched the testing-policy surface (`ProtectedPolicyChange`, `metasystem/internal/testpolicy/protection.go`). Any path that matches no surface also forces deep and collapses the selection to `contract.Unknown` (`select.go:155-157`, `187-190`).
4. The required set is `always.canary` plus `always.standard` plus every matched surface's `standard`, plus, in deep mode, every matched surface's `deep`; `critical` obligations pull in their provider groups; `crossCutting` obligations join only when the goal's accumulation is 2 or more (`select.go:160-195`).
5. Purpose `cadence` ignores the diff and selects exactly `contract.Cadence` (`select.go:205-208`).
6. Stages run canary, then standard, then deep, each minus what earlier stages hold (`select.go:210-220`). Delivery is sufficient only when every required group is `passed` or `reused` and every obligation of a required group is covered (`metasystem/internal/proofrun/test_result.go:186-227`).

Facts that matter for slice 1:

- 125 of the 128 goals in `metasystem/plans/goals/` that carry a risk line satisfy `requiresDeep` (98 percent). Exposure 3 alone appears on about 50 of them. The most common tuples (severity, novelty, exposure, accumulation) are (2,1,2,1) 17 times, (2,1,3,2) 15, (3,2,3,2) 13, (2,2,3,2) 10.
- So "risk-selected" testing is in practice the whole battery on nearly every landing.

### 2.3 Execution: how the selected groups run

Owner: `metasystem/internal/proofrun`. `test run` admits one attempt and launches one worker child (`metasystem test worker`) through `LaunchSuite` in `metasystem/internal/proofrun/launcher.go`.

- `RunTestPlan` (`metasystem/internal/proofrun/test_build.go:73`) loops stages in order and groups in order, calling `runTestGroup` inline (`test_build.go:108-125`). There is no concurrency in the package apart from stdout copying in the launcher. Every selected group runs to the end; the first non-pass status is remembered but nothing stops early (ruling R-16, continue and collect).
- Each group gets a fresh detached git worktree of the candidate tree (`test_build.go:224`, `gittree.Workspace.NewDetachedWorktree`). Inputs are digested before and after. On any failure the worktree's suite-failure evidence is preserved under the control root before the worktree is removed (`PreserveDetachedSuiteFailures`). Worktree creation costs about a second; the smallest group (`policy-canary`) takes 1.3 to 1.9 s end to end.
- The `go` adapter builds `go test -json -count=1 [-race] [-cover] [-run '^(A|B)$'] ./pkg...` (`metasystem/internal/proofrun/test_go.go:76-106`). Test names come from a cached `go list -json -deps -test` discovery, so the adapter already knows every test name per package before launching. Per-test pass and fail identities are checked against that expectation. The Go build cache is shared across worktrees; a fresh path costs about 3 s for `go build ./...` and about 20 s to compile every test binary, so cold compile is not the cost.
- The `section` adapter runs `bash scripts/agents/validate-section-selector.sh run <section>` (`test_build.go:703-707`), which execs `bash scripts/validate-metasystem.sh --enumeration-section <section>` with `METASYSTEM_ENUMERATION_DRIVER=1` (`metasystem/scripts/agents/validate-section-selector.sh:128-137`). That is one full process of the 3125-line suite driver per section group. Inside it, `section_selected` (`metasystem/scripts/validate-metasystem.sh:65-77`) returns false for every section except the requested one and records the others as `excluded`. The engine is not rebuilt: proofrun copies the admitted candidate binary into the worktree's `bin/metasystem` and sets `METASYSTEM_ENUMERATION_ENGINE_DEPENDENCY=ready` (`test_build.go:329-345`). What every section process still pays: the selector `context` subprocess, the placeholder scan, the headroom check, `gate register`, the checkout execution guard, `fixture-budget-initialization`, and `validation-mode-accounting`. Measured floor: about 2 s (`section/static-placeholder-scan` does nothing and takes 2.0 s).
- A group is skipped as `reused` only when a prior terminal-success attempt owns the same execution identity (`test_build.go:87-106`). The identity includes the candidate engine digest for every section group (`test_build.go:641-665`), so any engine rebuild invalidates reuse for all sections at once. In the retained records `reusedLaunches` is 0 in every run.

### 2.4 The landing path and the cadence battery

- `metasystem/scripts/agents/commit.sh:271-278` runs `metasystem test verify --tree <index tree> --mode auto --purpose delivery`, which is read-only: it recomputes the plan and projects retained evidence, and refuses when a required group is missing. Then `commit.sh:311-317` runs `scripts/agents/go-gate.sh --fast --proof-out` (gofmt, vet, staticcheck, refusal register, build) and the audit, even though the proof run already executed `fast-static-build`. That duplicate costs about 9 s per landing.
- `metasystem/scripts/agents/land.sh` never runs tests for a schema-2 landing; it checks the receipt tree equals the staged tree and re-verifies after rebase.
- Every landing adds behavior-surface weight (`commit.sh` pipes numstat into `gate weight-add`; `metasystem/internal/gaterun/weight.go`). At the threshold, default 60 (`metasystem/cmd/metasystem/gate_weight.go:20-31`, key `validation.weight-threshold`), the standing validator's custodian runs the battery through the governed run boundary as described in `metasystem/docs/collaboration.md:40-56`: `run launch ... -- bin/metasystem test run --mode deep --purpose cadence`. Only that shape discharges weight. Current weight on this Mac: `artifacts/agents/battery-weight.json` shows 1394 accumulated over 60 landings since 2026-08-26.
- The cadence run adds `section/go-engine-gate`, which is `scripts/agents/go-gate.sh` in full: gofmt, vet, staticcheck, two Linux cross-builds, govulncheck, then `go test -race -cover -timeout 60m ./internal/...` (package-parallel), then `go test -race ./cmd/...`, then the build and the coverage ratchet (`bin/metasystem audit coverage-ratchet`). On this Mac the race run alone is about 10 minutes, bounded by `internal/missionrunner` at 570 s and `internal/goal` at 379 s. The gate runs inside a `git archive HEAD` snapshot to produce a witness (`metasystem/scripts/agents/witness-gate.sh`). If the snapshot gate fails for any reason, `witness-gate.sh:157-159` falls back to running the full plain gate again. On 2026-08-30 the snapshot refused on the coverage ratchet (three packages without floors), and the fallback re-ran everything: 21 minutes for a deterministic answer.

### 2.5 The shell fixture sections

The big sections are bash scripts under `metasystem/scripts/agents/` and `metasystem/scripts/adopt-fixtures.sh`. Structural facts:

- Bed legs re-execute the whole script. `metasystem/scripts/agents/fixture-bed-scenarios.sh:47` runs each scenario as `"$script" --fixture-bed-child "$scenario"` and waits for it before starting the next. `dispatch-fixtures.sh` (4407 lines) has 10 legs, `supervision-fixtures.sh` (2875 lines) 17, `goal-cli-fixtures.sh` 16, `brain-fixtures.sh` and `land-fixtures.sh` 9 each. Every leg pays the script's prologue again (temp dirs, `git init`, adapter copies, engine staging).
- Poll loops are cheap: `METASYSTEM_FIXTURE_POLL_INTERVAL_SEC` is 0.02 s (`metasystem/scripts/agents/fixture-budget.sh:402,422`). The cost is in the work between polls, in literal sleeps, and in re-execution.
- `adopt-fixtures.sh` copies the tracked tree six times, runs `adopt.sh` seven times, runs the full `validate-metasystem.sh` twice nested (`:609`, `:628`), and calls `run_adoption_comparison` (a 30-minute-capped `go test -run '^TestAdoptionComparisonSelectedScenarios$' ./cmd/metasystem`) from two places, `:641` and `:743`. The retained log shows the two runs took 143 s and 93 s inside one 531 s section. `metasystem/plans/application-testing-contract-design.md` already names this pair as a duplicate to narrow ("the copied-registration variant runs another").
- `dispatch-fixtures.sh` builds a private engine three times (`:46`, `:169`, `:1164`) and copies repositories 13 times. Inside the section the fixture emits a `CENSUS-WAIT-MEASUREMENT` line per dispatch step; in the retained log there are 107 of them across 406 s, with a median gap of 3.5 s. So the section is about 100 sequential real dispatches at 3.5 s each, plus three long waits (46 s around "cannot mirror cancel-husk", 19 s, 16 s).
- `supervision-hook-fixtures.sh:376` defines a fake brain engine whose `sleep` failure mode does `sleep 30`, run once inside the failure-mode loop at `:393`. `supervision-fixtures.sh` has literal `sleep 1`, `sleep 2.5` and a background `/bin/sleep 2` in the stop-hook scenario (`:2837-2852`).

## 3. Why an hour

Ranked by contribution.

1. Deep mode is the default in practice, not the exception. `requiresDeep` fires on 98 percent of goals, so nearly every delivery runs the two coverage packages and every deep section of every touched surface. This is the whole difference between the 55 s runs and the 55 min runs.
2. Groups run one at a time. In the 56 min run the five largest groups sum to 40 minutes; on 18 cores they could overlap.
3. The two coverage packages are serial inside themselves. `internal/goal` has 382 tests (median 0.7 s, 176 tests over 1 s) and `internal/missionrunner` 302 (25 tests over 5 s account for 311 s). There is no `t.Parallel` anywhere in the 480 test files.
4. The fixture sections are serial inside themselves too, and the dispatcher one has a 3.5 s floor per dispatch.
5. Specific waste: the adoption comparison test run twice, one 41 s test, a 30 s fixture sleep, the witness fallback re-running a deterministic refusal, a 2 s prelude per section process, and the duplicate fast gate at commit.

## 4. Measurements

### 4.1 Per group, median over the retained delivery runs

| Group | Median | Max | In how many runs |
|---|---|---|---|
| section/dispatcher-adapter-and-mission-runner-fixtures | 574 s | 606 s | 10 |
| goal-full-coverage | 538 s | 596 s | 6 |
| section/adoption-fixtures | 490 s | 586 s | 6 |
| missionrunner-full-coverage | 444 s | 496 s | 6 |
| section/supervision-and-census-fixtures | 375 s | 405 s | 10 |
| governed-standard | 154 s | 190 s | 6 |
| section/goal-cli-fixtures | 89 s | 100 s | 10 |
| section/mission-fixtures | 81 s | 89 s | 10 |
| section/land-fixtures | 76 s | 88 s | 6 |
| section/static-reproof-fixtures | 40 s | 45 s | 6 |
| section/brain-fixtures | 33 s | 36 s | 10 |
| landing-command-standard | 16 s | 17 s | 6 |
| section/witness-gate-fixtures | 15 s | 19 s | 6 |
| runtime-owner-standard | 15 s | 17 s | 10 |
| section/conformance-fixtures | 15 s | 16 s | 6 |
| section/supervision-go-fixtures | 13 s | 14 s | 10 |
| section/gate-fail-open-tripwire | 11 s | 13 s | 6 |
| fast-static-build | 9 s | 11 s | 13 |
| everything else (25 groups) | 1 to 8 s each | | |

Attempt `proof-mtvajtsi-7a9b8bd5511e0dd2` (43 groups, 3375 s) is the reference run; its per-group logs are under `metasystem/artifacts/agents/proof-runs/proof-mtvajtsi-7a9b8bd5511e0dd2/testing/*/groups/`.

### 4.2 Inside the Go groups (from the `-json` logs of the reference run)

| Group | Tests | Sum | Slowest |
|---|---|---|---|
| goal-full-coverage | 382 | 593 s | TestPriorityRace 12.2 s, TestMixedArcJoinUsesOwnPairOrNewestAllParkedRecord 12.2 s, TestMixedArcCascadesMoveOnlyEligibleMembers 11.9 s; 30 tests over 5 s sum to 228 s |
| missionrunner-full-coverage | 302 | 492 s | TestInternalRunOverloadedHostStaysOffTheBreaker 23.9 s, TestInternalRunDispatchTerminalCycle 21.5 s, TestInternalRunFullCycle 21.1 s, TestResolveTaintRestore 20.8 s; 25 tests over 5 s sum to 311 s |
| governed-standard (gaterun + steward, all tests) | 294 | 155 s | TestNarrationCapsItsHistory 41.0 s, TestSlowFirstAttemptSurvivesSecondEnsureAndWatcherRepair 11.2 s |

`TestNarrationCapsItsHistory` (`metasystem/internal/steward/narrate_test.go:50`) calls `Narrate` 2025 times; each call reads the file back and rewrites it through `atomicfile.WriteText`, which fsyncs (`metasystem/internal/atomicfile/atomicfile.go:130`).

### 4.3 Sharding experiment (no code change)

Test names were listed with `go test -list '^Test'`, split round robin into 8 lists, and run as 8 concurrent `go test -count=1 -run '^(...)$'` processes from an extracted copy of HEAD. Plain mode, no `-cover`, one sample each.

| Package | Serial (reference run) | 8 shards, wall | Failures |
|---|---|---|---|
| internal/goal | 593 s | 134 s | 0 |
| internal/missionrunner | 492 s | 132 s | 0 |

Race and coverage overhead for reference: `internal/lease` takes 7.4 s wall plain and 14.9 s with `-race -cover`; `internal/humanauthority` 4.8 s and 9.7 s.

### 4.4 Goal risk answers

Of 128 goals with a risk line, 125 force deep. Only three goals carry (1,1,1,1).

## 5. The work

Five slices, in order. Each has a DONE sentence, the files it touches, how to prove it, and what it must not break. Slices 2 to 5 are independent of the decision in slice 1 and can start now.

### Slice 1: per-landing verification is standard; deep runs at cadence

DONE: `test plan --mode auto --purpose delivery` on an ordinary goal selects canary plus standard only, and a delivery run completes in about a minute; deep still runs at the weight cadence, on protected policy changes, on unknown paths, and on explicit request.

This is the law change and needs Wido's word (section 6, decision 1). Three ways to do it:

- Option A: raise the thresholds in `requiresDeep` to 3. Cheapest, but exposure 3 alone is on about 50 goals, so roughly 40 percent of landings stay deep. Not enough on its own.
- Option B (recommended): the goal's risk answers stop choosing per-landing depth. Per-landing depth is decided by the change: protected policy change, unknown paths, a surface whose `critical` obligations demand it, or an explicit `--mode deep`. The goal's risk answers instead scale cadence weight (a severity 3 goal reaches the battery sooner) and may add a one-time deep run when the goal concludes (R-3: "or before concluding a large goal"). This matches R-3 exactly.
- Option C: keep `requiresDeep` but shrink what deep means: move the two coverage groups and the big process sections out of every surface's `deep` list into `cadence` only, so deep for a dispatch change no longer drags adoption, supervision and both coverage packages. Can be combined with A or B.

Files: `metasystem/internal/testpolicy/risk.go` (`requiresDeep`), `metasystem/internal/testpolicy/select.go` (mode and required set), `metasystem/testing.json` (`deep` lists per surface), `metasystem/internal/gaterun/weight.go` if risk feeds weight, tests in `metasystem/internal/testpolicy/*_test.go`, and the prose in `metasystem/docs/collaboration.md:40-56`.

Watch out: `testing.json` and `internal/testpolicy` are the protected surface, so the landing that changes them runs deep plus every `testing-policy-*` group once. That is expected. `ModesDoNotLowerRequiredRisk` in `metasystem/internal/testpolicy/contract_test.go` and the protection tests encode the current law and will need updating with the new law, not deleting.

Proof: `bin/metasystem test plan --root . --goal <an ordinary goal> --mode auto --purpose delivery --json` shows `requiredMode: standard` and about 11 groups; a real delivery attempt record shows `actualDurationMs` near 60000; `test plan --purpose cadence` still lists all 45 cadence groups; a protected change still plans deep.

### Slice 2: groups run concurrently inside one attempt

DONE: `RunTestPlan` runs independent groups of a stage concurrently under a configurable cap, the attempt record and progress log are unchanged in shape, and the deep battery's wall time equals its longest group.

How: in `metasystem/internal/proofrun/test_build.go:108-125`, replace the inline loop with a bounded worker pool per stage. Keep the stage order (canary, standard, deep). Collect `GroupResult`s and append them in plan order so `result.Groups` stays deterministic. `testGroupProgress` start and end events will interleave; the reader in `metasystem/scripts/validate-metasystem.sh` and the steward's silence detection (`suite.progress-silence-min` in `metasystem/metasystem.conf`) key on section names, so interleaving is fine, but check `metasystem/scripts/agents/suite-progress-fixtures.sh` which asserts the progress grammar. `ChildDurationMS` stays the sum; add a `wallDurationMs` if the cost report needs it (`metasystem/internal/proofrun/test_cost.go` already has `WallDurationMS`).

Cap: a new contract or conf key (for example `testing.concurrency`), default `min(GOMAXPROCS/2, 6)` on the Mac and 2 on the 4-vCPU VM. Section groups spawn process trees (stewards, runners, fake adapters) so the cap is about processes, not cores.

Isolation is already there: each group has its own detached worktree, so `bin/`, `artifacts/`, `gate register` markers and the checkout execution guard are per worktree. Two things to verify by running two heavy sections side by side by hand before wiring the pool: that no fixture uses a fixed path under `TMPDIR` or `$HOME` (grep the fixture scripts for `mktemp` misses and for `METASYSTEM_SUPERVISION_REGISTRY_HOME` defaults), and that the fixture budget's timing caps (`fixture-budget.sh`, semantic caps scaled by census wall) do not trip under contention. The caps are hang bounds and the suite is documented to pass under load, but the census-wall scaling has never seen this shape of load.

Risk noted in memory: leaked fixture children compound. Concurrency makes a leak worse faster. The harness cleanup already sweeps its temp tree; keep `PreserveDetachedSuiteFailures` and worktree removal per group exactly as they are.

Proof: three consecutive deep cadence runs green with the pool on; wall time within 10 percent of the longest group; the steward's two-run observation window (`docs/collaboration.md`) sees the same group identities as before.

Expected: the battery drops from 56 min to about 10 min, bounded by the dispatcher section, before slice 3.

### Slice 3: shard the long groups

DONE: no single group takes more than about three minutes on the Mac.

3a, Go groups. Add a `shards` field to a `go` group in `metasystem/testing.json` (and to `testpolicy.Group` with contract validation). In `metasystem/internal/proofrun/test_go.go`, the discovery already yields every test name per package; partition the names round robin into `shards` lists, launch that many `go test -json -count=1 -run '^(...)$'` processes concurrently through the existing launcher, and merge the `-json` streams before the expected-versus-observed identity check. Measured: `internal/goal` 593 s to 134 s and `internal/missionrunner` 492 s to 132 s at 8 shards.

Coverage with shards: `checkGroupCoverage` (`test_go.go:448-487`) parses the `coverage: N% of statements` line per package, and per-shard percentages cannot be added. Run each shard with `-cover -args -test.gocoverdir=<group log dir>/cover` (all shards into one directory), then `go tool covdata percent -i=<dir>` prints the merged package percentage in the same shape `audit.ParseCoverage` reads. Verify this on Go 1.27 before relying on it; the fallback is `-coverprofile` per shard and a text-profile union.

The same shard mechanism should later feed the race gate (slice 5).

3b, sections. `fixture-bed-scenarios.sh` already takes the scenario names as arguments and runs each as a child, so scenarios are independent by construction. Give the big fixture scripts a `--scenario <name>` form, declare one section id per scenario in `metasystem/scripts/validate-metasystem.sh` (for example `dispatcher-fixtures/dispatch`, `dispatcher-fixtures/mission-runner`), list each as a `section/...` group in `metasystem/testing.json` and in `cadence`, and retire the umbrella group. The selector's `list` (`validate-section-selector.sh:104`) and the static contract audit that checks sections and groups correspond (`validate-metasystem.sh:990`) must be updated together or the audit refuses. Do the same for `adopt-fixtures.sh` legs and `supervision-fixtures.sh` legs.

Honest limit: the `dispatch` scenario alone is most of the dispatcher section (log lines 6 to 196 of 324, roughly 100 dispatch steps). After 3b it is a 4 to 5 minute group on its own. Getting under three minutes means either splitting that scenario's inner sequence into two or three scenarios, or lowering the 3.5 s per-dispatch floor in `metasystem/scripts/agents/dispatch.sh` (3121 lines; each dispatch arms supervision and calls the engine many times). Measure before choosing.

Proof: every new group passes three runs; `test plan --purpose cadence` lists the new groups and none of the retired ones; coverage floors in `metasystem/scripts/agents/coverage-ratchet.json` are still enforced at the merged percentage.

Expected after slices 2 and 3: deep battery about 3 to 5 minutes wall on the Mac.

### Slice 4: remove the local waste

Each item is independent and small. Savings are per run of the group.

1. `metasystem/scripts/adopt-fixtures.sh:641` and `:743` both call `run_adoption_comparison`. Prove the second run's inputs are the same as the first's (the design doc's condition) and drop it, or narrow it to the registration-only assertion. Saves 90 to 140 s.
2. `metasystem/scripts/agents/witness-gate.sh:157-159`: fall back to the plain gate only for environmental refusals (snapshot extraction, witness write, `witness_refusal` reasons). A test failure or a coverage ratchet refusal inside the snapshot is terminal. Saves about 10 minutes on every cadence run that fails, and makes the red readable.
3. `metasystem/internal/steward/narrate_test.go:50`: build the 2000-line history directly on disk, then call `Narrate` a handful of times to prove the cap. Saves about 40 s in `governed-standard`.
4. `metasystem/scripts/agents/supervision-hook-fixtures.sh:376`: the `sleep` failure mode only needs to outlast the hook's boot deadline; lower that deadline through the fixture environment and shrink the sleep to match. Saves about 30 s.
5. The dispatcher log's three long gaps (46 s after "cannot mirror cancel-husk", 19 s, 16 s) are waits, not work. Find what they wait for in `dispatch-fixtures.sh` and `dispatch.sh` and bound them. Saves up to 80 s.
6. `metasystem/internal/missionrunner`: the `TestInternalRun*` tests take 15 to 24 s each. `METASYSTEM_DRAIN_REAP_INTERVAL_MS` defaults to 5000 (`metasystem/internal/missionrunner/drain.go`) and only one test lowers it. Measure one of those tests with the interval at 300 ms; if the cycle is waiting on the reap, set it in a `TestMain` for the package. Not measured yet, so no number promised.
7. `metasystem/scripts/agents/commit.sh:311-317` re-runs `go-gate.sh --fast` after the proof run already executed `fast-static-build`. The design doc says commit owns a verify-only boundary. Saves 9 s per landing.
8. The 2 s prelude per section process. With slice 2 this stops costing wall time; skip unless the VM shows otherwise.

### Slice 5: the race gate in the cadence run

DONE: `section/go-engine-gate` completes in about 4 minutes on the Mac.

`go-gate.sh` already runs packages in parallel; its wall time is `internal/missionrunner` at 570 s and `internal/goal` at 379 s under `-race -cover`. Apply the shard mechanism from 3a to those two packages inside the gate (or have the gate consume the sharded groups' merged coverage and skip the packages), keeping the ratchet fed through `bin/metasystem audit coverage-ratchet`. Keep the gofmt, vet, staticcheck, cross-build and govulncheck stages as they are; they are seconds on a warm cache.

## 6. Decisions Wido has to make

1. The per-landing depth law (slice 1, options A, B, C). Recommendation: B, with C's move of the coverage groups to cadence only. This changes what a landing proves, so it is a ruling, not an implementation choice.
2. The concurrency default and the VM cap for slice 2.
3. Whether `governed-standard` keeps running all 267 steward tests in standard mode (154 s whenever `internal/steward` or `internal/gaterun` is touched) or gets a named list like the other standard groups.
4. Whether the `dispatch` scenario is split further or the per-dispatch floor is attacked (slice 3b's honest limit).

## 7. What must not change

- Every group keeps running at some cadence. No group leaves `testing.json`; the plan moves groups between per-landing and cadence, it never deletes them.
- Ruling R-16: runs continue and collect; every selected stage has a recorded result or the run is invalid. Concurrency must not introduce an early stop.
- Ruling R-18: a changed contract runs its callers; the landing that changes `testing.json` runs the protected set once.
- Weight discharge semantics (`gate weight-discharge` needs purpose cadence, deep required and executed) and the steward's two-run observation window stay as documented in `metasystem/docs/collaboration.md`.
- Decisions live in Go, plumbing in Bash (`metasystem/docs/architecture.md`). Sharding and the worker pool are Go changes in `internal/proofrun`; the fixture split is Bash plumbing plus contract entries.
- No scheduler, daemon or parallel proof store (`metasystem/plans/application-testing-contract-design.md`, section 2, point 5). The worker pool lives inside the existing single worker process.
- Evidence on failure is preserved per group exactly as today.
- The VM has 4 vCPUs. Nothing in this plan may make the VM's cadence run slower than today; cap concurrency there.

## 8. How to measure

Per-group durations of retained attempts:

```sh
cd metasystem
python3 - <<'EOF'
import json, glob, statistics, collections
g = collections.defaultdict(list)
for f in glob.glob('artifacts/agents/proof-runs/attempts/*.json'):
    d = json.load(open(f))['testResult']
    print(f, d['requiredMode'], d['purpose'], len(d['groups']), d['cost']['actualDurationMs'] // 1000, 's')
    for x in d['groups']:
        g[x['id']].append(x['durationMs'])
for k, v in sorted(g.items(), key=lambda kv: -statistics.median(kv[1])):
    print(f"{statistics.median(v)/1000:8.1f}s  {k}")
EOF
```

Slowest tests inside a `go` group: read `groups/<group>.log` in the attempt's `testing/<id>/` directory; each line is `go test -json`, and `Action: pass` with a `Test` field carries `Elapsed`.

Sharding by hand, from an extracted copy so the checkout is untouched (`/bin/bash` on the Mac is 3.2 and has no `mapfile`, hence Python):

```sh
S=$(mktemp -d); git -C metasystem archive HEAD | tar -x -C "$S"; cd "$S"
python3 - internal/goal 8 <<'EOF'
import subprocess, sys, time, re, os
pkg, n = sys.argv[1], int(sys.argv[2])
env = dict(os.environ, GOFLAGS='-mod=readonly')
tests = [l for l in subprocess.run(['go','test','-count=1','-list','^Test','./'+pkg], capture_output=True, text=True, env=env).stdout.splitlines() if l.startswith('Test')]
t0 = time.time(); ps = []
for s in range(n):
    pat = '^(' + '|'.join(re.escape(t) for t in tests[s::n]) + ')$'
    ps.append(subprocess.Popen(['go','test','-count=1','-run',pat,'./'+pkg], stdout=open(f'shard{s}.log','w'), stderr=subprocess.STDOUT, env=env))
print([p.wait() for p in ps], f'{time.time()-t0:.0f}s wall')
EOF
```

A plan for a goal without running anything: `bin/metasystem test plan --root . --goal <goal> --mode auto --purpose delivery --json`.

## 9. How much work, and how it is cut into goals

| Slice | Effort | What it buys | Design-bearing |
|---|---|---|---|
| 1. Per-landing depth law | 2 to 4 h once Wido has ruled; mostly test and doc updates | 56 min to about 1 min per delivery | yes, it is a ruling |
| 2. Concurrent groups in proofrun | 1 to 2 days, most of it proving fixture isolation under load | battery 56 min to about 10 min | yes |
| 3. Sharding Go groups and splitting sections | 2 to 3 days; the dispatch scenario split is open-ended | battery to 3 to 5 min | yes |
| 4. Local waste items | 30 min to 2 h each; item 6 is an investigation first | minutes per run each | no |
| 5. Race gate sharding | half a day after 3a exists | cadence gate 10 min to about 4 min | no, reuses 3a |

Slice 1 pays for itself at the next landing. The battery fires about every three landings today (weight 1394 over 60 landings against a threshold of 60), so slices 2 and 3 matter, but far less per day than slice 1.

This work is not folded into another goal. Three reasons:

- Slice 1 changes what every landing proves. It sits on the protected testing-policy surface, so its landing runs the protected set, and ruling R-18 says a changed contract runs its callers. Inside an unrelated goal that proof and receipt are unreadable, and it is the scope creep Wido flagged on 2026-09-09.
- Slices 2 and 3 change the proof machinery itself. A change to the judge must not land inside a landing the judge is certifying. It needs its own goal, its own battery, and three green cadence runs before anyone trusts it.
- Under ruling R-17 these are five to ten four-hour slices. Any goal they ride on blows its cap.

The cut:

1. Goal `landing-runs-standard-deep-runs-at-cadence` for slice 1. A law change, so per ruling R-2 it is drafted and brought to Wido; only his word opens it. Decision 1 in section 6 is that word.
2. Goal `deep-battery-under-ten-minutes` for slices 2, 3 and 5, arc-sized, three or four slices of about four hours each.
3. One small goal per slice 4 item, straight to the backlog per R-2. These are the only parts that may ride along with other work: a seat already editing `internal/steward/narrate_test.go` or `scripts/agents/supervision-hook-fixtures.sh` may take the matching item without changing its own goal's scope, and says so in its receipt.

## 10. What M1E does, in order

M1E owns this stream. Role lanes per ruling R-25: Fable designs, Sol critiques designs, Sol implements, Fable critiques implementations. Slices 1, 2 and 3 are design-bearing and get one design critique round each; slice 4 items and slice 5 go straight to implementation and code critique.

### Step 0: read

Read sections 2, 4 and 7 of this file, then `metasystem/memory/rulings.md` entries R-2, R-3, R-16, R-17, R-18, then `metasystem/docs/collaboration.md:40-56`. Confirm the reference attempt still exists: `metasystem/artifacts/agents/proof-runs/attempts/proof-mtvajtsi-7a9b8bd5511e0dd2.json`. If it is gone, the per-group numbers in section 4 stand as recorded here.

### Step 1: open the goals

Open two goals with `bin/metasystem goal open` and one small goal per slice 4 item. Records to write:

`landing-runs-standard-deep-runs-at-cadence` (draft until Wido's word):

- Intent: `test plan --mode auto --purpose delivery` on an ordinary goal selects canary plus standard only and a delivery run completes in about a minute; deep still runs at the weight cadence, on protected policy changes, on unknown paths, and on explicit request. DONE is that sentence.
- Risk: severity=2 novelty=1 exposure=3 accumulation=1, basis: severity 2 because a landing proves less than today until the cadence battery catches what standard missed; novelty 1 because the mode rule and the deep lists already exist and only their inputs change; exposure 3 because every landing on every seat is affected; accumulation 1 because the cadence run still sweeps the whole battery.
- Budget: elapsedLimit=4h attemptLimit=4 activeJobLimit=1 reviewRoundLimit=2.
- Next step: wait for Wido's answer to section 6 decision 1, then step 2.

`deep-battery-under-ten-minutes`:

- Intent: the cadence run (`test run --mode deep --purpose cadence`) completes in under ten minutes wall on the 18-core Mac with every cadence group still present and passing, and no slower than today on the 4-vCPU VM. DONE is that sentence.
- Risk: severity=2 novelty=2 exposure=3 accumulation=2, basis: severity 2 because a wrong merge of shard results could pass a red test unseen; novelty 2 because a worker pool and shard merging are new shapes in proofrun; exposure 3 because every cadence run and every seat consumes the result; accumulation 2 because a flake introduced by concurrency compounds across landings.
- Budget: elapsedLimit=3d attemptLimit=10 activeJobLimit=1 reviewRoundLimit=3.
- Next step: step 3.

Slice 4 small goals, one each, risk severity=1 novelty=1 exposure=1 accumulation=1, budget elapsedLimit=2h: `adoption-comparison-runs-once` (item 1), `witness-fallback-only-on-environmental-refusal` (item 2), `narration-cap-test-writes-its-history-once` (item 3), `brain-boot-sleep-fixture-matches-its-deadline` (item 4), `dispatcher-fixture-long-waits-bounded` (item 5), `missionrunner-cycle-tests-lower-the-reap-interval` (item 6, investigation first), `commit-does-not-repeat-the-fast-gate` (item 7).

### Step 2: slice 1, after Wido's word

1. Design (Fable): a one-page design under `metasystem/plans/landing-runs-standard-design.md` that states the chosen option from section 5 slice 1, the new `requiresDeep` rule in words, which `deep` entries in `metasystem/testing.json` move to cadence only, and how goal risk feeds weight if option B. One critique round (Sol). Stop when findings stop changing what gets built.
2. Implement (Sol): `metasystem/internal/testpolicy/risk.go` (`requiresDeep`), `metasystem/internal/testpolicy/select.go` if the required-set composition changes, `metasystem/testing.json` `deep` lists, `metasystem/internal/gaterun/weight.go` if risk scales weight, the tests in `metasystem/internal/testpolicy/` (update `TestModesDoNotLowerRequiredRisk` and the protection tests to the new law, never delete them), and the prose in `metasystem/docs/collaboration.md:40-56` and the R-3 row's enforcement note in `metasystem/memory/rulings.md`.
3. Prove before landing: `bin/metasystem test plan --root . --goal account-provenance-carried --mode auto --purpose delivery --json` shows `requiredMode` standard and about 11 groups; the same with `--purpose cadence` lists all cadence groups; `test plan` on a diff touching `metasystem/internal/testpolicy/` still plans deep; the landing's own delivery attempt record shows `actualDurationMs` under 120000 for a later ordinary landing.
4. Code critique (Fable), then land through `scripts/agents/land.sh`. Expect this landing itself to run the protected set once; that is the law working.
5. Receipt names the before and after minutes from real attempt records.

### Step 3: slice 2, concurrency

1. Spike first, no code: run two heavy section groups side by side by hand from two detached worktrees (`git worktree add --detach`), for example `bash scripts/agents/validate-section-selector.sh run supervision-and-census-fixtures` and `... run dispatcher-adapter-and-mission-runner-fixtures`, and watch for shared paths, fixture budget cap trips, and leaked children (`ps` before and after). Record what was seen in the goal's next step. Grep the fixture scripts for fixed paths under `TMPDIR` or `$HOME` and for `METASYSTEM_SUPERVISION_REGISTRY_HOME` defaults.
2. Design (Fable): the worker pool in `RunTestPlan`, the cap key and its defaults (Mac and VM), result ordering, progress event interleaving, and the failure path (a failed group must still preserve its evidence while others run). One critique round (Sol).
3. Implement (Sol): `metasystem/internal/proofrun/test_build.go:73-145`, `metasystem/internal/proofrun/test_cost.go` if wall time is reported, contract or conf key for the cap, tests in `metasystem/internal/proofrun/`, and `metasystem/scripts/agents/suite-progress-fixtures.sh` if the progress grammar needs interleaving allowed.
4. Prove: three consecutive green cadence runs on the Mac with the pool on; wall time within 10 percent of the longest group; one cadence run on the VM with the VM cap no slower than today. Record the three attempt ids in the receipt.
5. Code critique (Fable), land.

### Step 4: slice 3, sharding

1. Spike: on Go 1.27 confirm that several `go test -cover ./internal/goal -run '^(...)$' -args -test.gocoverdir=DIR` shards into one DIR followed by `go tool covdata percent -i=DIR` prints one merged `coverage: N% of statements` line for the package. If not, the fallback is `-coverprofile` per shard and a text-profile union computed in Go.
2. Design (Fable): the `shards` group field, the partition rule, stream merging before identity checking, coverage merging, and the section split (new section ids, selector `list`, the static contract audit, `cadence`). One critique round (Sol).
3. Implement (Sol), in two landings. First 3a: `metasystem/internal/testpolicy/contract.go` (field and validation), `metasystem/internal/proofrun/test_go.go` (partition, launch, merge, coverage), `metasystem/testing.json` (`shards` on the two coverage groups). Second 3b: `--scenario` entry in `metasystem/scripts/agents/dispatch-fixtures.sh`, `metasystem/scripts/adopt-fixtures.sh`, `metasystem/scripts/agents/supervision-fixtures.sh`; new sections in `metasystem/scripts/validate-metasystem.sh`; `metasystem/scripts/agents/validate-section-selector.sh` `list`; the audit at `validate-metasystem.sh:990`; groups and `cadence` in `metasystem/testing.json`.
4. Prove: `goal-full-coverage` and `missionrunner-full-coverage` each under 150 s with the ratchet floors still enforced at the merged percentage; no section group over 300 s; `test plan --purpose cadence` lists the new groups and none of the retired ones; three green cadence runs.
5. Code critique (Fable), land each.
6. Measure the `dispatch` scenario on its own. If it is still over three minutes, write the finding into the goal's next step and stop there; whether to split it further or attack the 3.5 s per-dispatch floor in `metasystem/scripts/agents/dispatch.sh` is section 6 decision 4.

### Step 5: slice 5 and the slice 4 goals

Slice 5 follows 3a directly: apply the same shard launch to `internal/goal` and `internal/missionrunner` inside `metasystem/scripts/agents/go-gate.sh`, or have the gate consume the sharded groups' merged coverage and skip the two packages; prove the ratchet still refuses a lowered floor.

The slice 4 goals are claimable by any seat at any time. M1E takes them in the gaps between the steps above, one landing each, code critique only.

### What M1E must not do

- Land slice 1 without Wido's word on section 6 decision 1.
- Carry any of this inside another goal's landing.
- Remove a group from `metasystem/testing.json`. Groups move between per-landing and cadence; none is deleted.
- Add an early stop to a run. Ruling R-16 stands: every selected group runs and every result is recorded.
- Change the concurrency cap or shard counts on the VM without a measured run there.
- Report a slice done from a single green run. Slices 2 and 3 need three consecutive green cadence runs, with the attempt ids in the receipt.
