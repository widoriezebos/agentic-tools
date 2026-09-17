VERDICT: land (0 material findings)

# Read: fixture-children unit 5f, the fix

Reader: fresh Opus read of the fix alone (`fcu5f-fix-r2.diff`). The unit diff's sha256 matches its `.sha` file
(2d7414f2...dde2). The worktree was only read. Mutations ran through `go test -overlay`, with the replacement files in
`.../1e0f004f-.../scratchpad/fcu5f-reread/`. Host: darwin/arm64, brew Go 1.27.1, `GOCACHE=/tmp/fcu5f-go-cache`. I
did not run anything on Linux. I read the VM results in the seat's check.

## Findings

No material findings. All four are non-material.

1. **Neither half of the log lock has a witness** (`internal/testenv/testenv.go:266-268`,
   `internal/testutil/fixture_test.go:262`). With the owner's `flock` removed (M1), or with the copy's lock wait
   removed (M2), `go test -count=1 ./internal/testutil/` passes. So do `-count=3` runs of the census witness, the
   cleanup-reads-record test and the ownership-record test. On the host, each mutation left 0 new custodian logs
   after 15 seconds, the same as the unmutated tree. The lock only shows in timing: the `-count=3` run took 12.9 s
   with it, and 3.4 s under either mutation. The package took 10.7 s with it, and 7.7 s and 7.5 s without.
   - What the lock protects is a custodian that writes after its owner dies, such as a refused scan or a kill line.
     Without the lock, the cleanup reads an empty log while the custodian is still working and removes it. The
     custodian's later line then goes to a file that no longer has a name.
   - That happens in the Codex sandbox, not on the host. The builder's sandbox runs kept 52 logs, all with content,
     and left no empty ones. Before the fix the same runs left 12 empty logs.
   - Not material: the brief's requirement (no log left) holds, the rule sits in test cleanup, and the 5e witnesses
     cover the custodian's own removal.
2. **The owner's lock has no bound of its own** (`testenv.go:266`). Only one process can hold that lock: a live
   custodian of an earlier owner that had the same registry path and the same pid.
   - That earlier owner is dead, since its pid was reused. Its custodian is therefore in exit cleanup, and
     `armCustodianHalt` ends that cleanup with `os.Exit` within 5 s plus a 1 s margin.
   - Only a stopped custodian would block the start for longer. The start runs before `m.Run`, so the
     `-test.timeout` alarm does not cover it; only cmd/go's kill of the binary does.
   - Every call site uses a new path: `Main` calls the start once per binary, and `testenv_test.go:280,287` use new
     `t.TempDir()` registries.
   - Not material. `LOCK_NB` with a named error would fail at once instead.
3. **The VM whole-package runs do not show `TestFixtureCleanupReadsOwnershipRecordBeforeTempDirRemoval`**
   (`check-5f-r2.out:120-125`). The VM testutil and testenv package runs failed on layout, not on this unit:
   `expect_test.go:211` found no module root, and `protection_test.go:67` could not open `/testing.json`. The check
   keeps only the last two lines (`tail -2`), which can hide earlier failures. The test is also missing from the
   VM's `$UFIX` list. On Linux, only the empty VM log count covers it. This is a gap in the seat's evidence for
   decisions item 1, not a defect in the fix.
4. **The copy helper builds the log name itself** (`fixture_test.go:104`, `.custodian-%d.log`). If testenv renames
   the log, `finishCustodianLog` gets ENOENT and returns without error, and no test fails. The 5e witness does the
   same. Not material.

## Folded checklist

1. **U5f-1.** Checked.
   - `identity.EncodeRef` (`internal/identity/ref.go:19-37`) writes `pid=N;micro=N` on darwin and
     `pid=N;ticks=N;boot=ID` on Linux. That is the pid plus the native start identity on each platform. It refuses a
     ref that is not native-exact, and on darwin it refuses inconsistent start seconds. Its errors reach
     `failOnFixtureError`.
   - All three sites compare encodings: the custodian swap and match at `fixture_test.go:144-151`, and the
     `ContainsFunc` after `Hold` at `:243-244`.
   - No comparison of a parsed ref against a probed struct is left in the unit diff:
     - `testenv_test.go` `binaryRef != current`: both values come from `FixtureCustodian()`, the same probed value.
     - The identity witness uses its parsed `started` only through `SignalExact`, which compares with
       `SameIdentity`.
     - `pid != custodian.Pid` in `TestFixtureRefusesWithoutARunningCustodian` only routes a fake prober.
   - VM: the census witness passed 3 times in both layouts, plain and with GOTMPDIR under TMPDIR
     (`check-5f-r2.out:110-113`, `:129-132`), and left no new logs (`:139-140`).
2. **S1, the production half.** Checked.
   - Only the custodian holds the description:
     - The owner's copy is opened by `os.OpenFile`, which sets close-on-exec, and is closed on return.
     - The custodian branch of `Main` starts no process (identity's non-test code has no exec) and never closes
       fd 2 early.
     - A race-detector symbolizer could inherit fd 2, but only while writing a race report, and a log with a report
       is kept anyway.
   - Failed starts end the process before they return, so no process keeps the lock:
     - The shell exits, then `Wait`.
     - Not ready: `Wait`.
     - Probe failure: `Kill`, then `Wait`.
     - `F_DUPFD` failure: `SIGKILL`, then `Wait`.
   - Unbounded wait: see finding 2.
   - The custodian's own removal still works. `quietFixtureCustodianLog` takes no lock, and unlink ignores advisory
     locks. `TestQuietCustodianRemovesItsLog` starts through `Main` with the lock held. It passed 3 times on the host
     (`check-5f-r2.out:41`) and on the VM in both layouts (`:82`, `:93`).
   - No test outcome changes, and the start stays unconditional (`testenv.go:169`).
3. **S1, the test half.** Checked.
   - Order: `Fixture` registers its teardown at construction (`fixture.go:116`), before the three copies register
     their cleanups. Cleanups run last-in first-out, so in each layout's subtest the copies' cleanups (wrong, then
     outside, then positive) run before teardown can SIGKILL a held custodian.
   - The wait ends when the custodian's fd 2 closes, which is at its exit. It returns at once on ENOENT, or when no
     one holds the lock. The timing in finding 1 shows the wait is real.
   - The empty-log removal would hide, in testutil only, a custodian that exited without its completion line. That
     case is witnessed elsewhere:
     - `TestQuietCustodianRemovesItsLog` waits for the log to be gone, and removal needs the exact completion line.
     - `TestQuietFixtureCustodianLogDecision/empty` covers the empty case.
   - `TestFixtureCleanupReadsOwnershipRecordBeforeTempDirRemoval` runs the recorder's cleanups inside the test body,
     so reaping still SIGKILLs the copy's custodian first. The fix covers it:
     - The copy's cleanup takes the lock after that custodian is gone, then removes the log, which is empty or holds
       only the completion line.
     - The seat's recount shows 0 logs for it (`logcount-5f.out`).
     - My unmutated `-count=3` run left 0.
4. **The seat's counts.** No empty log remains from any step.
   - `check-5f-r2.out:49` listed 8 logs straight after the host package runs, because settling waits for fixture
     processes only. None of those files exists now, and `/private/tmp` has no custodian log newer than 10:20, so
     finishing custodians removed them.
   - W1 and W2 each left one log, which the check removed (`:57`, `:65-67`).
   - The stress run (`:71`), the VM (`:140`) and the end of the check (`:142`) left none.
   - All five 15-second counts in `logcount-5f.out` are empty.
   - The 0-byte logs in `/private/tmp` date from 09:20 to 09:38, before this check (10:22). I left them alone.

## Mutations

| Mutation | testutil package | `-count=3`, three tests | New logs after 15 s |
|---|---|---|---|
| none | ok 10.65 s | ok 12.87 s | 0 |
| M1: owner `flock` removed | ok 7.65 s | ok 3.41 s | 0 |
| M2: copy's lock wait removed | ok 7.48 s | ok 3.44 s | 0 |

My runs left no custodian log: `/private/tmp` held 123 custodian logs before them and 123 after. I removed nothing.
Every folded finding was checked within the budget.
