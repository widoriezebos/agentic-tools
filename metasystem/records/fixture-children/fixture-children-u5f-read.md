# Read: fixture-children unit 5f (every test binary starts a custodian it has proved)

Reader: independent Opus read. I checked the diff `fcu5f-unit.diff`: its sha256 is
41b8085c7d40584b68a5ac12096be53901c4f8d8a88eb4692279f2fab96aa25b, which matches the `.sha` file (701 lines, 300
changed). I also read the fix-only diff `fcu5f-fix-r1.diff`, the build and fix briefs, both Codex reports, decisions
items 1, 2 and 6, rulings R1 to R6, the sandbox report, `check-5f.out` and `check-5f-r2.out`.

The worktree was read only. Every mutation ran through `go test -overlay`, with its replacement files in my own
scratchpad (`.../1e0f004f-.../scratchpad/fcu5f-read`). Host: darwin/arm64 with brew Go 1.27.1. I did not run on
Linux.

## Material findings

### U5f-1. The fixed census witness compares refs as structs, and that comparison is false on Linux

`TestCensusFindsAnUntaggedExecutableByRecord` accounts for the copy's custodian by comparing `identity.Ref` values
with `==` and `slices.Contains`. One side comes from `identity.ParseRef` and the other from `Exact.Ref()`. On Linux
those two never produce equal structs for the same process.

- `internal/identity/ref.go:70`, Linux shape: `ref = Ref{Pid: pid, StartTicks: ticks, BootID: values["boot"]}`.
  `StartedAtSec` is not set.
- `internal/identity/identity.go:56`: `ref := Ref{Pid: e.Pid, StartedAtSec: e.StartedAt.Unix()}`. This sets
  `StartedAtSec` on every platform, and `identity_linux.go:83` fills `StartedAt`, so the Linux value is non-zero.
- Darwin's micro shape (`ref.go:64`) sets `StartedAtSec` and `StartedAtUnixMicro` from the same micro value. The
  comparison happens to hold on darwin, which is why the host gate is green.

The comparisons that break:

- `internal/testutil/fixture_test.go:222`: `processCopy.custodian, err = identity.ParseRef(...)`, the ref the copy
  sends.
- `fixture_test.go:231-233`: `fixture.Hold(pid)`, then `if !slices.Contains(fixture.refs, processCopy.custodian)
  { t.Fatalf(...) }`. `Hold` records `exact.Ref()`, so on Linux this fails for every copy before the census runs.
- `fixture_test.go:142` (`got[0].Ref == positive.custodian`) and `:148` (`got[1].Ref != positive.custodian`) are
  the same kind of struct comparison against the parsed ref.

Evidence:

- A small overlay test in my scratchpad built a Linux-shaped `ParseRef` value and the `Exact.Ref()` value for the
  same pid, ticks and boot id. It printed `struct-equal=false`, while the `EncodeRef` strings were equal. This was a
  constructed demonstration run on darwin; I did not run the witness on Linux.
- The witness has no GOOS guard. The build brief asks for refs to be compared through their encodings.
- Decisions item 1 needs `./...` to pass on both platforms. `check-5f-r2.out` shows that the Linux VM step never
  ran.

This is not a load or timing failure. The witness is wrong on Linux every time, so the unit cannot be proved on the
second platform as it stands. The fix is to compare `identity.EncodeRef` values, or to use `SameIdentity` on refs
of the same shape, at all three sites.

## Checklist

1. **The start is unconditional.**
   - `testenv.Main` always calls `startFixtureCustodian(...)`. On error it prints `start fixture custodian: %v` and
     returns 2. No path skips it.
   - The removed start variable has no code reference left: not the constant, not `inheritedControlNames`, not the
     witness helpers. Only records and receipts still name it.
   - The custodian branch (fd 3 and fd 4 checks, then `RunCustodian`) returns before `prepare` and before the start,
     so a custodian never starts another custodian. Each `Main` starts exactly one.
   - `identity` non-test code starts no process.
2. **The handshake.**
   - The owner makes two pipes. The watch read end goes to the custodian's fd 3 and the ready write end to its fd 4,
     both through `ExtraFiles`. The owner closes its copy of the ready writer right after `Start`.
   - The custodian writes `ready\n` to fd 4 and closes it; `RunCustodian` also defers the close. Every other
     descriptor is close-on-exec in Go, and the custodian starts nothing, so no stray write end can keep the owner
     reading forever.
   - Owner on `ready`: it probes the custodian (alive, same identity, not a zombie) and dups the watch writer with
     `F_DUPFD_CLOEXEC`.
   - Owner on end of file: `Wait`, then an error quoting the exit status and the last 8 log lines. `Main` prints it
     and exits 2.
   - Owner on a partial `ready` with no newline at end of file: the scanner hands back the last token, so this is
     accepted as ready. The probe that follows then finds the custodian dead or a zombie, kills it and waits. See
     the notes.
   - `Wait` is called once on each failure path (end of file, failed probe, failed dup) and never on success, where
     the custodian stays an unwaited child.
3. **The ready line's position, and R6.**
   - The order in `RunCustodian` is: input check, self-probe, `signal.Ignore(SIGTERM, SIGHUP)`, chain parse, write
     `ready`, close, `runCustodian`. So `ready` comes after every check and before the watch.
   - A failed write is ignored and the custodian keeps watching. Go returns EPIPE for fds other than 1 and 2 instead
     of dying on SIGPIPE.
   - `metasystem proc custodian` passes nil and still keeps its log. The seat's `TestProcCustodianProcessBoundaries`
     passed, and the code path is unchanged apart from the extra nil argument.
4. **The custodian's ref.**
   - The probed pid belongs to the owner's own unwaited child, so the pid cannot be reused while the owner lives.
   - `fixtureCustodian` is set only in `Main`, once, before `m.Run`.
   - A witness child that re-executes a test binary gets its own `Main` and its own custodian. `FixtureCustodian()`
     inside a process always names that binary's custodian.
   - The census helper sends its own custodian on purpose. That is the copy's custodian, not the parent's.
5. **`Fixture` refuses without a custodian.**
   - `makeProcessFixture` refuses when there is no custodian: "needs a custodian: this binary's TestMain must call
     testenv.Main".
   - Otherwise it probes through the fixture's own prober. It refuses dead, mismatched and zombie custodians with
     "needs a running custodian: state=%s same-identity=%t zombie=%t err=%v".
   - Both checks run before the owner probe and before the key is minted. The table witness covers all four cases.
   - The recording double's `Fatalf` records the message in `errs` and panics with a sentinel that the witness
     recovers. A real setup failure goes through `testing.T.Fatalf` and fails the test.
   - The double's panic outside the witness's recover still ends the binary, as it did on the base. Not a
     regression.
6. **Custodian variables never reach a child.**
   - `identity.FixtureCustodianEnv` is now a prefix in `inheritedControlPrefixes`. It covers the one custodian
     variable left after this unit and any future `METASYSTEM_FIXTURE_CUSTODIAN*` name.
   - The custodian branch reads its variable before `prepare` runs.
   - `startFixtureCustodian` drops the owner and custodian names from the custodian's environment before setting
     `=1`.
   - `ProcessFixture.Env` strips the prefix.
   - Mutation ME confirms that `TestCustodianVariablesNeverReachFixtureChildren` catches a missing prefix.
7. **What the sandbox showed.**
   - The sandbox report matches the code. The start and the self-probe do not need `sysctl kern.proc.all`, so
     `ready` is sent and testenv passes.
   - Only the custodian's exit cleanup scan needs `sysctl kern.proc.all`. In the sandbox it is refused, and the
     custodian keeps a log saying so.
   - Nothing about this weakens the start. A custodian that cannot scan inside the sandbox is a limit of the
     earlier units' cleanup, not of this unit.
8. **Mutations the seat did not run.**
   - **MA:** the owner's probe after `ready` disabled (`if false && ...`).
     - Result: `TestCustodianStartRefusesACustodianThatExitsAtOnce` still passes. No witness covers that probe.
     - Left: nothing.
   - **MC:** the testenv branch's fd 4 check removed.
     - Result: `TestCustodianRejectsMissingReadyDescriptor` hung in `command.Wait`
       (`fixture_custodian_witness_test.go:209`) until the 60 s test timeout (FAIL 60.288s) instead of failing
       with a named state.
     - Left: two `identity.test` processes (pids 71063 and 71064), reparented to pid 1. They were still running at
       09:46:57 and were gone by 09:47:39 without any kill: their watch pipe closed and they finished cleanup.
   - **ME:** the custodian prefix removed from `inheritedControlPrefixes`.
     - Result: `TestCustodianVariablesNeverReachFixtureChildren` fails with "child environment 0 =
       [METASYSTEM_FIXTURE_CUSTODIAN=1 ... KEEP=value]". `TestBinaryExitScanNamesAChildThatOutlivedItsTest` still
       passes.
     - Left: nothing.
   - **MF:** the custodian calls `os.Exit(0)` right after writing `ready`.
     - Result: in three runs of `TestFixtureWritesItsOwnershipRecord`, runs 1 and 3 passed. Run 2 refused with
       "state=alive same-identity=true zombie=true".
     - Left: three empty custodian logs (pids 71110, 71166 and 71179), one per run, because the custodian exited
       before its cleanup. I removed them.
9. **Survivors.**
   - No process from my runs is left. At 09:47:39, nothing matched `1234abcd|detached[.]sh|read -r _` or a `.test`
     binary. The `testutil.test` pids 71086 and 71088 and `testenv.test` 71065, seen reparented during the runs,
     ended on their own. I killed nothing.
   - The three MF logs were removed. The `/private/tmp` custodian log count is back to the 123 that were there
     before my runs, and `$TMPDIR` has none.
   - No registry home was created in my run window.
   - The seat's `detached.sh` 34137 from its own P2 run was not mine. It no longer shows.
10. **Comments.** They are plain English about the code as it is, with no unit, round, finding, ruling or review
    references.
11. **The fix after the seat's first check.**
    - On darwin the census witness is exact. There are two results: the positive copy is matched by pid and exe,
      and its custodian by the ref it sends, which `Hold` records.
    - The outside and wrong copies are still checked, by count. There is no filter, retry, sleep, timed wait or
      custodian opt-out.
    - The witness is not exact on Linux (U5f-1).
    - A held custodian that teardown kills can leave an empty log (S1, below) but no registry home, because the
      copy reuses the parent's.
    - The lines won back do not change behaviour:
      - `Pid != 0` is a correct presence test.
      - Matching `NAME=` prefixes in `Env` restores base matching for the owner and leash names and adds the
        custodian prefix.
      - Keeping fatal messages in `errs` and probing through `liveWitnessProcessRef` keep the witnesses' checks.
      - The map loop's cases are independent, so iteration order does not matter.

## S1 and N1

- **S1.** The seat's cause is right, and a little wider than stated.
  - All three copies' custodians (positive, outside and wrong) are held in both layouts. Teardown SIGKILLs any of
    them that is still in its own exit cleanup, so whether a given copy leaves an empty log is a race. The test
    result does not depend on it.
  - In `TestFixtureCleanupReadsOwnershipRecordBeforeTempDirRemoval`, the copy's custodian is not recorded, and
    `reapKeySurvivors` SIGKILLs it.
  - No registry home is left, and the 7-day sweep removes these logs in time.
  - Alone, I would not call it material, but it does go against the rule that a quiet custodian leaves no log, so
    the fix round should handle it.
- **N1.** Not material. The start is not refused in the sandbox. Only the exit cleanup scan fails, and the log says
  so. The logs come from the sandbox, not the code, and the 7-day sweep limits how many pile up.

## Non-material notes

1. MA: nothing witnesses the owner's probe after `ready`. A custodian that dies after sending `ready` is refused
   only if it is already a zombie when the probe runs. MF showed this depends on timing: `Fixture` refused in only
   one of three runs. No witness asserts on it, so no test is flaky, but the case has no witness.
2. MC: without the fd 4 check, `TestCustodianRejectsMissingReadyDescriptor` hangs until the test timeout instead of
   failing with a named state. `command.Wait` there has no bound other than the watch pipe.
3. A partial `ready` with no newline at end of file counts as ready. The probe catches it only when the custodian
   has already exited. The real custodian always writes the newline.
4. If the scanner stops with an error (ErrTooLong) while the custodian is still alive, the owner calls `Wait`. That
   hangs, because the owner still holds the watch writer and the custodian never sees end of file. The real
   custodian cannot produce this.
5. When the dup fails, the owner now uses `SignalExact` instead of `Process.Kill`. Under descriptor exhaustion on
   Linux the probe behind `SignalExact` may fail too. The custodian is then not signalled, and `Wait` blocks.
6. The testenv custodian branch does not close inherited descriptors from 5 up, while `proc custodian` does (see
   `internal/steward/identity_test.go:307`, where fd 5 reaches the helper's custodian). This is harmless today.
7. ME: `TestBinaryExitScanNamesAChildThatOutlivedItsTest` does not cover the `prepare` scrub. Only the unit test
   covers it.
8. Neither the seat's evidence nor mine shows the full `./...` run on both platforms that decisions item 1 requires.
   The Linux VM step did not run.

VERDICT: NOT LAND
