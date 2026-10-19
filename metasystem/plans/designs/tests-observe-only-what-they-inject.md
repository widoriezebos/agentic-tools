# A test sees only the processes it was given, and its helpers end with it

- Kind: design
- Id: 01M5B0TESTS0BSERVE0NLY0INJ
- Status: accepted
- Goals: tests-observe-only-what-they-inject tests-observe-only-what-they-inject-2

The goal tests-observe-only-what-they-inject is opened by the steward in Wido's name (step-back 2026-10-17, cause C1, remedy R1). The Id above is permanent.

Anchors were read at origin/main 0128fc34d in /Users/wido/LocalStorage/GitHub/agentic-tools-m1e-reads/metasystem. Paths are relative to `metasystem/`. The J1 gate log was read in /Users/wido/LocalStorage/GitHub/agentic-tools-m1e-j1/.full-cmd-gate.log. Rulings:

- R-104 (`memory/rulings.md:163`): a test drives an injected clock and an injected prober; real-process tests are few wiring proofs.
- R-35-m3 (`:62`): no load-slowness into failure. This page adds no timing.
- R-142 and R-143 (`:201-202`): a person may raise an allowance; the machine says what that lets through.

The step-back's folded directions apply: DEFER-R1 (repair an inventoried path with a witness first, audit later with a baseline) and DEFER-OWNERS (382's target is the legacy-worker launch fixture; 373's helpers are not registered with the custodian testenv.Main starts).

## Evidence

**Finding 382** (2026-10-17 06:53): the J1 gate's testenv+proofrun leg ended red on `TestLaunchSuiteAuthenticatesLegacyWorker` (0.26 s). The finding quotes "proof admission: keeping dirty lease lease-heavy-710869c6… (owner pid 1630, group 84921): owner process group 84921 still has live members, settlement is unknown" and a `fixture-survivor pid=2590 pgid=84921 ppid=1 … key=setup-child/c8c30d58 exe=…/testenv.test`. It names the lease reclaim (`lease_reclaim.go:278`, `attemptload.go:305`) as the path. The test passes alone in 2.1 s.

**Finding 373** (05:52): thirteen `supervise.test` helpers (`--arming-owner-helper …`, `--takeover-component-helper …-watcher`/`-reaper`), parent 1, alive 74 minutes after their runs. The steward killed them. Cause: hypothesis (an interrupted `go test`).

**Findings 368, 375, 377, 384**: load-shaped reds and a wait with no producer check. 375 is fixed by a-proof-unit-ends-on-time U1. 368, 377 and 384 are not designed here.

## Problem

**382's premise was wrong. The test did not read the lease directory. It read the host's process table, and killed what it found.**

The gate log (`.full-cmd-gate.log:1197-1200`) shows three lines in a row: the lease line, the survivor line, then `--- FAIL … launcher_test.go:1114: legacy worker launch exit=1`. The launch returned 1. The lease line is not its cause:

- The lease reclaimer prints to the process's stderr (`lease_reclaim.go:67`). The gate runs plain `go test ./internal/testenv ./internal/proofrun` (`panes/full-cmd-gate-sharded.sh:41`), so any test's stderr lands in that stream unattributed.
- The same line appears inside a passing `TestHostResourceNestedCompletionCannotClearLostOuterCustody` in two earlier logs (`u1b-checks/reverse-dependents.log:18209`, `fixround2/reporter-five-branch.log:122591`). That test stages a dead lease owner.
- Hypothesis: group 84921 is the gate shell's process group. The testenv setup child is started without `Setpgid` (`internal/testenv/setup_custody_test.go:42`), so it shares that group, and any dead lease owner from that group looks "still live". Which proofrun process printed the line is not established.

What failed is the launcher's post-suite survivor scan:

- `LaunchSuite` reads the whole process table: `census.EnumerateConfiguredProcesses(filepath.Dir(options.ConfPath))` (`internal/proofrun/launcher.go:633`). With no `METASYSTEM_CENSUS_PROCESS_FILE` that is the host's table (`internal/census/production.go:15-20`).
- With no injected table and no `options.Signal`, the signal is `syscall.Kill` (`launcher.go:643-645`).
- `reapFixtureSurvivors` (`fixture_reap.go:20-43`) filters foreign survivors only when the launch has an attempt id (`:29`). The legacy launch has none. Every certain fixture survivor on the host is then SIGKILLed (`:34`). One started after the launcher's own start fails the launch (`:35-39`, `launcher.go:675-677`).
- The launcher here is the test process itself, so "started after the launcher" means anything born since `proofrun.test` started.
- The survivor line carries neither " foreign" nor " stale", so it was counted as the launch's own.

The survivor is testenv's own witness. `TestSetupChildIsEndedWhenTheBinaryIsKilled` mints `SetupFixtureTag("setup-child")` (`setup_custody_test.go:38`), SIGKILLs its inner binary, and holds the inner custodian with SIGSTOP (`:117`). For that window a certain fixture survivor exists on the host by design. Both packages ran at once in the gate leg. The survivor was gone by 06:57; the launcher's SIGKILL is one explanation (hypothesis).

So the defect is a test reaching a host-wide fact through production code. The seam already exists: an injected table (`METASYSTEM_CENSUS_PROCESS_FILE`, allowed only in a fake-runtime root, `internal/census/run.go:440-445`), which also switches the reaper to the table's prober and a no-op signal (`launcher.go:639-642`). `TestLauncherReapsSurvivorsIntoTheVerdict/LaunchSuite` uses it (`fixture_reap_test.go:131-138`). The legacy test does not: it writes an empty conf (`launcher_test.go:1063`) and sets no table.

**The other proofrun tests that reach `launcher.go:633` the same way.** Every `LaunchSuite` call in proofrun was classified by reading its test:

- *Same repair, not parallel, no own processes in the table, no attempt id:* `TestLaunchSuiteAuthenticatesLegacyWorker` (`launcher_test.go:1029`), `TestLaunchSuiteKeepsTheWatchdogsLastLine` (`:1170`, 25 launches), `TestLaunchSuiteCreationClaimSpansBothStartsAndSecondFenceRead` (`:480`; the second fence read does not return early, `launcher.go:543-545`). The last two expect exit 1 anyway, so a foreign survivor does not turn them red. It does get killed.
- *Parallel, so `t.Setenv` is not possible:* `TestLaunchSuiteWritesBannerProgressAndReapsWatchdog` (`:115`), `TestLaunchSuiteRejectsZeroChildWithoutNormalFullProgress` (`:334`), `TestLaunchSuiteSecondFenceErrorUsesPhysicalStopClock` (`:533`). These go to U3.
- *Custody runs that need the real table for their own grandchildren:* `resource_custody_output_test.go:54`, `:172`, `:227`, `:294`, `:426`, `:547`, `:611`; `host_resource_custody_test.go:421`. Their drain reads the table owner-scoped (`resource_custody.go:217-226`), but the post-suite scan at `launcher.go:633` is not. These go to U3. Finding 377 failed in the first of these with "released suite status=1" (`resource_custody_output_test.go:88`); that it is this mechanism is a hypothesis, because its error output went to a temp file.
- *Attempt-bearing launches:* `launcher_test.go:663`, `:739`, `:861`, `:905`, `:970`. They can reach the host scan, but foreign survivors are labelled " foreign" and neither killed nor counted (`fixture_reap.go:29-31`). They go to U3's baseline. The pre-cancelled launch at `:791` does not reach the scan: it cancels its attempt first (`launcher_test.go:786`), and the launcher returns at its admission check (`launcher.go:209-211`).
- *Already injected:* `fixture_reap_test.go:142`, `launcher_test.go:282`, `:402`, `host_resources_test.go:565`, `host_resources_review_test.go:310`, `:338`.

**373: the arming helpers are not registered with the custodian.**

- testenv.Main starts the binary's custodian (`internal/supervise/arming_test.go:158` → `internal/testenv/testenv.go:279`). It ends a dead owner's processes by three routes: records, descendants it observed while the owner lived, and a scan for processes tagged with the dead owner's key (`internal/identity/fixture_custodian.go:497-577`).
- The arming tests use none of the durable routes. The owner helper is started by production `EnsureArmed` through the test's command factory (`arming_test.go:243-250`), and production replaces its environment and detaches it (`internal/supervise/arming.go:997-998`). The helper starts the watcher and the reaper detached (`arming_test.go:203-204`). Two tests start component helpers directly, detached (`:1396-1397`, `:1912-1913`). None carries a fixture tag.
- Observation is a poll while the owner lives (`fixture_custodian.go:290-318`). A helper orphaned between polls, or after a takeover killed its owner, is never observed (hypothesis for 373's escape).
- Production matches a supervision instance tag as one whole argv token (`internal/identity/identity.go:272-279`), and supervise production reads no fixture tags. An extra argv word is safe.
- The census accepts a fixture tag as an argv word (`internal/identity/fixture_survivors.go:20`, `:46-62`).

**Nothing audits either shape.** The existing audits cover wall time (`internal/testenv/walltime_test.go:78`), polls by attempt count, a process read once, private proofrun reservations (`internal/proofrun/private_admission_audit_test.go:14`), and production scans that must take a table (`cmd/metasystem/host_process_scan_audit_test.go:45`, confining identity's `AllPids` and `TakeProcessCensus`, `:26`). None sees a test that reaches the host table through a production launcher, or a detached start without a tag.

## Decisions

**D1. A proofrun test that drives `LaunchSuite` without an attempt id hands it a process table. (U1)**

- The rule: such a test runs in a fake-runtime root and sets `METASYSTEM_CENSUS_PROCESS_FILE` to a table it wrote. The mechanism is the existing seam: `census.ConfiguredProcessFixture` (`census/production.go:25-31`, gate `run.go:443`), read at `launcher.go:633`, prober and signal at `:639-642`.
- One test helper, `injectProcessTable(t, confPath, rows)`, writes `metasystem.runtimes=fake` to `confPath`, writes `rows` with `writeProofProcessTable` (`fixture_reap_test.go:257`) and calls `t.Setenv`.
- The legacy test becomes two cases over one setup, so the injection cannot be dropped from one case alone:
  - **stale:** the table holds one foreign survivor started long before the launcher. The worker is authenticated, the exit is 0, the line ends in " stale", and the injected `Signal` recorder saw that pid once. This proves the original case reads the injected table.
  - **fresh:** the table holds one foreign survivor started after every launch (the fixed 2100 instant, as `fixture_reap_test.go:136`). The exit is 1, the log names the pid without " stale", and the recorder saw it once. This is the regression witness of 382: the launch acts on a survivor that exists only in its table, so it never consults the host's.
- The watchdog and fence-race tests call the helper with an empty table.
- No production line changes. The production legacy path keeps its rule (see OPEN).
- **Remedy.** A test that reaches `launcher.go:633` uninjected kills foreign processes on the host and may fail on them. The fix is one `injectProcessTable` call; if the test is parallel, U3 decides its route.

**D2. A detached arming helper carries the test binary's fixture tag. (U2)**

- The rule: every process the supervise tests start detached, or cause production to start detached, carries `METASYSTEM_FIXTURE_OWNER=<key>` as an argv word. The key is owned by the test binary and registered for the exit scan.
- Mechanism: supervise's TestMain calls `testenv.MainWithSetup` (`testenv.go:217`) with a setup that mints the key through `testenv.SetupFixtureTag("supervise-helpers")` (`testenv.go:146-166`). `armingOwnerCommand` (`arming_test.go:243-250`) appends the word. `armingOwnerHelper` (`:174-240`) finds it in its own arguments and appends it to each component it starts (`:203`). The two direct component starts (`:1393-1396`, `:1912`) append it too.
- Then the custodian ends them when the owner dies by the tagged-survivor scan (`fixture_custodian.go:554-577`), observed or not, and the exit scan (`testenv.go:311-342`) fails the package if one outlives a normal exit.
- Exempt, with reason: the held process at `arming_test.go:46-77` blocks on a pipe whose writer the test holds, so it ends at EOF when the test dies.
- **Remedy.** A leaked helper is named by the custodian log (`action=kill pid=… carrier=argv-word`) or by the exit scan ("fixture child outlived test"); either way it is ended, and the exit scan fails the package.

**D3. A repository audit fails when a test reaches a host-wide fact or starts a detached process without a tag; its baseline may only shrink. (U3, later)**

- Shape: the wall-time audit's allowance (`walltime_test.go:78-129`; TSV of path, count, reason, `testdata/walltime-allowance.tsv`). More found than allowed fails; fewer found than allowed fails until the row is lowered (`:125-128`). An update variable rewrites the file and always fails, so the rewrite is reviewed (`:19`, `:91-100`).
- What it counts, per `_test.go` and test-support file, in a new `TestNoTestObservesTheHost` in internal/testenv:
  - a call of `LaunchSuite` without an attempt id, in a function that does not inject a process table;
  - a direct call of a host-table reader: `census.EnumerateProcesses`, `census.EnumerateConfiguredProcesses`, `identity.KernelProcessTable{}`;
  - a host-root resolution: `user.Current`, `os.UserHomeDir`, `~/.metasystem`;
  - a `SysProcAttr` with `Setsid` or `Setpgid` in a function that hands its command no fixture tag (`fixture.Env`, `fixture.Shell`, a `SetupFixtureTag` word, `FixtureOwnerEnv`).
- The detectors are syntactic. They will miss indirect reach, and they will count a few legitimate wiring proofs. Each counted row carries its reason, as the wall-time rows do.
- **Baseline.** Produced by running the audit in update mode on U3's build tree after U1 and U2 have landed, then each row's reason written from this page's inventory (the parallel launcher tests, the custody runs, the attempt-bearing launches, the helper processes that skip proofrun's binary admission setup at `proofrun/testmain_test.go:75`, `:94`, and testenv's own survivor witness). Owner: the testenv package, like the wall-time allowance. Any unit that removes a violation lowers its row in the same commit, because the audit fails otherwise.
- **Remedy.** A new violation fails with the file, the line and the rule. Its fix is the injection, or, for a parallel test, an owned subprocess (as `launcher_test.go:211-213` does) or the per-call seam U3 decides. A person may raise a row; the commit records who, why, and what it lets through (R-142, R-143).

## Units

**U1 (D1).** Inject the process table in the three proofrun tests; the legacy test becomes the two-case witness. It works alone: after it, the legacy-worker test never reads or signals the host's processes, and 382's interference cannot recur in it. Inventory and bound in `briefs/tests-observe-only-what-they-inject-U1-brief.md`: 0 production lines (stop line: any), test allocation at most 200.

**U2 (D2).** The binary-owned tag on every detached arming helper, and the death witness. It works alone: after it, a supervise test binary that dies leaves no arming helper behind.

Inventory, all in `internal/supervise/arming_test.go`, test code:

| Site | Change | Lines |
|---|---|---:|
| TestMain `:141-158` | `MainWithSetup` with the tag setup; package word | 8–10 |
| `armingOwnerCommand` `:243-250` | append the word | 1–2 |
| `armingOwnerHelper` `:174-240`, component start `:203` | find and forward the word | 4–6 |
| direct starts `:1393-1396`, `:1912` | append the word | 2 |
| new witness test and inner process | see Tests | 90–130 |

Production lines: 0. If the nested start needs a testenv export to build a clean child environment, that export is counted as production in the U2 brief (estimate 10–15; the identity witness tests read the registry variable by name instead, `internal/identity/fixture_custodian_witness_test.go:144`). Stop line for production: 30.

**U3 (D3).** The audit, its detectors, the baseline TSV with reasons, and the decision for the parallel launcher tests. It works alone: no new violation lands. Production lines: 0 expected (the audit is a test file); a per-call process-table seam on `LaunchOptions`, if U3 chooses it for the parallel tests, is production and is inventoried in U3's brief. Test allocation and the baseline count are set at U3's brief, after U1 and U2 land.

## Tests

**U1.**

- `TestLaunchSuiteAuthenticatesLegacyWorker`, changed, two cases `stale` and `fresh` over one setup, as D1.
- `TestLaunchSuiteKeepsTheWatchdogsLastLine` and `TestLaunchSuiteCreationClaimSpansBothStartsAndSecondFenceRead`, changed by one helper call each, meaning unchanged.

Mutations, each with its named test:

1. `injectProcessTable` does not call `t.Setenv` → `TestLaunchSuiteAuthenticatesLegacyWorker/fresh` (the injected pid's survivor line and signal are missing; the exit code is not the criterion).
2. `injectProcessTable` does not write the fake conf → `…/fresh` (the launcher prints "fixture survivor table is unreadable" and exits 0).
3. The `stale` case writes no row → `…/stale` (no signal recorded).
4. `AuthenticateWorker` (`proofrun/record.go:226`) refuses every legacy worker → `…/stale` (exit not 0, marker missing). This keeps the original meaning tested.

Removing the helper call from the watchdog or fence-race test is not caught by a U1 test; U3's audit catches it.

**U2.**

- `TestArmingHelpersEndWhenTheirTestBinaryDies`, new, a real-process wiring proof (R-104 allows few). Modelled on `setup_custody_test.go:64-140`:
  - the parent starts an inner supervise binary running `TestArmingHelpersOwnerProcess` with a clean testenv environment; the inner reports its custodian ref and waits on stdin;
  - the parent stops the inner custodian with SIGSTOP, so it observes no descendant while the inner lives;
  - the parent tells the inner to arm; the inner runs `EnsureArmed(armingOptions(root))` and reports the owner, watcher and reaper pids and its key;
  - before the kill, the parent asserts each of the three carries the key as an argv word (`identity.FixtureTag`, carrier `argv-word`);
  - the parent SIGKILLs the inner (the owner dies before any cleanup runs) and sends SIGCONT to the custodian;
  - under the binary deadline it waits until each captured exact helper identity is dead, replaced by another process, or a zombie, as the model does (`setup_custody_test.go:143-145`). A bare `kill(pid, 0)` wait (`testenv.AwaitProcessTargetGone`, `process_group.go:199`) is not used: a zombie or a reused pid would keep it waiting after a correct cleanup. Then it asserts `identity.FixtureSurvivors(key)` is empty and the custodian log has `action=kill pid=<pid> carrier=argv-word` for each.
  - No sleep; the only bound is the binary's deadline. Cleanup kills any helper by exact ref and resumes the custodian.
- `TestArmingHelpersOwnerProcess`, the inner entrypoint, skips unless its marker variable is set.

Mutations:

1. `armingOwnerCommand` omits the word → `TestArmingHelpersEndWhenTheirTestBinaryDies` (the owner's pre-kill tag assertion).
2. `armingOwnerHelper` does not forward the word → the same test (the watcher's and reaper's pre-kill tag assertion).
The SIGSTOP of the inner custodian is part of the witness, not of the change: it is what makes the `carrier=argv-word` check discriminate the tag from descendant observation.

**U3.** The audit's own table of detector cases (one positive and one negative per rule) and the shrink-only rule against a scratch tree, at its brief.

## Not in scope

- The load flakes 368, 377 and 384. The flake registry opens their fix goals on a second sighting. If U3's inventory confirms 377 as this mechanism, it says so there.
- R-35-m3 timing policy.
- The production legacy launch's survivor rule (OPEN).
- testenv's setup-custody witness making a host-visible survivor on purpose. It is the subject of its own test; D1 makes the readers blind to it.

## OPEN

- **Production legacy launches kill foreign survivors.** `metasystem proof-run launch` without a goal can launch with no attempt id (`cmd/metasystem/proof_run.go:139`, `:196`). It then SIGKILLs every certain fixture survivor on the host and fails on any started after it (`fixture_reap.go:29-39`). An owner-scoped scan, as custody uses (`resource_custody.go:224-225`), would fix it upstream, but `TestLauncherReapsSurvivorsIntoTheVerdict/LaunchSuite` asserts today's rule. Wido's decision: leave it, or open a follow-up goal.
- Which proofrun process printed 382's lease line (hypothesis: a helper that skipped the binary's admission setup at `testmain_test.go:75`, `:94` and resolved the host directory at `host_resources.go:189-198`). U3's inventory looks.
- 384's test (`cmd/metasystem/proof_run_test.go:2492`) writes a fake conf; whether its engine child has a table is not read. U3.

## Critique record

- Round 1 (Astra, gpt-6-astra, 08:39, scratchpad/codex-crit-inject-1.log; 0 material, 2 minor): both folded; accepted.
  - F1: U2's witness waits on the exact helper identity (dead, replaced or zombie), not on `kill(pid, 0)`.
  - F2: the pre-cancelled launch `launcher_test.go:791` is out of the host-scan inventory.

## Steward decisions (08:40, Wido's name)

1. The production legacy `proof-run launch` (proof_run.go:139) kills foreign fixture survivors on the host under today's rule, which an existing test asserts; this page does not change production behaviour. Whether a launch without an attempt id should scope its reaping to its own owner is a product question recorded for Wido in Not in scope; the lane test's steward does not decide it.
2. Who printed the unrelated lease line and whether 384's engine child gets an injected table are not U1's business; U3's inventory names them if they matter.
3. Build order: U1 after the U1.2b build (one Codex build at a time); its landing follows the queue (U1.2a → seat U1b → a-proof-unit U1 → this U1 → U1.2b).

## Recut (2026-10-19, steward in Wido's name)

U1 (D1) landed on main by the hand route on 2026-10-19: sharded gate GATE OK 16:02 on code tree 34e658d5 (evidence lane-test-20261009/gates/tests-observe-u1-20261019), then a fresh full proof of main. The goal tests-observe-only-what-they-inject concludes with U1 under the whole-goal rule (a goal delivers what landed). U2 (D2) and U3 (D3) keep their inventories and bounds above and are delivered under tests-observe-only-what-they-inject-2, opened in Wido's name with the same risk answers; the two flake incidents this goal owned pass to it, with the lane-fix flake the lane registered on 2026-10-19 (TestLaneFixLapsedGoalBoxAdmitsCommittedReadRealDispatch, its third record); the proofrun inventory red cleared in the alone proof of 17:00.
