# Read: fixture-children unit 5a

Reader: independent. Tree: `g18/wt-fcu5a`. Base tree cc03bb9c; 4a and 4b staged; 5a unstaged.
All mutations and extra tests ran on my own copy of `metasystem/`, outside the worktree (`1e0f004f.../scratchpad/mut/ms`). I restored the copy afterwards and checked that it matched the worktree byte for byte (`cmp`). The only thing I did on the VM was copy two cross-compiled test binaries into `/tmp/reader-fcu5a` on `metasystem-debian`, run them, and delete the directory.

## Material findings

### U5a-1: witness 17 cases 1 and 2 fail on Linux; the exe assertion hard-codes `/bin/sh`

`fixture_test.go` asserts that the failure text contains `exe="/bin/sh"`. On Linux, `Exact.Exe` is `readlink /proc/<pid>/exe`, which gives the resolved binary, not the path passed to `execve`. On the validation VM (Debian 12, `/bin/sh -> dash`) that is `/usr/bin/dash`.

Evidence: I cross-compiled the unmodified test binary (`GOOS=linux GOARCH=arm64 go test -c ./internal/testutil/`) and ran `-test.run '^TestHelperFailingBeforeItsKillPointLeavesNoChild$'` on `metasystem-debian`:

```
--- FAIL: .../helper_write_failed
  failure text "helper write failed\nfinished child found running at teardown: pid=53415 exe=\"/usr/bin/dash\" argv=[\"/bin/sh\" ...]" does not contain "exe=\"/bin/sh\""
--- FAIL: .../helper_ownership_mismatched   (same)
--- PASS: .../helper_did_not_observe_exit
```

The behaviour is right on Linux: the child is named with pid, exe and argv, and it is killed. The test is what's wrong. Still, a permanent test in this unit fails on a platform the suite is validated on, so the tree cannot land as it is.

The fix is small. Assert the exe that the probe actually reports: take it from the child's own probe, or from `filepath.EvalSymlinks("/bin/sh")`. `argv=["/bin/sh"` already holds on both platforms. The diff sits at 299 of 300 lines, so the fix has to be line-neutral, or the seat has to allow it.

The gate did not catch this because `go-gate.sh --fast` and all of the seat's runs were on Darwin, and `GOOS=linux go vet` does not run tests.

## Checks, item by item

1. **Boundary: holds.**
   - Against the base tree, the unit touches exactly 7 files: 294 insertions and 5 deletions, 299 lines in all.
   - Outside `internal/testutil`, the only changes are the `Zombie` field, the Darwin and Linux fills, the Linux parse table case and the zombie witness.
   - I found none of these in `fixture.go`: `Hold`, key scan, leash, `ShellPrologue`, ownership record, `testenv.Main` change, gate change, custodian change or converted fixture.
   - Nothing outside the tests calls `testutil.Fixture`.

2. **Zombie field: correct.**
   - **Darwin offsets.** In the SDK `sys/proc.h`, `struct extern_proc` starts with the `p_un` union (16 bytes). Then come `p_vmspace` at 16, `p_sigacts` at 24, `p_flag` (int) at 32, and `p_stat` (char) at 36, followed by 3 bytes of padding and `p_pid` at 40. That agrees with the `enumerate_darwin.go` comment. `SZOMB` is defined as 5 at `sys/proc.h:152`.
   - **Darwin length check.** It moved from `< 12` to `<= 36`. `ReadStart` calls `unix.SysctlRaw` directly, not the injectable `sysctlRaw`, and a real reply is 648 bytes, so this changes nothing in practice.
   - **Linux parse.** `procStatZombie` uses the last `)`. I ran the same function body on its own:
     - `1 (a) Z b) S 1 2` gives false;
     - `1 (w) Z) Z 1 2` gives true;
     - `1 (sh) Z 1 2` gives true;
     - `1 (Z ) R 1` gives false.
   - **Linux live.** `TestParseProcStat` and `TestProbeZombieKeepsItsExactLiveIdentity` both pass on the Debian VM.
   - **Liveness.** A zombie stays `Alive` with the same identity on both platforms. The witness passes on Darwin (my run) and on Linux (VM run). `ReadStart`, `Probe` and `AliveRef` still return `Alive`.
   - **Callers.** `Zombie` is read only in `testutil/fixture.go` and in tests. `Exact` has no json tags and is never marshalled whole. The places that hold it:
     - `humanauthority.Snapshot` records `refOf(exact)` and an argv digest;
     - `census.TaggedProcess` compares through `SameIdentity`;
     - `proofrun.processIdentity` copies fields one by one;
     - `lease` and `run/waiter` build or read single fields.
   - No non-test `reflect.DeepEqual` touches `Exact`. The custodian, janitor, supervise, dispatch and proofrun all decide through `Compare`, `SameIdentity` or `AliveRef`, and none of those read the new field. No record, file or digest changes.

3. **Cleanup step 1, against revision 5: matches.**
   - Mismatch, or `Dead` with no error: returns silently.
   - `Alive`, matching and a zombie: nothing is sent and nothing is reported.
   - `Alive`, matching and running: `SignalExact(f.prober, ref, SIGKILL, f.signal)`, then an unconditional `Errorf` "finished child found running at teardown: pid exe argv". Ownership proof affects only the extra "unproven at signal" line. It never suppresses the report.
   - An error or `Unknown`: nothing is sent, and `Errorf` reports "identity unproven at teardown: ref=...".
   - The only send path is `SignalExact`. It calls `AliveRef`, which uses `ReadStart` for the kernel prober, right before the send. Nothing signals a bare pid or a group.
   - Temp witness (sequence A, A, B): cleanup sends 0 and reports once, so a re-proof that fails at signal time is honoured.

4. **Step 2: matches.**
   - A 10 ms ticker, a 5 s deadline, and a pending set of error, `Unknown`, or running match. It fails by ref.
   - An unproven ref costs the full 5 s and is reported twice. Temp test: `took=5.002s`, failures "identity unproven at teardown" and "did not exit within five seconds". This only happens on a test that is already failing, so I think it is acceptable (see note 3).

5. **Fixture, Env, Shell, Record: all behave as the brief asks.** I checked each with temporary tests on my copy.
   - **Cleanup registration.** `t.Cleanup` is registered inside `newProcessFixture`, before it returns.
   - **Refused name.** `newProcessFixture(tb, "TestBad|name", ...)` calls `Fatalf` with `create process fixture for test name "TestBad|name": identity: invalid fixture key` and registers no cleanup. The name is not rewritten.
   - **Env.** A base holding two old tags comes back as `["A=1" "B=2" <tag>]`, and the caller's slice is unchanged.
   - **Shell argv.** The kernel's argv is `["/bin/sh" "-c" "shift\n..." "sh" "METASYSTEM_FIXTURE_OWNER=pid=...;micro=...|TestReaderShell|d508a079" "first" "second arg"]`.
   - **Shell tag and arguments.** There is exactly one tag word. `FixtureTag` over argv alone returns the fixture's key with carrier `argv-word`. The environment has one tag. The script printed `first|second arg|2`.
   - **Record of a dead pid.** One log line, no refs stored, no failure.
   - **Injection.** `newProcessFixture` and every field are unexported. The exported API is `Fixture`, `Key`, `Env`, `Shell` and `Record`, with no test hooks.

6. **Witnesses.**
   - The zombie witness makes the zombie by closing stdin and withholding `Wait`. Witness 17 case 3 makes it with `Kill` and a withheld `Wait`, then polls for the state. Neither uses a sleep.
   - Case 1 and case 2 assert the helper's text, "found running", the pid, the exe and the argv prefix, and that `Wait` returns within 2 s.
   - Case 3 asserts that the failure text is exactly the helper's.
   - After cleanup in case 1 and 2, the 2 s bound on `Wait` holds up under load, because step 2 has already waited for the zombie.
   - Beyond U5a-1, see notes 2 and 5.

7. **Mutations on my copy, each against the named test.**

   | Mutation | Result |
   |---|---|
   | M1: step 1 kills a running match but reports nothing (`Errorf` replaced by `Logf`) | Witness 17 cases 1 and 2 FAIL: `failure text "helper write failed" does not contain "finished child found running at teardown"`. Behavioural. |
   | M1b: step 1 returns silently, with no kill and no report | Cases 1 and 2 FAIL: `fixture child remained after cleanup`, 7 s each. |
   | M2: step 1 ignores `Zombie` | Case 3 FAILS: `zombie failures = "helper did not observe exit\nfinished child found running at teardown: pid=6934 exe=\"\" argv=[]"`. |
   | M3: skip the identity comparison and call `f.signal(int(ref.Pid), SIGKILL)` | The whole permanent `internal/testutil` package PASSES, so no permanent test catches it. A temporary table in witness 2's shape (A at Record, B at cleanup) FAILS: `sent=1`. |
   | M3b: keep the first comparison, bypass only `SignalExact` | The permanent package PASSES. The witness-2 table (A then B) also PASSES. Only a temporary three-read sequence (A, A, B) FAILS: `sent=1`. |
   | M4: Darwin `ReadStart` returns `Dead` for `SZOMB` | Zombie witness FAILS: `unreaped child probed dead`. Witness 17 case 3 also FAILS: `killed child did not become a zombie: state=dead`. |
   | M5: step 2 removed | The whole permanent `internal/testutil` package PASSES. |

   **Is M3 without a permanent guard material?** Not for 5a. Nothing calls `Fixture` except witness 17, and the code sends only through `SignalExact` (item 3). The seat lands witness 2 in the next unit, before any caller. That witness, as the page specifies it, catches M3 but not M3b (see note 1).

8. **Survivors: none.**
   - Host: `ps -axo` filtered for the tag, `kill -STOP`, `read value`, `read x` and `trap '' TERM`, plus stopped or zombie `sh` processes. Nothing found after all runs, including the 7 s M1b runs and every witness 17 run.
   - VM: the same `ps` filter found nothing, and `/tmp/reader-fcu5a` was removed.

9. **Comments: clean.**
   - `// SZOMB is 5 in Darwin's sys/proc.h.`, the field comment, and the witness comment on custodian-log assertions are plain English and describe the code.
   - No unit, round, finding, review or amendment references.

## Non-material notes

1. **The witness 2 that 5b brings should include a re-proof-at-signal case.** The page's table (pid 500 at A when `Record` runs, at B at cleanup) catches a bypass of both the comparison and `SignalExact` (M3). It does not catch a bypass of `SignalExact` alone (M3b), because the first comparison already turns the table away. A three-read sequence (A at record, A at the step 1 probe, B at the signal-time re-proof), expecting `sent=0` and one "found running" failure, catches it. My temp test did exactly that.
2. **Step 2 has no permanent witness (M5 survived).** A table where the injected prober stays a running match after the kill, expecting "did not exit within five seconds", would guard it. The brief's proof list did not ask for one.
3. **An unproven ref costs 5 s and two failure lines.** Waiting rarely turns `Unknown` into a proof. Skipping refs that step 1 already reported unproven would save the 5 s, but the test fails either way.
4. **The `EncodeKey` refusal has no permanent witness.** The behaviour is right (item 5), and it is a single `Fatalf` branch with no caller yet.
5. **Witness 17 case 3 has a 2 s hang bound.** `waitForFixtureZombie` gives SIGKILL 2 s to turn the child into a zombie. That bound only breaks under load. The identity zombie witness waits for the same change with `wiringBound` (30 s), and using the same bound here would remove the flake risk.
6. **A setup failure would crash the test binary.** `recordingFixtureTB.Fatalf` panics, so a setup failure inside `newProcessFixture` in witness 17 would crash the binary instead of failing the subtest. It is only reachable if the owner probe fails.
7. **The exported fixture identifiers have no doc comments.** That covers `ProcessFixture`, `Fixture`, `Key`, `Env`, `Shell` and `Record`. Staticcheck passed on the seat's run.
8. **The builder's report is accurate on everything I checked.** Its offsets, its line counts, and the M1, M2 and M4 outcomes all match mine. Its worktree path has a typo (`GitHub/agentic-tools-m1c`). It did not run staticcheck or the full identity race suite; the seat's gate run covers both. It did not run any Linux test, which is how U5a-1 got through.

VERDICT: NOT LAND
