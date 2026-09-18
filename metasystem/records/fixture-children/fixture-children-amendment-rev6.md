## Amendment, revision 6: where the record is read, and what ties an unreadable process

Appended after units 3 to 5c were built and run on m1c (darwin/arm64) and
on the Linux VM. The evidence is the code as built, what its witnesses and
stress runs showed, and the toolchain's own source; the ruling on the key
scan was made while unit 5b was being fixed and is written here as it
stands. This is a record of what building found, one decision on the
ownership record, and one wording choice. The mechanism, the carrier
decision of revision 4 and revision 5 stand.

### The record, checked

Section 3.5 puts the ownership record at
`<go-tmp>/<TestName><random>/fixture-owner`, the parent of `t.TempDir()`,
and the census reads it while walking up from an executable to the nearest
directory named `go-tmp`. Whether the two meet depends on `GOTMPDIR`. In Go
1.27, the toolchain go.mod requires, `t.TempDir()` makes the per-test
directory under `GOTMPDIR` when it is set and under `TMPDIR` otherwise. The
Claude adapter sets `GOTMPDIR=<scratch>/go-tmp` beside `TMPDIR=<scratch>`,
and a job worktree's chain cache sets
`GOTMPDIR=<git dir>/metasystem-build-cache/go-tmp`; under either, the
record sits below `go-tmp` and is read, as 3.5 says. `metasystem test run`
passes `TMPDIR` and `GOTMPDIR` through unchanged and sets neither; a
terminal `go test`, this machine's own proofs and the VM's shells have no
`GOTMPDIR`. There the per-test directory is `<TMPDIR>/<TestName><random>/`,
no ancestor is named `go-tmp`, and the walk never starts. The thirteen
`steward run` orphans revision 5 records lived under
`/var/folders/.../T/TestPendingWaitFromChildShell*/002`: a record beside
each, unread. So the net is up under the adapter and down on every run from
a shell, which is where the leaks were seen. The same holds for
`unowned-in-cache`: `go-tmp` is the adapters' directory name, not Go's, so
the class exists only where `GOTMPDIR` names one; from a shell Go builds
under `TMPDIR/go-build*` and the class is empty.

A second gap is in `Fixture(t)` as built. It registers its cleanup and then
calls `t.TempDir()`, so Go's removal of the per-test directory is registered
later and, cleanups running last in first out, runs first. At cleanup step 3
the record is already gone.

### The decision

The census reads the record wherever the executable sits. The walk goes up
from the executable's directory reading `fixture-owner` at each level until
it finds one or reaches the root; the `go-tmp` stop leaves the record's
walk and stays where 3.5 uses it for the classes. The first record found
decides: a record the rules below reject means no record, and the walk does
not go on. The cost is one failed open per ancestor of every untagged
executable, a few thousand per scan; scans run once per teardown, exit or
census, never per poll.

Running tests with `TMPDIR` below `go-tmp` was rejected. It would have to
be set by the adapter, by `metasystem test run`, by go-gate.sh and by every
shell a human or the VM runs tests from; the last two cannot be made to,
and the census would then depend on a scratch layout the adapters own.
Dropping the net was rejected because the class it catches, an untagged
process whose executable a test built or copied into its own temporary
directory, is the shape of the `steward run` orphans with the environment
scrubbed somewhere below `up`; nothing else names it.

What stops a record from claiming an unrelated process:

- A record claims only executables below its own directory. Nothing under
  `/usr`, `/opt` or a checkout's `bin/` has one above it.
- The directory must be the one Go made for the test the record names: its
  name begins with the top-level test function name in the key
  (`TestPendingWaitFromChildShell` for `TestPendingWaitFromChildShell/leak`),
  cut to its first 64 bytes, which is how `t.TempDir()` spells it in Go
  1.27 (a rune cut in half is dropped). A record in
  `/tmp` or at a scratch root fails this and is rejected.
- A tagged process is classed by its tag. The record is read only for a
  process with no tag in either slot.
- The scan's own rule still applies: the key scan wants the record's key to
  be its key, the dead-owner scan wants the record's owner to be the owner
  it proved dead. The record adds a way to find a process, never a way to
  act; every kill still goes through `SignalExact`.
- Only `Fixture(t)` writes these files.

`Fixture(t)` writes the record before it registers its cleanup, so Go's
removal of the directory runs after cleanup step 3. When a test ends
normally Go removes the directory and the record with it; from then on a
leaked process is reached by its tag alone, in the exit scan and in every
later census. That is acceptable: step 3 has already run with the record
present, and the directory of a test whose binary died before its cleanup
stays, as revision 5's thirteen did, which is the case the custodian's and
the census's dead-owner scan exist for. The record is a third carrier,
`record`, named in the survivor line and in the custodian's `carrier=` field
beside `argv-word` and `environment`.

### The key scan's narrower rule

3.1's "separately the signalable-but-unreadable set" and revision 4's
scoping of `?` describe the census. The key scan is narrower. Argv is
unreadable for the moment a process execs or exits, and under a `go-tmp`
layout every test binary of every package runs under `go-tmp`, so a rule
that took any unreadable `go-tmp` process failed a test at step 3 on a
stranger from another package. Ruled: `FixtureSurvivors(key)` returns an
unreadable process only when its process group or session is led by a
certain survivor of that key. `FixtureSurvivorsOfDeadOwner` keeps the
`go-tmp` case beside it, because the census and the custodian only report
that class and never act on it. Step 3 still fails by name on everything the
key scan returns; a `?` line now appears only beside a certain survivor, and
that test is failing already. The exit scan in `testenv.Main` uses the same
scan and the same rule.

Residual: a fixture grandchild caught mid-exec or mid-exit at teardown, with
no certain sibling leading its group or session, is not named by step 3.
Mid-exec lasts microseconds; when the process is readable again it carries
the tag in its environment and the exit scan names it after `m.Run()`, or it
has exited. After the binary dies the custodian's dead-owner scan reaps it
as tagged, or reports it as `?` under `go-tmp`.

### "Led by", not "shares"

Revision 4 says "in a group or session led by a dead-owned tagged process".
The code as built accepts a process in the same group or session as a
certain survivor, whether or not that survivor leads it. The page means "led
by", and the code narrows to it. Sharing borrows: a fixture child started
without `Setpgid` is in the test binary's group with every parallel test's
children, and on a run from a shell it is in the login session with
everything the user started there; both put strangers beside a certain
survivor at step 3 and in `health`. Led by ties the process to one the
fixture provably owns: the `Setpgid` git wrapper of 3.7 and the `sleep 1`
its loop forks, supervision started `Setsid` under `up` and what it runs,
witness 5's `Setsid` grandchild. The tie is the leader's pid equal to the
process's pgid or sid, the leader alive and certain in the same scan. A
group whose tagged leader has already died ties nothing; its members are
the census's stray problem, as revision 4 rules, and a `sleep 1` there dies
within a second.

### Limits on macOS, for the leash and for witnesses

3.4: on macOS a FIFO reader that is entering `read(2)` when the last writer
closes can miss the end of file and block with no writer left (three shells
in one stress run; 5 in 7,200 in a standalone probe, with a raw `close` and
`O_RDWR` writers as well, so it is the kernel, not Go's poller). A fresh
writer opened and closed releases it. So when an owner dies the leash's fast
exit can be lost on macOS, and the custodian is then the only safety; 3.4
already says the leash is not the safety, and this is why. A witness of the
leash closes the writer before the reader can be entering its read, or holds
the reader on another descriptor until the close has happened; witness 6's
leash-disabled leg is the one that proves the safety.

Section 6, rules for every witness on this page:

- A witness releases a stopped child through a pipe, never with `SIGCONT`.
  On macOS a `SIGCONT` sent at once after `Wait4(WUNTRACED)` reports a stop
  was lost 308 times in 9,600: the kernel marks the process stopped and
  wakes the parent before the task is suspended. A `SIGKILL` in the same
  window was never lost in 9,600, so killing a stopped child right after
  its stop is reported is safe.
- A witness compares fixture keys by `EncodeKey`, never as structs. On Linux
  the encoded owner carries start ticks and boot id, so a key parsed from
  argv or a record is not `==` to the minted one.
- A witness that drives cleanup through a recording `testing.TB` registers
  its own kill on the real `t` for every child it starts; otherwise a
  failing witness leaks them.
- A witness that asserts a pid or an executable path takes both from the
  probe: Linux reports the resolved binary and macOS the resolved temporary
  directory.

### Witness 19

19. `TestCensusFindsAnUntaggedExecutableByRecord` (testutil, live): the test
    copies its own binary into `t.TempDir()` (a copied `/bin/sleep` is
    killed on macOS, exit 137) and starts the copy in a helper mode that
    blocks on a pipe, with `METASYSTEM_FIXTURE_OWNER` removed from its
    environment and no word in its argv, `Hold`-recorded. In the body,
    `FixtureSurvivors(key)` names exactly one process: the copy, by the
    probe's pid and exe, class `fixture-survivor`, carrier `record`.
    Negatives in the same test: the same copy started from a directory made
    with `os.MkdirTemp` outside the per-test directory is not named; a
    `fixture-owner` copied into a directory not named for the test is
    rejected and the process under it is not named. The body runs twice,
    once with `GOTMPDIR` unset and once with it naming a `go-tmp` directory
    under `TMPDIR`, so both layouts are proved in one run. Cleanup kills the
    copies without a word, and the custodian log is empty.

Witness 8's fourth process, "untagged whose directory's `fixture-owner` is
dead", is unchanged in substance: the directory is the test's own, under
any temporary root. Unit 2's table witness for the `?` scope gains two
cases for the tie: an unreadable process with the pgid of a certain survivor
that is not its leader is not returned; one whose pgid is the certain
survivor's pid is.

### Wording that changes

3.1: the key scan's rule, and `record` as the third carrier. 3.2:
`Fixture(t)` writes the record before registering its cleanup; step 3
reaches an untagged executable under the test's directory through the
record. 3.4: the end-of-file limit. 3.5: the record paragraph, location
`<per-test directory>/fixture-owner` under `GOTMPDIR` or `TMPDIR`, the walk
and the tie, the residual after Go removes the directory,
`unowned-in-cache` only where `GOTMPDIR` names a `go-tmp` directory.
Revision 4's `?` scope: "led by" stands, with the tie spelled out. Section
6: the four witness rules, witness 19, the cases above. Unchecked, new
bullet: whether Darwin still reports the executable path of a process whose
binary Go's cleanup has removed; if not, that process is reached by its tag
alone.

### Units

Unit 5 gains a slice after 5c, 5d, about 140 lines:
`internal/identity/fixture_survivors.go` (the record's walk to the root,
the directory-name tie, the `record` carrier, the scope narrowed to the
leader's pid), `internal/identity/fixture_survivors_test.go` (the tie's
cases, the record under a plain temporary root),
`internal/testutil/fixture.go` (the record before the cleanup
registration), `internal/testutil/fixture_test.go` (witness 19). The count:
about 40 for the walk, tie and carrier, about 10 for the scope and the
order, about 90 for witness 19 with its two layouts and negatives and the
table cases. The slices before it keep their numbers; every other unit is
unchanged. No run's environment changes, so nothing here waits on Wido.
