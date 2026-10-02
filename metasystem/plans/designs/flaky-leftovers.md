# Flaky leftovers: inherited scratch locks, fixture exit bounds, swapped seams

- Kind: design
- Id: 01M3Y1P7GDX118WC3BQ9K67SE8
- Status: draft
- Goals: flaky-leftovers

Three things the no-flaky-tests program left on its "later list" (evidence log `flaky-20261002/log.md` 08:53 and 10:32; discovery clusters C and E). Facts below were read at `ada2497d0` unless marked *(recited)*.

## Threat model and rabbit-hole risks (read before critiquing)

Threat model: the template default. Our own processes crash, hang or make mistakes; nobody attacks. A finding about tampering with lock files, forged environment variables or hostile children is out of scope.

The ways this goal could end in a rabbit hole, each with what keeps it out:

1. **Chasing the `StartChild` fork window to "provably zero".** Fork-locks, pipes or a lock-passing protocol would add a mechanism to close a window that already fails closed (the root is kept and the sweeper removes it). Kept out: the window is accepted as stated in part 1. A finding on it is material only if it shows a root removed under a live holder, or a test that asserts removal failing.
2. **Replacing pacing tickers with kernel exit events (pidfd, kqueue).** They fix no flake. Kept out: Deferred.
3. **Seam conversion growing with every new marker.** Kept out: step 1 converts the ten seams named in part 3. A seam that an extended audit finds later goes on the Deferred list unless a parallel test swaps it.
4. **Redesigning fixture custody as a whole.** Kept out: part 2 only deletes bounds; the custodian's own deadlines stay.
5. **Folding `proofrun.scratchDrain` in "while we're there".** Kept out: Deferred.

## 1. WriterDrain: the owner's lock is copied by unrelated forks

**Facts.**
- The process scratch root has one writer lock description `s.writer`, taken `LOCK_EX` (`internal/diskstore/process.go:465-470`). `prepareLocked` hands that same description to every prepared child through `ExtraFiles` (`:263`), with the ancestors' locks this process inherited (`s.inherited`, `:138-151`).
- At release the owner closes its copy without `LOCK_UN` (`closeWriter`, `:400-404`). Then it re-takes the lock through a fresh description and re-probes for 2 s (`WriterDrain`, `:296-335`; used at `:683-689`).
- Two kinds of holder stand behind that re-probe:
  - **On purpose:** prepared children and their descendants, including an orphaned grandchild (`process_test.go:150-172`, the `grand-child-prep` mode). `runtime_hook_worker.go:40-42` relies on a surviving worker keeping the root.
  - **By accident:** any fork by any goroutine copies `s.writer` until that child execs. Go releases `ForkLock` before the exec (comment `:296-305`).
- `LOCK_UN` acts on the description. It frees every fork copy at once, but it would also free the real inheritors, because today they share the same description. So **`LOCK_UN` alone is not enough here**. An owner whose prepared child has exited would then remove the root under a live orphaned grandchild, which breaks fail-closed.
- There are only two production `PrepareChild` callers, and both call `Start` on the next line: `cmd/metasystem/brain_boot.go:115-120` and `internal/hooks/runtime_hook_worker.go:43-46`.
- The test tolerance at `cmd/metasystem/intent_disk_trim_test.go:150-154` is **already gone**: cluster C replaced it with a comment. Nothing is left to delete there.
- The flock audit exempts `closeWriter` (`cmd/metasystem/audit_flock_release_test.go:38`) for the reason above.

**Shape.** The owner and its children share one description, so the owner cannot free the fork copies without freeing the children.

**Fix: one description for the owner and one per child, all shared locks.**
1. The owner takes its own description `LOCK_SH`, not `LOCK_EX`. At release it runs `unlockAndClose` (LOCK_UN, then Close). That frees every fork copy of the owner's description, deterministically.
2. Replace `PrepareChild` with `StartChild(cmd) error`. Under `scratchMu` it opens a fresh description of `.writer-lock` (`O_RDONLY|O_CLOEXEC|O_NOFOLLOW`), takes `LOCK_SH|LOCK_NB`, appends it to `ExtraFiles`, sets the environment as today and records the child. Then it calls `cmd.Start()` and closes the parent's copy, whether or not Start succeeded. The child and its descendants hold that description. The owner never holds it again and never unlocks it.
3. An engine child still passes its inherited descriptions on to its own children, unchanged.
4. Release keeps today's order: check users, check running prepared children, close, then take `LOCK_EX|LOCK_NB` once through a fresh description. A shared lock held by any descendant refuses that exclusive take, so the result is `Keep` and the record stays for the sweeper, exactly as today. The sweeper's proof (`ProbeWriterLock`, the exclusive take) is unchanged.
5. Delete `WriterDrain`, `WriterDrainWindow`, `WriterDrainStep`, the `drain` parameter of `ReleaseProcessScratch` and `releaseScratchRoot`, and the clock arguments at `cmd/metasystem/main.go:263`, `cmd/devgate/main.go:62` and `internal/ui/httpd/walkthrough/main.go:65`.

**Residual, stated plainly.** Another goroutine's fork can still copy a child's description in the moment between `open` and `close` inside `StartChild`. That copy goes away at that fork's exec. For it to matter, it would have to outlive the prepared child's whole run and the owner's release that follows. If it does, the owner keeps the root and the sweeper removes it later. Nothing waits on a clock, and nothing is removed wrongly. Test helpers do not fork concurrently with `StartChild`, so tests that assert removal are deterministic.

**Tests.**
- *Red before:* `TestOwnerReleaseFreesForkCopiesOfItsLock`. Create a scratch and hold `unix.Dup(writer)` open, which is a fork copy in everything but name. Then release. Today the result is "kept for the sweeper". After the fix the root is gone. No fork and no clock are involved.
- `TestStartChildLeavesNoParentCopy`. After `StartChild` and Wait, no descriptor in this process (from `/dev/fd` plus fstat) has the lock file's device and inode, apart from the owner's own.
- The existing grandchild test (`process_test.go` `grand-child-prep`) stays green: an orphaned holder keeps the root. *Mutation check:* make release unlock the children's descriptions too, and this test must go red.
- Delete the drain tests at `process_d1_test.go:232-252` and `:440-480`.

**Audit.** In `audit_flock_release_test.go`, replace the `closeWriter` allowance with a `StartChild` entry: "closes the parent's copy of the child's own description; LOCK_UN would free the child". The existing audit then forces `LOCK_UN` at the owner's release.

## 2. The eight fixture-custody wall-clock calls

The wall-time allowance (`internal/testenv/testdata/walltime-allowance.tsv`) lists the eight as 3 + 2 + 3. Four are real exit bounds. The other four are 10 ms tickers that only pace a poll of the fact; they end on the fact and decide nothing, like `testenv.Await`.

| # | Site | What it waits for | Verdict |
|---|---|---|---|
| 1 | `testenv/process_group.go:72` `WithTimeout(15s)` | An orderly stop action returning | Bound: remove it |
| 2 | `testenv/process_group.go:75` `WithTimeout(exitBound)` | A SIGKILLed group being reaped (`kill(-pgid,0)` returns ESRCH) | Bound: remove it |
| 3 | `testenv/process_group.go:179` ticker | Pace of that probe | Pacing: keep |
| 4 | `testenv/testenv.go:348` ticker | Pace of the exit-scan probe | Pacing: keep |
| 5 | `testenv/testenv.go:350` `time.After(bound)` | A SIGKILLed survivor exiting, after `m.Run` | Real deadline: keep |
| 6 | `testutil/fixture.go:247` ticker | `WaitForNoUnrecordedChildren`, which ends on the fact or the caller's context | Pacing: keep |
| 7 | `testutil/fixture.go:352` ticker | Pace in `awaitExits` | Pacing: keep |
| 8 | `testutil/fixture.go:354` `time.After(bound)` | A SIGKILLed recorded child exiting (teardown, and `AwaitExactExit`) | Bound: remove it |

**Why the bounds are flake sources.**
- Bounds 1, 2 and 8 run inside `t.Cleanup` or a test body. Each fails the test when the event is merely slow: 15 s for a stop, and 10 × the custodian bound × scale (50 s by default, `testenv.go:89-118`) for a SIGKILLed exit.
- SIGKILL cannot be refused, so the exit always comes. The only open question is when.
- The binary's own `go test -timeout` alarm already ends a wait that never comes. It is still armed while cleanups run, and it panics with every goroutine's stack, which names the stuck wait *(recited: Go testing internals)*.
- If the binary dies there, the fixture custodian, a separate process, kills the recorded fixtures of its dead owner (`identity/fixture_custodian.go:494-606`). That keeps custody intact. *(Recited, not traced:* whether its scan covers every detached group that `ReapFixtureProcessGroups` reaps.)

**Why #5 stays.** It runs after `m.Run` returns. The `-timeout` alarm has stopped by then *(recited)*, so nothing else would end a wait for a process stuck in the kernel. And `exitScan` has already set the failing exit code (`testenv.go:313-332`), so the bound only ends a report on a run that has already failed. It is a deadline, not a flake source.

**Not in the eight.** The custodian's own bounds (`fixture_custodian.go:454, 472, 490, 597, 626`) are deadlines for an owner it can no longer observe. They run on the injected `custodianClock`, and tests drive them with a fake. They stay.

**Fix (subtraction).**
- In `fixtureProcessGroupOps`, replace `cleanupContext` and `exitContext` with one `done context.Context`. Production passes `context.Background()`. Delete `fixtureCleanupBound`.
- `awaitExits(prober, refs, bound, exited)` becomes `awaitExits(ctx, prober, refs, exited)`. Teardown and `AwaitExactExit` pass `context.Background()`. Delete `ProcessFixture.waitBound`, `awaitExactExitWithin` and the bound in the error text.
- `FixtureExitWaitBound` stays, for #5 only.

**Tests.**
- The failure-path tests that set `waitBound = time.Millisecond` (`testutil/fixture_test.go:578, 642`) and `testFixtureContext` (`protection_test.go`) pass an already-cancelled context instead, as `TestAwaitProcessTargetGoneWaitsForTheReap` already does (`fixture_exit_test.go:113-140`).
- *Red before:* `TestFixtureTeardownOutwaitsASlowExit`. The fake prober blocks its first probe on a channel and the test releases it, then the prober reports Dead. Today, with a 1 ms bound, the result is "child did not exit after 1ms". After the fix the teardown is clean. Add the same case for `ReapFixtureProcessGroups` with a fake `wait`.

**Audit.** The wall-time audit already fails while an allowance is higher than the calls it covers (`walltime_test.go:127`). Lower `process_group.go` 3→1 and `testutil/fixture.go` 3→2, and rewrite their reasons as "pacing only". Rewrite the `testenv.go` reason as "deadline after m.Run; outcome already failed".

## 3. Swapped process seams in sequential tests

**Facts.** These package variables are assigned by tests outside `TestMain`. None of their test files calls `t.Parallel`.
- `waitRegisterProber`: `cmd/metasystem/wait_register.go:20`, 3 assignments
- `handoffProber`: `internal/steward/handoff.go:16`, 5
- `armingOwnerProbe`: `internal/supervise/arming.go:386`, 4
- `takeoverComponentControl`: `arming.go:874`, 12
- `stopSignal`: `internal/run/stop.go:198`, 16
- `hostAdmissionDirectoryForTest`: `internal/proofrun/host_resources.go:32`. `TestMain` sets it (`testmain_test.go:99`), but 9 tests reassign it.
- `commandLoadOptions`: `attemptload.go:71`. It is set in production `init` (`:154`) and reassigned by 6 tests.

`TestAuditNoTestAssignsAProcessSeam` (`cmd/metasystem/host_process_scan_audit_test.go:406`) misses the first five because its markers (`:261`) name only process tables, on purpose (comment `:258-259`). It misses the last two because they are not process readers.

**Fix.** Give each seam a per-call carrier.
- The probers and signallers become fields on the struct the call already has, or parameters: `metarun.Store.Prober` already exists for wait-register, `recordedComponentControl` is already a struct, and `stopMechanism.signal` already exists.
- Tests that need a private admission directory set the request's existing `testHostAdmissionDirectory` field. Request-less callers (`host_lease_inspect.go:46, 68`) take a directory parameter.
- Load options go through the existing variadic `options ...loadSampleOption` on `sampleLoad` and `sampleNestedProofLauncher`. `resourceLegacyLauncherCount` takes them as a parameter.
- `TestMain` and `init` keep the binary defaults.

**Audit.** Add identity `Prober` and `KernelProber` and kernel `Kill` and `Signal` to the markers, and delete the "one-pid seam is not a table" exemption. A scratch AST count with those markers found three more seams on today's tree: `lease.sweepKill` (8), `mission.probeGroupGone` (2) and `steward.runnerSignal` (4). Convert them in the same slice.
For the two configuration variables, add a two-name list to the same audit: "assigned only in TestMain or init".
*Red before:* the extended audit names all ten seams on base.

## Step 1

All three parts land as one slice each, in this order: 3 (mechanical), then 2, then 1. Each is small and stands alone.

## Deferred

- **`proofrun.scratchDrain`** (`internal/proofrun/scratch.go:582-600`). Same shape, but a borrowed run re-takes `LOCK_EX` on the inherited descriptor (`:520`). Later home: the same per-child-description pattern in that file.
- **Kernel exit events** (kqueue `NOTE_EXIT`, pidfd) in place of the 10 ms pacing in the custody polls. Later home: `waitForFixtureProcessTarget` and `awaitExits`. They do not fix a flake, and a group has no exit event.
- **The audit walking `artifacts/` copies of the tree.** The scratch count saw stale worktree copies there. Later home: the audit's `WalkDir` skip list.

## Risks

- **The writer lock changes from exclusive to shared.** If any reader expects the owner to hold `LOCK_EX`, it would misread the lock. All three probes (`ProbeWriterLock`, the release take, the sweeper) take an exclusive lock through a fresh description, so a shared holder still reads "held". The existing `process_test.go` and `process_d1_test.go` suites cover the sweeper and nested children.
- **A future `ExtraFiles` launcher that bypasses `StartChild`** would start a child with no lock. The removed export makes that a compile error.
- **Unbounded teardown waits** turn a stuck stop into a `-timeout` panic instead of a named error. The panic's stack still names the wait. The red-before tests prove that a slow event no longer fails the test.
- **Seam conversion touches about 60 test sites.** The packages' own tests and their reverse dependents are the gate.
