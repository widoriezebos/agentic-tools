# Read: fixture-children unit 5d (record read wherever the test ran; "led by" ties an unreadable process)

Reader: independent. I did not write this change. Worktree files unchanged (sha1 of every identity and testutil .go
file before and after match; `git status --short` line count unchanged). All experiments ran through `go test -overlay`
with replacement files in my own scratchpad (`.../1e0f004f-.../scratchpad/r5d`).

Diff checked: sha256 cf7c6339...8687 matches `fcu5d-r1b-diff.sha256`; `git diff --numstat 94a3a174c e9e3065c7 -- metasystem`
gives 26/13, 62/28, 25/4, 141/0 = 299 changed lines, under the 300 ceiling.

## Material findings

### U5d-1. The directory-name rule rejects every record of a test whose top-level name is longer than 64 bytes

`fixtureOwnershipRecord` accepts a record only when `strings.HasPrefix(filepath.Base(directory), topLevelTest)`.
Revision 6 bases this on "which is how `t.TempDir()` spells it in every Go release". That is not true for the
toolchain go.mod requires. Go 1.27.1 `testing.(*common).makeTempDir` (`$GOROOT/src/testing/testing.go:1602-1613`):

    pattern := c.Name()
    // Limit length of file names on disk.
    pattern = pattern[:min(len(pattern), 64)]
    ...
    c.tempDir, c.tempDirErr = os.MkdirTemp(os.Getenv("GOTMPDIR"), pattern)

When the top-level function name is longer than 64 bytes, the directory name holds only its first 64 bytes, the
prefix check fails, and the record is rejected. Because the first record decides, the walk stops there, so the process
is not named at all.

Evidence (live, darwin/arm64, overlay test file in testutil that calls `Fixture(t)` and starts an untagged copy under
`t.TempDir()` with the witness's own `startUntaggedFixtureCopy`):

- 81-byte name, as built, `GOTMPDIR` unset: record present (`stat` ok), directory base
  `TestReaderLongTopLevelNameRecordIsReadByTheCensusWhenTheNameExce2780421186`,
  `FixtureSurvivors(key)` = `nil`. FAIL.
- Short-name control in the same run: the copy is named. PASS.
- 81-byte name, adapter layout (`TMPDIR=<dir>`, `GOTMPDIR=<dir>/go-tmp`), base-tree walk semantics (stop at `go-tmp`,
  no name check): the copy is named. PASS.
- Same layout, as built: `nil`. FAIL.

So this is both a hole in the new plain layout and a change of existing behaviour in the `go-tmp` layout: a
long-named test whose untagged child the base code found by record is no longer found. Nothing fails when this
happens. Step 3, the exit scan and the census just miss the process.

Scope today: no test outside testutil calls `Fixture(t)` yet, so no current run changes. But 123 of the 4,289 `Test`
functions in this module have names over 64 bytes (the longest is 87 bytes), and those tests are the ones that will
start using `Fixture(t)` next. Once that happens the net is off for them and nothing shows it. That makes the rule wrong
once it lands. The fix is small: compare against the key's top-level name cut to 64 bytes (the way `makeTempDir` cuts
it), add a table case with a top-level name over 64 bytes, and correct revision 6's "every Go release" sentence on the
page.

## Non-material notes

- **N1. Witness 19's "plain temporary root" leg is plain only when the outer run has no `GOTMPDIR`.** `root :=
  t.TempDir()` sits under the outer `GOTMPDIR` when one is set (the Claude adapter, a job worktree's chain cache, or
  `metasystem test run` passing either through). The "plain" leg then runs below a `go-tmp` directory and proves the
  `go-tmp` layout twice. Evidence: with mutation 1 (the walk stops at `go-tmp` again), `GOTMPDIR` unset fails the plain
  leg at `fixture_test.go:87` ("want one result"), but `TMPDIR=<dir> GOTMPDIR=<dir>/go-tmp` passes both legs. The
  identity table case (`/tmp` literal) catches mutation 1 in every environment, and the seat's host package run,
  host stress and VM plain runs had no outer `GOTMPDIR`, so the unit is still proved. The witness still does not live
  up to "both layouts in one run" everywhere. Suggested fix: make `root` outside any `go-tmp` (for example
  `os.MkdirTemp(os.TempDir(), ...)` with its own cleanup), or assert that the plain leg's per-test directory has no
  `go-tmp` ancestor.
- **N2 (was 5b's N4). A tagged process of another key tying an unreadable process is still not pinned.** No table has
  an other-key tagged leader. The code is right: `certain` holds only matching keys.
- **N3 (was 5b's N5). The session-only tie is still not pinned.** Mutation "drop the `sid` clause": every
  `TestFixtureScan|TestFixtureSurvivors` case passes. Both the scoped-unreadable case and the led-by case tie by pgid.
- **N4. "The leader must be certain" is not pinned.** Mutation "the tie also accepts a `?` entry as leader" (pass
  `certain` plus `unreadable` to `ledByFixtureSurvivor`): all table cases pass. The default table scope gives every
  process pgid 900 and sid 901, so no unreadable process leads itself. On a live table this mutation would let any
  self-led unreadable process tie itself, which is the key-scan failure the ruling removed. The code is right. A case
  with an unreadable process whose pgid equals its own pid and no certain leader would pin it.
- **N5. "First record decides even when rejected" is not pinned.** Mutation "a rejected record lets the walk go on":
  all table cases pass, because nothing above the wrong-directory record exists to be found. The rule itself is sound.
  The walk goes from the bottom up, so the nearest record wins, and a stray record high in the tree (`/tmp`, `$HOME`,
  a scratch root, or a `go-tmp` root) can never hide a per-test record below it. It only ends the walk for executables
  with no nearer record, and a name check rejects it anyway. Stopping is the conservative choice.
- **N6. The malformed-tag guard is not pinned.** Mutation "`containsFixtureTagWord` check disabled": all table cases
  pass. The "tag decides before record" case uses a valid tag of another key, which is handled one branch earlier. The
  code does what revision 6 says: the record is read only when no known slot holds any
  `METASYSTEM_FIXTURE_OWNER=` word. A process with both slots unknown but `ExeKnown` still reads the record and can be
  certain by it. That was the base behaviour under `go-tmp` too.
- **N7. The prefix rule is loose, and that does not matter.** `TestFoo` accepts a `TestFooBar123` directory. Records
  are written only by `Fixture(t)`, into its own per-test directory, with its own full key. The key scan compares full
  `EncodeKey` strings, and the dead-owner scan compares owners. `TestFoo` and `TestFooBar` in one binary share the
  owner, so the looseness can only move a process between tests of the same dead binary. The name check guards against
  records copied or placed outside a test's directory, and it does that.
- **N8. Where the `go-tmp` stop still decides.** Only in the dead-owner scan's unreadable case
  (`fixture_survivors.go:148`). `unowned-in-cache` is declared but not produced anywhere in this tree.
- **N9. Cost.** Measured live on m1c with an overlay test in identity. 862 pids and 859 untagged executables with
  known paths give 5,517 ancestor levels. The walk alone takes 8.8 to 10.9 ms per scan (about 1.8 µs per failed
  open). The base walk took 0.7 to 1.9 ms, doing file I/O only below `go-tmp`. A full key scan took 86 to 485 ms on the
  loaded machine, so the walk adds roughly a tenth of a quiet scan. The exit scan runs one scan per registered key, so
  a binary with 50 `Fixture` tests spends about 0.5 s in the walk out of about 5 s of scanning. Not a defect.
  A related note: the walk now opens `fixture-owner` in every ancestor of every running executable. An executable on
  a hung network mount would stall the scan, and a TCC-protected folder could prompt. Right now no process here runs
  from `~/Documents`, `~/Desktop`, `~/Downloads`, iCloud or `/Volumes`, and a permission error ends the walk with no
  record.
- **N10. Carried N7 from 5c (a second `Fixture` call overwrites the record).** Unchanged. It now matters in every
  layout. Untagged processes under the directory are reached only by the second key. The second fixture's cleanup runs
  first (last in, first out) and still names them, under the second fixture.
- **N11. Order on a real `testing.T`.** `Fixture` calls `t.TempDir()` (inside `newRecordedProcessFixture`, through
  `tempDir()`) before `registerProcessFixture`. Go registers the per-test directory's removal on the first
  `TempDir` call (`testing.go:1615`), so the removal runs after cleanup step 3, whether or not the test called
  `TempDir` earlier. The order witness registers its removal inside the same `tempDir` callback and runs recorder
  cleanups from last to first, the same order a real `testing.T` uses. I ran mutation 4 myself (cleanup registered
  before `tempDir()` and the write): the order witness fails at `fixture_test.go:124` with
  `cleanup failures "" do not contain "unrecorded fixture child found running at teardown"`. Step 3 found nothing
  because the record was already gone, which is the right reason. Witness 19 still passed under that mutation, as
  expected. On a setup failure before registration, the deferred `closeLeash` replaces the base code's cleanup-time
  close. The key is not registered for the exit scan in that case, and nothing was started.
- **N12. Witness 19 under load and on part-way failure.** It has no time bound. Ready and release go through pipes,
  and it takes pid and exe from the probe and compares keys by `EncodeKey`. A kill is registered on the real (sub)test
  `t` right after `Start`, before any step that can fail. The kill goes through `SignalExact` once the copy is probed,
  and through `Process.Kill` before that, which is safe because the child has not been waited. The helper has no
  children. The order witness's only real-time wait is step 3's own 5 s exit wait after SIGKILL, and it checks only
  `Contains`, so an extra timeout line would not fail it. The helper removes `METASYSTEM_FIXTURE_OWNER` but not
  `METASYSTEM_FIXTURE_LEASH`. That does not affect what the witness proves.
- **N13. Renamed witnesses.** `...ToBeLedByCertainSurvivor` covers a pgid-led process that is returned and a
  shared-but-not-led process under `go-tmp` that is not, and it now asserts carriers. The dropped second leg ("untied
  `go-tmp` process, no certain survivor") is covered, because pid 703 is under `go-tmp` and is excluded from the key
  scan. `...WhereTestRan` covers a plain root with the dead-owner scan and carrier `record`, a wrong directory, and a
  tag deciding before the record. The `go-tmp` record case moved to witness 19's live leg. I found no weakened
  assertion. `TestFixtureScanClassifiesScopedUnreadableProcess` now makes 701 the group leader, which is needed under
  "led by".
- **N14. Style.** No comments were added. The existing doc comments ("ties them to a certain result") still read
  correctly. There are no unit, round or finding references. The local variable `copy` shadows the builtin in
  `fixture_test.go`. Vet and staticcheck accept it, but a different name would read better.
- **What changes for existing runs.** Newly returned: untagged processes whose executable is under a per-test
  directory holding a matching, correctly named record, in layouts without `go-tmp` (terminal `go test`, the VM, and
  `metasystem test run` without `GOTMPDIR`). Also returned: records above the nearest `go-tmp`, such as nested
  `go test` runs inside a test's directory. No longer returned: unreadable processes that only share a group or
  session with a certain survivor that does not lead it, and, through U5d-1, records of tests with top-level names
  over 64 bytes, including under `go-tmp`. No process outside a test's own directory can be named or killed because of
  a record. A kill still needs `SignalExact` and a certain class.

## Mutations I ran (not run by the seat or Codex)

| Mutation | Run | Result |
|---|---|---|
| Name check uses the whole key test name, subtests included | identity tables | caught: `...WhereTestRan/plain_temporary_root` "want pid 701 as one certain record survivor" |
| Tie accepts a `?` entry as leader | identity tables | not caught (N4) |
| Session clause of the tie removed | identity tables | not caught (N3) |
| Rejected record lets the walk go on | identity tables | not caught (N5) |
| Malformed-tag guard disabled | identity tables | not caught (N6) |
| Walk stops at `go-tmp` (Codex's 1) | witness 19 live, outer `GOTMPDIR` unset / set | caught / not caught (N1) |
| Cleanup registered before the record (seat's 4) | order witness live | caught, for the right reason (N11) |
| Top-level test name over 64 bytes (not a mutation, a probe) | live testutil, both layouts | as built misses the process; base walk under `go-tmp` finds it (U5d-1) |

Unmutated baseline: `go test -count=1 -run Fixture ./internal/identity/` ok. `TestCensusFindsAnUntaggedExecutableByRecord`
and `TestFixtureCleanupReadsOwnershipRecordBeforeTempDirRemoval` pass as built with `GOTMPDIR` unset.

## Survivors

My runs left no processes (no `fixture-copy`, `testutil.test` or `identity.test`) and no `TestReader*`,
`TestCensus*`, `TestFixtureCleanup*`, `WrongDirectory*` or `metasystem-fixture-*` entries under `$TMPDIR` or `/tmp`.
I removed my `outer` layout directory. One stray record not from my runs:
`/var/folders/jg/c9g5tnhx0c519rr0xfmw86d40000gn/T/fcu5c-layout.RtNnAH/go-tmp/fixture-owner` (written 23:11:57 by
pid 61501, `TestFixtureWritesItsOwnershipRecord`, before this read started). It sits directly in a `go-tmp` directory
from another seat's 5c layout check. Under 5d it is rejected by name and only ends walks below that otherwise empty
directory. The seat may want to delete it.

VERDICT: NOT LAND
