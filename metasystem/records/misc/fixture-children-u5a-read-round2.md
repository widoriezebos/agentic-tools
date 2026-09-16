# Read: fixture-children unit 5a, after its exe assertion fixes

Reader: independent. I did not write the change or do the first read.

Tree: `g18/wt-fcu5a`, HEAD b9a97d465, index tree cc03bb9c. Unit 5a is unstaged on top.

I ran every mutation and extra test on my own copy of `metasystem/`, in `1e0f004f-.../scratchpad/r2/ms`, outside the worktree. After the mutations I restored `fixture.go` and checked it against the worktree with `cmp`. The temporary probe test was then moved out of the copy. On the VM, all I did was copy three cross-compiled test binaries and one script into `/tmp` on `metasystem-debian`, run them, and delete them. Nothing inside the worktree was created or changed.

## Material findings

### U5a-R2-1: witness 17's `argv=["/bin/sh"` assertion fails under load on both platforms

This assertion was not changed by the fixes. It has the same kind of fault that round 3 removed from the exe assertion: its result depends on when cleanup's probe runs compared with the child's exec.

`exec.Cmd.Start` returns as soon as the close-on-exec pipe closes, which happens before the kernel has finished setting up the new image. On Darwin, `/bin/sh` also execs a second time, into `/bin/bash`. If a probe lands inside either window, it returns `Alive` with no error, a known exe and an unreadable argv:
- on Darwin, `kern.procargs2` fails and `Probe` falls back to `kernelExecutablePath`;
- on Linux, `/proc/<pid>/cmdline` is empty, so `ReadArgv` returns false.

Step 1 then reports `argv=[]`, and the test fails at `fixture_test.go:82`. In cases 1 and 2, cleanup runs within microseconds of `Start`, because `Record`, the injected helper failure and cleanup follow one another with nothing in between. So whether a probe lands in the window is decided only by how the scheduler treats the child's exec.

**Evidence 1: a direct probe of the window.** I added a temporary test to my copy. It started `/bin/sh -c "trap '' TERM\nkill -STOP $$"` 200 times and probed each child 300 times in a tight loop.
- Darwin, three runs: 10, 11 and 12 of about 60,000 probes per run came back `Alive`, `err=nil`, `exeKnown=true`, `argvKnown=false`. Some carried exe `/bin/sh`, some `/bin/bash`.
- VM (Debian, dash): 1 of 60,000 came back the same way, and it was one of the first two probes after `Start`.

**Evidence 2: the real witness under parallel load, with no code change.** These are the pristine binaries built from my copy of this tree.
- **Host (18 CPUs).** I ran 16 plain processes × `-test.count=300` and 10 `-race` processes × 100, at the same time, in the foreground. That is 17,400 subtests. **13 failed**, and every one had this shape:
  `failure text "helper write failed\nfinished child found running at teardown: pid=N exe=\"/bin/sh\" argv=[]" does not contain "argv=[\"/bin/sh\""`
  - 7 were in the helper-write-failed case and 6 in the ownership-mismatch case.
  - Exe was `/bin/sh` in 8 and `/bin/bash` in 5.
  - 10 of the 26 processes exited with rc=1.
- **VM (4 CPUs).** 12 processes × `-test.count=100`, which is 3,600 subtests. **23 failed**, the same way:
  `... exe=\"/usr/bin/dash\" argv=[]" does not contain "argv=[\"/bin/sh\""`
  10 of the 12 processes exited with rc=1.
- **Unloaded runs pass.** That includes the seat's runs, my 10 `-race` runs on the host, and 5 on the VM. That is why no run caught it.

The production code is right. A child probed mid-exec is still killed through `SignalExact`, and the test still fails by pid and exe; argv is best-effort by design (`ArgvKnown`). The witness is what's defective: machine load alone breaks it. The binding rule says a test like that is defective, and that it gets fixed at its cause.

**A fix at the cause (the seat's decision; not applied).** In cases 1 and 2, wait for the child to stop itself before cleanup runs, for example with `syscall.Wait4(pid, &status, syscall.WUNTRACED, nil)`. `kill -STOP $$` runs inside the final shell image, so after the stop both exe and argv are fixed. I checked this with the same temporary test:
- The probe after the stop gave `exe="/bin/bash"` in 200 of 200 Darwin children and `exe="/usr/bin/dash"` in 200 of 200 VM children.
- A later `cmd.Wait` reaped every child normally on both platforms.
- On Darwin, `WaitStatus.Stopped()` returns false for a SIGSTOP stop (status 0x117f). Test the low byte against 0x7f, or skip the check.

The unit is at 299 of 300 lines, so this needs the seat's line allowance.

## Checks, item by item

1. **The fixes and nothing else: holds.**
   - `diff fcu5a-r1.diff fcu5a-r3.diff` shows exactly three differences:
     - the new-file blob index of `fixture_test.go`;
     - `exe="/bin/sh"` changed to `exe="/`;
     - `2 * time.Second` changed to `30 * time.Second` in `waitForFixtureZombie`.
   - Every production hunk (`identity.go`, `identity_darwin.go`, `identity_linux.go`, `fixture.go`) is byte-identical to r1.
   - The worktree's `git diff` is byte-identical to `fcu5a-r3.diff` (`cmp`), and its sha256 matches `g18/fcu5a-r3-diff.sha256`. `git write-tree` of the index is cc03bb9c. The round 2 probe is gone.

2. **U5a-1 is closed.**
   - The seat's VM section in `check-5a-r3.out` shows witness 17 passing 5 times with `/usr/bin/dash`, and the three identity witnesses passing 3 times each.
   - I ran it myself: pristine cross-compiled binaries on `metasystem-debian` (aarch64, `/bin/sh -> /usr/bin/dash`). All three cases of witness 17 passed 5 of 5, and `TestParseProcStat`, `TestProbeReapedChildIsDead` and `TestProbeZombieKeepsItsExactLiveIdentity` passed 3 of 3 each.
   - Under load, Linux fails for the argv reason in U5a-R2-1, not the exe reason.
   - I also read `gate-fast-5a-r3.out`: rc=0, staticcheck included.

3. **Is `exe="/` a real check?** Partly.
   - **What it rejects, by mutation.** Each of these fails cases 1 and 2 with `does not contain "exe=\"/"`:
     - M6, exe blank (`exe=""`, what a probe with `ExeKnown=false` prints);
     - M7, a relative name (`exe="sh"`);
     - M8, exe dropped from the message.
   - **What it cannot tell apart.** Any absolute path passes, including a wrong one such as the test binary's own path. The pid and argv assertions are what tie the text to this child.
   - **A stronger platform-neutral assertion exists.** After the child has stopped itself (see the fix above), probe it and assert `exe=%q` of that probe exactly. After the stop, the exe no longer depends on the launcher exec: 200 of 200 on each platform.
   - **Does the weaker one matter?** Not by itself. Witness 17 must prove that the failure names the child by pid, exe and argv (amendment, revision 5, 3.2 step 1). Pid and argv identify the child, and `exe="/` proves an exe is named. The same stop wait that fixes U5a-R2-1 would make the exact assertion free, though.

4. **The launcher exec against production: holds.**
   - `Compare`, the only rule behind `SameIdentity`, reads `Pid`, `StartedAt.UnixMicro()`, `StartTicks` and `BootID` only. It never reads `Exe`, `Argv`, `Environ` or `Zombie`.
   - `AliveRef` goes through `ReadStart` (Darwin: the kinfo sysctl; Linux: `/proc/<pid>/stat`), then `Compare`. `SignalExact` goes through `AliveRef`. Step 1 is `Probe` then `SameIdentity`, and step 2 is `Probe` then `SameIdentity && !Zombie`. None of these compares exe.
   - Darwin `Probe` returns `Alive` with no error even when `procargs2` fails, so the mid-exec read does not fall into the "unproven" path either.
   - **Empirically:**
     - Across about 180,000 Darwin probes that spanned the `/bin/sh` to `/bin/bash` exec, `SameIdentity` against the first probe's ref was true every time.
     - `SignalExact(prober, ref, SIGKILL)` succeeded on all 600 children after their exec.
     - The VM showed the same.
   - No path in step 1 or step 2 would treat such a child as mismatched, gone and silent.

5. **Timing.** The `argv=["/bin/sh"` assertion is load-breakable (U5a-R2-1). The other assertions and bounds:
   - `waitForFixtureZombie`, 30 s: a hang bound only.
   - The identity zombie witness uses `wiringBound`, 30 s. Its first probe right after `Start` checks only state, error and `Zombie`, all of which were stable in the window probes, and never argv.
   - Case 1's `SignalExact` on a mismatched ref is deterministic.
   - Case 3's exact failure text is deterministic once the zombie is observed.
   - The 2 s bound on `command.Wait` after cleanup relies on step 2 having already observed the zombie. See note 1.
   - Across the 21,000 loaded subtests, no failure other than the argv one appeared.

6. **Mutations on my copy.** Both still fail witness 17 after the assertion change:

   | Mutation | Result |
   |---|---|
   | M1: step 1 kills a running match but reports nothing (`Errorf` becomes `Logf`) | Cases 1 and 2 FAIL: `failure text "helper write failed" does not contain "finished child found running at teardown"`, and the same for the ownership-mismatch case. |
   | M2: step 1 ignores `Zombie` | Case 3 FAILS: `zombie failures = "helper did not observe exit\nfinished child found running at teardown: pid=16454 exe=\"\" argv=[]", want only "helper did not observe exit"`. |
   | M6, M7, M8: exe blank, relative or dropped | Cases 1 and 2 FAIL on `exe="/` (item 3). |

   The baseline before the mutations, `-race -count=10`, passed.

7. **Survivors: none.**
   - **Host.** After each batch (baseline, mutations, window probes and the parallel stress), I ran `ps -axo` filtered for `kill -STOP`, `trap ''`, `read value`, `METASYSTEM_FIXTURE_OWNER` and `w17`. It found nothing each time.
   - **VM.** `ps -eo` with the same filter plus `linux.test` found nothing. `/tmp/reader-fcu5a-r2` and the script are removed.

8. **Comments: clean.** The added comments are:
   - the `Zombie` field comment;
   - `// SZOMB is 5 in Darwin's sys/proc.h.`;
   - the witness's two-line custodian-log comment.

   All three are plain English and describe the code as it is. None mentions a unit, round, finding, review or amendment.

## Non-material notes

1. **The 2 s bound on `Wait` after cleanup in cases 1 and 2 is only as strong as step 2.**
   - Step 2 returns once the child is a zombie, and then `Wait` is immediate. That is why none of the 21,000 loaded subtests failed there.
   - If step 2's design-given 5 s bound expired first, the test would give SIGKILL only 2 more seconds, 7 s in all. That is the same kernel transition case 3 now waits 30 s for.
   - A hang bound here would match case 3. It is not needed to land.
2. **Production reports `argv=[]` for a child probed mid-exec.** That only happens when cleanup runs within a moment of `Start`. The report still fails the test by pid and exe, and argv is best-effort by design.
3. **The first read's carried items still stand.** M3b, M5 and the key-refusal witness are carried to later units on purpose. I did not re-run those mutations.
4. **Darwin's `WaitStatus.Stopped()` returns false for a SIGSTOP stop.** Anyone writing the stop wait should know this (status 0x117f observed; see the fix under U5a-R2-1).

VERDICT: NOT LAND
