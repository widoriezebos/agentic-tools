# fixture-children: decisions for unit 5e

Unit 5e turns the custodian start on for every test binary and settles what
that start needs: a start check that cannot be fooled, `Fixture` refusing to
run without a custodian, cleanup step 4, recorded refs the custodian can reap
without the process table, log retention, and the start variable. Revisions 3
to 5 of the design page stand; nothing here changes the ownership record or
the key scan, which are unit 5d's, and 5d lands before this work, so every
change below to `testutil/fixture.go` builds on 5d's version of it, where the
ownership record is written before the cleanup is registered. Each item gives
the decision, why, and the witness.

## 1. Turning the start on

**Decision.** Every binary whose TestMain calls `testenv.Main` starts a
custodian before `m.Run()`, unconditionally. `Fixture(t)` never starts one; in
a binary without a custodian it fails (item 2). `METASYSTEM_FIXTURE_CUSTODIAN_START`
is removed, with its entry in the inherited-control list. There is no opt-out
and nobody may set one.

**Why.** The custodian is the test binary re-executed, and the branch that
makes that re-execution a custodian lives in `testenv.Main`. A lazy start from
`Fixture` in a binary whose TestMain is something else would re-execute a
binary with no such branch, and that binary would run its whole test list as
the "custodian", with a pipe on descriptor 3 and nothing to stop it. Starting
in `testenv.Main` costs one fork of an already loaded binary and a 250 ms poll
of a few pids. An opt-out is a switch that removes the safety with no record;
a binary that cannot start a custodian exits 2 naming the cause, which is the
page's fail-fast rule.

**What changes for the 82 packages (R-115-m1e).** (a) One extra process per
test binary, started before `m.Run()`; a start that fails exits 2 before any
test runs. (b) A malformed, dead or non-ancestor `METASYSTEM_RUN_OWNER` exits
2 before any test runs. (c) The binary is killed when its watched launcher
dies: the exported run owner, or the first ancestor that is not go tooling.
(d) One log file per binary beside its registry home, until item 5 removes
the quiet ones. (e) `testenv.Main` reads `METASYSTEM_RUN_OWNER`, so its value
joins Go's test-cache key. Section 8 approves (a) to (c) in as many words;
(d) is the log 3.3 already names; (e) is below. No test outcome changes.

**The test cache.** Go's test cache keys on every variable the binary reads
through `os.Getenv` or `os.LookupEnv`. go-gate.sh's shards run `go test -race
-cover -timeout 60m` without `-count=1`, so the cache can serve them. Today
the variable is unset there, the recorded value is stable, and cached runs
keep hitting. When go-gate.sh exports the variable (unit 10) every gate run
re-runs every package. That is the honest outcome, not a loss: a cache hit
never runs the binary, so nothing the gate proves changes; the cost is gate
wall time on an unchanged tree, and proofs already run `-count=1`. Unit 10
states this when it exports. Reading the variable through `os.Environ()` to
keep it out of the cache key is rejected: it hides an input from Go on
purpose. The comment at the run-owner exclusion in
`internal/proofrun/test_build.go` changes to say that the value locates
fixture custody, a bad one fails the binary at start and a good one changes
no outcome, so binding it to the proof key would only prevent reuse. The
exclusion stands.

**Witness.** The custodian witnesses that today pass
`METASYSTEM_FIXTURE_CUSTODIAN_START=1` to their helper binaries drop it and
still pass; the identifier no longer compiles. Before the unit lands, every
package under `./...` passes with the start on, on both platforms.

## 2. The custodian's own checks

**Decision.** The start check becomes a handshake. `startFixtureCustodian`
gives the custodian a second pipe, its write end on descriptor 4.
`RunCustodian` writes `ready` and closes it after `signal.Ignore`, the chain
parse and its self-probe have succeeded, right before it blocks on the watch.
The owner reads descriptor 4 until newline or EOF, with a five-second read
deadline as a hang cap: `ready` means started; EOF without it means the
custodian exited during its checks, and the owner exits 2 quoting the last
lines of the custodian log; the deadline means it hung, and the owner kills it
by exact ref and exits 2. `testenv` keeps the custodian's exact ref and
exposes `testenv.FixtureCustodian() (identity.Ref, bool)`. `Fixture(t)` calls
it before minting the key: `false` means the binary has no custodian and it
fails with "process fixture needs a custodian: this binary's TestMain must
call testenv.Main"; otherwise it probes the ref and fails unless the state is
Alive, `SameIdentity` holds and `Zombie` is false. `proc custodian` passes a
nil ready writer; beds keep their own started check (unit 10).

**Why.** A probe after `Start` sees a process that exists, and a custodian
that exits during its checks is never waited on, so it stays a zombie that
probes Alive; a probe plus a `Zombie` check closes that only for an exit that
has already happened, and a custodian that exits a millisecond later still
passes. The pipe reports the exit itself, whenever it happens during the
checks, with no wait and no timing. `Zombie` is still what `Fixture(t)` must
read: the owner never waits for its custodian, so one that dies later is a
zombie for the rest of the binary's life.

**Witness.** `TestCustodianStartRefusesACustodianThatExitsAtOnce` (testenv):
`startFixtureCustodian` takes an injectable command builder; with `sh -c 'exit
2'` in the custodian's place the start returns an error naming the exit and
no test runs; with the real re-execution it returns nil and
`FixtureCustodian()` probes Alive and not `Zombie`.
`TestFixtureRefusesWithoutARunningCustodian` (testutil, table):
`newProcessFixture` takes the custodian ref; nil, a prober answering Dead, a
mismatched identity, and `Zombie: true` each fire the recording double's
`Fatalf` with the message above (the double records the message and panics
with a sentinel the witness recovers, so a setup failure no longer crashes the
binary); Alive and matching mints a key. Live: every test in the tree that
calls `Fixture(t)` now runs under a real custodian.

## 3. Cleanup step 4

**Decision.** Step 4 is dropped from `testutil.Fixture`'s cleanup and from
`harness_fixture_reap`; the list in 3.2 ends at step 3, and nothing replaces
it in the fixture. The custodian log's readers are the witnesses that assert
an action after an owner's death (3, 4, 5, 6, the second leg of 10, 16) and
the landing note's look at every log that item 5 leaves behind after a full
run, which are exactly the logs holding an action.

**Why.** One custodian serves the binary and acts in two cases: the owner is
dead, when no test in it is alive to read anything, or a chain member is dead,
when its first act is to KILL the owner and its first log line follows that
kill. There is no third case, so a live test can never see an action line in
its own custodian's log, and a step that cannot fail proves nothing. A bed is
the same: its custodian is silent while the bed lives.

**Witness.** None to add. The clause "the custodian log is empty" in witnesses
9 and 17 becomes "the binary's custodian removed its log at exit" (item 5), a
check the landing note makes after the run, not a step inside a test.

## 4. Recorded refs the custodian reaps without the process table

**Decision.** Each binary gets a record file beside its registry home,
`<registry>.fixture-refs-<owner pid>`. `startFixtureCustodian` creates it,
hands the path to the custodian in `METASYSTEM_FIXTURE_CUSTODIAN_RECORDS`, and
exposes it to `Fixture` through `testenv`. `Record` and `Hold` append one line
`+<EncodeRef(ref)>` in a single `O_APPEND` write right after the probe stores
the ref; cleanup appends `-<EncodeRef(ref)>` for a ref only once step 1 found
it gone or step 2 proved it dead, mismatched or a zombie. A ref still running
when step 2's bound ends gets no `-` line. The kind is not recorded: with the
owner dead there is no test left to fail, so the custodian treats a held and
a finished child alike, kills a live match and logs it.

The custodian reads the file only after the owner is proved dead, so no
writer remains. It keeps every complete line, drops a final line with no
newline, and logs `error=malformed-record line=<n>` for a line that does not
parse, acting on nothing there. The live set is every `+` ref with no matching
`-`. In `reapDeadOwner`, before the first table scan, it sends
`SignalExact(KILL)` to each ref in the live set through the runtime's sender,
one line each: `action=kill pid=<n> carrier=record result=<...>`; then on
every rescan it re-probes the set beside the table scan until each ref is
dead, mismatched or a zombie; the quiet window and the bound cover both. When
the table read fails, as a denied `kern.proc.all` does, it logs
`scan=unavailable error=<...>` once and still completes when the live set is
empty and quiet; today it loops to its bound and exits with an error having
killed nothing. At `action=complete` it removes the record file. Nothing here
signals except through `SignalExact` with a per-pid probe, so a recycled pid
mismatches and is gone.

A `Hold` child: cleanup kills it at step 1 and writes its `-` once it is
dead; if the binary dies first, the custodian kills it by its record with one
log line, tag or no tag. A test that already cleaned up wrote a `-` for every
`+` before it returned, so the custodian's live set for it is empty.

**Why.** A binary that dies before its cleanups run leaves its children to
the custodian, and where the sandbox denies the whole-table read the tag scan
finds nothing; the exact refs the fixture already proved are the one thing
the custodian can act on with a per-pid probe alone. The file lives beside
the home because the home is removed while the custodian may still be
reading, and per owner because a helper binary that joins its parent's
registry home has a custodian of its own.

**Witness.** `TestCustodianReapsRecordedRefsWithoutTheTable` (identity,
table): `reapDeadOwner` with the table scan injected to fail as a denied
`kern.proc.all`, a fake prober holding pid 500 at A, and a record file: `+A`
sends exactly one KILL to 500 and logs `carrier=record`; `+A` then `-A` sends
nothing; `+A` then a torn `+B` with no newline sends one KILL to A and nothing
to B; `+A` with 500 now at B sends nothing and logs gone; a line that does not
parse is logged and nothing is sent; in every case the custodian completes,
logs the scan error once and removes the record file.
`TestFixtureRecordsAndReleasesEachChild` (testutil, table): after `Record` and
`Hold` the file holds one `+` each; after cleanup each has its `-`; a child
the fake prober keeps running past step 2's bound has none. Live: witness 3's
helper also `Record`s a child started outside `fixture.Shell`, so it carries
no tag; after the helper is KILLed that child is gone within two seconds and
the log names it with `carrier=record`, beside the tagged shell's own line.

## 5. Log retention

**Decision.** Both policies. The log moves to
`<registry>.custodian-<owner pid>.log`, per owner, because two custodians in
one registry home must not share one log or delete each other's. First, the
custodian tracks whether it wrote a kill, kill-owner, error or scan line; at
`action=complete` with none written it removes its log, and it removes the
record file at every complete. A log that remains means its custodian acted,
failed or died; a crash reaches the log through stderr and never reaches
complete. Second, the registry sweep at every binary's start
(`removeDeadRegistryHomes`, on the same roots) also removes
`metasystem-test-registry-*.custodian-*.log` and `*.fixture-refs-*` files
whose mtime is older than seven days. Who deletes: the custodian at its clean
exit, for quiet logs and record files; the next binary's start, for aged
files. The sweep never touches a young log, whatever it holds.

**Why.** The custodian is its log's only writer and removes only its own
file as its last act, so "provably gone" holds by construction; seven days
exceeds any test binary's life by orders of magnitude (the longest timeout in
the tree is an hour), so no live custodian owns a file that old, and a week
keeps a kill line where a person can read it.

**Witness.** `TestQuietCustodianRemovesItsLog` (testenv): a helper binary runs
one `Fixture` test to a clean exit; once its custodian's ref probes dead
(bounded poll, hang cap), neither the log nor the record file exists.
`TestCustodianLogOutlivesTheRegistryHome` (testenv): the shape of witness 3,
the helper KILLed under a held shell; `removeDeadRegistryHomes` then removes
the helper's home while the log remains and holds `action=kill`; the same
sweep with its clock injected seven days ahead removes the log and the record
file and leaves a young log alone. Table: the aged-file sweep on fake files
removes an old log, keeps a young one, and leaves an unrelated name and a
symlink untouched.

## 6. Start variable leak

**Decision.** Unchanged and confirmed. The custodian branch is the first thing
`testenv.Main` does, and a binary that finds `METASYSTEM_FIXTURE_CUSTODIAN=1`
without a pipe read end on descriptor 3 exits 2 before `prepare`. The
inherited-control list gets the prefix `METASYSTEM_FIXTURE_CUSTODIAN`, which
replaces the removed `_START` name, so `prepare` scrubs every custodian
variable a launcher passed and `WithoutInheritedControls` strips them from a
child environment; `Fixture.Env` strips the same prefix. The owner never sets
a custodian variable in its own environment (`startFixtureCustodian` builds
the custodian's `command.Env`), so `Env(os.Environ())` and every
`os.Args[0]` re-execution a test makes from `os.Environ()` carry none.

**Witness.** The descriptor-3 refusal test stays.
`TestCustodianVariablesNeverReachFixtureChildren` (table):
`WithoutInheritedControls` and `Env` each drop every one of the six custodian
names. Live: the exit-scan helper's child prints its environment names that
start with `METASYSTEM_FIXTURE_CUSTODIAN`; there are none.

## 7. Size and split

Estimates include witnesses.

- Item 1: testenv gate removal and helper-environment edits, the test_build
  comment: about 30.
- Item 2: ready pipe in testenv and `RunCustodian`, `FixtureCustodian()`,
  the `Fixture` check, `proc custodian`'s nil writer, two witnesses and the
  double's `Fatalf`: about 180.
- Item 3: none.
- Item 4: record file in testenv, `+`/`-` lines in `Fixture`, the custodian's
  record reaping and scan-error handling, three witnesses: about 195.
- Item 5: per-owner log name, quiet self-delete, aged-file sweep, three
  witnesses: about 140.
- Item 6: the prefix, `Env`'s strip, one witness: about 25.

Total about 570, so three landings in dependency order, each ending on
runnable behaviour, all after unit 5d (the ownership record's walk and the
"led by" tie), whose `fixture.go` they build on:

- **5e, the start on and proved: items 1, 2, 6, and item 5's log name.**
  `internal/testenv/testenv.go`, `internal/identity/fixture_custodian.go`,
  `cmd/metasystem/identity.go`, `internal/testutil/fixture.go`,
  `internal/proofrun/test_build.go` (comment), the helper environments in the
  identity and testutil witnesses, and the new witnesses. About 250. Ends
  with every test binary running under a custodian proved running, `Fixture`
  refusing without one, and no custodian variable reaching a child.
- **5f, records: item 4.** `testutil/fixture.go`, `testenv.go`,
  `fixture_custodian.go`, and its witnesses. About 195. Ends with the
  custodian reaping recorded children where the table is denied.
- **5g, retention: the rest of item 5.** `fixture_custodian.go`,
  `testenv.go`, and its witnesses. About 140. Ends with quiet logs gone at
  the custodian's exit and aged files swept. Between 5e and 5g every binary
  leaves one empty log; accepted for landings on the same day.

If a computed diff passes 300 it splits at its witness boundary, as section
7 already rules.

## Wording that changes on the page

3.2: step 4 removed; `Record` and `Hold` write the record line and cleanup
writes the release line. 3.3: the start is unconditional in `testenv.Main`;
"probes the custodian after starting it" becomes the ready handshake; the log
and record file paths and their retention; `Fixture(t)` fails in a binary
without a custodian. Witness 3 gains the untagged recorded child and its
`carrier=record` line; witnesses 9 and 17 read "the custodian removed its log
at exit" for "the custodian log is empty". Section 7's unit 5 row lists 5e,
5f and 5g.

## Questions for Wido

None. Section 8 already approves every consequence of the start for the 82
packages; the removed variable was scaffolding nothing set.
