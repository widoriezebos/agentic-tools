# Read: fixture-children unit 4a (the custodian watches its launcher)

Tree read: `g18/wt-4a-r5`. HEAD e1cc059ca, index tree aac0e4ad52e0a62760f5a26d71e2c6074c490167 (3b staged), 4a unstaged on top. `git diff --stat` against the index: 4 files, 291 insertions, 9 deletions. That is 300 lines, exactly at the ceiling. The diff matches `fcu4a-r5.diff`.

Experiments ran on my own copy: `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/fcu4a/metasystem` (go1.27.1 darwin/arm64). I changed nothing in the worktree. I did not take the testrun lock.

Materiality test used: would the change ship a defect, violate its brief, or damage what certifies it?

## Material findings

### U4a-1: witness 4 kills the test binary, not a launcher, so the heuristic launcher-loss path is uncertified

`TestLauncherDeathKillsTheTest` starts `/bin/sh -c '"$1" -test.run="^$2$" -test.count=1' sh <binary> <name>`. On this machine `/bin/sh` is bash (`/private/var/select/sh -> /bin/bash`). Bash replaces itself with the command when a `-c` string holds a single simple command, so no `sh` sits between the test and the helper. `command.Process.Kill()` then sends SIGKILL to the owner itself. The pipe closes, and the landed dead-owner reap removes the children. The witness proves witness 3 again. It never proves that a launcher's death kills the test.

Evidence:
- Direct check. From shell 96082, `/bin/sh -c '"$1" -c "echo child=\$\$ parent=\$PPID"' sh /bin/sh` printed `parent=96082`, so no intermediate shell existed.
- Instrumented copy, with only a `t.Logf` added. Witness 4 logged `launcher-pid=30656 owner-pid=30656 same=true`. Witness 16 logged `launcher-pid=31336 owner-pid=31381 same=false`. This explains the 0.09 s pass the seat saw (0.17 to 0.42 s here). The real launcher-loss path needs at least one 250 ms poll, a kill, and a dead probe.
- Mutation 1 (a dead chain member is treated as alive): witness 4 still passes (`--- PASS: TestLauncherDeathKillsTheTest (0.38s)`, `same=true`). Only witness 16 fails.
- Mutation 3 (unreadable argv is skipped): witness 4 still passes.
- Mutation 4, which I added: in `startFixtureCustodian`, set `chain = nil` whenever `METASYSTEM_RUN_OWNER` is unset. Then `go test -count=1 ./internal/identity/ ./internal/testenv/` reports `ok` for both packages. A heuristic chain that never reaches the custodian ships green. `TestRunOwnerResolution` covers only the in-memory walk.
- The production code is correct, so this is a proof defect. In the copy I changed the witness shell to `...; exit $?`, which stops bash from replacing itself, and asserted that the killed pid is not the owner's pid. Witness 4 then passed 3 of 3 (0.57 to 0.63 s, `same=false`). With mutation 1 added it failed 3 of 3 (`fixture child 93864 survived owner death`).
- Specification: design section 6, witness 4, says "The test KILLs only the `sh`; helper and shell are gone within two seconds". The build brief requires witness 4 "as section 6 states them".

What changes: the plain `sh -c` branch of `runLauncherDeathWitness` in `metasystem/internal/identity/fixture_custodian_witness_test.go`. It needs:
- a shell that cannot replace itself with the helper;
- an assertion that the killed pid is not the owner's pid;
- preferably, a check that the custodian log names the `sh` as `dead-launcher=`.

(On Linux, dash also replaces itself with the last command of a `-c` string in current versions. I did not measure this on the VM.)

## Conformance and the brief's checks

1. **Boundary.** The diff touches only `internal/identity/fixture_custodian.go`, `fixture_custodian_test.go`, `fixture_custodian_witness_test.go` and `internal/testenv/testenv.go`. It adds:
   - no `proc ref` verb;
   - no export from `LaunchSuite` or `test run`;
   - no `testutil.Fixture`.

   `inheritedControlNames` and `inheritedControlPrefixes` are unchanged. `git grep` finds `METASYSTEM_RUN_OWNER` and `METASYSTEM_FIXTURE_CUSTODIAN_CHAIN` only in identity and testenv, and no environment catalogue or digest mentions them. The real-process witnesses are in the witness file. The in-package table file gains `TestControlledLaunchersExportTheirOwnRef`, which probes only its own process and starts nothing.

2. **Chain resolution.**
   - When `METASYSTEM_RUN_OWNER` is set, the value must parse, probe Alive, match `SameIdentity`, and be reached by the ancestor walk. Any failure returns an error. testenv wraps the error with the value and returns before it creates the pipe, the log or the custodian.
   - There is no fallback. Mutation 2 (an invalid set value falls back to the heuristic) failed all three negatives, each with `err=<nil> output="PASS\n" sink="test ran"`.
   - Unreadable argv ends the walk. Mutation 3 failed the table: `chain=[701 702 703]; want length 2 ending at 702`.
   - The chain the custodian gets is the one that was resolved. testenv strips any inherited `METASYSTEM_FIXTURE_CUSTODIAN_CHAIN` from the custodian's environment and appends the resolved chain. `custodianChain` reads that variable only under `METASYSTEM_FIXTURE_CUSTODIAN=1`.

3. **Launcher-loss kill.**
   - Chain members are checked with `AliveRef`. A reused pid compares as mismatched, which reads as Dead. That is correct, because the member really is gone.
   - The only kill is `SignalExact` on the owner's exact ref, which re-proves the identity just before sending. An unrelated process on a reused pid cannot be the target.
   - Every phase has a bound. A halt timer (bound 5 s plus 1 s margin, then `os.Exit(2)`) is armed around each poll's probes, `proveOwnerDead`, `reapLostLauncher` and `reapDeadOwner`. A blocked probe therefore ends the process.
   - The custodian lives as long as its owner, plus at most about 6 s for each kill or reap phase. The only unarmed probing is the fallback chain resolution inside `RunCustodian` (the `proc custodian` path; see notes).

4. **Mutations.** Mutation 1 failed witness 16 only (`fixture child 52717 survived owner death`, after 2.33 s) and passed witness 4 (see U4a-1). Mutation 2 failed all three witness 16 negatives. Mutation 3 failed `TestRunOwnerResolution` and passed witness 4. Every failure was on behaviour; every mutated copy compiled.

5. **Gate D2.** `METASYSTEM_RUN_OWNER` is read in only two places:
   - `startFixtureCustodian`, which runs only when `METASYSTEM_FIXTURE_CUSTODIAN_START=1` was set before `prepare()`;
   - `RunCustodian`, which is reachable only with `METASYSTEM_FIXTURE_CUSTODIAN=1` or through `proc custodian`.

   An ungated test binary has no new read, walk, child, log or failure path. I confirmed this by reading the code and did not rerun the seat's ungated check.

6. **Survivors.** See below.

7. **Comments.** The diff adds no comments and has no unit, round or finding references. One existing comment is now stale (see notes).

## The incident: does the witness's cleanup depend on process enumeration?

I tested this, and it does not. I compiled `identity.test` from my copy and ran both launcher witnesses under `sandbox-exec -p '(version 1)(allow default)(deny sysctl-read (sysctl-name "kern.proc.all"))'`:
- Both failed loudly, naming a pid: `fixture child 93155 survived owner death` (2.09 s) and `fixture child 95880 survived owner death` (2.45 s). They had passed outside the sandbox, so the denial took effect in the survivor scan. My `ps` check of the denial could not run, because sandbox-exec refuses setuid `ps`.
- A census right after the run found no tagged fixture child. Two custodians were still running (93144 at 5 s old, 95874 at 2 s old), both inside their 5 s reap bound. The next census found neither.

The witnesses clean up by the exact refs they already hold. Their deferred `SignalExact` calls use per-pid `kern.proc.pid` probes, not the table scan. An enumeration denial on its own does not reproduce the builder's leak on this tree.

The leak evidence shows the owner gone, both children alive at 43 minutes, and no custodian. That fits a test process that died before its deferred and cleanup functions could run, such as a `-test.timeout` panic or a caller killing `go test`. It also fits an earlier revision of the witness. The evidence file cannot tell these apart.

Two leak windows remain. Each needs a second fault on top of the custodian being unable to scan:
- (a) The witness registers the child and other kill defers only after it has proved all three refs (ownerpid, pid0, pid1). If proving a later ref fails, a child that already started is left to the custodian alone. The landed `custodianWitness` registers each defer right after its own wait. This is a cheap reorder in this unit, but not material.
- (b) No in-process cleanup survives the death of the test process itself. A witness cannot fix that. I recommend recording it for unit 5's fixture: the reap path for recorded exact refs should not depend on scanning the process table.

## Non-material notes

- The `RunCustodian` doc comment ("reaps only children attributed to its exact dead identity") no longer describes the code. The custodian now also kills a live owner when a chain member dies.
- `ExportRunOwner` and `ResolveRunOwner` are exported without doc comments.
- `waitWitnessDead` dropped from `wiringBound` (30 s) to 2 s. This also tightens the landed witnesses 3 and 5. It matches section 6's "within two seconds" and weakens nothing, but it is a flake risk under suite load for witnesses that previously had a 30 s hang bound. The helper's failure text also calls the owner a "fixture child".
- `proc custodian`, the unit 3c verb, now resolves a chain through `RunOwnerEnv` or the heuristic inside the custodian process, because it passes no chain variable. This is consistent with design 3.3 ("Beds start the same code") and the verb has no consumer yet, but it gains a failure path: exit 1 on an invalid run owner, after the bed has already started it. The resolution runs before any halt timer is armed. `TestProcCustodianProcessBoundaries` still passes on 4a (1.92 s).
- Heuristic walk:
  - When a parent pid cannot be read, the walk returns the partial chain, possibly empty, with no error. It watches less and cannot kill anything live.
  - At pid 1 on Darwin, launchd's argv is unreadable to a normal user, so the walk ends with pid 1 in the chain; pid 1 never dies.
  - An ancestor whose identity cannot be proved fails fast.
- The witness 16 negatives implement the "signal sink" as a file the test body writes. That proves no test ran, not that no signal was sent. The live non-ancestor `sleep 30` is not re-probed afterwards. The start path contains no signal call, so this hides no defect.
- No negative covers a live ancestor pid with the wrong start time. The walk's `SameIdentity` handles that case, and the design lists only three negatives.
- In `reapLostLauncher`, an owner still unreaped 5 s after the kill (a zombie whose living parent does not wait) ends the custodian with an error and no reap. I did not measure whether Darwin's `kern.proc.pid` reports a zombie as alive. This follows the brief's rule.
- `ExportRunOwner` appends to the caller's slice, so it can write into the caller's backing array. No current consumer is affected.
- Each killed owner leaves a `/tmp/metasystem-test-registry-*.custodian.log` behind (the registry home is swept, the log beside it is not). That is unit 3b behaviour, and these witnesses add more of those files. My runs left some in `/private/tmp`.

## Survivors from my runs

Tagged processes my runs created:
- the custodians 93144 and 95874 from the sandbox run;
- the custodian 95077 from the probe with the extra assertion and mutation 1.

All three exited at their bound without help. The final census, taken with environment variables (`ps -axwwE`), matched none of `METASYSTEM_FIXTURE_OWNER`, `FIXTURE_LAUNCHER_WITNESS_MODE`, `METASYSTEM_FIXTURE_CUSTODIAN`, `FIXTURE_CUSTODIAN_WITNESS` or `detached.sh`. I killed nothing.

One untagged `metasystem.test -test.run=^TestAdoptionComparisonSelectedScenarios$` (pid 49037, parent 38456) started before my runs. It is not mine and I left it alone.

Material findings: 1 (U4a-1).

VERDICT: NOT LAND
