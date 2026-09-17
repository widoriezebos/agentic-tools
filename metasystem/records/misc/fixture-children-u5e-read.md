# Read: fixture-children unit 5e (the custodian log is kept only when it says something)

Reader: independent, fresh. Diff `fcu5e-r2.diff` sha256 77b6ee94...83b7d matches `fcu5e-r2-diff.sha256`. The worktree
files hash to tree f56f85f72672e8839593ecdaf10619bfbd5e8e76 (checked with a temporary index, outside the real index).
Numstat against e9e3065c7: 8/2, 98/10, 60/6, 113/1, 1/1, for 300 changed lines, which is the ceiling.

## What I ran (host, Darwin, go1.27.1)

- `go test -race -count=1` of `TestQuietFixtureCustodianLogDecision` and
  `TestRemoveDeadRegistryHomesKeepsLiveAndUnrelatedDirectories`: pass.
- `go test -race -count=2` of the five live identity witnesses (quiet, both kill witnesses, both launcher witnesses):
  pass, 22 s. No custodian log was left in `/tmp` or `$TMPDIR` afterwards, and no process survived.
- Five overlay mutations, listed under check 8.

## Material findings

### U5e-1. The quiet witness can read an empty log path, so load alone can fail it

`TestQuietCustodianRemovesItsLog`, parent side:

    waitWitnessFile(t, filepath.Join(dir, "logpath"))
    logPath, err := os.ReadFile(filepath.Join(dir, "logpath"))

The owner publishes that file with `os.WriteFile`, which is two system calls: an open that creates the file empty, and
then a write. `waitWitnessFile` returns as soon as `os.Stat` finds the file, so if the parent polls between the owner's
open and write, it reads `""`. It then waits for a file named `""`, which never exists, and fails after the 30 s
limit with "fixture child did not publish .". On a quiet machine the gap is tens of microseconds against a 10 ms
poll, so this rarely happens. On a loaded machine it happens whenever the owner is descheduled between the two
calls. That makes this a test that load alone can break.

The existing witnesses do not have this problem. They read `logpath` only after files that the owner writes later
have appeared (`waitWitnessRef` also treats an empty pid file as not yet published).

Evidence. I ran an overlay that only widens the gap: the owner creates `logpath`, sleeps 200 ms (standing in for a
descheduled owner), then writes it. Result: `fixture_custodian_witness_test.go:339: fixture child did not publish .`
and `--- FAIL: TestQuietCustodianRemovesItsLog (30.01s)`. Nothing was left behind: the owner was killed at cleanup, and
its quiet custodian removed its own log.

Fix at the cause: read `logpath` only after the owner has published something it writes later (for example, wait
for `custodianpid` first), or have the owner publish `logpath` by renaming a finished file into place.

### U5e-2. The kill witnesses now leave fixture children running forever when they fail before reading every ref

In `custodianWitness` and `runLauncherDeathWitness`, the unit now finds the custodian and registers its kill
**before** it reads the fixture refs:

    custodian := waitWitnessCustodian(t, owner.Ref())
    t.Cleanup(func() { _ = SignalExact(KernelProber{}, custodian, syscall.SIGKILL) })
    ... waitWitnessRef(pid0), register kill; waitWitnessRef(pid1), register kill ...

The fixture children run in their own sessions, so killing the owner's group does not reach them. Only the
custodian or a registered ref kill does. That leaves two paths where they outlive the test. The base witnesses had
neither:

1. **Failure between finding the custodian and reading the last ref.** Cleanups run last-in, first-out: the custodian
   is killed first, and the owner's group after it. The custodian dies while the owner is still alive, so nothing
   ever reaps the child whose ref was not yet read.
   Evidence, overlay that makes the second ref never publish (waits for `pid9` instead of `pid1`):
   - With the unit's witness: the test fails after 30 s. Fixture child pid 69499
     (`/bin/sh -c printf ... while :; do sleep 1; done`, carrying the owner tag, parent 1) was still running 45 s
     later, and I killed it. An empty (0-byte) custodian log was left in `/tmp`.
   - With the base witness (`git show e9e3065c7:...fixture_custodian_witness_test.go` plus the same one-line change,
     run twice on the unit's production code): the same failure, but no survivor. The custodian killed the child and
     logged `action=kill pid=71508 carrier=argv-word result=<nil>` followed by the completion line.
2. **A custodian that cannot be found and cannot reap**, such as the builder sandbox, which denies `kern.proc.all`.
   Codex's report says both kill witnesses "failed while locating their custodians" there. No ref kill has been
   registered at that point, and the sandboxed custodian cannot scan either, so both children of each witness run
   forever. Before the unit, the refs were read first and their deferred kills ran on failure.
   Supporting observation: at 00:13 this machine held 8 fixture children of exactly this shape:
   - `detached.sh` and the `printf/sleep` loop, for `TestKilledTestBinaryLeavesNoFixtureChild` and
     `TestCustodianReapsStoppedAndDetached`;
   - pids 75932, 75942, 76122, 76131 (started 23:38:26) and 42550, 42553, 42776, 42779 (started 23:39:20);
   - owners dead, parent 1, temp directories gone.

   That fits two sandboxed runs failing at the custodian lookup. I did not start them and did not kill them, and they
   were gone by 00:18. I cannot prove where they came from. The seat's r2 check reports "host survivors:" as empty,
   so its survivor count does not cover processes like these.

This is the leak the plan exists to prevent, and leaked children compound into flakes. The unit's witness rewrite
introduced it. Fix: in both witnesses, read the refs and register their kills first, then find the custodian (the
owner is still held alive at that point in both), so every fixture child has a kill before any wait that can fail.
Registering the log removal when the path is first known would also stop failed runs leaving logs (see N7).

## Answers to the checks

1. **Conformance.** All four items are built as briefed. Seven witnesses are present:
   - the quiet witness;
   - both kill modes on the per-owner path, with the custodian found through the kernel and waited dead before the
     re-read;
   - the decision table with all nine rows;
   - the symlink and different-file cases;
   - the sweep table with every briefed case;
   - `proc_custodian_test.go` unchanged;
   - `TestQuietFixtureCustodianLogDecision` added to `test-environment-standard`, which the inventory test requires.
     The identity witnesses are not registered, which matches the existing custodian witnesses.

   Extras beyond the brief:
   - the launcher-death witness also finds its custodian and registers a kill;
   - `defer` became `t.Cleanup`;
   - `runWitnessOwner` registers kills for its commands;
   - the soft mode of `custodianWitness` now also requires both kill lines. Before, only the hard mode checked them,
     and Codex's report does not mention this change (N6).

   Existing results that change:
   - the log name;
   - quiet logs are removed;
   - old logs are swept;
   - the stronger soft-mode check;
   - the sweep test's backdating goes from 2 h to 8 days (its existing assertions still hold);
   - U5e-2's failure-path regression.
2. **Decision (R3).** Removal happens only when the whole content equals `FixtureCustodianCompletionLine(owner)`, the
   one function the writer in `reapDeadOwner` also uses. Nothing else writes `action=complete`. After `RunCustodian`
   returns, only `watch.Close()` runs before the check, and only `os.Exit(0)` after the removal. No goroutine writes
   to stderr in that window, so a late race report or panic can only be lost in microseconds that have no source
   today (N3). Nothing is written after the check.
3. **Same-file rule.** The code opens with `O_NOFOLLOW`, checks `Fstat` for a regular file with the same inode as
   fd 2, reads through the fd, then runs `Lstat` again and requires the same file. A window remains between that
   `Lstat` and `os.Remove`: a rename over the path in that window gets the new name unlinked (never a symlink target).
   In `/tmp` (sticky) only the same user can rename over this user's file, and the per-user `$TMPDIR` is private. The
   same user can delete the file anyway, so this is a note (N1).
4. **Where removal runs.** Only in `testenv.Main`'s custodian branch, only after `RunCustodian` returns nil, and only
   with the log variable set. `cmd/metasystem/identity.go` and `RunCustodian` are unchanged, so `proc custodian`
   keeps its log. An error return exits 2 before the removal. A halt is `os.Exit(2)` from a timer, so it never
   reaches the removal, except for the race in N2.
5. **One log per owner.** Two live binaries have different pids, so they get different logs. The log is opened with
   `O_APPEND` and no truncate, so an existing file at the path keeps its content. The appended log is then not quiet,
   and it is kept, which is the safe direction. A shared log needs a pid reused while the older custodian of that pid
   in the same registry home is still running (a few seconds). Only then could the older custodian remove the shared
   file (N4). Pids are allocated in sequence on both platforms, so this is not practical.
6. **Sweep.** One `time.Now()` per pass. `entry.Info()` is an lstat, so symlinks and directories are refused. The
   names accepted are `metasystem-test-registry-*` ending in `.custodian.log` or `.custodian-<digits>.log`, and
   files must be strictly older than seven days. A live custodian's log goes only if its owner lives more than seven
   days without the custodian writing anything (for example a `-timeout 0` hang). That is not practical, and
   `proc custodian` logs use explicit `--log` paths (N5).
7. **Witnesses.**
   - The custodian pid comes from the kernel (children of the owner whose environment carries the owner variable),
     and the ref is taken while the owner is held. Every wait is for a state, and 30 s is only a hang cap.
   - L2 fails `custodianWitness` every time. It waits for the custodian to be dead before re-reading, and removal
     precedes exit. If the witness misses the completion line first, the wait fails instead.
   - The launcher witnesses are not L2 witnesses and need not be.
   - On a pass, no witness leaves a log: I counted 0 after my runs.
   - Load: U5e-1.
   - The cleanup removal does not hide an assertion, because it runs after all of them. It does remove the log
     before anyone can read it when `waitWitnessLog` fails, and that failure message does not quote the content (N7).
   - Failure paths: U5e-2.
8. **Mutations (overlay, my copies):**
   - M1, the completion check ignores the owner (any single `fixture-custodian owner=... action=complete` line):
     **fails** `TestQuietFixtureCustodianLogDecision/another_owner` ("removed=true, want false").
   - M2, the sweep compares ages with the wrong sign: **fails** the sweep test. Both old logs survived, and both young
     logs plus the dead home's young log were removed.
   - M3, the removal skips the second `Lstat`: **survives**. Both tables pass, and no witness covers a path replaced
     after the open. That check is defence beyond the brief's rule, so this is a note (N1).
   - M4, the owner's `logpath` publication widened by 200 ms: **fails** the quiet witness (U5e-1).
   - M5, the kill witness's second ref never publishes: fails, and **leaves a fixture child and an empty log**. Against
     the base witness it fails with no survivor (U5e-2).

   The seat's L1 and L2 and Codex's five mutations were not repeated.
9. **Survivors from my runs.**
   - I killed fixture child 69499 from M5.
   - I removed the three custodian logs my M5 and base-comparison runs left in `/tmp` (the 0-byte one, and
     `...1770352201.custodian-71505.log` and `...1478746582.custodian-76752.log`).
   - I removed my overlay copies.
   - Afterwards: 0 custodian logs in `/tmp` and `$TMPDIR`, and no process of mine alive. The 8 older survivors named
     in U5e-2 were not mine.
   - Side effect: the M2 and M3 mutated test binaries also run their own `prepare()` sweep on the real `/tmp`. At the
     time, `/tmp` held no custodian logs, so M2's wrong-sign sweep removed nothing there. Other readers running
     sweep mutations through `-overlay` should know that they sweep the real `/tmp`.
   - Six old-name `.custodian.log` files that were in `/tmp` at 00:08 were gone by 00:10, after my unmutated table
     run swept `/tmp` (or another seat's run did). The seven-day rule removes only old files.
10. **Comments and layout.** Plain, about the code as it is. No references to units, rounds, findings or rulings. The
    new `testenv` functions have no comments, which is fine for unexported helpers.

## Non-material notes

- N1. The second `Lstat` has no witness (M3 survives), and a replace between that `Lstat` and `Remove` is still
  possible for a same-user writer. Either accept this, or open the parent directory and use `unlinkat` after an
  `fstatat` comparison.
- N2. The halt timer is stopped by `defer stop()` in `reapDeadOwner`, but `timer.Stop` cannot stop a halt that is
  already running. If the halt fires at the moment of completion, a custodian can exit 2 and still have removed a
  log that held only its completion line. Nothing a person could read is lost. The halt design predates this unit.
- N3. Anything written to stderr after the re-read (no source exists today) would go to an unlinked file.
- N4. Pid reuse inside a live older custodian's lifetime, in the same registry home, lets the older custodian remove
  a log the newer one shares. This is not practical.
- N5. The sweep could remove the log of a custodian whose owner has lived over seven days without the custodian
  writing. This is not practical for test binaries.
- N6. The soft mode now asserts kill lines for both refs. This is a stronger existing result, not in the brief
  ("the kill lines it checks today"), and not reported by the builder.
- N7. The kill witnesses read `logpath` only after their dead-waits, so any earlier failure leaves the log behind (M5
  left a 0-byte log). The fix brief said "as soon as the path is known". The seven-day sweep bounds this. When
  `waitWitnessLog` fails, the log is removed without its content being quoted.
- N8. A directory whose name has the log form now skips the registry-home check. `MkdirTemp` names never end in
  `.log`, so nothing changes in practice.

VERDICT: NOT LAND
