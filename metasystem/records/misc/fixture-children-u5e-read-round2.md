# Read: fixture-children unit 5e, round 2 (the witnesses never leak on failure and never read a half-written file)

Reader: independent and fresh. I did not write this change or read its first round.

Inputs checked:
- `fcu5e-r3.diff` has sha256 5fd7fd1a...69c6, which matches `fcu5e-r3-diff.sha256`.
- `git diff e9e3065c7 b87001bfc` hashes to the same value.
- The worktree files hash to tree b87001bfc7e138a860b0010d362bf0cdc3f2c8e2. I checked this with a copied index in my own
  scratchpad, and the real index was not touched.
- Numstat against e9e3065c7: 8/2, 94/13, 60/6, 114/1, 1/1, which is 300 changed lines.
- This round's own change (`fcu5e-r2to3.diff`) touches only `fixture_custodian_witness_test.go` and one line of
  `testenv_test.go`. No production code changed.

## What I ran (host, Darwin, go1.27.1, before the coordinator's stop)

All binaries were built with `go test -c -overlay` from the worktree and written to my scratchpad. The mutated sources
are in `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/u5e-mut/<name>/f.go`.
The script is `u5e-mut/run.sh` and its output is `u5e-mut/run.out`. The testrun lock was absent at every start.

| Run | Change | Result | Left behind |
|---|---|---|---|
| U | none, `-race`, the five live witnesses plus `TestCustodianRejectsRuntimePollerAsWatch` | all PASS | nothing |
| ME | the owner in `runWitnessOwner` (line 268) creates `logpath`, sleeps 200 ms, then writes it | the kill and launcher witnesses PASS | nothing |
| MA | the exported launcher witness waits for `shellpid9` (line 112) | FAIL at 30 s, `fixture child did not publish its pid` | nothing |
| MB | the hard witness waits for `stopped9` (line 228) | FAIL at 30 s, `fixture child did not publish stopped9` | nothing |
| MC | `custodianWitness` waits for `pid8` for its first ref (line 216) | FAIL at 30 s | no process; **one custodian log** holding two `action=kill ... result=<nil>` lines and the completion line |
| MD | the quiet witness's parent waits for `custodianpid9` (line 148) | FAIL at 30 s | nothing: the quiet custodian removed its own 0-byte log after the owner was killed |

About the census:
- The "processes left" lines after MC and MD in `run.out` are my own tool shells. Their command lines contain the
  scratchpad path the census matched. They are not fixture processes.
- After everything finished, `ps` for `1234abcd|detached[.]sh|read -r _|identity[.]test|FIXTURE` showed nothing.

## Material findings

None.

## Answers to the checks

1. **U5e-1 (half-written files).**
   - The quiet owner writes `logpath` at line 137 and `custodianpid` at line 139. Line 137 is wrapped in
     `checkWitness`, so a failed write stops the owner before `custodianpid` exists.
   - The parent reads `logpath` (line 150) only after `waitWitnessRef` has read `custodianpid` as a live pid. By then
     the owner's `os.WriteFile` of `logpath` has returned. This holds on every ordering, and the reasoning is true in
     the code.
   - In the kill and launcher witnesses, `runWitnessOwner` writes `logpath` (line 268) before it starts either child
     (lines 269 to 273). `pid0` is written by a child forked after that write returned, so `logpath` is complete when
     the parent reads it after the first ref. ME confirms this: a 200 ms gap inside that write still passes.
   - The other files:
     - `ownerpid`, `shellpid` and `custodianpid` are single `os.WriteFile` calls.
     - `pid0` and `pid1` come from the shell's `printf %s $$ > file`: an open with truncate, then one write.
     - `stop`, `stopped` and `release` are checked only for existence.
   - `waitWitnessRef` treats an empty file as not published, because `ParseInt("")` fails and the loop continues. A
     prefix of a pid would need a write of a few bytes to become visible part-way, which a single `write(2)` to a
     regular file does not do on either kernel. Even then, the prefix would also have to probe alive.
2. **U5e-2 (failure paths, cleanups last in, first out).**
   - Every process named below is reached as stated.
   - *`custodianWitness`*, cleanups registered in this order: owner group kill (212), `pid0` kill, log removal, `pid1`
     kill, custodian kill (225).
     - Failure at the `pid0` wait: only the group kill runs. The custodian lives in its own session
       (`startFixtureCustodian` sets `Setsid`), is not killed, sees the owner die and reaps both children. MC shows
       this: both kill lines were logged and no process was left.
     - Failure at the `pid1` wait: log removal, then the `pid0` kill, then the group kill. The custodian still lives
       and reaps `pid1`'s child. The seat's M5 left nothing on the host or the VM.
     - Failure at the custodian lookup: `pid1` and `pid0` are killed by exact ref, and the log is removed. The seat's M7
       left nothing. This is also the sandbox case, where the lookup always fails.
     - Failure at the `stopped` wait, a dead-wait, the log wait or the log re-read: the custodian is killed first while
       the owner may still live. Both fixture children already have exact-ref kills, which run next. The owner has no
       other child the custodian alone could reap. MB left nothing.
     - The custodian is never killed while a child only it could reap is still alive.
   - *`runLauncherDeathWitness`*, cleanups in this order: launcher group kill (100), child kill, log removal, other
     kill, custodian kill (109).
     - Failure at the owner ref or the `pid0` wait: only the group kill runs (launcher, owner and, when exported, the
       intermediate shell). The custodian survives and reaps the children.
     - Failure at the `pid1` wait: the same, plus the removal and the child kill. The seat's M6 left nothing.
     - Failure at the lookup, the shell ref, `liveWitnessProcessRef`, the dead-waits, the shell check or the log wait:
       both children are killed by exact ref, and everything else is in the launcher's process group. MA left nothing.
   - *Owner alive at lookup.*
     - In `custodianWitness`, the owner sits in `runWitnessOwner`'s endless sleep, and the parent has sent it nothing
       yet.
     - In the launcher witness, the launcher is killed only at line 115, after the lookup.
     - In the quiet owner, the lookup runs inside the owner itself.
   - *Success proofs.*
     - The quiet witness still checks that the log exists (line 153) before `release`, then waits for the custodian
       to die and the log to go.
     - The kill witnesses still see the completion line, wait for the custodian to die, and re-read the kill lines for
       both refs.
     - The launcher witnesses' lookup feeds only a cleanup.
     - Nothing a witness proves on success changed.
3. **The log removal.**
   - `logpath` is complete when it is read (see 1).
   - The log is created by the owner in `startFixtureCustodian` (`O_CREATE`) before the custodian starts. The custodian
     only writes to its descriptor 2 and opens the path read-only (`O_NOFOLLOW`) for its quiet check. A custodian that
     writes after the removal therefore writes to an unlinked file, and its quiet check fails to open the path. It
     cannot bring the log back.
   - On a pass, and on every failure after the first ref, the log is removed. MC shows the one remaining case (see N1).
   - `waitWitnessLog` now quotes the log. The seat's L2 run shows the quote.
   - Failures other than `waitWitnessLog` do not quote the log and now lose it (N2).
4. **The other cleanups.**
   - `TestCustodianRejectsRuntimePollerAsWatch`: at cleanup time the `Wait` goroutine has always finished. Both select
     branches receive from `done` before anything can fail. With Go 1.27, `Process.Kill` after `Wait` returns
     `ErrProcessDone` and sends no signal. The cleanup never races the wait and is safe, though in practice it is dead
     code.
   - `startRegistryOwnerProcess`: every `Wait` runs in the test goroutine. A second `Cmd.Wait` returns "Wait was
     already called", and a `Kill` after `Wait` returns `ErrProcessDone`. It is correct, and it now also covers the
     "empty home" failure, which leaked before.
   - The other starts in `internal/identity` and `internal/testenv` register their kill right after `Start`, or wait on
     the process at once. The exception is N3, which is older than the unit.
5. **Fit.**
   - The inlined constant is spelled the same at both uses (lines 130 and 145).
   - The inlined `stderrPath` names the same file at all three uses.
   - The dropped loop copies are correct under per-iteration loop variables (go 1.27).
   - The owner's "quiet custodian log does not exist" message became a bare `os.Stat` error. That error still names
     the log path and "no such file or directory", and the parent's `waitWitnessRef` failure quotes `owner.stderr`, so
     a reader can still tell what failed. Because `checkWitness` has no `t.Helper()`, the reported line is inside
     `checkWitness` rather than line 136.
   - Three blank lines between functions are gone (before lines 162, 342 and 366). `gofmt -l` is clean.
   - Nothing a reader needs was lost.
6. **Mutations.** See the table. MA and MB are the two the brief suggested, and neither left anything. MC names the
   one failure path that still leaves a log.
7. **Survivors from my runs.**
   - Processes: none. A final `ps` found no fixture child, owner, custodian or `identity.test`.
   - Custodian logs: MC left `/private/tmp/metasystem-test-registry-1636663288.custodian-52519.log`, and I removed it.
     The 0-byte log from MD was removed by its own custodian.
   - The six old-name `.custodian.log` files in `/tmp` (00:35) were there before my start and are not mine.
   - Registry homes: the SIGKILLed launcher-mode processes left dead homes, and the next test binary's `prepare`
     sweep removed them. None of mine remained at the end.
   - I deleted my mutated binaries and my copied index. Only small sources and outputs remain in `u5e-mut` (152 K).
   - Side effect: every mutated binary ran `prepare()`, which sweeps dead homes and logs older than seven days from
     the real `/tmp`.
8. **Comments.** This round adds no comments. The unit's one new comment
   (`FixtureCustodianCompletionLine`) is plain and describes the code as it is.

## Non-material notes

- **N1. A failure before the first ref still leaves a log.**
  - If `pid0` never publishes, the removal is not registered yet. The custodian kills both children, records the
    kills and keeps its log (MC: 215 bytes, three lines).
  - This matches the fix brief ("read it right after the refs are published") and the record's rule that a log which
    records a kill is kept. The seven-day sweep bounds it.
  - If it matters, the owner could publish `logpath` before `ownerpid`, and the parent could register the removal
    right after the owner ref.
- **N2. Cleanup removal hides the log on failures that do not quote it.**
  - After the first ref, a failure in `waitWitnessDead` ("process N stayed alive"), `waitWitnessCustodian`,
    `waitWitnessFile(stopped)` or the shell check now deletes the custodian log.
  - For a dead-wait failure, that log holds exactly the `action=kill ... result=` line a person would need.
  - In the tree the first read covered, those failures kept the log. Only `waitWitnessLog` quotes the log.
  - Also, `log=""` cannot tell a missing file from an empty one, because the read error is dropped.
  - Quoting the log in the kill witnesses' dead-wait failure, or quoting the read error, would close this.
- **N3. Kills registered late, in code older than the unit.**
  - In `custodianWitness`, the owner's group kill (line 212) is registered after `Probe` and `EncodeRef`
    (lines 206 to 211). A failure there leaves the owner, its two children and its custodian running for good.
  - `startWitnessCommand` can also fail on `stderr.Close()` after a successful `Start`.
  - Neither is realistic, because both calls act on the test's own freshly started child or on a regular file. The
    base had the same order.
  - Moving line 212 up to line 206 costs no lines.
- **N4. A timing bound older than the unit.** `TestCustodianRejectsRuntimePollerAsWatch` fails if the custodian has
  not exited within `time.After(2 * time.Second)`. A heavily loaded machine alone could break that. The line is in the
  base tree, and this round only added a cleanup beside it. It belongs in the backlog, not in this unit.
- **N5. Group kill by a pid that may be reused, older than the unit.** The group-kill cleanups
  (`syscall.Kill(-command.Process.Pid, SIGKILL)`) run after the launcher or owner was reaped. If the whole group is
  gone and the number has been reused as another process group's id, the signal reaches that group. This is in base
  and unlikely within seconds.
- **N6. The seat's census does not see owners or custodians.**
  - `check-5e-r3.sh` runs the mutated binaries as `./identity.test` from inside the check directory, so their owners
    and custodians carry neither the directory path nor any census pattern.
  - The package runs use `go test`'s temporary binary path.
  - "processes left: none" therefore covers fixture children only. My final `ps` for `identity[.]test` covered the
    rest for my runs.

## Runs not made

The coordinator stopped all `go test` and `go build` runs. My verdict does not depend on these runs. They would only
widen evidence the seat and I already have. Run each from
`/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/e67ca063-900a-46ec-9bc8-9fbdcbc3765e/scratchpad/g18/wt-fcu5e/metasystem`,
with `M=/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/u5e-mut`.
Take a census before and after, using the pattern `1234abcd|detached[.]sh|read -r _|identity[.]test`.

1. MA and MB on the Linux VM (the seat's VM run covered only M5 and M7):
   `for m in MA MB; do GOOS=linux GOARCH=arm64 go test -c -overlay $M/$m/overlay.json -o $M/$m/identity-linux.test ./internal/identity/; done`
   then `limactl copy` each binary to `/tmp/seat-fcu5e-r3/$m/` and run
   `./identity-linux.test -test.count=1 -test.v -test.run '^TestCustodianWatchesTheExportedLauncher$'` (MA) and
   `./identity-linux.test -test.count=1 -test.v -test.run '^TestCustodianReapsStoppedAndDetached$'` (MB). Expected
   result: FAIL, with no process and no new custodian log left.
2. MC on the Linux VM, to confirm N1 there:
   `GOOS=linux GOARCH=arm64 go test -c -overlay $M/MC/overlay.json -o $M/MC/identity-linux.test ./internal/identity/`,
   then run `./identity-linux.test -test.count=1 -test.v -test.run '^TestKilledTestBinaryLeavesNoFixtureChild$'`.
   Expected result: FAIL, no process left, and one log holding two kill lines. Remove that log afterwards.

VERDICT: LAND
