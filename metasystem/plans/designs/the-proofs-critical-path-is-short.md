# The proof's critical path is short

- Kind: design
- Id: the-proofs-critical-path-is-short
- Status: accepted
- Goals: the-proofs-critical-path-is-short the-proofs-critical-path-is-reported
- Critique: round 1 (Astra, `designs/the-proofs-critical-path-is-short-critique-1.log`): 2 material, 7 minor; verdict build after the folds; all nine accepted and folded in revision 2; on the U1 brief, round 1 (Astra, `designs/the-proofs-critical-path-is-short-U1-brief-critique-1.log`): 7 material, 3 minor; B1 and B2 folded here in revision 3, the rest in the brief

Accepted 2026-11-04 by a1 in Wido's name (decision 197, throughput first): the design critique loop closed at round 1 (no critical finding; every finding folded in revision 2); the gain is an estimate until the N=1 against N>1 comparison the page names.

Revision 3 (decision 197, Astra B1/B2 on the U1 brief), 2026-11-04, written by opus-designer (Claude Opus 5.5) from the coordinator's decisions in Wido's name. The status stays accepted; only the delivery and the Units table change. No decision text changes.

- B1, the delivery. This goal delivers U1a and U1b only. It hands in after U1b, which is built with `work build --last`. So the N=1 against N=4 measurement (section 6) runs on deployed U1a and U1b before anything else is built. A Units table that declared U2 to U4 would refuse the hand-in until they were built (`metasystem/cmd/metasystem/intent_delivery.go:2554-2565`).
- U2 (the readout, D4), U3 (the order, D2) and U4 (the load-fragile record, D5) move to a split follow-up goal, `the-proofs-critical-path-is-reported`, opened blocked by this goal. Their design text stays on this page, marked as the follow-up's (section 4b). The follow-up builds them under its own Units table on its own split page.
- B2, the rows. The engine matches a build's unit name to a row's name exactly, or as a prefix followed by "." or a space (`metasystem/cmd/metasystem/intent_delivery.go:2462-2464`; `metasystem/internal/launch/admit.go:224`). It reads only the first sized table, with a header cell exactly `Unit` and a production column exactly `Production lines` (`metasystem/internal/launch/unit_sizes.go:31-44`, `admit.go:207-210`). A production cell counts only when the whole trimmed cell is an integer (`unit_sizes.go:73-85`). An unsized row is refused (`admit.go:233-234`). "about 106, doubled 212: …" read as unknown. So the Units table now has the rows `U1a` and `U1b`, with the integer cells 113 and 35. The U1 brief's recount after Astra B8 (gofmt realignment) and B6 (a named stamping helper) raised them from 109 and 33. Build names and commit trailers match the rows.

Revision 2, 2026-11-04, written by opus-designer (Claude Opus 5.5): the folds of Astra's round 1, with decisions taken by the steward in Wido's name (folds 1 to 9 of the coordinator's message). D1 now chooses N at each package launch's admission from a live load sample and records it per launch (finding 1); T1 holds the admitted cohort (finding 2); D4 separates work, the longest launch and the span (finding 3); D2 admits through one dispatcher with per-launch weights (finding 4); D5 records "failed in parallel, passed alone" and claims only tested coverage (finding 5); the gain stays an estimate until an N=1 against N>1 comparison (finding 6); section 1's measurement statements are corrected (finding 7); the mutation claims are completed (finding 8); U1 and U2 are recounted (finding 9). D3 stays deferred (Astra's Q6). Revision 1 is not kept beside this page; its text is in the critique log's quoted line numbers.

Revision 1, the same day: the first draft from decision 197's measurement. Code read at m1e main `cb112adb78`; every `file:line` below was read there. The proof log and scope record were read in the landing checkout (`agentic-tools-landing/metasystem/artifacts/agents/landing/proofs/20261104T104910.935087000Z.{log,scope.json}`, results row 1022 of `results.jsonl`). Numbers I computed from the log are marked "computed"; Astra recomputed the package sum, the floor, the leaf split and the CSV means and found them the same. A number nobody measured is marked "estimate" or "hypothesis".

Premises corrected against the request:

1. The eight adopt integration tests take 34.5 minutes of test time, not 18.4. The 18.4 is the top-level `Elapsed`, which leaves out their parallel subtests.
2. The adopt fixture is already shared: one template snapshot and one engine per package run, through `sync.Once` (`metasystem/internal/adopt/adopt_git_integration_test.go:45-55`, `:204-269`, `:271-289`). D3 as asked would change nothing.
3. The longest package is not the only bound. The summed package time over the 18 worker slots is 38.0 minutes, almost the 40.5 of the longest package.
4. How much CPU the proof uses is not known. The sampler's means (5.5 cores for all processes, load 17.0) are whole-host readings and bound nothing about the proof. U1 records the CPU each package launch captured.
5. A quarter of today (about 12 minutes) is out of reach for this page. Section 7 names what it would take.

## 1. Problem

### Intent

The landing lane's full proof ends in about the time of its slowest launch. Tests that declare themselves parallel run in parallel inside their package, and each launch takes that parallelism only when the host has spare CPU at the moment it starts. A test that fails in parallel and passes alone is named so. A full proof of main or of a batch takes well under half of today's time, and every proof says its work, its longest launch and its span.

### Evidence, measured 2026-11-04 (decision 197)

- The proof of main, attempt `20261104T104910.935087000Z`, green: 50.1 minutes from start to result (`results.jsonl` row 1022, `minutes` 50.07). The reporter's own clock says 46.9 (`landing clock total 2816903`, log line 212212). The 3.2 minutes before are the worktree and the reporter build (`metasystem/proof/full.sh:6`).
- Inside the reporter: `refusal-register-standard` 9.7 s and `fast-static-build` 170.6 s run first, serially (log lines 227-228; scope record `durations`). Then 239 package launches (`landing planned 239`, log line 229).
- Computed from the 239 `landing package` lines: summed 683.3 minutes. `cmd/metasystem` is 54 shards and 414.1 of those minutes; its longest shard is 21.9. The longest launch is `internal/adopt`, 40.5. Then `internal/launch` 30.7, `internal/steward` 28.8, `internal/landing/plain` 22.3, the `cmd/metasystem` shard 11 at 21.9, `internal/goal` 21.0, `internal/goal/branch` 16.9, `internal/proofrun` 13.0.
- Computed from the JSON `start` and package `pass` events: at most 18 launches at once. 683.3 / 18 = 38.0 minutes, the floor of the package phase at 18 slots if durations stay as they are.
- Order, computed: `internal/adopt` started 3.0 minutes after the first JSON event of the log (a group's event, before the native phase) and ended at 43.5; the last `cmd/metasystem` shard started at 38.1 and ended at 46.5.
- Test time, computed from leaf tests: 649 minutes; 506 in tests that call `t.Parallel()` (a `pause` event in the log), 143 in serial tests.

| Package | Launch minutes | Serial test minutes | Parallel test minutes | Longest parallel test (leaf sum) |
|---|---|---|---|---|
| cmd/metasystem (54 shards, summed) | 414.1 | 102.8 | 289.7 | TestProviderRecoveryAskPublicLifecycle 7.7 |
| internal/adopt | 40.5 | 0.0 | 34.5 | TestAdoptGitIntegrationSeedsOrKeepsTheLandingRef 7.8 |
| internal/launch | 30.7 | 1.2 | 29.4 | TestReviseRefusedWhenMaterialDoesNotFall 2.1 |
| internal/steward | 28.8 | 7.3 | 21.5 | TestAFailingDutyIsARowAndStandsAtFive 3.1 |
| internal/landing/plain | 22.3 | 0.0 | 22.3 | TestLaneFixTopologyRealGitRefusals 2.3 |
| internal/goal | 21.0 | 0.0 | 20.3 | 0.6 |
| internal/goal/branch | 16.9 | 0.1 | 15.9 | TestLandingPublicationGitAdapter 2.9 |
| internal/proofrun | 13.0 | 3.9 | 8.9 | 1.1 |

- `internal/adopt`, computed: the template snapshot is built once, inside `TestAdoptGitIntegrationWritesOnlyTheDeclaredInventory`, 11:55:06 to 11:58:58 (3.9 minutes). The integration leaves then run one at a time, 34.5 minutes. The longest leaf logs its steps: engine ready after 1.8 s, adoption after 2m23s, `git add -A` after 5m04s, commit after 5m48s, end 375.7 s (`adopt_git_integration_test.go:1057-1124`, log output 12:00:47 to 12:07:02).
- The sampler (`panes/measure-proof-cpu.sh`, `panes/proof-cpu-20261104-1003.csv`): 46 samples, 44 to 158 seconds apart (the script sleeps 30 s, `measure-proof-cpu.sh`, plus its own `landing status` calls). Each sample sums `ps` CPU over a population: the `go test` and `.test` processes (mean 36%, max 191%), and all processes on the host (mean 547%); and the one-minute load (mean 17.0 on 18 cores, from 7.8 to 33.9). The first package lines appear by the 11:55:42 sample (37 of 239 done, load 11.84), so no sample marks the native start. The results row has load 24.1 at the proof's start and 12.1 at its end.
  - Hypothesis, kept as one: `ps` misses processes that start and end between samples (the tests' `git` and engine children), and the pattern does not match them either. This bounds nothing about the proof's CPU; other seats share the host.
- The last sixteen full rows of `results.jsonl` (rows 964 to 1022, 2026-11-03 14:08 to 2026-11-04 11:39): 43.3 to 69.25 minutes; fifteen of them 43.3 to 50.1, row 969 at 69.25.

### What the code does

- Every Go package launch gets `-p=1 -parallel=1` (`metasystem/internal/proofrun/test_go.go:354`, in `goNativeTestArguments`, `:349-365`), pinned by `TestGoNativeArgumentsPinEveryNestedParallelismLayer` (`metasystem/internal/proofrun/test_go_workers_test.go:28-57`). So a `t.Parallel()` test has never run beside another test of its package in a proof. Inference: the same argv serves every testing group, so no check on this host has run them in parallel. One test runs its own child at `-test.parallel=21` (`metasystem/cmd/metasystem/provider_recovery_test.go:36-39`); it was the load flake of 2026-10-24/25.
- Every test process runs with `GOMAXPROCS=1` (`metasystem/internal/proofrun/test_build.go:1501-1504`; groups at `:3106-3109`; the rerun at `metasystem/internal/proofrun/test_rerun.go:74-77`).
- The full proof: groups first (`metasystem/internal/repoproof/full.go:174-206`), one native launch set for `./...` (`:281-298`), the `batchtest` pass (`:299-301`), the section reports (`:303-347`). The native request sets neither `Race` nor `Coverage` (`:219`), so the coverage-only rerun of failed tests does not run here (`test_go.go:219-226`). A replay sets `LANDING_ONLY`, so `full` is false (`full.go:149-174`). The reporter's clock origin is `started` (`full.go:118`), common to both native calls.
- Native requests carry no result schema, so `reusePolicy` forces `-count=1` on every launch (`test_go.go:185`; `test_build.go:238-245`; `metasystem/internal/proofrun/reuse_policy.go:53-56`; Astra's Q5). No cached pass is lost by changing `-parallel`.
- Partitions: one package per launch, sorted by name; `cmd/metasystem` dealt into `3 × workers` shards (`test_go.go:906-923`). Workers are the reporter's `GOMAXPROCS`, 18 here, unless an environment override is set (`test_go.go:132-147`).
- The order is not kept. `runGoPartitions` starts one goroutine per partition (`test_build.go:1518-1631`); each waits for its package's readiness and then `acquireTestWorkers(…, 1)` (`:1550-1561`). A release wakes every waiter by closing one channel (`metasystem/internal/proofrun/test_workers.go:129-139`), and the waiters race (`:42-61`). A goroutine that is late to register can still win.
- Go keeps serial tests serial: a top-level test without `t.Parallel()` runs alone, and the parallel ones resume after. The ratchet counts declarations, not safety (`metasystem/internal/parallelratchet/parallel.go:216-235`, `:239-273`; refused at `metasystem/cmd/devgate/owners.go:93-114`).
- The supervisor's CPU for a launch is the larger of its sampled process-tree CPU and the waited process's rusage (`metasystem/internal/proofrun/supervisor.go:278-286`). On darwin the waited rusage includes descendants reaped through that chain; detached or unwaited descendants are not counted (Astra's Q1, from XNU `kern_exit.c`). It is captured native-launch CPU, not the proof's whole CPU. `runGoPartitions` sums it (`test_build.go:1641`) and drops it. `supervisorOutcome` has no start time (`supervisor.go:73-88`); the start is a local at `:212`.
- The proof writes `landing package <unit> <shard> <status> <ms>` (`full.go:229`, `:246`); the lane parses exactly six fields into the scope record (`metasystem/internal/landing/plain/scope.go:428-445`, `:24-34`, `:345`) and counts them for the running status (`metasystem/internal/landing/plain/status.go:620-636`, carried at `metasystem/internal/landing/plain/shadow.go:286` into `RenderInputs`, `metasystem/internal/landing/state/describe.go:18-26`). The clock prints the longest launch (`metasystem/internal/landing/plain/clock.go:134-152`, `:185-191`) and skips proofs of main (`clock.go:127`). The running line is "proving {subject} ({done} of {total} units, {minutes} min)" (`describe.go:73`, `:159-170`).
- `hostload.Read` returns load and cores; on failure it returns zero load with `Available` false (`metasystem/internal/hostload/hostload.go:26-35`). `Saturated` is load1 ≥ cores (`:39-41`). The host admission caps top-level proofs and exempts nested work; it does not cap a proof's children (`metasystem/internal/proofrun/admission.go:47-60`).
- A red is repeated alone before it is blamed, where the lane repeats: a proof of main always (`metasystem/internal/landing/plain/replay.go:118-127`); a batch only when the judge knows every failed unit and the tree has no prior result, otherwise it is replayed on the base without a repeat (`replay.go:128-148`). `repeatRed` runs each unit with `LANDING_ONLY` (`:152-166`, `:206-225`); a green repeat is a flake "alone" (`metasystem/internal/landing/plain/report.go:116-145`; the record `metasystem/internal/landing/plain/prove.go:163-169` keeps the red's `Load` and both outputs).
- The lane passes the proof its scope in the environment (`prove.go:1225-1227`); `proof.full` is `sh proof/full.sh` (`metasystem/metasystem.conf:16`). The reporter already reads a host fact from the lane, `host.proof-vm` (`full.go:81-92`; `metasystem/internal/config/defaults.go:88-89`).

## 2. Decisions

- **D1 Each package launch takes parallelism from the load at its admission, in the full proof only.** (Wido's name, fold 1.)
  - The rule. In a full proof (`full` true in `runHost`), when a launch has its worker slot, it reads one live sample. N is 1 when the sample is unavailable. Otherwise, with spare = cores − load1: N = 4 when spare is at least three quarters of the cores, 3 when at least half, 2 when at least a quarter, else 1; never above M. At 18 cores: load below 4.5 gives 4; 4.5 up to 9 gives 3; 9 up to 13.5 gives 2; 13.5 and above gives 1. The bands are a heuristic for Wido to set, not a CPU reservation (Astra's Q2).
  - M is the host setting `host.proof-package-parallel-max`, default 4, valid 1 to 16; 1 is today's behaviour. 0 is refused by settings validation (`metasystem/internal/config/validate.go`, beside `:715-719`), and a reporter that reads anything outside 1 to 16 uses 1 and says so.
  - The effective N is recorded per launch (D6). No N is frozen for the phase.
  - Serial tests stay serial by Go's rule. `-p=1` and `GOMAXPROCS=1` stay: a package's own Go code still uses one core; its children's CPU is what the sample watches.
  - The mechanism. `NativeInventoryRequest` gains `PackageParallel func() int` (`test_go.go:73-85`), carried into the run request (`:185`) as an unexported field. In `runGoPartitions`, after `acquireTestWorkers` (`test_build.go:1555-1561`) and before the command is built (`:1571`), the launch calls it and swaps the argv's `-parallel=1` for `-parallel=<N>` when N is above 1; N is kept on the run. `goNativeTestArguments` and its pin test do not change, so every testing group, unit check, rerun and replay keeps `-parallel=1`. `proofrun.PackageParallelFor(sample hostload.Sample, max int) int` holds the bands. The reporter builds the function from `hostload.Read` (through a `Load` seam on `HostRunners`, `full.go:27-32`) and M, and only when `full`.
  - The reporter reads M as it reads `host.proof-vm` (`full.go:81-92`). Unreadable (an older serving engine without the key, or any error) means M = 1, with the reason in its first line: `landing parallel max <M> workers <W>` or `landing parallel max 1 (<reason>) workers <W>`. W is the resolved worker count; `NativeInventoryResult` gains `Workers` (`test_go.go:89-98`, resolved at `:132-147`).
  - Rejected: one N for the phase from a reading at its start (revision 1). The load rose from 11.84 to 20.22 within five minutes of the native phase (CSV rows 6 to 8); a start reading says nothing about later launches (Astra finding 1).
  - Rejected: the ratchet as a selector (packages with no serial tests). It leaves `cmd/metasystem`, `internal/launch` and `internal/steward` serial; Astra computes it lowers the floor by about 3.5 minutes, which may be measurable but is not the shortening this goal asks for.
  - Rejected: a fixed N. Wido 2026-09-28.
  - Rejected: raising `GOMAXPROCS`. One package could take many cores before any sample sees it.

- **D2 One dispatcher admits ready launches, heaviest first.** (Wido's name, fold 4.) Built by the follow-up goal `the-proofs-critical-path-is-reported` (revision 3).
  - The lane passes the latest full scope record of its own as `LANDING_PRIOR_SCOPE=<path>` (`prove.go:1225-1227`). The reporter reads its `packages` (unit, shard, ms) and gives `NativeInventoryRequest.Weights`, keyed by launch: package and shard. A launch not in it weighs 0. The weights are a scheduling hint, never proof authority (Astra's Q7); a batch and main prove different trees, and `cmd/metasystem`'s shard numbers hold only while the worker count holds.
  - In `runGoPartitions`, the launch goroutines no longer call `acquireTestWorkers` themselves. After its readiness (`test_build.go:1550`), a launch registers as ready with one dispatcher for the run; the dispatcher holds the pool's slots and, whenever a slot is free, admits the heaviest ready launch, the lower partition index on a tie. Capacity and the pool (`test_workers.go`) do not change; the dispatcher is its only caller in this path.
  - Rejected: a ticket per partition in the pool (revision 1). A late registrant can still win (`test_build.go:1518-1555`; Astra finding 4).
  - Rejected: the package's summed time as every shard's weight (revision 1). It puts all 54 `cmd/metasystem` shards ahead of adopt.
  - Rejected: weights from the test count, or a committed timing file (revision 1's reasons hold).

- **D3 The adopt fixture is changed only if adopt stays the longest launch.** Premise 2. With D1 at N = 4, adopt is estimated at about 12.5 minutes (3.9 template plus max(34.5 / 4, 6.3)). If U1's measurement shows adopt longer than the next launch by more than 3 minutes, a follow-up reads where an adoption spends its 2 to 3 minutes. The copy already leaves out most of `plans` and `records`, non-regular files and the engine binary (`adopt_git_integration_test.go:153-176`), so the payload hypothesis is narrow and waits (Astra's Q6).

- **D4 Every full proof says its work, its longest launch and its span.** (Wido's name, fold 3.) Built by the follow-up goal `the-proofs-critical-path-is-reported` (revision 3).
  - Three numbers, from the scope record, for a proof of main as for a batch: work, the summed launch time; the longest launch (unit and shard); the span, from the first launch's start to the last launch's end. Beside them the actual worker slots W (D1's line, not the cores), the floor work / W, and the captured launch CPU (D6). Today's proof would read: "full proof 46.9 min (static 2.8, workers 18): span <min>, longest launch internal/adopt 40.5, work 683 min, floor 38.0, captured CPU unknown".
  - Where: the clock words (`clock.go:185-191`) for batches; `landing status --verbose` for the latest full proof of main (beside `cmd/metasystem/intent_landing.go:715`), which the clock skips (`clock.go:127`); and the running line "longest launch so far <unit> <min>" from the package lines already seen (`status.go:620-636` into a new `RenderInputs` field, `describe.go:18-26`, `:73`).
  - The six-field `landing package` line is unchanged, so an older lane still parses and counts it (`scope.go:430`, `status.go:632`).
  - Rejected: the longest as summed shards per package (revision 1). It makes `cmd/metasystem` "longest" at 414.1 minutes in a 46.9-minute proof (Astra finding 3).

- **D5 A unit that failed in parallel and passed alone is recorded so.** (Wido's name, fold 5.) Built by the follow-up goal `the-proofs-critical-path-is-reported` (revision 3).
  - Where the lane repeats (D5's evidence above), the repeat runs with `LANDING_ONLY`, so N is 1. `FlakeRecord` gains `Parallel int`: the largest effective N among the failing unit's launches in the red run, read from their `landing package-cost` lines (D6). When it is above 1 and the repeat passed, every reader that says "flake" says "failed in parallel (N=<n>, load <l>), passed alone". The record, both outputs, its fix goal and its closure stay as they are; nothing claims a test defect or a product race, because the repeat changed both the concurrency and the selection (Astra finding 5). The diagnosis stays with the person or agent who takes the fix goal.
  - The fallback. While such a record is open, the lane passes its packages in `LANDING_SERIAL_PACKAGES` (`prove.go:1225-1227`); the reporter puts them in `NativeInventoryRequest.SerialPackages`; D1's function returns 1 for their launches. It covers what T7 tests: full-proof native launches of a listed package, after a red on main and after a red on a batch that was repeated. A batch red that is replayed without a repeat (`replay.go:141-143`) records no such flake and lists nothing; this page claims nothing for it.
  - Until U4 lands, the repeat alone is the fail-safe where the lane repeats, as today.
  - Rejected: retry at N (Wido 2026-09-12). Rejected: the whole proof at N = 1 after one such record.

- **D6 The proof writes each launch's cost.** For every launch the reporter prints `landing package-cost <unit> <shard> <N> <cpu-ms> <start-ms>` beside its `landing package` line. `PackageExecution` (`metasystem/internal/proofrun/test_result.go:45-52`) gains `Parallel int`, `CPUMS *int64` and `StartedAt time.Time`; `supervisorOutcome` gains `StartedAt`, set from its local start (`supervisor.go:212`). `runGoPartitions` sets them from the run's outcome (`test_build.go:1608`) in the progress block (`:1610-1620`) and the merge (`:1677-1700`). The reporter turns `StartedAt` into milliseconds after its own `started` (`full.go:118`), so both native calls share one origin. Unknown is written as `-`, never 0. The CPU is captured native-launch CPU (Astra's Q1), named so wherever it is shown.
  - Rejected: keep sampling with `ps` as the measure (section 1).

## 3. What does not change

- `goNativeTestArguments` and its argv for every testing group, unit check, rerun and replay (`test_go.go:349-365`; `test_rerun.go:65-66`).
- `-p=1`, `GOMAXPROCS=1`, the worker count, the `cmd/metasystem` shard count (`test_go.go:916-919`).
- The host admission and its slot cap (`metasystem/internal/proofrun/host_resources.go:751-758`, `admission.go:20-60`); the-lanes-proof-goes-first and one-host-admits-heavy-work own who proves when.
- The flake repeat, its record and its closure, except `Parallel` and the words (D5).
- The six-field `landing package` line and its two parsers.
- Scoped batch proofs: they run testing groups, not the native phase (the 207 scoped scope records in the landing checkout have no package lines).

## 4. Units

Production lines are additions plus deletions; a replaced line counts twice; imports and gofmt's realignments are included. Recounted by file at `d37946f634` for the U1 brief (`briefs/the-proofs-critical-path-is-short-U1-brief.md` in the evidence folder), after Astra's B6 and B8 on it. The brief carries the additions and deletions per file. Estimates run double in practice (2026-10-25); the doubled figure stands beside the count, against the stop line of 250. Each brief carries `Read each round: yes` and BRIEF-CHECKLIST. This goal ends with U1b, built with `work build --last` (revision 3).

| Unit | Decisions | What it does | Production lines | Doubled | Split seam | Step value |
|---|---|---|---|---|---|---|
| U1a | D1 | full-proof launches take N from a live sample at their admission; the setting `host.proof-package-parallel-max` with its bounds and validation; the line `landing parallel max <M> workers <W>` | 113 | 226 | its own stop at 125 counted lines | the next full proof runs its parallel tests at N when the host has spare cores; estimated 20 to 25 minutes |
| U1b | D6 | every launch line has its effective N, captured CPU and start on one clock origin | 35 | 70 | none | the N=1 against N=4 comparison of section 6 can be read |

U1b builds on U1a's commit. D3 is not a unit; it becomes one only on its trigger (section 6).

Reverse dependents run whole (R-18): `internal/proofrun` and its importers through `internal/repoproof`, `metasystem/proof`, `internal/landing/plain`, `internal/landing/state`, `cmd/metasystem`; `internal/config` for the key.

### 4b. Built by the follow-up goal `the-proofs-critical-path-is-reported`

The follow-up goal is opened blocked by this goal. It builds D4, D2 and D5 from the text on this page, after U1b's measurement, with its own split page and its own Units table. The sizes below were counted at `cb112adb78` and are recounted when that page is cut. They are a list, not a table, so this page's size reader sees only U1a and U1b.

- **U2 the proof says its work, longest launch and span** (D4). The scope record keeps workers, N, CPU and start; the clock, `landing status --verbose` and the running line say them. About 75, doubled 150: `internal/landing/plain/scope.go` 22 (the first line and the cost lines parsed; fields on `scopeRecord` and `PackageTiming`); `internal/landing/plain/clock.go` 26 (work, longest launch, span, floor on W, captured CPU in the words; the latest full proof of main for the verbose status); `cmd/metasystem/intent_landing.go` 10; the running input: `internal/landing/plain/status.go` 8 (longest so far from the package lines, `:620-636`), `internal/landing/plain/shadow.go` 2 (carried at `:286`), `internal/landing/state/describe.go` 7 (the field at `:18-26` and the phrase at `:73`, `:159-170`). Split seam: the running line (17). Step value: a regression of the span or the longest launch is read in the status.
- **U3 heaviest ready launch first** (D2). One dispatcher admits ready launches by the last full proof's per-launch times. About 74, doubled 148: `internal/proofrun/test_build.go` 42 (the dispatcher 34; the launch registers with it in place of its own acquire, 2 replaced 4; the weight lookup 4); `internal/proofrun/test_go.go` 4 (`Weights` and carried); `internal/repoproof/full.go` 16 (read `LANDING_PRIOR_SCOPE` into per-launch weights); `internal/landing/plain/prove.go` 12 (the latest full scope record's path in the env). Split seam: none: the dispatcher is the unit; sorting alone gives no order. Step value: the span is the longest launch or the floor, whichever is larger, not the chance of the order.
- **U4 failed in parallel, passed alone (`--last`)** (D5). The record says so with its N; its package launches at N = 1 while it is open. About 66, doubled 132: `internal/landing/plain/prove.go` 3 (`Parallel` on `FlakeRecord`); `internal/landing/plain/scope.go` 6 (the failing unit's largest N from the cost lines); `internal/landing/plain/report.go` 8 (set and word it); `internal/landing/plain/stop.go` 6 (the words beside `:299`); the serial list from open records into the env 22 (`prove.go:1225`, through the flake recorder's store, `cmd/metasystem/intent_landing_prove.go:137-139`); `internal/repoproof/full.go` 10 (`LANDING_SERIAL_PACKAGES` into the function); `internal/proofrun/test_go.go` 2 (`SerialPackages`); flake-word readers 9. Split seam: the serial list (22). Step value: such a test stops costing a repeat per proof and is named for its fix.
- The follow-up's last unit is U4; it carries `--last`.

## 5. Tests

Through the production composition. No test waits on wall time or reads a real load: the load is an injected `hostload.Sample`, the start times come from the supervisor's injected `now` (R-104-m1e). Barriers are files and output events released by the test; real-process launches end on the test's cancellation only as the deadman, as `TestAutomaticGoPartitionsUseWorkerCapacityBetweenIsolatedProcesses` does (`test_go_workers_test.go:412-497`, the hold at `:473-497`), through the `goShardLifecycleHooks` output seam (`test_build.go:1591-1593`). Each test names what fails on main and the mutation it kills. T1 to T4 are this goal's (the U1 brief refines them); T5, T6 and T7 belong to the follow-up goal.

- **T1** `TestAFullProofLaunchHoldsNParallelTestsAtOnce` (new, `internal/proofrun`). Through `RunNativeInventory`. Fixture: one package with six `t.Parallel()` tests, each writing `<name>.ready`, printing "barrier ready", and waiting for one shared `release` file; and one serial test printing "serial done". For N in 1, 2, 3, 4 (the function returns N): the test waits for exactly N ready events through the output seam, then checks that exactly N `.ready` files exist and no further one, that "serial done" came before the first "barrier ready", and that the launch's argv (recorded by a lifecycle hook) carries `-parallel=<N>`; then it writes `release`, waits for N more, and so on to six. On main: no field (compile), then N = 2 sees one ready file. Kills: the swap not applied (one file at N ≥ 2); the swap with another N (the argv check); `-p` changed instead (the argv check); N read before the slot (T3's admission case).
- **T2** `TestTestingGroupsAndRerunsKeepOneTestAtATime` (new, `internal/proofrun`). The fixture of T1 through `runTestGroup` as `:412-439` builds its group: one ready file at a time. And through `runFailedTestAgain` (`test_rerun.go:59-66`) on a fixture whose one failed top-level test has four `t.Parallel()` subtests at the barrier: one ready file at a time and `-parallel=1` in its argv. Kills: the field read for groups or reruns; the swap moved into `goNativeTestArguments`. `TestGoNativeArgumentsPinEveryNestedParallelismLayer` stays and still runs.
- **T3** `TestTheReporterTakesNFromTheLoadAtEachAdmission` (new, `internal/repoproof` and `internal/proofrun`). `PackageParallelFor` at 18 cores, M 4: unavailable gives 1; load 0, 4.49 give 4; 4.5, 8.99 give 3; 9, 13.49 give 2; 13.5, 18, 30 give 1; M 2 at load 0 gives 2; M 1 gives 1. Through `Run`, with the `Command` stub for `landing status`, `host.proof-vm` and the new key, `HostRunners.Native` calling the request's function at each simulated admission, and `HostRunners.Load` returning a scripted sequence (load 3, then 10, then 14): three launches get 4, 2, 1, in that order, on the `./...` call and on the `batchtest` call. `LANDING_ONLY` set: the function is absent and every launch gets 1. The settings call fails, answers without the key, or answers 0 or 17: M is 1 and the first line names the reason. Kills: N read once for the phase (the sequence); unavailable treated as zero load; a band boundary off by one; N in a repeat; the batchtest call left at 1; 0 taken as "no cap".
- **T4** `TestLaunchCostLinesCarryNCPUAndStart` (new, `internal/proofrun` and `internal/repoproof`). In `internal/proofrun`, through `runGoPartitions` with two controlled launches of a two-package fixture, the supervisor's `now` injected to return 10:00:00.000 for the first start and 10:00:02.500 for the second, and the function returning 3 then 1: the executions carry `StartedAt` exactly those times and `Parallel` 3 and 1. In `internal/repoproof`, with `Native` returning those executions, `CPUMS` 4200 and absent, and the reporter's `Now` at 09:59:59.000: the lines are `landing package-cost <a> 1 3 4200 1000` and `landing package-cost <b> 2 1 - 3500`; the `landing package` lines are byte for byte as today. Kills: all-zero starts; a per-call origin (the second native call's origin is the same `started`); unknown CPU printed as 0; the six-field line changed.
- **T5** `TestTheClockSaysWorkLongestLaunchAndSpan` (changed, `metasystem/internal/landing/plain/clock_test.go`; the fixture writes the report as `clock_test.go:24` does). A report with `landing parallel max 4 workers 3` and four launches: package p shard 1, 10 min from +0; package q shards 1 and 2, 6 min each from +0 and +1; package r, 2 min from +10. The words: work 24 min, longest launch p shard 1 at 10.0, span 12.0 (from +0 to +12), floor 8.0 on 3 workers. A proof of main shows in `landing status --verbose`; the running line shows "longest launch so far p 10.0" once p's line is in. An old report without the new lines yields today's words. Kills: the longest taken as a package's summed shards (that reads q at 12 and must fail); the floor on cores instead of workers; trunk proofs still skipped.
- **T6** `TestTheDispatcherAdmitsTheHeaviestReadyLaunch` (new, `internal/proofrun`). Through `runGoPartitions` with one worker and three fixture packages at weights a 1, b 9, c 5: a blocker launch holds the slot; a lifecycle hook confirms a, then c, then b registered ready, in that order (the old pool's arrival order); the test releases the blocker; the JSON `start` events come b, c, a. Without weights: a, b, c by index. A launch missing from the weights goes last. Kills: arrival order kept; the wake-all race kept (the forced order makes the old code start a first); the package's summed weight given to each shard.
- **T7** `TestFailedInParallelPassedAloneIsRecordedAndItsPackageLaunchesAlone` (new, `internal/landing/plain`, through the lane's prove with the repeat seams as the existing repeat tests). (a) A proof of main red in one unit whose cost line says N 4, load 3.0; the repeat alone passes: the record has `Parallel` 4, the words read "failed in parallel (N=4, load 3.0), passed alone"; the next full proof's environment has `LANDING_SERIAL_PACKAGES=<package>`; after the record closes, it does not. (b) The same for a batch whose red the judge knows (`replay.go:128-147`). (c) A red in a package already in the list (its launch N 1) that passes alone: an ordinary flake, `Parallel` 1, no "in parallel" words. In `internal/repoproof`: a listed package's launches get 1 while the others keep the sample's N. Kills: the words without the N; the list kept after the closure; the whole proof at 1; a serial launch's flake called parallel.

## 6. Triggers and measurement

- Before (decision 197): attempt `20261104T104910.935087000Z`, 50.1 minutes, reporter clock 46.9, longest launch 40.5, work 683 minutes on 18 slots (floor 38.0); captured CPU not recorded.
- U1a and U1b build now. Their measurement (Wido's name, fold 6) is two fresh full proofs of the same candidate tree after U1a and U1b are deployed, back to back, one with M = 1 and one with M = 4, each started only when the load is below 9 at its start, both at `-count=1` (section 1). Report for both: the results row's minutes, `landing clock total`, the span, the longest launch, work, and the summed captured launch CPU; and per launch, CPU divided by its elapsed time against the sampler's readings in the same window. The gain is the difference of the two wall clocks.
- Expected, an estimate until that comparison: 20 to 25 minutes at M = 4 with spare CPU, if the parallel tests mostly wait. The longest launch would be `internal/steward` at about 12.7 (7.3 serial plus 21.5 / 4), the floor about 15 (143 + 506 / 4 = 270 minutes on 18 slots). These assume the tests do not slow each other; if they mostly compute, the gain is smaller and the CPU figures say so, and the steward raises it with Wido before U2 to U4.
- The follow-up goal is unblocked when this goal lands. Its U2 builds after U1b's comparison, whatever that shows.
- The follow-up's U3 builds when a full proof after U2 has a span more than 3 minutes above the larger of its longest launch and its floor.
- The follow-up's U4 builds on the first flake whose red launch ran at N > 1, or before M is raised above 4, whichever comes first.
- D3 opens a follow-up goal when, after U1, `internal/adopt` is the longest launch by more than 3 minutes.

## 7. Later, named, not built

- A quarter of today needs: the serial tests (143 minutes, 102.8 of them in `cmd/metasystem`) made parallel through the ratchet; the prelude (3.2 minutes before the reporter, 2.8 of static checks) overlapped with the native phase; and more than 18 slots when the CPU figures show room.
- A memory reading in the N rule, after U1's comparison measures peak memory at N > 1 (Astra's Q4).
- A `-race` run of the parallel packages. Repeat-alone does not replace race detection, but no concrete collision was found (Astra's Q3: homes and registries per binary, `metasystem/internal/testenv/testenv.go:541`; port fixtures keep their sockets, `metasystem/cmd/metasystem/app_fixture_listener_test.go:25-74`).
- Parallel tests in testing groups and unit checks.

## 8. Rulings

- R-104-m1e: artificial clocks; a test that fails under load is a defect of the test, never a reason to retry (D5's rejection, section 5).
- Wido 2026-09-19, measure before claiming: section 1 says only what the log and the CSV say; the gain stays an estimate until the comparison (section 6).
- Wido 2026-09-28, parallel only with spare CPU, concurrency from current load: N per admission from a live sample (D1).
- A default is not evidence the path runs (2026-09-20): T3 drives the setting and the sample through the reporter into each admission; unreadable means 1 and says so.
- No builder tests beside a lane proof (2026-10-25): load is added only inside the lane's own proof, launch by launch, only with spare CPU.
- Wido 2026-10-22, not a nuclear reactor: one generic fallback, the serial list (D5).
- R-121-m0: the smallest thing first; U1 alone is the shortening.
- R-18: callers run (section 4). R-107-m1e: the admission is not touched (section 3).
- Wido 2026-10-07 and 2026-10-25: units at most 250 lines, estimates doubled, a split seam per unit.

## 9. Questions answered (Astra round 1)

- Q1: captured native-launch CPU; detached descendants are not counted (D6).
- Q2: a heuristic, read per admission (D1).
- Q3: no `-race` blocker for U1 (section 7).
- Q4: measure peak memory first (section 7).
- Q5: no cache cost; N is not limited to 1, 2 and 4 (section 1).
- Q6: fixture sharing confirmed; D3 waits.
- Q7: scheduling hints only (D2).

## Critique round 1 (Astra)

2026-11-04, `designs/the-proofs-critical-path-is-short-critique-1.log` (final answer before `codex-exit=0`): 2 material, 7 minor; verdict build after the folds. Every finding accepted; the folds by the steward in Wido's name.

1. Material, D1's start sample does not cover later launches: accepted; N per admission from a live sample, unavailable means 1, effective N per launch (D1, D6, T3).
2. Material, T1 can reject a correct build: accepted; the test holds N ready before release, checks the argv's flag, cancellation only as deadman (T1).
3. Minor, D4 confuses work with the critical path: accepted; work, longest launch and span apart, actual workers, T5's mutation reversed (D4, T5).
4. Minor, D2 still races and over-weights shards: accepted; one dispatcher, per-launch weights, missing weighs 0 (D2, T6, U3).
5. Minor, "green alone" is not a test defect: accepted; "failed in parallel, passed alone" with the launch's N, diagnosis kept, coverage claimed only where tested (D5, T7).
6. Minor, the gain is unattributed: accepted; same-candidate N=1 against N>1 comparison, estimate until then (section 6).
7. Minor, measurement statements overstated: accepted; sample gaps 44 to 158 s, the populations, no native-start sample, row 969 at 69.25, no CPU interval (section 1, premise 4).
8. Minor, mutation claims incomplete: accepted; parallel subtests for the rerun, N 3 and band boundaries, unavailable and M 0, two launches with exact injected times, a forced admission order (T2, T3, T4, T6; M 0 refused, D1).
9. Minor, estimates omit plumbing: accepted; U1 recounted with `StartedAt` and the common origin (106, doubled 212), U2 with the running input (75, doubled 150), U3 kept whole (74, doubled 148) (section 4).
