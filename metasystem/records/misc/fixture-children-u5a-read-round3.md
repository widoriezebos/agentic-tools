# Read: fixture-children unit 5a, after witness 17 waits for the child's stop

Reader: independent. I did not write the change, and I did not do an earlier read.

Tree: `g18/wt-fcu5a`, HEAD b9a97d465, index tree cc03bb9c. Unit 5a is unstaged on top.

What I touched:
- I copied `go.mod`, `go.sum` and `internal/` from the worktree into `1e0f004f-.../scratchpad/r3/ms`, and checked the copy with `diff -r` (exact). I built every test binary there, and made the mutations there. I then restored both files and checked them against the worktree with `cmp`.
- On `metasystem-debian`, I copied five cross-compiled test binaries and one script into `/tmp/reader-fcu5a-r3`, ran them, and removed the directory.
- One slip: my third call wrote a copy of `git diff` to `/tmp/x`. That is outside the worktree, and the next call deleted it.
- Nothing inside the worktree was created or changed. `git diff --check` is clean, `gofmt -l` is clean on both packages, and the worktree diff still hashes to `4d3453123b2c37d3...`.

## Material findings

None.

## Checks, item by item

### 1. The fix and nothing else: holds

- **The tree.**
  - The worktree's `git diff` has sha256 `4d3453123b2c37d3236125785809319a5c95dfae78e881621aeb504cace94cf4`, which matches `g18/fcu5a-r4-diff.sha256`.
  - `git write-tree` gives cc03bb9c, and HEAD is b9a97d465.
  - `git diff --numstat -M cc03bb9c -- metasystem` adds up to 300.
- **Production and the other test files are unchanged.** I compared r3 and r4 from the `identity` hunks through the start of `fixture_test.go`. That covers `identity.go`, `identity_darwin.go`, `identity_linux.go`, `identity_linux_test.go`, `identity_test.go` and `fixture.go`, and they are byte-identical. `diff fcu5a-r3.diff fcu5a-r4.diff` touches only `fixture_test.go`: its blob index, its hunk length, and the lines below.
- **Hunk 1 (`@@ -11,37 +11,39 @@`):**
  - `recordingFixtureTB` becomes `recordingTB`, and `failures` becomes `errs`. This is a rename for compaction, and every use is updated.
  - The four recorder methods go onto one line each, and `Errorf`'s parameter `format` becomes `f`. This is compaction. The bodies are the same: `Logf` does nothing, `Fatalf` panics with the formatted text, `Cleanup` appends, and `Errorf` appends the formatted text.
  - `prober := ...` and `recorder := ...` become one `:=`. This is compaction, with the same values.
  - `var status` plus `syscall.Wait4(pid, &status, WUNTRACED, nil)`, failing on `err != nil || status&0xff != 0x7f`. This is item 1. It sits after `Start` and the deferred `Kill`, and before `Record`, so it runs in all three cases.
  - `stopped, _, err := prober.Probe(pid)`, failing on `err`. This is item 3.
  - The blank line after `recorder.Errorf("%s", helperFailure)` is dropped. This is compaction.
- **Hunk 2 (`@@ -59,25 +61,24 @@`):**
  - The blank line before `recorder.cleanups[0]()` is dropped. This is compaction.
  - `2 * time.Second` becomes `30 * time.Second`. This is item 2, and the rest of the select is unchanged.
  - `recorder.failures` becomes `recorder.errs`. This is the rename.
  - `` `exe="/` `` becomes `fmt.Sprintf("exe=%q", stopped.Exe)`. This is item 3.

No assertion or case is removed.

### 2. U5a-R2-1 is closed at its cause: holds

- **Reasoning.**
  - `kill -STOP $$` is a builtin in bash (macOS `/bin/sh` runs it), in dash (Debian) and in zsh. So the stop comes from inside the final shell image, after every exec has finished.
  - The macOS `/bin/sh` launcher only picks a shell and execs it. It never reads the script, so it cannot be the image that stops.
  - The kernel keeps a stopped child's image, `procargs2`, `/proc/<pid>/cmdline` and `/proc/<pid>/exe` fixed until SIGCONT or SIGKILL.
  - In cases 1 and 2, nothing sends either signal before cleanup's step 1 probe. Case 1's `SignalExact` on the mismatched ref returns `ErrGone` without signalling.
  - So the test's probe and cleanup's probe read the same frozen image.
- **Evidence under parallel load, with a control.** The control binary is r4 with lines 36-39 (the `Wait4` block) deleted. Its probe and `Record` then follow `Start` directly, which puts back the timing the fix removed. The seat's other check was running on both machines at the same time, which added load.
  - **Host, all at once (20:23:28 to 20:23:35):**

    | Binary | Processes × count | Subtests | Result |
    |---|---|---|---|
    | pristine, plain | 8 × 150 | 3,600 | all rc=0, no failure lines |
    | pristine, `-race` | 4 × 40 | 480 | all rc=0 |
    | control, plain | 6 × 150 | 2,700 | 3 of 6 rc=1, 5 failed subtests |
    | control, `-race` | 2 × 40 | 240 | 2 of 2 rc=1, 18 failed subtests |

    Every control failure had this shape: `failure text "...exe=\"/bin/bash\" argv=[\"/bin/sh\" ...]" does not contain "exe=\"/bin/sh\""`. The probe right after `Start` caught the launcher image, and cleanup saw bash.
  - **VM (aarch64, 4 CPUs, `/bin/sh` → `/usr/bin/dash`), all at once:**
    - Pristine, 6 × 100, which is 1,800 subtests: all rc=0, no failure lines.
    - Control, 6 × 100, which is 1,800 subtests: 4 of 6 rc=1, with 12 failures, all of U5a-R2-1's shape: `exe=\"/usr/bin/dash\" argv=[]" does not contain "argv=[\"/bin/sh\""`. 9 were in the write-failed case and 3 in the ownership-mismatched case.
  - So this stress reproduces the flake on both platforms once the stop wait is removed, and the unit as written passed all 5,880 of its subtests.

### 3. `Wait4` and `exec.Cmd`: sound

- **Darwin.**
  - In Go 1.27.1, `os/wait_unimp.go` (built for darwin) makes `blockUntilWaitable` return false.
  - `pidWait` then calls `syscall.Wait4(pid, &status, 0, &rusage)` inside `ignoringEINTR2`.
  - Without `WUNTRACED`, stop reports are never returned. The stop the test consumed cannot be reaped twice, and the call blocks only until the child exits.
- **Linux.**
  - `pidfdWait` calls `unix.Waitid(P_PIDFD, fd, &info, WEXITED, &rusage)`. The fallback, `wait_waitid.go`, uses `WEXITED|WNOWAIT`.
  - Both report exits only, so a stop status that was already consumed changes nothing.
  - `Process.Kill` goes through the pidfd.
- **The error that `case <-waited:` ignores.**
  - In cases 1 and 2 it is the expected "signal: killed". In case 3 the child is a zombie that was killed the same way.
  - An `ECHILD` there would need an earlier reap. The only other reaper is the raw `Wait4`, and it reaps only when the child exits, and then the test fails first. So ignoring the error cannot let a live child pass.
- **Case 3 (kill the stopped child, then wait for a zombie).**
  - SIGKILL ends a stopped process on both kernels.
  - `waitForFixtureZombie` has a 30 s hang bound.
  - Step 1 and step 2 both see `Zombie`, and `command.Wait` reaps afterwards. It passed in every stress run, and M2 below shows the zombie is really observed.
- **`0x7f`.**
  - A stop status is `(sig<<8)|0x7f` on both kernels: Darwin SIGSTOP gives `0x117f`, and Linux gives `0x137f`.
  - An exit or death by signal never has that low byte.
  - Darwin's `0x137f` "continued" status also ends in `0x7f`, but it is reported only with `WCONTINUED`, which the test does not pass.
  - Go's darwin `Stopped()` compares against SIGSTOP (`syscall_bsd.go:127`), so avoiding it was right.
- **The child exits instead of stopping.** I tested this with mutation M3, where the script is `trap '' TERM\nexit 3`.
  - All three cases fail at `fixture_test.go:38` with `child did not stop: status=0x300 err=<nil>`.
  - This took 0.00 to 0.01 s on the host and on the VM. There was no hang.

### 4. The exact exe assertion: timing-independent

- Both reads happen while the child is stopped: the test's probe right after the stop, and cleanup's probe in `stopRunningFinishedChild`. So they read the same exe (see item 2).
- The previous read's exe mutations are still caught, and more strictly:
  - an exe left blank or relative in the report no longer matches `exe="/bin/bash"` or `exe="/usr/bin/dash"`;
  - neither does a wrong absolute path.
- A probe error calls `t.Fatalf` straight away, and nothing blocks after it. The deferred `Kill` SIGKILLs the stopped child. See note 2.

### 5. Timing: nothing else can break under load

- **The `Wait4` itself.** It returns as soon as the child stops or exits, and it has no wall-clock bound. The only thing bounding it is `go test`'s timeout, which is a hang bound.
- **Step 2's five seconds (the previous read's note 1).** This is closed.
  - If `waitForRecordedExits` timed out in case 1 or 2, it would only add one more line to the failure text. The case assertions use `strings.Contains`, so they still pass.
  - The `command.Wait` bound after it is now 30 s, which matches case 3's zombie wait.
  - In case 3, step 2 sees the zombie on its first pass.
- **Other checks.**
  - Case 2's `SignalExact` on a mismatched ref is deterministic.
  - Case 3's exact failure text is deterministic once the zombie is observed.
  - The fixture's owner probe is deterministic.
- **The identity tests** (`TestProbeZombieKeepsItsExactLiveIdentity`, `TestParseProcStat`) did not change between r3 and r4. The previous read's analysis of them still holds.

### 6. Mutations on my copy: both still fail witness 17, on both platforms

| Mutation | Host (darwin/arm64) | VM (linux/arm64) |
|---|---|---|
| M1: step 1 kills a running child but reports nothing (`Errorf` becomes `Logf` for "finished child found running") | Cases 1 and 2 FAIL at `fixture_test.go:83`: `failure text "helper write failed" does not contain "finished child found running at teardown"`, and the same for ownership mismatched | Same, both cases FAIL at `:83` |
| M2: step 1 ignores `Zombie` (`if false && exact.Zombie`) | Case 3 FAILS at `:77`: `zombie failures = "helper did not observe exit\nfinished child found running at teardown: pid=23508 exe=\"\" argv=[]", want only "helper did not observe exit"` | Same shape, pid 61750 |
| M3: the child exits instead of stopping | All cases FAIL at `:38`, `status=0x300`, no hang | Same |

The pristine binary passed all three cases on both platforms before and after the mutations. Every binary had a different sha256, so none of the mutations was left out of its build.

### 7. Survivors: none of mine

- **Host.** I took two censuses, at about 20:25 and at 20:26:26, with `ps -axo` filtered for `kill -STOP`, `METASYSTEM_FIXTURE`, `read value` and my binary names. They found no process from my runs: no `pristine-*`, `ctl-*` or `m[123]-*` binary, and no witness 17 child.
- **Host, not mine.** Three orphaned shells are running `TestLeashedShellExitsWhenItsOwnerLetsGo`: pids 19168, 22526 and 23368.
  - Their ppid is 1.
  - Their keys name owners pid 14089 and pid 14105, with `micro=17895829046...` (20:21:44, before my first build). Neither owner is still alive.
  - They were still present at 20:26:26, waiting in `read -r _ <&3` on the leash.
  - That test is not in unit 5a. These come from the other check, and are for the seat to look at (note 5).
- **VM.**
  - My directory is gone, and no process from my binaries is left.
  - Not mine: pid 61626, `./testutil.test -test.count=40 -test.run ^(TestHelperFailing...|TestKeyScanNamesAnUnrecordedTaggedGrandchild|TestLeashedShellExitsWhenItsOwnerLetsGo|TestCleanupIsScopedToTheFixtureKey)$`, is the other check. Its children 65852 and 65853 carry the `TestKeyScanNamesAnUnrecordedTaggedGrandchild` key. The second census still showed them with ppid 61626.

### 8. Comments: clean

- r4 adds no comment.
- The unit's comments are:
  - the `Zombie` field comment;
  - `// SZOMB is 5 in Darwin's sys/proc.h.`;
  - the witness's two-line comment about the custodian log.
- All of them are plain English and describe the code as it is. None mentions a unit, round, finding, review or amendment.
- The new `t.Fatalf` messages, "child did not stop" and "probe stopped child", are plain.

## Non-material notes

1. **`stopped.ExeKnown` is not asserted.**
   - Suppose a probe of a stopped child could not read exe at all. Both probes would then print `exe=""`, and the exact assertion would pass, where r3's `exe="/` would have failed.
   - No timing makes this happen for a stopped, unreaped child on either platform. A report that leaves out or blanks the exe is still caught, because the test's own probe reads the real one.
   - Adding `|| !stopped.ExeKnown` to the condition on line 41 would put that check back without adding a line.
2. **Failure paths leave a zombie or signal a pid.** Both happen only after a test failure and never cause a hang.
   - If the probe after the stop fails, the deferred `Kill` SIGKILLs the child, but nothing reaps it until the test binary exits. It is a zombie, not a running survivor.
   - If the child exits instead of stopping, the raw `Wait4` reaps it, and on Darwin the deferred `Kill` then signals that pid by number. On Linux it goes through the pidfd.
3. **The renamed recorder type.** `recordingTB` is package-local and does not collide in this tree. A later unit that adds the same name to package `testutil` would fail to compile; nothing would break silently.
4. **Line 81 is 177 characters.** gofmt and staticcheck accept it. This is readability only.
5. **For the seat, outside this unit:** the three orphaned `TestLeashedShellExitsWhenItsOwnerLetsGo` shells on the host (item 7). Their owners are gone and they are still blocked on the leash read. That may be a leak in the check that started them.
6. **What earlier reads carried forward is unchanged:** M3b, M5 and the key-refusal witness. I did not run them again.

VERDICT: LAND
