# Build read: fixture-children unit 3, round 3 (3a, 3b and 3c)

Reader: Opus 5, build-read, the same reader as round 2. I did not write this change. Worktree `g18/wt-fcu3`, base 9cafaae5f.
My scratch copies and outputs are under `R=/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/r3`.

The worktree did not change during the read. I took `$R/snapshot-start.sha` (3719 files) and `$R/status-start.txt` before my first run. At 16:08, `shasum -c` reported no mismatch, and `git status --porcelain` matched the start. I created nothing inside the worktree and never used `git stash`.

The machine was loaded the whole time (load averages 5 to 18). No run failed under that load.

## Verdicts

- **3a: LAND.** On its own, 3a builds, vets and passes the identity race run. Its three new table witnesses fail on behaviour for M4, M5 and M6. The carrier witness now fails a fixed printed carrier. M1, M2 and all carrier mutations are caught.
- **3b: LAND.** Witness 5 passed 15 of 15 runs, and removing the stopped child's `Setsid` fails it 5 of 5 times. The new runtime-poller witness fails on the `F_GETFD` revert. The log sits beside the registry home, and moving it back inside fails both real-process witnesses. Identity and testenv race runs are green on 3a+3b and on the full tree.
- **3c: LAND.** Run for real, `proc custodian` now does four things:
  - It rejects the runtime's kqueue on fd 3 (exit 2).
  - It refuses an inherited owner tag, even an empty one, before any session change.
  - It drops the caller's fd 4, while staying alive and in its own session.
  - It reaps a tagged child after its owner is killed.

  Each of the F-2, F-3 and F-4 reverts fails its own subtest. The cmd package race run on 3a+3c is green.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| N-1 | low | no | On Darwin the Go runtime's kqueue reports `S_IFIFO`. The pipe-type check that my round-2 fix proposed does not reject it; the builder's added read-end check does | `$R/liveout/nb-fd3.out`: fd 3 is `KQUEUE`, `ifmt=010000 isfifo=true getfl=0x2`. Removing the read-end check fails both the 3b and 3c witnesses |
| N-2 | low | no | The FIFO-type check has no witness of its own | Removing only that check, with the read-end check kept, still passes `TestProcCustodianProcessBoundaries`. The case-2 temp file is opened read-write, so the read-end check rejects it first |
| N-3 | low | no | The budget witness catches the named M6 merge but not every way of shrinking the cleanup budget | M7 (reap bound cut to a tenth) and M6b (reap gets only the budget the proof left) both pass the identity package |
| N-4 | low | no | The F-5 witness proves the log's path, not that the log outlives removal of the home | With the log moved back inside the registry, the witnesses fail because they read the sibling path. The helper shares the outer binary's registry, and that registry is not removed during the test |
| N-5 | low | no | `proc custodian` opens its log without `O_NOFOLLOW`, unlike the testenv start | `cmd/metasystem/identity.go:69` against `internal/testenv/testenv.go:166` |
| N-6 | low | no | With the F-3 ruling, a bed that starts the custodian in the background and forgets `env -u` has no custodian, and it sees no error unless it checks | My env-tag harness: the custodian exited 2 and wrote no log. Tagged child 62328 outlived its owner until I killed it |

Material count: **0**.

## Sizes (`git add -N` in a copy, then `git diff --stat -M HEAD --` per set)

Output: `$R/sizes.out`.

- **3a:** `4 files changed, 263 insertions(+), 18 deletions(-)` = **281**.
  - fixture_custodian.go +138
  - fixture_custodian_test.go +97
  - fixture_survivors.go +25/-16
  - fixture_survivors_test.go +3/-2
- **3b:** `3 files changed, 296 insertions(+), 1 deletion(-)` = **297**.
  - fixture_custodian_witness_test.go +202
  - testmain_test.go +1/-1
  - testenv.go +93
- **3c:** `3 files changed, 298 insertions(+)` = **298**.
  - identity.go +104
  - main.go +1
  - proc_custodian_test.go +193
- **Outside the sets:** only `metasystem/memory/receipts.log`. It holds the same single line dated `2026-09-16T11:09:40Z` as in round 2, and it did not change during my read. There are no untracked files.
- All three sets are under 300. The totals agree with the builder's report.

## Closure of the round-2 findings

- **F-1 closed.** `fixture_custodian_witness_test.go:110` starts `commands[1]` with `Setsid`. `:91` requires `action=kill pid=<n> ` exactly.
  - On the full copy, `TestCustodianReapsStoppedAndDetached` passed 15 of 15 runs (1.75 to 2.22s), and `TestKilledTestBinaryLeavesNoFixtureChild` passed 15 of 15 (1.81 to 2.46s). Output: `$R/witness-x15.out`.
  - The revert that drops only line 110 failed 5 of 5 runs with `omits pid <stopped child>` (`$R/mut-3b.out`).
  - This also closes round-2 note 3 (bare pid match).
- **F-2 closed in both entry points.** Each checks, in order: `Fstat` must succeed, the type must be `S_IFIFO`, and the access mode must be `O_RDONLY`. All three run before `os.NewFile(3)` (`identity.go:36-49`, `testenv.go:99-111`).
  - Live, on the full binary with fd 3 closed:
    - With blocking output: exit 2, `watch descriptor 3 is unavailable; it must be a pipe: bad file descriptor` (`$R/live-fd3-a.out`).
    - With stdout and stderr made non-blocking: exit 2 in under 3s, `watch descriptor 3 is not a pipe read end: flags=0x2`, and no log created (`$R/liveout/nb-fd3.out`).
  - The testenv witness `TestCustodianRejectsRuntimePollerAsWatch` fails both on the revert to `F_GETFD` and on removal of the read-end check: `custodian accepted descriptor 3 opened by the Go runtime`.
  - The 3c witness fails in two ways. Reverting to `F_GETFD` fails cases 1 and 2 (`did not reject descriptor 3`). Removing the read-end check fails case 1.
  - See N-1 and N-2 for what the FIFO check does and does not do.
- **F-3 closed as ruled.** `identity.go:17` exits 2 when `os.LookupEnv` finds `METASYSTEM_FIXTURE_OWNER`. The check is the first statement, so it runs before `Setsid` and before any redirect.
  - Live, with the variable set to a value and set empty: exit 2 naming the variable, and no log file (`$R/live-envtag.out`, `$R/liveout/envempty-set-*.out`).
  - Case 3 starts the helper as a group leader. Three mutations each fail case 3 with `create session: operation not permitted` instead of the variable name: removing the check, allowing an empty value, and moving the check after `Setsid`.
- **F-4 closed.** `closeInheritedDescriptors` (`identity.go:92-116`) reads the whole of `/dev/fd` first, never touches 0 to 3, and closes only descriptors without `FD_CLOEXEC`.
  - Live (`$R/live-domains.out`):
    - The caller's fd 4 pipe and its stdout/stderr pipe both reached EOF while the custodian was alive.
    - `lsof` showed fds 0 and 1 on /dev/null, 2 on the log, 3 the watch pipe, and 5 and 7 as the custodian's own close-on-exec /dev/null and log. There was no fd 4.
    - pid = pgid = sid.
    - The custodian survived TERM and HUP.
    - After the owner was killed, the tagged child was dead in 44ms, and the custodian exited cleanly after 1.751s with `action=kill ... carrier=argv-word` and `action=complete` in its log.
  - Removing the call fails case 4: `custodian kept inherited descriptor 4 open`.
- **F-5 closed as ruled.** `testenv.go:165-166` writes to `registry + ".custodian.log"`, opened with `O_NOFOLLOW`. The witness derives the same path (`witness_test.go:115`).
  - Moving the log back inside the registry fails both real-process witnesses: `log did not publish ... action=complete`.
  - See N-4.
- **F-6 closed.** There are three named table witnesses in `fixture_custodian_test.go`. Each fails on behaviour, not on compile:
  - `TestCustodianWaitsForDeadOwnerAndExcludesItself`: M4 and M4b.
  - `TestCustodianHardHaltBoundsBlockedScan`: M5 and M5b.
  - `TestCustodianKeepsSeparateProofAndCleanupBudgets`: M4 and M6.

  The mutations table below has the messages. See N-3 for the variants that are not caught.
- **F-7: no action needed.** The brief now allows `memory/receipts.log` outside the sets, and the file is unchanged (see Sizes).
- **Round-2 note 1 closed.** Pid 703 is an argv-word child, and the test requires both carriers. M3c (always print `environment`) and M3c2 (always print `argv-word`) both fail with `does not name each child's carrier`.

## Check results

1. **Closure:** see above.
2. **Sizes:** see above.
3. **Sub-units, in copies.** I checked each copy file by file against the worktree and against an unchanged HEAD copy `$R/base`.
   - **3a alone** (`$R/a3`: 3b and 3c files at HEAD, new files removed): gofmt clean, `go build ./...` rc=0, vet rc=0. `go test -race -count=1 ./internal/identity/...`: `ok 2.685s`.
   - **3a+3b** (`$R/a3b`): build and vet rc=0. Identity `ok 6.942s`, testenv `ok 8.580s`.
   - **3a+3c** (`$R/a3c`): build and vet rc=0. `go test -race -count=1 ./cmd/metasystem/` with only `TestDispatchBriefBoundsAdmission` skipped: `ok 425.058s`, rc=0. That test needs `.git`, which a copy lacks, as I showed in round 2.
4. **Full tree** (`$R/full`, byte-identical to the worktree's ten files):
   - build and vet rc=0.
   - `go test -race -count=1 -timeout 40m ./internal/identity/... ./internal/testenv/...`: identity `ok 6.719s`, testenv `ok 8.125s`.
   - `TestProcCustodianProcessBoundaries` with `-race`: `ok 4.159s` at 15:49, and `-count=3` `ok 10.107s` at 16:09, with all four subtests passing every time (`$R/full-cmd-race-x3.out`).
   - Witness 5 and witness 3: 15 runs each, all pass.
   - The three timing-sensitive table witnesses with `-count=500`: rc=0 (`$R/table-stress-500.out`).
5. **Live `proc custodian` checks:** see F-2, F-3 and F-4 above.
6. **Mutations:** see the table.
7. **Census** (argv census and `ps -E` environment census, `$R/tagged3.sh`):
   - Empty before my first run (15:46:20).
   - Empty at 16:06:08, at 16:08:20, and after the final 16:09 run.
   - One process was left during my runs, and my harness caused it, not the unit. In the env-tag scenario the custodian correctly refused to start, and the harness then killed the owner. That left the tagged `sh` child 62328 with ppid 1. I killed it and confirmed it was gone (see N-6).
   - No custodian, helper `identity.test` or tagged process was left.
   - The `sleep 30` processes seen at 16:06 were a few seconds old and had other parents. They belong to other seats.

## Mutations

Identity and 3b mutations ran on the full copy. The cmd mutations ran on the 3c tree. Each mutation was applied to a fresh copy and run with `go test -race`. Outputs: `$R/mut-identity.out`, `$R/mut-3b.out`, `$R/mut-cmd.out`.

| Mutation | Result | Failing test and message |
| --- | --- | --- |
| M1 reap scan without the liveness check | FAIL | WaitsForDeadOwner: `live owner allowed reap: signaled=[702 703]` |
| M2 self-exclusion removed | FAIL | `signaled=[701 702 703] ... never self 701` |
| M3a carrier slots swapped | FAIL | custodian table and `TestCleanupIsScopedToTheFixtureKey` |
| M3b / M3b2 scan hard-codes argv / environment | FAIL | both tests, each time |
| M3c / M3c2 custodian prints a fixed carrier | FAIL | WaitsForDeadOwner: carrier lines wrong |
| M4 first EOF reaps at once | FAIL | `cleanup exceeded 30ms: fixture owner 700 is alive`, and the budgets test |
| M4b first EOF returns | FAIL | both table tests, and both real-process witnesses (`survived owner death`) |
| M5 hard halt never armed | FAIL | HardHalt: `blocked fixture scan was not hard-halted` |
| M5b halt at bound + 3 margins | FAIL | same |
| M6 Unknown after EOF reaps at once | FAIL | Budgets: `cleanup exceeded 60ms: <nil>` |
| M7 reap bound divided by 10 | pass | not caught (N-3) |
| M6b reap gets the remaining budget | pass | not caught (N-3) |
| F-1 revert (no `Setsid` on the stopped child), 5 runs | FAIL 5/5 | `omits pid <n>` |
| F-2 revert in testenv (`F_GETFD`) | FAIL | `accepted descriptor 3 opened by the Go runtime` |
| F-2 testenv without the read-end check | FAIL | same |
| F-5 revert (log inside registry) | FAIL | both real-process witnesses: `did not publish ... action=complete` |
| F-2 revert in cmd (`F_GETFD`) | FAIL | cases 1 and 2 |
| F-2 cmd without the read-end check | FAIL | case 1 |
| F-2 cmd without the FIFO check | pass | not caught (N-2) |
| F-3 revert / empty value allowed / check after `Setsid` | FAIL | case 3: `create session: operation not permitted` |
| F-4 revert | FAIL | case 4: `custodian kept inherited descriptor 4 open` |

## Non-material notes

1. **N-1.** Round 2's smallest fix for F-2 rested on a false premise: "a kqueue fails that test". Darwin reports the kqueue as a FIFO. The read-end check closes F-2, so any later Darwin check that treats `S_IFIFO` alone as "a pipe" needs the same access-mode test.
2. **N-2.** A read-only regular file on fd 3 is covered only by the FIFO check. Opening case 2's file `O_RDONLY` would give that check its own witness.
3. **N-3.** The table reap finishes almost at once, so a shrunken cleanup budget does not show. A table child whose kill needs a few polls to confirm would expose M7 and M6b.
4. **N-4.** This matches the ruling. Survival of the log after the home is removed is not proven by any test.
5. **N-5.** This matters only if unit 10 puts the log path in a shared temporary directory.
6. **N-6.** This is not a ruling disagreement but a unit-10 obligation. The bed prologue must confirm that the custodian got past its startup checks, because exit 2 is invisible to a backgrounded start. Round-2 note 9 is the same concern for testenv.
7. The cmd helper subprocesses go through `testenv.Main` and `prepare()`, then leave with `os.Exit` inside the test function, so their registry homes wait for the dead-home sweep. I found this by reading the code; it does no harm.
8. The witness log is shared by every `-count` run in one binary; the F-1 revert output shows kill lines from earlier runs. The exact `pid=<n> ` match and the micro-stamped `complete` line keep the assertions sound, unless a pid is reused within one binary's life.
9. 3b and 3c are at 297 and 298 of 300, so any further fix will likely need another split.
10. Ruling disagreements: none.

## What I could not check

- **Linux:**
  - the orphaned-group behaviour of witness 5;
  - `/dev/fd` listing and `F_GETFL` on pipe ends there;
  - blocked `/proc` reads under the halt;
  - the environment carrier through `/proc/<pid>/environ`.
- **Unit 10's bed prologue:** it does not exist yet, so `env -u`, the fd 3 pipe it will pass, and its startup confirmation could not be checked (N-6).
- **Log retention** and the D2 gate turned on belong to unit 5.
- **Seat-only checks:** `metasystem test run`, the battery and the testrun lock.
- **`TestDispatchBriefBoundsAdmission`** on 3a+3c: it cannot run in a copy without `.git`.
- **Round-2 notes 2 and 4 to 10:** outside this brief's checks, and I did not recheck them.
