# Read: fixture-children unit 4b (controlled launchers export the run owner)

Reader: independent, did not write the change. Tree: `g18/wt-4b-r2`. HEAD 23f470d37, and `git write-tree` of the index gives 19e7e829a56b8003ba1b8963c35ba1cc97f9c7d8, which matches the base named in the brief. I did not change anything in the worktree. All experiments ran on a copy at `1e0f004f-.../scratchpad/fc4b/metasystem`, and their output is in `fc4b/run.out`, `fc4b/run2.out`, `fc4b/A.out` and `fc4b/B.out`.

## Material findings

None.

## What I checked, with evidence

### 1. Boundary

The unstaged diff against the base tree is the same as `fcu4b.diff` (130 insertions, 5 deletions, 9 files). The only difference is one hunk header: in this tree the `launcher.go` hunk starts at line 197, not 195, because the diff file was cut in the builder's older worktree. The content is identical.

The diff contains only:
- the `proc ref` verb (`identity.go`, `main.go`);
- the export in `LaunchSuite` (`launcher.go:200`) and in `test run` (`test.go:908`);
- `METASYSTEM_RUN_OWNER` added to `testingEnvironment` (`test.go:1411`) and to `inheritedTestingEnvironment` (`test.go:1426`);
- the `digestEnvironment` exclusion (`test_build.go:1646`);
- tests for all of the above.

The diff does not touch the custodian's reading side (`fixture_custodian.go`, `testenv.go`), `go-gate.sh` or `harness_fixture_owner`. Neither allow-list gains any other name.

### 2. The exported value

- **Whose identity.** Both launch points call `identity.ExportRunOwner`. It probes `os.Getpid()` with `KernelProber` and writes `EncodeRef(exact.Ref())`. The value is the exporter's own identity, never a bare pid and never a parent's.
- **Only when unset.** `ExportRunOwner` returns an environment unchanged if it already has the name. Mutation M1a below shows a test catches an overwrite.
- **No `os.Setenv`.** Neither `test.go` nor `launcher.go` calls it. `ExportRunOwner` returns a new slice, so `prepared.Environment` is not changed. `workerEnvironment` is used only at `test.go:919`, as `LaunchSuite`'s `Environment`, so nothing that `test run` starts later inherits the value.
- **Inside `LaunchSuite`.** The export at `launcher.go:200` runs after the joined attempt's identity is captured. The new `childEnvironment` is used for `suite.Env` and for the parity check after the run (`launcher.go:452`). `ProofIdentity` has no environment field (`attempt.go:917-923`), so parity does not change.

### 3. Trap 1: the value reaches the leaf

Line numbers below are from this tree.

1. `test.go:367`: `prepared.Environment = testingEnvironment(os.Environ())`. The allow-list (`test.go:1407-1411`) now admits the name. This matters only when `test run` inherits an owner from an outer launcher, because the fresh export happens later.
2. `test.go:908`: `test run` exports its own ref into `workerEnvironment`, which becomes `LaunchSuite`'s `Environment` at `test.go:919`.
3. `launcher.go:94`: `proofChildEnvironment(options.Environment)`. Its list of names to remove (`launcher.go:525-548`) does not include the run owner, so the value stays.
4. `launcher.go:200`: `ExportRunOwner` keeps the value it was given. `launcher.go:207` sets `suite.Env`.
5. The suite command is `test worker`. At `test.go:1030`, `inheritedTestingEnvironment(packet environment, os.Environ())` takes the name back from the worker's own environment (`test.go:1423-1426`).
6. `test_build.go:732` calls `groupTestEnvironment` (`test_build.go:1601`), then `mergeTestEnvironment` (`test_build.go:1571`). The `section` adapter drops only `METASYSTEM_BIN`.
7. `explicitEnvironmentCommand` (`test_build.go:1691`) sets `command.Env` at `test_build.go:1726`.
8. `go test` passes its environment on to the test binary. I checked this with a throwaway module run under `env -i ... METASYSTEM_RUN_OWNER='pid=1;micro=2' go test -v`. The test binary logged `owner=pid=1;micro=2`.

The builder's trace is correct in substance. Two small differences:
- Its line numbers come from its own worktree, so the `launcher.go` lines are off by 2 or 3 here.
- It puts `testingEnvironment` on the path of the fresh export. In fact that filter runs before the export and matters only for an inherited owner. The trace also leaves out `mergeTestEnvironment`, which keeps the value.

Other paths I looked at:
- **Policy probe worker** (`test_protection.go:238`, `:373`): uses `inheritedTestingEnvironment`, so the value is kept.
- **Git calls** (`judge.go:98`, `test.go:662/747/1392`): use `ScrubbedEnviron`, but they run only git.
- **Section catalog** (`test_go.go:374`): passes `os.Environ()` through.
- **`env -i` in `path-class-fixtures.sh`, `static-reproof-fixtures.sh` and `land-fixtures.sh`**: wraps git and `land.sh` fixtures only, never `go test`.
- **Sandboxes**: there is no `sandbox-exec` anywhere in `proofrun`, `cmd/metasystem` or `scripts`.

A delegate's raw `go test` is the heuristic path the design already describes. I found no path that drops the value between a controlled Go launcher and a Go test binary.

### 4. Trap 2: proof identity

- **Environment digest.** `digestEnvironment` skips the name. The digest feeds the prepared and actual group digests (`test_build.go:745`, `:748`, `:1094`, `:1135`), `groupExecutionIdentity` (`test_build.go:1198-1224`) and the discovery cache key (`test_go.go:116`).
- **Proof identity.** `ProofIdentity` is built from the manifest, configuration, sections, platform, toolchain, ratchet and behaviour policy, with no environment field.
- **Configuration digest.** `effectiveProofConfigurationDigest` picks up keys that exist only in the environment through `config.Keys` (`resolve.go:156-186`). That happens only for names whose suffix is all digits, plus four named `suite.*` keys. `RUN_OWNER` matches neither.
- **Toolchain identity.** It runs `go version` and `go env` for named variables. Neither output depends on this value.
- **Packet digest.** `packet-sha256` is an integrity check inside one run and is not compared across runs.
- **Recorded fields.** The only environment fields written to JSON are `environmentDigest` fields.

On reading, two runs on the same tree get the same identity. I could not confirm this with a real proof run, because the brief forbids one.

### 5. Behaviour with the custodian gate off

Only three places read the variable:
- `testenv.startFixtureCustodian`, which runs only when `METASYSTEM_FIXTURE_CUSTODIAN_START=1` (`testenv.go:118`, `:133`, `:152`);
- `identity.custodianChain` (`fixture_custodian.go:127`), used by `RunCustodian`;
- the `proc custodian` verb, which has no production caller.

**Experiment.** I ran the full `internal/identity`, `internal/testenv`, `cmd/metasystem` and `internal/proofrun` packages on the copy twice:
- **Run A:** variable unset.
- **Run B:** variable set to the exact ref of the script that runs `go test`, which is an ancestor of every test binary, as the `test run` process is in a real proof.

The results were the same. Every package passed except `TestDispatchBriefBoundsAdmission`, which failed in both runs with "cannot resolve installation prefix: exit status 128". That failure comes from the copy not being a git checkout.

`cmd/metasystem` took 305 s in A and 605 s in B. The machine was loaded and another process was running identity tests at the same time (see 7), so I do not read the difference as a defect.

`proc_custodian_test.go` and the custodian witnesses that build on `os.Environ()` (`fixture_custodian_witness_test.go:137`, `:165`, `:223`) pass with the owner set. I also ran `TestCustodianReapsStoppedAndDetached` on its own four times, unset and set alternately. It passed every time and left no survivors.

Not covered: a test binary whose inherited owner is not an ancestor, for example one started under a detached chain. That case only arises with the gate on or through `proc custodian`, and failing fast there is the designed behaviour.

### 6. Mutations on my copy

Each mutation was run with focused tests, and each file was restored and compared with the worktree afterwards.

| Mutation | Result |
|---|---|
| M1a: `ExportRunOwner` overwrites an inherited value (the preserve check is disabled) | `TestControlledLaunchersExportTheirOwnRef` fails: "existing run owner changed: [A=b METASYSTEM_RUN_OWNER=outer METASYSTEM_RUN_OWNER=pid=6195;...]". Behaviour failure. |
| M2a: name removed from `testingEnvironment` | `TestRunOwnerSurvivesTestingWorkerFilters` fails: "[PATH=/fixture/bin]". Behaviour failure. |
| M2b: name removed from `inheritedTestingEnvironment` | Same test fails: "[METASYSTEM_PROOF_CONTROL_ROOT=/proof PATH=/fixture/bin]". Behaviour failure. |
| M2c: name added to `proofChildEnvironment`'s list of names to remove | `TestRunOwnerSurvivesProofFiltersToLeafCommand` fails: leaf owner "". Behaviour failure. |
| M3: digest exclusion removed | My first attempt did not compile (unused import). With the import kept, `TestEnvironmentDigestIgnoresCacheAndTemporaryLocations` fails: "a new run owner's exact process reference changed the environment digest". Behaviour failure. |
| M5: `LaunchSuite` export removed | `TestLaunchSuiteWritesBannerProgressAndReapsWatchdog` fails: suite run owner "". Behaviour failure. |
| M1b: `LaunchSuite` strips an inherited owner at the call site, then exports | No test fails (see notes). |
| M4: `test run` export removed | No test fails (see notes). |

### 7. Survivors

- **Before my second script:** the census found no tagged process.
- **After run B:** the census showed two tagged processes with parent pid 1: pid 60188 (`detached.sh`) and pid 60192 (`sh -c ... while :; do sleep 1; done`). Their tag was `METASYSTEM_FIXTURE_OWNER=pid=60176;micro=1789578947116244|TestCustodianReapsStoppedAndDetached|1234abcd`.
- **Who made them.** They did not come from my runs. The owner's start time (micro 1789578947) is 19:15:47 local time, measured against my own script's ref (micro 1789578040, started about 19:00:40). That test lives in `internal/identity`, which in run B finished about 19:06. At 19:15:47 my run was executing only `cmd/metasystem`. Another process on the machine was running the identity tests then.
- **Now.** About a minute later both were gone before my next census, so my clean-up step found nothing to kill. My four isolated runs of that test left none. No tagged process from my runs remains.

### 8. Comments

The single added code comment (`test_build.go:1643-1645`) is plain English and describes the code as it is. No comment or test message mentions units, rounds, findings or reviews.

## Non-material notes

- **N1.** The export in `test run` (`test.go:908`) has no observable effect. `LaunchSuite` runs in the same process and would export the same ref. Removing it (M4) is not detected, and its only side effect is that the value is present during the joined identity capture, which ignores it. It does match the design's list of launch points. No test observes `test run`'s own worker environment; that clause of witness 15 is covered only through `LaunchSuite`'s test.
- **N2.** The rule "never overwrite an inherited value" is tested only in the shared helper. No `LaunchSuite` test passes in an inherited owner, so an overwrite at the call site (M1b) goes unseen. Reaching it takes deliberate extra code, so the gap is small. It will matter once `go-gate.sh` exports (unit 10) and `proof run` is nested under it.
- **N3.** The `LaunchSuite` witness checks only `owner.Pid == os.Getpid()`, not the full exact identity including start time.
- **N4.** `TestProcessRefPrintsOneExactEncodedLineOrNothing` uses a fake prober. The shell half of witness 15 (`sh -c 'metasystem proc ref --pid $$'`) is not an automated test; the builder, the seat and I each ran it by hand. On my run the ref's pid matched the shell's pid, an absent pid gave exit 1 with 0 bytes, and an extra argument gave exit 2.
- **N5.** The new comment says the run owner "does not change test behavior". That is slightly too broad when the custodian gate is on, because a set value is then checked and a bad one fails fast.
- **N6.** With the gate on, `testenv` reads the variable, so Go's test result cache would miss on every run for those packages. This has no effect with the gate off, and I did not check whether proof groups use the cache.
- **N7.** The builder's staticcheck claim and race claim were not repeated by me. The seat's `gate-fast-4b.out` shows "fast mode passed (gofmt, vet, staticcheck, ...)" with rc=0, and `race-cmd-4b.out` ends with "ok ... cmd/metasystem 537.590s".

VERDICT: LAND
