# Read: fixture-children unit 4a, after its witness fix

Tree read: `g18/wt-4a-r2`, HEAD 23f470d371a9c19f36bf33ad07d40b6849073dbb (trunk), 4a unstaged on top. `git diff` in the worktree is byte-identical to `fcu4a-r2.diff`, and that file's sha256 (a210078b...4b4446) matches `g18/fcu4a-r2-diff.sha256`. Size against trunk: 138 + 38 + 108 + 16 = 300 changed lines, at the ceiling.

I changed nothing in the worktree and took no testrun lock. Experiments ran on my own copy of `go.mod`, `go.sum` and `internal/` under `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/cp`. I checked that the copy matched the worktree before building and after each mutation was undone. The binaries were built without `-race`, with go1.27.1 on darwin/arm64.

Materiality test: would the change ship a defect, violate its brief, or damage what certifies it?

## Material findings

None.

## 1. U4a-1 is closed

The plain branch now runs `/bin/sh -c '"$1" -test.run="^$2$" -test.count=1; exit $?'`. Before the kill it probes `command.Process.Pid` through `liveWitnessProcessRef(..., owner.Pid)`, which fails if that pid is the owner's. After the kill it waits for the owner and both children to die, then reads the log path the owner wrote (`logpath`, set to `registry + ".custodian.log"` in `runWitnessOwner`) and requires `dead-launcher=<the shell's exact ref>`. Only `reapLostLauncher` writes that text.

Evidence:
- **Shell behaviour.** In each run the shell ran `/bin/ps -o comm= -p $$`. `/bin/sh` (bash 3.2), `/bin/bash` and `/bin/dash`, each given a single command, printed `/bin/ps`: the shell had replaced itself. With `; exit $?` added, each printed its own name (`/bin/sh`, `/bin/bash`, `/bin/dash`), so the shell stayed. I did not run the Linux VM. Dash only execs the last command of a `-c` string, and here the last command is `exit`, so the helper is never exec'd. If some shell did exec it anyway, the pid check below fails loudly, so it cannot pass quietly.
- **Unmutated copy, run twice.** `TestLauncherDeathKillsTheTest` and `TestCustodianWatchesTheExportedLauncher` both passed twice.
- **Mutant: shell execs the helper** (`; exit $?` removed). Failed 2 of 2 within about a second, on the pid check: `fixture_custodian_witness_test.go:59: probe process 97169: disallowed=97169 state=alive probe=<nil> encode=<nil>`. That is behaviour, not a compile error or a hang cap.
- **Mutant: shell execs the helper and the pid check is off** (disallowed set to 0). Still failed, this time on the log check after its 30 s bound: `fixture custodian log did not publish "dead-launcher=pid=80655;micro=1789577192884710"`. The log check guards against a shell that execs even without the pid check, on bash or dash.
- **Mutant: the custodian logs the owner's ref as the dead launcher** (`EncodeRef(member)` changed to `EncodeRef(owner)` in `reapLostLauncher`). Both witnesses got past `waitWitnessDead`, which means the custodian did kill the owner, and then failed only on the exact ref: `did not publish "dead-launcher=pid=97813;micro=..."` and `"dead-launcher=pid=34505;micro=..."`. The check reads the log this custodian writes, and it compares the exact ref, not just the key.
- **The seat's mutations** (`g18/check-4a-r2.out`, script `check-4a-r2.sh`). All three mutated trees compiled and ran; each shows `--- FAIL` lines, and none shows a build failure or a `-test.timeout` panic.
  - "dead-member-alive" failed witness 4 and witness 16, 3 of 3 each, with `process N stayed alive`. That is the witness's own `waitWitnessDead` assertion on the owner, the first ref it waits for, at `wiringBound`.
  - "unset-owner-no-chain" failed witness 4 3 of 3 with the same assertion.
  - "shell-execs-helper" failed 3 of 3 in 0.375 s. The seat's grep filter does not match the pid-check message, so the seat's output does not name the assertion. The timing fits only the pid check, and my rerun above names it.

## 2. Items 2 to 5 of the fix brief

- **Item 2 (cleanup order).** `child := waitWitnessRef(pid0)` is followed at once by its `defer SignalExact`, and `other := waitWitnessRef(pid1)` by its own. That matches `custodianWitness`. The owner and, in the exported case, the shell S are in the command's process group, and the `t.Cleanup` group kill is registered before any wait.
- **Item 3.** `waitWitnessDead` uses `wiringBound` again (the deadline line is gone from the diff). Its failure text is `process %d stayed alive`.
- **Item 4.** The three doc comments are accurate:
  - `RunCustodian`: "watches the owner and its launcher chain, kills the owner after launcher loss, and reaps its attributed children";
  - `ExportRunOwner`: "preserves an existing run owner or appends the caller's exact identity";
  - `ResolveRunOwner`: "returns the exact ancestor chain that the custodian must watch".
- **Item 5.** `append(append([]string(nil), env...), ...)`. The table test now passes `backing[:0]` with capacity 1 and requires `backing[0] == ""`. The old `append(env, ...)` would have written into `backing[0]`, so the test catches the old form. The preserve path returns `env` without writing to it.
- **Witness 16.** `launcherValue` now comes from probing L's pid (`command.Process.Pid`, with the owner's pid disallowed) while L is alive, instead of from a file L wrote. The claim is unchanged: the log names L's exact ref, S still probes alive, and H and both children are dead. A broken export cannot pass either:
  - with no export, the heuristic chain is [S], and S survives, so H is never killed;
  - with a wrong export, `ResolveRunOwner` fails and the owner never starts.

  The ref now comes from the kernel rather than from the helper's own claim, so the check is independent of the code under test.

## 3. Nothing else changed

`diff fcu4a-r5.diff fcu4a-r2.diff` holds items 1 to 5 plus one related refactor: `launcherProcessRef` lost its `keep` parameter, and the shared probe moved into `liveWitnessProcessRef` (see note 1).

The boundary holds. The diff touches only the four unit files, and it has no match for `LaunchSuite`, `testutil`, `proc ref`, allow-list, digest or `inheritedControl`. The custodian start stays gated. The seat's run shows a malformed run owner failing with the gate set (`start fixture custodian: run owner "malformed": ...`, FAIL) and `ok` with it unset.

## 4. Survivors

The final census ran after all my runs, including two direct helper runs for note 1. It used `ps -axwwE` and searched for `METASYSTEM_FIXTURE_OWNER`, `FIXTURE_LAUNCHER_WITNESS_MODE`, `METASYSTEM_FIXTURE_CUSTODIAN`, `FIXTURE_CUSTODIAN_WITNESS`, `detached.sh` and my binary names (`orig.test`, `execs.test`, `wronglog.test`, `execsnopid.test`). It found nothing. My runs left no tagged process, live or with a dead owner, and I killed nothing.

## 5. Comments

The diff adds only the three doc comments above. They are plain English, and none mentions units, rounds, reads, findings or reviews.

## Non-material notes

1. The `launcherProcessRef` refactor was not one of the five items. Its dead-ref negative now uses `fixtureExact(1<<30, 1)` instead of probing its own process and replacing the pid. I ran the helper by hand with that value (`pid=1073741824;micro=100000000`) and with a probe-shaped value (`pid=1073741824;micro=<now>`). Both stopped at the same point, `identity: required run owner is not alive at its exact identity`, with an empty sink. The negative still reaches the liveness proof rather than the parser, so its proof is unchanged. The refactor serves item 1's shell probe.
2. `TestControlledLaunchersExportTheirOwnRef` no longer calls `ExportRunOwner(nil)`. `backing[:0]` is still an empty environment, so it still covers witness 15's producer case "on an empty environment yields one variable".
3. Design section 6 says "within two seconds" for witnesses 4 and 16. The witnesses bound the wait with `wiringBound` (30 s), as the fix brief's item 3 requires, so the two-second figure is not asserted. In my runs, each witness 4 pass took well under a second.
4. The seat's mutation grep (`survived|stayed alive|equals|owner pid|dead-launcher`) does not match the pid-check message (`probe process N: disallowed=N`). A future mutation that fails on that check will show no reason in the seat's output.
5. Each killed owner still leaves a `metasystem-test-registry-*.custodian.log` behind, as the first read noted (unit 3b behaviour). My runs added 9 of them under the temp directories.

Material findings: 0.

VERDICT: LAND
