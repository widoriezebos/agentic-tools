# fixture-children-cannot-outlive-their-test: design, revision 3

- Kind: design
- Id: 01M3A2YHDMX5B1JTEYZNZ2N1KB
- Status: done
- Goals: fixture-children-cannot-outlive-their-test

Priority 1, tier 2, opened by Wido 2026-09-16 (goal
plans/goals/fixture-children-cannot-outlive-their-test.md). Revision 3
answers FC-6, FC-7 and FC-8 from the round 2 critique and adds producer 2's
exact leak path. Both review rounds are used; the seat reads this page and
lands it.

## What changed in revision 3

Nothing the round 2 critique recorded as closed has changed: FC-1 to FC-5
stand as revision 2 wrote them (custodian outside every kill domain, exact
refs re-proved before every signal, census acting only on a provably dead
owner, every scenario behaviour kept).

- FC-6: 3.1 gives every fixture a full key (owner ref, test name, nonce);
  3.2 scopes cooperative cleanup to it; 3.3 keeps owner-wide reaping for the
  custodian after the owner is proved dead; 3.5 makes `--owner` dead-only
  and adds `--key`; witness 12.
- FC-7: 3.5 names each consumer's function; witnesses 13 and 14; units 7
  and 8; section 8 gates on them.
- FC-8: 3.3 states the producer and consumer contract for
  `METASYSTEM_RUN_OWNER`; witnesses 15 and 16; unit 4.
- Producer 2's exact path (section 1), rule 6 and its use in 3.2; witness 17.
- Non-material notes: the custodian carries no tag and excludes its own ref
  (3.3); `--owner` has one meaning (3.5).

## 1. What leaks, and how

Three shapes of one defect on m1c within twelve hours
(live-specimen-fixture-child-2026-09-16.md).

**Producer 1** (internal/goal/attention_test.go:129-240): a wrapper `git`
ignores TERM and spins; a `defer` kills a group by a borrowed id
(attention.go:52 sets `Setpgid`, the fixture never checks), error discarded.
A `-timeout` panic, suite abort or session limit never runs the defer;
sixteen loops made a load of 40.

**Producer 2, TestPendingWaitFromChildShell**
(cmd/metasystem/wait_verb_test.go:1147, landed 53c6d2ee7) starts its child
at :1205: `exec.Command("/bin/bash", "-c", "exec \"$0\" job watch --root
\"$1\" --job \"$2\" --caller-pid $$", ...)`. Bash execs itself away, so the
orphan shows in `ps` as `metasystem job watch`, not a shell; a hunt for
leaked shells misses it. No `Setpgid`: when the binary exits the child
reparents to launchd outside the run's group, its `--caller-pid` naming a
dead shell. Its hook runs `up`, which launches supervision `Setsid`
(missionrunner/host.go:365, launch.go:691); nothing records those
grandchildren. Cleanup is not missing: `t.Cleanup` at :1291 calls
`finishChild`, which kills on its timeout path at :1254. The leak is every
early return before that select (`writePendingWaitJobStatus` failing, a
notify output file failing to create, the notify run failing at :1240), and
:1253, which returns without killing when ownership cannot be proved: right
about strangers, still an orphan. Eight were found ten hours later.

**Shape 3.** At 08:01 a goal.test binary outlived the bash that started it
and ran to its 45-minute timeout.

## 2. The rule

A run's processes die with the run, whatever ignores whatever. "Run" is the
process that started the work; "processes" are everything below it through
any exec, `Setpgid`, `Setsid` or reparenting. Reaping never depends on the
dying process's cooperation, a leaderless group or a recyclable pid, and
never kills a live run. Consequences, each with a witness in section 6:

1. Every process a fixture launches carries an exact owner identity and the
   key of the one fixture that launched it.
2. Every signal goes to an identity re-proved immediately before it.
3. One custodian outside the run's kill domains reaps the run's processes
   when the run's owner or the owner's launcher dies.
4. A census names what outlived its run and reaps only the exact class.
5. No fixture simulates an unresponsive process with a bare busy loop.
6. The kill is registered in its own cleanup at the moment of `Start`, never
   inside the function that also does the work: reaping in a helper's
   success path leaks on every error path of that helper.

## 3. The mechanism

### 3.1 Identity: exact refs and fixture keys (FC-4, FC-6)

`internal/identity` gains `EncodeRef`/`ParseRef` in the native shape: Darwin
`pid=<pid>;micro=<StartedAtUnixMicro>`, Linux
`pid=<pid>;ticks=<StartTicks>;boot=<BootID>`. `ParseRef` rejects a value
whose `Ref.NativeExact()` is false (revision 1's `<pid>:<start ticks>` was
wrong on Darwin, where `StartTicks == 0`).

**The fixture key.** `identity.FixtureKey{Owner Ref; Test, Nonce string}`,
encoded `<EncodeRef(owner)>|<test name>|<nonce>`; the nonce is eight hex
characters from `crypto/rand`, minted per `testutil.Fixture(t)` call and per
bed key. `ParseKey` rejects fewer than three fields or a non-native owner.
The tag `METASYSTEM_FIXTURE_OWNER=<EncodeKey(key)>` goes in the environment
of every process a fixture launches; environment survives exec, `Setpgid`,
`Setsid` and reparenting, and already travels the supervision tree in
fixture mode. `Probe` gains `Environ` and `Exe`; the fixture process table
gains the same fields, so every scan runs against a file.

Two scans; the signature fixes which one a caller may use:

- `FixtureSurvivors(key)`: live processes whose tag parses to exactly that
  key. Safe while the owner lives: a sibling test has another name and
  nonce, so a parallel test's live fixture is never selected.
- `FixtureSurvivorsOfDeadOwner(prober, owner)`: probes the owner first.
  Alive or Unknown returns an error and selects nothing; Dead or mismatched
  returns every live process whose tag's owner field is that ref, any name
  or nonce. The only owner-wide selection; nothing skips the probe.

Both return exact ref, pgid, exe and argv, and separately the
signalable-but-unreadable set (indeterminacy never acts).

### 3.2 Recording and reaping by exact identity (FC-4, FC-6)

`SignalAuthenticated` (proofrun/watchdog.go:284) moves to
`identity.SignalExact(prober, ref, sig)`: probe, `SameIdentity`, signal;
mismatch or absence returns "gone" and sends nothing. Every reaper on this
page sends through it; nothing signals a bare or negative pid.

`testutil.Fixture(t)` mints the key. `Env(base)` adds the tag and, for
shells, the leash of 3.4. `Record(pidFile)` reads the pid a fixture shell
wrote, probes it at once and stores `Exact.Ref()`; a pid already dead is
noted, not stored. `Record` is called right after `Start`, before any work
or check that can return early (rule 6); its cleanup owns the kill, and a
helper's own kill, such as `finishChild`'s timeout path, is behaviour under
test, not the safety. The `t.Cleanup` runs on return and on `t.Fatal`:

1. `SignalExact(KILL)` each recorded ref; "gone" accepted, other errors
   `t.Errorf`;
2. wait until each is dead or a zombie, five seconds, then fail;
3. `FixtureSurvivors(key)`: kill each by exact ref, wait the same way, fail
   naming pid, exe and argv. This reaches producer 2's grandchildren, never
   recorded but tagged, and never a sibling test's child;
4. read the custodian log: an action taken while the test was alive means
   the cooperative path missed something, `t.Errorf`.

A child a finished test leaked under a live binary matches no running key.
`testenv.Main` catches it: after `m.Run()` it scans every key this binary
minted, kills what it finds and exits non-zero naming the test. A report
path; the custodian's owner-wide scan at binary death is the safety.

`waitForGroupAbsence` stays only as the production assertion that git's
group was reaped, after the wrapper wrote `$$` and `ps -o pgid= -p $$` and
the test proved them equal. No test kills a group of its own.

### 3.3 The custodian (FC-1, FC-2, FC-8)

**What it is.** A small process in its own session (`Setsid`), std streams
on `/dev/null` and a log file, holding nothing else open, ignoring TERM and
HUP. It is the test binary re-executed: `testenv.Main` (the shared TestMain)
checks `METASYSTEM_FIXTURE_CUSTODIAN=1` before `m.Run()` and runs
`identity.RunCustodian` instead of tests, so every test binary gets one
without a built metasystem. Beds start the same code as `metasystem proc
custodian`. Its environment carries no `METASYSTEM_FIXTURE_OWNER` and every
scan excludes its own exact ref.

**What it knows.** Its owner's exact ref (test binary or bed); the read end
of a pipe whose only write end is in the owner (`ExtraFiles`; Go opens files
close-on-exec, so no fixture child inherits the writer); and the watched
chain, the exact refs of every ancestor from the owner's parent up to and
including the run owner.

**Run owner, producer side.** Every controlled launch point exports
`METASYSTEM_RUN_OWNER=<EncodeRef(self)>` immediately before starting what it
launches, and only when the variable is unset: the outermost controlled
launcher owns the run; the intermediates below it are in the chain anyway.
Launch points: `metasystem test run` (cmd/metasystem/test.go, the worker
command's environment), the proof-run launcher (`LaunchSuite`, beside
`METASYSTEM_PROOF_CONTROL_ROOT` in `suite.Env`), go-gate.sh before its `go
test` shards, every bed in `harness_fixture_owner`; shells use `metasystem
proc ref --pid $$`. The Go exporters share `identity.ExportRunOwner(env)`;
both paths are `EncodeRef`, so the format cannot drift.

**Run owner, consumer side.** When set, the custodian requires a
native-exact value that probes Alive and is an exact ancestor of the owner
(walking parent pids, matching by `SameIdentity`). Anything else, malformed,
dead, mismatched or not an ancestor, fails fast: `testenv.Main` exits
non-zero before `m.Run()`, naming the value; no signal is sent. A silent
fall-back would hide the broken exporter FC-8 describes, and a stale dead
value must never kill a live test at start. When unset, the run owner is the
first ancestor that is not go tooling (argv `go test` or `go tool`;
unreadable argv is not skipped): for a delegate's raw `go test`, the
delegate's bash, the shape 3 launcher.

**What it does.** It blocks on the pipe and polls the chain and the owner
every 250 ms. Pipe EOF, or an owner probe of dead or mismatched, means the
owner is gone: `FixtureSurvivorsOfDeadOwner`, kill each by exact ref, rescan
until empty or five seconds, one log line per action, exit. A chain member
dead or mismatched while the owner lives means the launcher is gone: kill
the owner by exact ref, wait for it to probe dead, proceed as above. Shape 3
dies within 250 ms of its bash.

**Why it is outside the kill domains.** A `-timeout` panic, `os.Exit` or
KILL of the test binary closes the pipe without cooperation. A kill of the
suite's group (proofrun/launcher.go:231) or the launcher's group does not
reach another session. Job cancellation kills the attempt's recorded
processes, launcher, suite group and watchdog; the custodian is none of
them. A delegate killed at its session limit takes its session; the
custodian is not in it. A SIGSTOPped child cannot run a handler; SIGKILL
does not need it to. A `Setsid` Go grandchild reads no leash but carries the
tag. The custodian never kills anything without its owner's tag, and never
kills the owner unless a chain member is gone.

**Fail fast, never degrade.** `testenv.Main` probes the custodian after
starting it and exits non-zero if it is not alive; `testutil.Fixture(t)`
re-probes it before the first child starts and `t.Fatal`s otherwise.

### 3.4 The leash: fast exit for shells

Fixture shells that must hang read from a FIFO whose only writer is the
owner: `read -r _ <"${METASYSTEM_FIXTURE_LEASH:?}"`. When the owner dies the
read returns EOF and the shell exits before the custodian's first poll. A
fast path, not the safety: witnesses 3 and 5 run with the leash disabled.
Beds hold it on fd 9 and close it for children (`9>&-`). The code under test
still sees a TERM-ignoring hang.

### 3.5 The census (FC-3, FC-6, FC-7)

`metasystem proc fixture-survivors [--owner <ref> | --key <key>] [--reap]`,
beside `proc census|alive|find-ancestor`, backed by `internal/census`,
fixture-driven under `METASYSTEM_CENSUS_PROCESS_FILE` (allowed under
`metasystem.runtimes=fake`, as the existing proc verb tests do). Classes:

- `fixture-survivor`: environment readable, tag parses, the key's owner
  probes dead or mismatched. The only class `--reap` touches, through
  `SignalExact`.
- `fixture-survivor?`: signalable but environment unreadable, or owner probe
  Unknown. Printed, never reaped.
- `unowned-in-cache`: exe under a `metasystem-build-cache/go-tmp` directory
  with no tag. Printed, never reaped, unless the per-test directory holds
  the ownership record and that owner probes dead; then `fixture-survivor`
  by record.

Selection is explicit. No flag: the whole table, dead-owner classes only.
`--owner <ref>`: that owner's tagged processes, only if it probes dead or
mismatched; a live or Unknown owner is refused, exit 2, naming it. `--key
<key>`: that key's processes whether the owner lives or not, never wider
than one key; the caller is the key's owner or its teardown.

The ownership record: `testutil.Fixture(t)` writes `EncodeKey(key)` to
`<go-tmp>/<TestName><random>/fixture-owner` before any child starts (the
parent of `t.TempDir()`); the census probes its owner field. A path match
with a live recorded owner is not printed. Per finding:

    fixture-survivor pid=41233 pgid=41233 ppid=1 since=2026-09-16T00:58Z owner=pid=39001;micro=1789200123456 dead key=TestPendingWaitFromChildShell/3f9a1c07 exe=/.../go-tmp/metasystem argv="metasystem job watch --root ..."

Exit 1 when any certain survivor is named.

**Consumers**, each a named function a witness drives from a process file
with an injected signal sink; the second net behind the custodian:

- `harness_fixture_reap` at every bed's end: `--key` per minted key; fails
  the bed on survivors.
- `metasystem health`: `census.FixtureSurvivorLines(prober, table)`, one
  line per survivor, exit non-zero on a certain one; never signals.
- The proof-run launcher: `proofrun.reapFixtureSurvivors(prober, table,
  sink, launcherStart, verdict)` runs in `LaunchSuite` after the suite and
  watchdog have been waited; reaps the exact class through `SignalExact`;
  one verdict line per survivor. A survivor started after the launcher was
  born under this run and turns the result to 1; an older one is reaped,
  named `stale`, and does not fail this proof.

### 3.6 No bare busy loops

The bare loop becomes the leash read wherever a fixture hangs, and `while :;
do sleep 1; done` where it must loop. `TestFixtureSourcesHaveNoBareBusyLoop`
(cmd/metasystem) parses every `while`, `until` and `for ((;;))` loop in
`scripts/agents/**/*.sh` and the shell literals of `*_test.go`, and fails on
a body with no `sleep`, `read`, `wait` or blocking command, naming file,
line and loop.

### 3.7 Each fixture, converted

- attention_test.go hanging git: spinner becomes a leashed read; wrapper
  writes `$$` and its pgid; cleanup is `testutil.Fixture` with wrapper and
  child recorded; `waitForGroupAbsence(groupID, 2s)` stays as the production
  assertion after the leadership check. The transport tests (:555, :623) get
  the same wrapper; their `sleep 1` loops become leashed reads.
- wait_verb_test.go: `Record` right after the `Start` at :1205; `Env()` on
  the child and `up`; `finishChild` keeps its ownership proof and timeout
  kill as behaviour under test; the key scan fails on any missed grandchild.
- suite-progress-fixtures.sh `__stopped`/`__detached`: the bed exports the
  run owner, mints its key, starts a custodian; `__detached` reads the leash
  instead of looping; the three pid files stay as assertion inputs. The
  watchdog's CONT/TERM/KILL ladder and the guard sweep remain what the
  existing leg proves; the custodian is silent while the bed lives.
- fixture-bed-scenarios-fixtures.sh `hang`: both `sleep 600`s become leashed
  reads; the grandchild pid file stays.
- hosts/fake.sh hold: `: >"$turn_dir/host-ready"` first, then the leashed
  read when `METASYSTEM_FIXTURE_LEASH` is set, then the existing `while
  true; do sleep 1; done`. Without a leash the script is unchanged.
- suite-progress-fixtures.sh `cleanup()` and `fixture_bed_parent_cleanup`
  call `harness_fixture_reap` instead of a bare `kill "$pid"`. A bed mints
  one key, plus one per scenario with its own children (`harness_fixture_key
  <scenario>`), listed in a file the reap reads.

## 4. What does not change

What each fixture proves (R-115-m1e): an unresponsive git, a detached bed
member, a stopped runner, a held host that ignores TERM. Production kill
ladders and their `Setpgid`/`Setsid` choices, the proof-run watchdog's
duties, `TaggedSurvivors` and the supervision census.

## 5. What is not covered, and why that is acceptable (FC-2)

The custodian dies only to a KILL or STOP aimed at its own pid, a kill of
every process of the user (logout, `pkill -u`, reboot), or the OOM killer.
None is a test abort: the first is deliberate, the user-wide kills take the
children too, the OOM killer is rare. In each case the census names the
survivors, `health` exits non-zero, `--reap` removes the exact class without
risk to a live run, and a custodian stopped or alive more than five seconds
past its owner is itself named. Witnesses 3 to 6, 10, 11 and 16 kill
everything else.

Risks that remain: a launcher that scrubs `METASYSTEM_*` between fixture and
grandchild (known scrubs keep it; the ownership record is the second net); a
bed that forgets `9>&-` (delays the leash only; witness 6 catches it); the
go-tooling argv rule (an unreadable argv shortens the chain, never loses the
owner); an unlisted controlled launch point (its tests run under the
heuristic, as shape 3 does; witness 15's source check names the known ones).

## 6. Witnesses

Each runs on a quiet machine in seconds, needing no load, leak or aborted
proof. Signals go through an injectable kill function so a witness can
assert nothing was sent.

1. `TestOwnerRefEncodesNativeShape` (identity): Darwin and Linux `Exact`
   values round-trip through `EncodeRef`/`ParseRef` and `EncodeKey`/
   `ParseKey`; a whole-second ref and a two-field key are rejected.
2. `TestRecordedChildIsReprovedBeforeKill` (testutil): table has pid 500
   started at A; `Record` stores A; the table then shows 500 started at B;
   cleanup sends nothing and reports it gone. Live variant: a recorded `sh
   -c 'exit 0'` gets no signal after it exits.
3. `TestKilledTestBinaryLeavesNoFixtureChild`: the test re-executes its
   binary as a helper (env-flagged, so `testenv.Main` gives it a custodian);
   the helper starts a TERM-ignoring tagged shell, no leash, and blocks. The
   test KILLs the helper by pid; the shell is gone within two seconds and
   `proc fixture-survivors --owner` prints nothing.
4. `TestLauncherDeathKillsTheTest` (FC-1): `sh -c` runs the helper of
   witness 3 with `METASYSTEM_RUN_OWNER` unset; the custodian walks to the
   `sh`. The test KILLs only the `sh`; helper and shell are gone within two
   seconds. Table-driven: the walk skips `go test` and `go tool` argv.
5. `TestCustodianReapsStoppedAndDetached`: the helper starts a `Setsid`
   grandchild through `sh -c 'exec sh script'` and a child it then
   SIGSTOPs; the test kills the helper's whole group, as a job cancel does;
   both are gone within two seconds and the log names both. Variant:
   reaping only recorded pids fails on the grandchild; the scan passes.
6. Bed witness (fixture-bed-scenarios-fixtures.sh): after the bed exits on
   INT the `hang` grandchild is gone; after the runner is KILLed the child
   exits on the leash before the custodian polls; leash disabled, the
   custodian removes it.
7. `TestFixtureSourcesHaveNoBareBusyLoop` on the tree, plus `while :; do :;
   done` and `while true; do sleep 1; done` as strings: one finding, none.
8. `TestCensusReapsOnlyDeadOwnedSurvivors` (FC-3): three otherwise identical
   `go-tmp` processes, tagged dead-owned, tagged live-owned, untagged with
   no record, plus one untagged whose directory's `fixture-owner` is dead.
   `--reap` selects the first and fourth, prints the third with `?`, never
   touches the second.
9. The converted attention and wait_verb tests are green; their `go-tmp`
   holds no survivor line and the custodian logs are empty.
10. Suite-progress witness (FC-5): the `__stopped` leg keeps its assertions
    (stalled section named, pids gone after the ladder) and gains, while the
    bed lives: `suite.pid` in state T, `detached.pid` alive after a TERM.
    Second leg: `__stopped` under a throwaway owner shell, no watchdog; the
    owner is KILLed; stopped runner and detached member are gone within two
    seconds.
11. Fake-host witness (FC-5): fake.sh with HOLD and IGNORE_TERM under a
    fixture owner; `host-ready` within two seconds; a TERM leaves it alive
    after half a second; the owner is KILLed; the host is gone within two
    seconds.
12. `TestCleanupIsScopedToTheFixtureKey` (FC-6): a live owner pid 700 with
    two live tagged children, key A (`TestOne`, nonce 1) and key B
    (`TestTwo`, nonce 2). Cleaning A sends exactly one KILL, to A's exact
    ref; B is untouched. `FixtureSurvivorsOfDeadOwner(700)` errors and sends
    nothing while 700 is in the table; with 700 removed it returns both.
    `--owner <700>` exits 2 while 700 lives; `--key A` names only A. Live
    variant: two `testutil.Fixture` values in one test, each with a `sleep
    30` child; cleaning the first leaves the second alive, same identity.
13. `TestHealthNamesTheCertainFixtureSurvivor` (FC-7): under
    `metasystem.runtimes=fake`, one tagged process whose owner is absent
    from the table, one whose owner is present, one signalable-unreadable.
    `metasystem health`, through the entry the existing health tests use,
    prints the first with pid, exe, argv and `owner ... dead`, the third
    with `?`, nothing for the second, exits non-zero; the sink is empty.
    First and third removed: exit zero, no line.
14. `TestLauncherReapsSurvivorsIntoTheVerdict` (FC-7): the same three plus
    a fourth, dead-owned, started before the launcher.
    `reapFixtureSurvivors` sends exactly two KILLs, to the first and fourth
    exact refs; the verdict names both, the fourth as `stale`; the result is
    1 for the first alone; the live-owned one is neither named nor
    signalled. Second case: `LaunchSuite` under the existing launcher test
    fixture, suite command `sh -c 'exit 0'`, same process file; the survivor
    lines are read from the combined log.
15. `TestControlledLaunchersExportTheirOwnRef` (FC-8, producer):
    `ExportRunOwner` on an empty environment yields one variable that
    `ParseRef`s to the caller's probed identity; an environment already
    carrying it passes through unchanged. The proof-run launcher's suite
    environment and `test run`'s worker environment contain it. Shell side:
    `sh -c 'metasystem proc ref --pid $$'` parses to that shell's exact ref;
    a source check that go-gate.sh exports before its first `go test` and
    `harness_fixture_owner` before its first child.
16. `TestCustodianWatchesTheExportedLauncher` (FC-8, consumer): the test
    starts L, its own binary in helper mode `run-owner-launcher`, which
    probes itself, exports its ref and starts `sh -c` (S) running the
    helper of witness 3 (H) with a TERM-ignoring tagged shell, no leash. The
    test KILLs L only. Within two seconds H and its shell are gone while S
    still probes alive; the log names L's ref as the dead chain member.
    Under the heuristic this shape leaves H running, S being the first
    non-go ancestor. Table-driven negatives, each ending before any test
    runs with an empty sink: variable malformed, naming a dead ref, naming a
    live non-ancestor.
17. `TestHelperFailingBeforeItsKillPointLeavesNoChild` (rule 6, testutil): a
    fixture starts a TERM-ignoring tagged child, `Record` right after
    `Start`, then its helper returns an error before its own kill (an
    injected write failure standing for `writePendingWaitJobStatus`). The
    test's cleanup runs; the child is gone within two seconds; the custodian
    log is empty. Second case: the helper's ownership proof fails on a
    mismatched ref and returns without killing, as :1253 does; the child is
    still gone by cleanup step 1.

## 7. Units

Dependency order; each at most 300 changed lines, ending on runnable
behaviour, rebuilding and re-arming the seat.

1. **identity: refs, keys, probes.** `EncodeRef`/`ParseRef`, `FixtureKey`,
   `EncodeKey`/`ParseKey`; `Probe` exposes `Environ` and `Exe`; table
   fields; `SignalExact` lifted from the watchdog. Witness 1. About 240.
2. **identity: the two scans.** `FixtureSurvivors(key)`,
   `FixtureSurvivorsOfDeadOwner`, the classes, the record lookup. Witness 12
   (table part). About 220.
3. **Custodian core.** `RunCustodian`: pipe, owner poll, dead-owner reap,
   self-exclusion, log, bounded exit; `testenv.Main` start and probe; `proc
   custodian`. Witnesses 3 and 5. About 260.
4. **Run owner.** `ExportRunOwner`, ancestor check and fail-fast, heuristic
   walk, launcher-loss kill, `proc ref`; `LaunchSuite` and `metasystem test
   run` export. Witnesses 4, 15 (Go part), 16. About 260.
5. **testutil.Fixture.** Key minting, `Env`, `Record`, cleanup steps 1 to 4,
   the binary's exit scan, the ownership record, the FIFO leash. Witnesses
   2, 12 (live part), 17. About 270.
6. **proc fixture-survivors.** Verb, `--owner` refusing a live owner,
   `--key`, `--reap`, process-file mode. Witness 8. About 230.
7. **Health consumer.** `FixtureSurvivorLines` and the health line. Witness
   13. About 140.
8. **Launcher consumer.** `reapFixtureSurvivors`, the post-suite call, the
   verdict lines, the born-under-this-run rule. Witness 14. About 170.
9. **Convert the Go producers.** attention_test.go's three wrappers and
   wait_verb_test.go, as in 3.7. Witness 9. About 200.
10. **Shell bed helpers.** fixture-budget.sh `harness_fixture_owner` (run
    owner export, key, custodian, leash on fd 9), `harness_fixture_key`,
    `_record_pid`, `_reap`; go-gate.sh's export; fixture-bed-scenarios.sh
    cleanup and `9>&-`; the `hang` scenario. Witnesses 6, 15 (shell part).
    About 230.
11. **Suite-progress and fake host.** `__stopped`/`__detached`, `cleanup()`,
    fake.sh. Witnesses 10 and 11. About 180.
12. **Busy-loop audit and the rule.** `TestFixtureSourcesHaveNoBareBusyLoop`
    and one paragraph of harness fixture doctrine stating section 2.
    Witness 7. About 130.

A unit whose computed diff passes 300 splits at its witness boundary before
review.

## 8. Proof of DONE and approvals

Proof of DONE is the seventeen witnesses green on a quiet machine, plus: no
`Kill(-` in any `*_test.go` without a leadership proof and no bare-pid kill
in any fixture (grep in the landing note); `health` on the seat prints no
survivor after the full fixture suite. Rule 4's gate is witnesses 13 and 14,
which fail if health or the launcher stops consulting the census; the quiet
seat's empty health line is confirmation, not the gate. The next aborted
deep proof leaving `proc fixture-survivors` empty is supplemental.

Wido approves: `METASYSTEM_FIXTURE_OWNER` and `METASYSTEM_RUN_OWNER` as
`METASYSTEM_*` variables every launcher passes through, the second set only
by the outermost controlled launcher; every test binary and bed starting one
custodian that ignores TERM and HUP and lives at most five seconds past its
owner; the custodian killing a live test binary when its launcher dies; a
test binary refusing to start under a malformed, dead or non-ancestor run
owner; the launcher reaping the exact class without a human and failing a
proof on a survivor born under it; the leash making "hang until killed" mean
"or until the test is dead".

## Unchecked

- Whether `KERN_PROCARGS2` returns the environment of every same-uid process
  on macOS; unit 1 finds out. If not, Darwin leans on the ownership record
  and the tagged class degrades to `fixture-survivor?`, never to a wrong
  kill.
- How `go test` groups its binaries and how a delegate runtime ends a
  session; this changes how often shape 3 arises, not whether the chain
  watch covers it.
- Whether the wait_verb hook passes the environment through `up` unchanged;
  unit 9 confirms; the record covers a scrub.
- Whether the proof-run watchdog reacts to launcher death today; unit 4
  defines it either way.
- Whether `metasystem test run` passes its environment to go-gate.sh
  unfiltered; unit 4 confirms; outermost-wins holds either way.

## Amendment, revision 4: the carrier on Darwin

Appended after unit 1 was measured on m1c (darwin/arm64).
`kern.procargs2` returns the environment only for the calling process and
for targets that are not restricted binaries; for a SIP platform binary such
as `/bin/sh` it returns the exec path and argv, nothing else. A `/bin/sh`
child started with the tag reads back zero environment words, tag not found;
a Go child reads back all 65, tag found. Linux procfs has no such
restriction. Every fixture 3.7 converts is `/bin/sh`, so 3.1's carrier
reaches none of them here.

### The decision

The tag stays one word, `METASYSTEM_FIXTURE_OWNER=<EncodeKey(key)>`. A shell
carries it as one complete argv word (option 1); a Go process carries it in
its environment, as 3.1 says. One matcher reads both slots; two slots
because no single slot exists.

- argv is the only slot Darwin shows for a restricted binary, and the fixture
  authors every shell, so it can fill it. argv does not survive exec, and a
  production exec (`bash -c 'exec "$0" job watch'`, `up` launching
  supervision `Setsid`) rebuilds it from code this page must not touch.
  Below it the process is a Go binary, whose environment is readable on
  both platforms.
- The ownership record (option 2) names a test, not a process. Tying a
  `/bin/sh` process to it needs a per-process channel, and its exe is
  `/bin/sh`, so 3.5's `go-tmp` directory rule never matches it. It stays the
  second net for untagged `go-tmp` exes.
- Accepted degradation (option 3) leaves producer 1, the hanging git
  wrapper, in `fixture-survivor?` on the seat platform: reported, never
  reaped, where the leak was observed. Rejected.

### How the word gets into argv, and how it survives

A shell the fixture starts directly: `exec.Command("/bin/sh", "-c", script,
"sh", token)` from Go, `sh -c "$script" sh "$token"` from a bed. The token is
`$1`; a `shift` changes positional parameters, not the argv the kernel
recorded at exec.

A fixture-authored script that production code starts (the git wrapper on
`PATH`, `fake.sh`, the `hang` script, the bed's `__stopped`/`__detached`
entry) begins with a prologue, one source text in `testutil.ShellPrologue`
and `harness_fixture_prologue`:

    if [ -n "${METASYSTEM_FIXTURE_OWNER-}" ]; then
      tag="METASYSTEM_FIXTURE_OWNER=$METASYSTEM_FIXTURE_OWNER"
      [ "${1-}" = "$tag" ] || exec /bin/sh "$0" "$tag" "$@"
      shift
    fi

The script's own environment is intact, only the outside read is blocked;
it re-execs itself once (`bash` for the bash beds) with the word first,
keeping pid, pgid and session; without the variable it is unchanged, keeping
fake.sh's promise in 3.7. Shell literals in `*_test.go` get it too.

`Setpgid`, `Setsid` and reparenting change neither slot. Exec replaces both:
the environment passes unless scrubbed (section 5 stands), argv is rebuilt,
hence the prologue. A `sleep 1` forked by an allowed loop has no slot to
fill and dies within a second, inside the custodian's five-second rescan.

### Reading it

`identity.FixtureTag(exact) (FixtureKey, bool)` finds one word with the
prefix `METASYSTEM_FIXTURE_OWNER=` in argv, then in the environment, and
parses the key; both scans of 3.1 call nothing else. Linux reads both slots
from procfs; Darwin reads argv for every same-uid process and the
environment for unrestricted ones. Zero environment words on a live process
is indistinguishable from a restricted one: `EnvironKnown` false; a fixture
child always has the tag.

Neither slot readable and signalable is indeterminate; indeterminacy never
acts. This exposes an under-scoped class: same-uid unreadable processes are
chronic on macOS (survivors.go), so 3.5's `fixture-survivor?` as written
would print the user's daemons at every `health`. Scoped: unreadable in both
slots and in a group or session led by a dead-owned tagged process or with
an exe under `go-tmp`, or a parsed tag whose owner probes Unknown. The rest
is the census's stray problem, as `TaggedSurvivors` rules. The
safety rules stand: unproven ownership reports; a dead-owner scan errors
while the owner lives; cleanup selects on the full key.

### Wording that changes

3.1: "goes in the environment of every process a fixture launches" becomes
"is one word, in the environment of every process a fixture launches and in
the argv of every shell". 3.5: `fixture-survivor?` as scoped above.
Unchecked, first bullet: answered. Witness 1 adds `FixtureTag` from argv,
from environment, from neither, and the empty-environment rule. In 3, 5, 16
and 17 "tagged shell" means the word in argv; 5's `sh -c 'exec sh script'`
relies on the script's prologue. 8's fourth process is untagged in both slots
with its exe under `go-tmp`. 12's live variant replaces `sleep 30` with
leashed tagged shells. 13's unreadable specimen sits in the dead-owned one's
group. New witness 18, unit 12: every shell literal in a `*_test.go` and
every `scripts/agents/**` script that reads `METASYSTEM_FIXTURE_LEASH`
starts with the prologue; a git wrapper started through `PATH` probes with
the word in argv.

### Units re-estimated

1. `FixtureTag`, the empty-environment rule, Darwin environment for
   unrestricted targets: +40, about 280.
2. Both slots through `FixtureTag`, the `?` scope: +30, about 250.
3. No carrier logic of its own; the log names the slot: +10, about 270.
5. `ShellPrologue`, `Shell(script, args...)`, witness 12's variant: +40,
   about 310; splits at witness 17 under section 7's rule.
6. The `?` scope and its wording: +20, about 250.
9. Prologue on the three wrappers: +15, about 215.
10. `harness_fixture_prologue`, the `hang` script: +20, about 250.
11. Prologue in fake.sh and the bed entry: +10, about 190.
12. Witness 18: +40, about 170.

Units 4, 7 and 8: unchanged.
