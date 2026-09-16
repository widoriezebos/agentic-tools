# Build read: fixture-children unit 3, round 2 (3a and 3b)

Reader: Opus 5, build-read. I did not write this change. Worktree `g18/wt-fcu3`, base 9cafaae5f.
Scratch copies and outputs: `S=/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/r2`.
The worktree did not change during the read: `shasum -c $S/snapshot-start.sha` (taken 14:15:46) matched every file at 14:4x. I created nothing inside the worktree.

## Verdicts

- **3a: NOT LAND.** `proc custodian` still fails three of its own guarantees when run for real: the fd 3 guard passes on the Go runtime's own kqueue (U3-4 not closed), it keeps the caller's `METASYSTEM_FIXTURE_OWNER`, and it holds caller descriptors above 3.
- **3b: NOT LAND.** The required run `go test -race -count=1 -timeout 40m ./internal/identity/...` fails: `TestCustodianReapsStoppedAndDetached` failed 11 of 11 runs, because the kernel's orphaned-group HUP kills the stopped child before the custodian can.

## Findings

| Id | Severity | Material | Claim | Evidence |
| --- | --- | --- | --- | --- |
| F-1 | high | yes | 3b witness `TestCustodianReapsStoppedAndDetached` fails every run: the kernel kills the stopped child through orphaned-group HUP before the custodian scans | Required identity race run exit 1, and 10 more runs all failed with `omits pid`. With no custodian the stopped child is dead at t=0. With Setsid on that child, 3 of 3 passed |
| F-2 | high | yes | U3-4 closed in wording only: the `FcntlInt(3)` guard passes on the Go runtime's kqueue when inherited stdout and stderr are non-blocking | `3<&-` from the agent shell: custodian alive in 6 of 6 and 4 of 4 trials, lsof `3u KQUEUE`. Deferred `watch.Close()` then crashes with `netpoll failed` |
| F-3 | medium | yes | `proc custodian` keeps the caller's `METASYSTEM_FIXTURE_OWNER`, so other key-scoped reapers kill it as a certain survivor | Live probe: `tagged=true carrier=environment`. Dead-owner scan names the custodian `class=fixture-survivor custodian=true` |
| F-4 | medium | yes | `proc custodian` keeps caller descriptors above 3 open | Live check: caller's fd 4 pipe never reaches EOF, lsof `4 PIPE` on custodian 19495 |
| F-5 | medium | yes | The testenv custodian log sits in the registry home, which the owner's cleanup and the dead-home sweep delete | START=1 atomicfile run: registry dir `No such file or directory`, custodian 20430 still writing fd 2 to that path |
| F-6 | low | yes | Nothing in the tree fails when the U3-5, U3-6 or D4 fixes are reverted | Mutations M4, M5 and M6 each fail only witness 5, which already fails (F-1) |
| F-7 | low | yes | `memory/receipts.log` is in the worktree diff, outside both file sets | `git status --short`: ` M memory/receipts.log` |

## Sizes (`git diff --stat -M HEAD --` per file set)

- 3a (6 files): `6 files changed, 277 insertions(+), 18 deletions(-)` = 295 changed lines. identity.go +59, main.go +1, fixture_custodian.go +138, fixture_custodian_test.go +51, fixture_survivors.go +25/-16, fixture_survivors_test.go +3/-2.
- 3b (3 files): `3 files changed, 250 insertions(+), 1 deletion(-)` = 251 changed lines. testenv.go +83, testmain_test.go +1/-1, fixture_custodian_witness_test.go +166.
- Outside both sets: `metasystem/memory/receipts.log` (F-7).

## Material findings, most material first

### F-1 (3b, high) Witness 5 fails every time: the stopped child is killed by the kernel, not by the custodian
- Where: `internal/identity/fixture_custodian_witness_test.go:74` (the second helper child is started in the helper's own process group), with the check at `:54-58`.
- Evidence:
  - Required run on a full copy (`$S/full`), 14:20:18, `go test -race -count=1 -timeout 40m ./internal/identity/... -v`, exit 1. `TestCustodianWaitsForDeadOwnerAndExcludesItself` PASS, `TestKilledTestBinaryLeavesNoFixtureChild` PASS (1.80s), `--- FAIL: TestCustodianReapsStoppedAndDetached (1.81s)`: `fixture_custodian_witness_test.go:57: custodian log "...kill pid=98703 carrier=argv-word...owner=pid=98701;... action=complete\n" omits pid 98704`. Full output: `$S/full-identity-race.out`.
  - The same test run 10 more times with -race: failed 10 of 10, and each log names only the detached pid (`$S/hard-witness-x10.out`).
  - Cause, shown on a copy with START removed so no custodian runs, polling `AliveRef` for both refs: the detached child stayed alive 3s, the stopped child was dead at t=0. The helper's group gets SIGTERM, the helper dies, the group is now orphaned with a stopped member, so the kernel sends it SIGHUP then SIGCONT, and the pending TERM or the HUP kills the child. With `commands[1].SysProcAttr = &syscall.SysProcAttr{Setsid: true}` and no custodian, both children stayed alive. With that one line and the custodian, the witness passed 3 of 3 (1.86 to 1.91s).
- Failure: 3b's own proof is red on the first real-process run anyone has made (the builder's sandbox could not run it). As written, the witness can never show that a custodian reaps a stopped child on Darwin, because that child is already dead before the scan.
- Smallest fix: start the stopped child outside the helper's process group so orphaning does not reach it. `Setsid` on `commands[1]` is one line. Leave the assertion alone. Do not use `trap '' HUP`: SIGCONT would then resume the child and it would no longer be stopped.

### F-2 (3a, high; also affects the 3b branch) U3-4 is closed in wording only: the fd 3 guard checks the Go runtime's own kqueue
- Where: `cmd/metasystem/identity.go:31` (`unix.FcntlInt(3, unix.F_GETFD, 0)`) and `:64-65` (`os.NewFile(3, ...)`, `defer watch.Close()`). Same pattern in `internal/testenv/testenv.go:98-103`.
- Evidence (binaries built in my copies, run from the agent's Bash tool; full record in `$S/live-fd3-trials.txt`):
  - `metasystem proc custodian --owner <live ref> --log X 3<&-`: the custodian was still alive after 1.5s in 6 of 6 trials. `lsof -d 3` showed `3u KQUEUE` every time. My first attempt ran in the foreground for more than 2 minutes, until I sent SIGKILL.
  - `METASYSTEM_FIXTURE_CUSTODIAN=1 METASYSTEM_FIXTURE_CUSTODIAN_OWNER=<ref> identity.test 3<&-`: alive in 4 of 4 trials, fd 3 a KQUEUE.
  - Mechanism: in this shell, stdout and stderr are regular files opened with O_NONBLOCK (`fcntl F_GETFL`: fd 1 and fd 2 O_NONBLOCK). At startup Go registers a non-blocking stdio descriptor with its netpoller, so the kqueue is created before `runFixtureCustodian` runs and takes the lowest free descriptor, 3. With all three streams blocking (`3<&- </dev/null >/dev/null 2>/dev/null`, or a Go harness using /dev/null and a pipe) the guard fires: exit 2, `invalid watch descriptor 3: bad file descriptor`.
  - After the miss: the custodian "watches" its own poller. It still reaped a tagged child by polling after the owner was killed. On return, though, the deferred `watch.Close()` closes the runtime's kqueue: `runtime: kevent on fd 3 failed with 9`, `fatal error: runtime: netpoll failed`, exit 2, and a full goroutine dump in the custodian log (trial `stdinonly2`).
- Failure: the fail-fast the brief asked for does not happen when the caller is an agent runtime with non-blocking output streams, which is how beds get run. A bed that forgets fd 3 gets no error. Its custodian loses the pipe notification (the only way out of an Unknown owner probe) and ends in a runtime crash that fills its log. The round-1 finding's own smallest fix was not enough, and the builder applied it as written.
- Smallest fix: check the type as well as the number. `unix.Fstat(3, &st)` must succeed and `st.Mode&unix.S_IFMT == unix.S_IFIFO`, otherwise exit 2. Do this in both entry points. A kqueue fails that test.

### F-3 (3a, medium-high) `proc custodian` keeps the caller's owner tag, and other scans then name it as a fixture survivor
- Where: `cmd/metasystem/identity.go:15-71`. Nothing checks or removes `identity.FixtureOwnerEnv`. Only the testenv start path (`testenv.go:162-166`) filters it out.
- Evidence: live, `$S/live-envtag.out`. I started the custodian with `METASYSTEM_FIXTURE_OWNER=<key for its owner>` in its environment, which is what a bed inherits when a Go fixture launches it (the Go carrier is the environment). `FixtureTag` on the custodian gives `tagged=true carrier=environment`. After the owner was killed, `FixtureSurvivorsOfDeadOwner` returned `pid=40990 class=fixture-survivor carrier=environment custodian=true`. The custodian skipped itself (self-exclusion works) and reaped the real child.
- Failure: this breaks the brief's fixed rule "carries no owner tag" and design 3.3 ("Its environment carries no METASYSTEM_FIXTURE_OWNER"). Self-exclusion only protects the custodian from its own scan. Any other reaper keyed on that owner will SIGKILL it as a certain survivor: the Go test binary's own custodian from 3b, or unit 5's key-scoped `t.Cleanup`. Concretely: a Go fixture starts a bed shell, the bed starts `proc custodian`, and the test binary dies. The binary's custodian kills the bed's custodian, because it carries the binary's key in its environment. The bed's own children carry the shell's key as an argv word, and `FixtureTag` checks argv first, so the binary's custodian does not match them. The only process that would have reaped them is gone. `os.Unsetenv` cannot fix this, because kern.procargs2 and /proc/pid/environ show the environment from exec time.
- Smallest fix: exit 2 naming the variable when `FixtureOwnerEnv` is set. Alternatively, re-exec with a cleaned environment (see F-4; one re-exec fixes both).

### F-4 (3a, medium) `proc custodian` keeps the caller's descriptors above 3
- Where: `cmd/metasystem/identity.go:42-62`. Fds 0, 1 and 2 are redirected. Nothing above 3 is closed.
- Evidence: live, `$S/live-domains2.out`. The caller passed a pipe's write end as fd 4. `lsof` on the custodian showed `4 PIPE`, and the caller's reader got no EOF within 2s while the owner was alive. Fds 0 and 1 were /dev/null and fd 2 was the log, as intended. Descriptors like this are normal in the repo's fixture shells, for example `scripts/agents/acp-fixtures.sh:40` and `:88`: `exec 9>"$dir/server-out" 8<"$dir/server-in"` (FIFO ends that a bed shell passes to every command it runs).
- Failure: design 3.3 says "holding nothing else open", U3-3's fix brief says "holds nothing of the caller's", and my guarantee check 3 includes it. A pipe or FIFO reader waiting for EOF waits until the custodian exits, which only happens after the owner dies. If the owner is that reader, it deadlocks. This is U3-3's stdout hazard again, on fds above 3.
- Smallest fix: after the dup2 calls, set FD_CLOEXEC on every descriptor above 3 (the runtime's kqueue already has it) and `syscall.Exec` the binary again with the same argv and an environment without `METASYSTEM_FIXTURE_OWNER`. The pid, session and fds 0 to 3 stay the same. That closes F-3 and F-4 together. The narrower alternative is to close each inherited fd above 3 when the function starts, skipping the runtime poller.

### F-5 (3b, medium) The custodian log lives in the registry home that testenv deletes
- Where: `internal/testenv/testenv.go:155` (`filepath.Join(registry, "fixture-custodian.log")`), deleted by `:394` (`os.RemoveAll(registry.path)` at owner cleanup) and `:398-417` (`removeDeadRegistryHomes("/tmp")`, called at `:237` when any test binary prepares).
- Evidence: an atomicfile test binary with START=1 exited normally. Right after it exited, `ls /tmp/metasystem-test-registry-3591368434` returned `No such file or directory`. Custodian 20430 was still running (`Ss`) and `lsof` showed its fd 2 still open on `/private/tmp/metasystem-test-registry-3591368434/fixture-custodian.log`, a path that no longer existed. The earlier START=1 run went the same way: its registry `-3179650409` was gone. The dead-home sweep removes a killed binary's home as soon as its flock is free, and the custodian does not hold that flock. The witnesses pass only because their helper binary shares the outer test's registry home (`fixture_custodian_witness_test.go:75-79`), a setup that top-level binaries never have.
- Failure: when unit 5 turns the gate on, everything the custodian writes after its owner dies is unlinked in exactly the cases where it matters: its kill lines, `cleanup exceeded`, `could not prove owner dead`, and a runtime crash dump. Nobody learns that the safety net failed. The witness proves the log in a setup production does not have. U3-8's brief pointed the log at the registry home, so the seat may choose to adjudicate this as a unit-5 obligation.
- Smallest fix: write the log somewhere the testenv sweeps do not delete, for example a sibling of the home such as `/tmp/metasystem-fixture-custodian-<owner pid>-<micro>.log`, and derive the witness path the same way.

### F-6 (3a+3b, low) Nothing in the tree fails if the U3-5, U3-6 or D4 fixes are reverted
- Where: `internal/identity/fixture_custodian_test.go` (the only table witness), against `fixture_custodian.go:56-89` and `:92-95`.
- Evidence: mutation runs on my copy (`$S/mutations-b.out`). Whole identity package: M4, EOF reaps at once, which makes the first EOF terminal again (U3-5 reverted): only the witness-5 failure from F-1. M5, hard halt disabled (D4): only witness-5. M6, Unknown after EOF reaps at once, merging the two budgets (U3-6): only witness-5. M7 (reap bound cut to a tenth, table witnesses only): passed. I checked the behaviour myself (see "What passed"), but the tree would not catch a regression.
- Failure: the round-1 findings U3-5 and U3-6 are closed in behaviour today, but no test proves it. By the skill's test that is a proof gap, and a later edit can undo either fix with every test green.
- Smallest fix: two rows in the in-package table. First, EOF on the watch with an owner that probes Alive, then Dead after N polls: expect the reap, not an exit. Second, an injected `halt` that records its argument, with a prober whose scan blocks: expect `halt(2)` within bound+margin. Both fit the existing `custodianRuntime` injection.

### F-7 (neither set, low) `memory/receipts.log` is in the worktree diff
- Where: `metasystem/memory/receipts.log`, one appended line dated 11:09:40Z (from before this round).
- Evidence: `git status --short` in the worktree shows ` M metasystem/memory/receipts.log` next to the nine files in the two sets. Its hash did not change during my read, so the builder did not write it this round, as the fix brief required.
- Failure: the read brief says any file outside the two sets is a finding. If the diff were landed whole, it would write an append-only ledger the delegate does not own. It blocks neither verdict if the seat cuts the diff by path.
- Smallest fix: leave it out of both landings (the seat's own instruction). No builder action.

Material count: 7 (3a: F-2, F-3, F-4; 3b: F-1, F-5; both: F-6; neither set: F-7).

## Round-1 findings, closed or not

- U3-1 closed: 295 and 251 changed lines, both under 300. `grep "diagnostic path"` finds nothing. The tree stayed still during my read.
- U3-2 not closed for 3b: the pid publication race is fixed and helper stderr is captured (`startWitnessCommand`, and `waitWitnessRef` prints the stderr files). Witness 3 passes, but witness 5 fails 11 of 11 (F-1).
- U3-3 partly closed: live check. After startup the custodian has pgid = sid = its own pid (19495). Fds 0 and 1 are /dev/null, fd 2 is the log. The caller's stdout pipe reached EOF. It was still alive after TERM and HUP. It exits 2 loudly when it is a group leader. Still open: caller fds above 3 (F-4) and the inherited owner tag (F-3).
- U3-4 not closed: the new guard is present, but it passes on the runtime kqueue when inherited streams are non-blocking (F-2).
- U3-5 closed in behaviour, no witness: live check (`$S/live-eof.out`). The owner never held the writer, so EOF came immediately. The custodian was still alive 7s later while the owner lived, and after the owner was killed it reaped the child in 270ms and logged `complete`. Nothing fails if this is reverted (F-6).
- U3-6 closed in behaviour on Darwin, no witness: live check (`$S/live-halt.out`) with a one-hour sleep inserted before the scan. The custodian exited with status 2 after 6.114s, which is bound plus margin. `proveOwnerDead` (`:78-89`) has its own 5s budget, separate from `reapDeadOwner`'s deadline (`:96`). The D4 halt is only on `RunCustodian` (`:44`), and the table witness leaves it nil. The Linux D-state case was not checked. Nothing fails if this is reverted (F-6).
- U3-7 closed: gate checked on the machine (below).
- U3-8 closed as briefed: no `Setenv`. The log path is set only in `command.Env` (`testenv.go:169-170`), and the filter uses `identity.FixtureOwnerEnv`. The log's location has its own problem (F-5).
- U3-9 closed: `FixtureTag` returns the slot by index. Live logs show `carrier=argv-word` for a shell child, and the scan shows `carrier=environment` for an environment tag. Carrier mutations M3a and M3b are caught (below). Two notes are listed under non-material.

## Check results

1. Round-1 closure: see above.
2. 3a stands alone. I used my own copy, `$S/a3` (full tree with testenv.go and testmain_test.go taken from HEAD and the witness file deleted), not wt-3acheck. gofmt clean, `go build ./...` rc=0, `go vet` over identity, testenv and cmd rc=0. `go test -race -count=1`: identity `ok 1.540s`, testenv `ok 7.825s`. For cmd/metasystem my copy failed only `TestDispatchBriefBoundsAdmission` (`brief admission cannot resolve installation prefix: exit status 128`), and it fails the same way on an unchanged HEAD copy with no `.git` (`$S/base`), so the copy caused it, not the change. For that package I also cite the seat's `g18/check-3a-alone.out`, which ran in a git worktree: `ok cmd/metasystem 336.996s`, identity `ok 1.577s`.
3. Guarantees:
   - Reap only through the dead-owner scan and only the certain class: yes (`:104`, `:108`). M1 caught.
   - Excludes its own exact ref: yes in its own scan (`:108`, `sameExactRef`). M2 caught. But other scans see it when a tag was inherited (F-3).
   - Outside kill domains: session, streams and TERM/HUP hold. Caller fds above 3 are kept (F-4).
   - Cannot outlive its run: with the owner dead, the reap is bounded by the halt at about 6.1s even when the scan blocks (checked live). With the owner Unknown after EOF, it gives up after 5s. With the owner Alive, it lives as long as the owner does, which is the design (the run-owner chain belongs to unit 4).
4. Mutations, run by me on `$S/mut`, which I then restored:
   - M1 (the reap scan swapped for the scan without liveness): `live owner allowed reap: signaled=[702]`, FAIL.
   - M2 (self-exclusion removed): `signaled=[701 702]`, FAIL.
   - Carrier mutations:
     - M3a (slots swapped in `FixtureTag`): both the custodian table and `TestCleanupIsScopedToTheFixtureKey` FAIL.
     - M3b (scan hard-codes argv-word, with `_ = carrier` so it compiles): both FAIL.
     - M3c (the custodian prints the literal "environment"): PASS, not caught (non-material note 1).
   - Every failure was in behaviour, none in the compile.
5. Gate D2: probe test in `internal/atomicfile` (a package with no fixtures), run in copies only.
   - HEAD: no child process, registry `[".owner"]`, FIXTURE env `[]`.
   - Full tree with START unset: identical, and the whole atomicfile package passes.
   - Full tree with START=1: child `atomicfile.test` in state Ss, registry `[".owner" "fixture-custodian.log"]`.
   - Nothing in the tree sets START except the witness.
6. Boundary: on the nine-file diff, `grep` for ExportRunOwner, METASYSTEM_RUN_OWNER, RunOwner, testutil, fixture-survivors, `Record(`, leash, ShellPrologue, reapFixtureSurvivors, census, chain, health and launcher found nothing (rc=1). `FixtureCustodian`, `RunCustodian` and `FixtureCarrier` appear only in files inside the two sets. Identity has no `init()`. Clean.
7. Survivors: the census (tagged argv, custodians, identity.test, detached.sh) was empty at 14:19:57, my baseline before any run. Two later checks each showed one reparented custodian (ppid 1) still inside its quiet window: `identity.test` 5668 at 14:21:26 and `atomicfile.test` 18915 at 14:24:53. Both were gone at the next check (`ps -p 18915` at 14:25:13 found nothing). Final censuses at 14:49:49 and 14:50:01, after all my runs had ended, were empty. One exception, made by my own harness rather than the unit: at 14:37 my harness panicked on its own probe and left `sleep 300` (a live owner) and its tagged child. The owner was alive, so this was not a dead-owner survivor. I killed both at 14:40 and the census was clean afterwards. The code under review left no tagged process with a dead owner.

## Non-material notes

1. The table witness accepts a literal carrier string (M3c passes): only the environment child is ever killed, because 701, the argv child, is the custodian itself. An extra argv-word child 703 would close that.
2. Ownership-record survivors (`fixture_survivors.go:127-129`) have no carrier, so the kill line reads `carrier=`. Something like `ownership-record` would be clearer.
3. The hard witness checks each pid with a bare `strings.Contains` over a log shared by every run in the outer binary (`witness_test.go:56`). The pid can match inside another line's `micro=` value. Match `"kill pid=<n> "` instead.
4. The witness waits use `wiringBound` (30s), while design witness 5 says "gone within two seconds", so no timing claim is checked.
5. The hard halt exits without writing a log line, so a halted reap leaves no record.
6. For a cold binary, the custodian is still in the caller's group and session for a moment before `Setsid` runs. My first live probe at +300ms saw pgid and sid equal to the caller's; at +2s they were its own. A group kill inside that window takes it, and a bed's post-start probe cannot tell.
7. `proc custodian` started as a process-group leader (job control) exits 2: `create session: operation not permitted`. That is loud, but beds must not use `set -m`.
8. `testenv.Main` runs the custodian branch whenever `METASYSTEM_FIXTURE_CUSTODIAN=1` is set, whatever START says. If that variable leaked, any test binary would turn into a custodian.
9. testenv's post-start probe only proves the fork happened. The child is never waited on, so a custodian that exits at once becomes a zombie that can still probe Alive.
10. `AliveRef` in `runCustodian` and `proveOwnerDead` runs outside the halt. A blocking owner probe (Linux /proc read) is unbounded. The halt covers only the reap.

## What passed

- Full-tree identity race run: everything except witness 5 (F-1).
- Full-tree `go test -race ./internal/testenv/...`: `ok 9.052s`.
- Full-tree cmd/metasystem race run, skipping only the git-dependent `TestDispatchBriefBoundsAdmission`: two runs, `ok 517.280s` (14:32:34 to 14:41:30; the zsh shell does not set `PIPESTATUS`, so its rc line is empty, but the go test output shows no FAIL) and `ok 503.198s` (rc=0, finished 14:49:52).
- 3a alone: build, vet, identity and testenv green on my copy; cmd green on the seat's git worktree.
- The four mutations required by the brief (M1, M2, M3a, M3b) each fail on behaviour.
- Gate D2 behaves exactly like HEAD with START unset.
- Live `proc custodian` checks:
  - Own session after startup, streams redirected, TERM and HUP ignored.
  - Missing `--log` exits 2 (`--log or METASYSTEM_FIXTURE_CUSTODIAN_LOG is required`).
  - A bad `--owner` exits 2.
  - Reaps a Setsid, TERM/HUP-trapped tagged child 100 to 270ms after the owner's KILL and exits 0 about 1.8 to 1.9s later.
  - Early EOF with a live owner keeps it polling.
  - A blocked scan is halted at 6.1s.
- The boundary is clean.
- Sizes are under both caps.

## What I could not check

- Linux: blocked `/proc/<pid>/cmdline` and `environ` reads under the halt (D-state), the orphaned-group behaviour of witness 5 there, and the environment carrier through `/proc/<pid>/environ`.
- `metasystem test run`, the battery and the testrun lock (seat only).
- The other test binaries with START unset. I measured one no-fixture package (atomicfile) plus identity, testenv and cmd.
- Whether the one-line Setsid fix for F-1 still models the design's "job cancel kills the helper's whole group" well enough. It passed 3 of 3 in my copy, but that is my experiment, not the builder's code.
