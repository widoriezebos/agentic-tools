# Read: fixture-children unit 5d, round 2 (long top-level test names)

Reader: independent. I did not write this change and did not read its first round. I started at 00:15:37.

- The diff's sha256 is `53e09654...ee7c89`, which matches `fcu5d-r2-diff.sha256`. `git diff --numstat 94a3a174c 4c8c89307 -- metasystem`
  gives 27/13, 61/27, 25/4 and 141/0, which is 298 changed lines. The fix round changed 2/1, 25/25 and 15/15 lines.
- The worktree's `fixture_survivors.go` and `fixture_survivors_test.go` match tree 4c8c89307 byte for byte. The
  sha1 of every `.go` file in identity and testutil was the same before and after my read.
- Every mutation ran through `go test -overlay`, with its replacement file in my own scratchpad
  (`.../1e0f004f-.../scratchpad/r5d2`). I made the layout directories there and removed them afterwards.
- Toolchain: go1.27.1 darwin/arm64.

## Material findings

### U5d-R2-1. The "wrong directory" table case fails whenever the test's temporary directory is below a directory named `go-tmp`

The fix round changed two things in `TestFixtureScanUsesOwnershipRecordWhereTestRan`:

- `fixtureRecordExecutable` now makes its directory under `t.TempDir()`. Before, it used `os.MkdirTemp("/tmp", ...)`.
- "wrong directory" now runs through `FixtureSurvivorsOfDeadOwner`. Before, it ran through `FixtureSurvivors(key)`.

The table child is `fixtureExact(701, 71)` with only `Exe` set. Its argv and environment are unknown and it is
signalable, so a rejected record puts it on the unreadable list. For an unreadable process whose executable is under
`go-tmp`, the dead-owner scan returns it as `fixture-survivor?` (`includeGoTmpUnreadable && ExeKnown && underGoTmp`).
With `t.TempDir()` below `GOTMPDIR=<dir>/go-tmp`, the executable is under `go-tmp`. The case then gets one result
and wants none.

Evidence. Each run used `-run 'TestFixtureScan|TestFixtureSurvivors|TestCleanupIsScoped' ./internal/identity/` on the
unit as built:

- With `GOTMPDIR=<scratch>/lay/go-tmp`, the test fails. `--- FAIL: .../wrong_directory` at `fixture_survivors_test.go:166`:
  `record-backed scan = []identity.FixtureSurvivor{{Class:"fixture-survivor?", Ref:{Pid:701 ...},
  Exe:".../lay/go-tmp/TestFixtureScanUsesOwnershipRecordWhereTestRanwrong_directory1715069206/001/WrongDirectory123/bin/metasystem" ...}}, <nil>; want 0 certain record survivors`.
- The same run with `-race -count=3` fails 3 times out of 3. The failure does not depend on timing.
- With `TMPDIR=<scratch>/lay/go-tmp` and `GOTMPDIR` unset, it fails the same way.
- With `GOTMPDIR=<scratch>/lay/other-tmp` it passes, and with `GOTMPDIR` unset it passes. The directory name `go-tmp`
  is what breaks it.
- Round 1's test file (`git show e9e3065c7:.../fixture_survivors_test.go`) passes every case against this unit's source
  with `GOTMPDIR=<scratch>/lay/go-tmp`. The fix round introduced the failure.

This layout is used in practice. `scripts/agents/adapters/claude.sh:150-152` exports `GOTMPDIR="$scratch_dir/go-tmp"` for
every Claude delegate. `job_build_cache_env` (`scripts/agents/adapters/runtime-common.sh:557-569`) exports
`GOTMPDIR=<git dir>/metasystem-build-cache/go-tmp` for job worktrees, and `claude.sh:155` and `codex.sh:157` use it.
Revision 6 names both layouts. The seat's check ran the identity tables only with `GOTMPDIR` unset: in
`check-5d-r2.out`, only the testutil witnesses ran "GOTMPDIR under TMPDIR". Codex built in a scratchpad worktree that
`job_build_cache_env` does not match. That is why neither of them saw the failure.

Why this is material: once the unit lands, anyone who runs `go test ./internal/identity/` or the full gate from a Claude
delegate or a job worktree gets a red package they did not cause. That includes the standard proof lines in these build
and read briefs. The unit cannot be proved green in a layout the design names. The code is right, because a dead
owner's unreadable process under `go-tmp` belongs to the existing `?` class. The test is wrong: this case's zero
depends on where the test runs, not only on the directory name. That is the question in brief item 3.

Two ways to fix it (I applied neither):
- Give the table child a readable argv with no tag word, for example
  `child.Argv, child.ArgvKnown = []string{"metasystem"}, true`. A rejected record then leaves it off the unreadable
  list in every layout.
- Or root `fixtureRecordExecutable` outside any `go-tmp` again, as round 1's `os.MkdirTemp("/tmp", ...)` with its
  cleanup did. That costs one line, for 299 in total. It also makes "plain temporary root" a plain-layout case in
  every environment again (see N-R2-2).

Either way, the proof should include the identity tables run with `GOTMPDIR=<dir>/go-tmp`.

## The checks, by brief item

1. **U5d-1 is fixed.** The check now spells the top-level name the way Go 1.27.1 does.
   - `makeTempDir` (`testing.go:1584-1613`) cuts the whole `c.Name()` to 64 bytes first. Then
     `removeSymbolsExcept` keeps `unicode.IsLetter`, `unicode.IsNumber` and `!#$%&()+,-.=@^_{}~ `, and drops a half
     rune as `RuneError`.
   - A Go identifier holds only `unicode.IsLetter` runes, `_` and `unicode.IsDigit` runes, which are a subset of
     `IsNumber`. Go therefore removes nothing from a top-level name. Removing symbols after the cut cannot move the
     prefix.
   - When the top-level name is 64 bytes or longer, the first 64 bytes of the full name are the first 64 bytes of the
     top-level name. When it is shorter, the name's `/` and any dropped runes in a `t.Run` name come after the
     prefix. So cutting the top-level name alone is right.
   - `strings.ToValidUTF8(s, "")` removes the trailing partial rune exactly as the `strings.Map` mapper does.
   - I checked all of this against the real toolchain with a scratch module:
     - `TestOwnershipRecordWithATopLevelNameThatContinuesBeyondSixtyFourBytesForFixtureDiscovery` (88 bytes) and its
       `/subtest` both got `TestOwnershipRecordWithATopLevelNameThatContinuesBeyondSixtyFour<random>`.
     - `...BeforeALetteréSuffix` (71 bytes) and its `/subtest` both got `...BeforeALetter<random>` (63 bytes).
     - `TestΩmega٣Letters_ǅ` (an Arabic-Indic digit and a titlecase letter) kept every rune.
     - A 94-byte benchmark and its `b.Run` child both got the 64-byte cut, so `Fixture(b)` holds too.
     - `FuzzShortName/seed#0` has the top-level name `FuzzShortName`.
     - `TestDup` defined in both the internal and the external test package was named `TestDup` in both, with no `#01`.
   - The comment's claim is true for every test that `go test` generates. See N-R2-4 for a nit.
2. **The new cases** are string literals written by hand, not built by the code under test.
   - Byte counts: the long name is 88 bytes and its directory literal is 64 bytes. In the multi-byte name, `é` is at
     byte offsets 63 and 64 (`b[62:66] = 'r' c3 a9 'S'`), so the 64-byte cut keeps only `c3`. The directory literal
     is the 63 bytes before it.
   - Both literals match what Go 1.27.1 actually made, as shown in item 1.
   - Against round 1's source (overlay of `e9e3065c7:.../fixture_survivors.go`), `long_top-level_name` and
     `multi-byte_letter_at_limit` both fail, and the other cases pass. The seat's M2 fails only the multi-byte case.
3. **The table rewrite** has the defect in U5d-R2-1.
   - The positive cases still assert pid, class `fixture-survivor` and carrier `record`. Only the failure message was
     merged.
   - The identity tables no longer have a key-scan case that rejects a record by its name. The live witness 19 still
     has one: `FixtureSurvivors(fixture.Key())` against the `WrongDirectory-` copy, which is not named. So no witness of
     the key scan on a record was lost overall.
   - "tag decides before record" still runs the key scan, and its child has a known environment, so it does not
     depend on the layout.
   - "plain temporary root" passes for its stated reason only when no outer `go-tmp` exists (N-R2-2).
4. **N4.** Process 704 is `fixtureExact(704, 74)`: no argv, environment or executable, with the default scope
   `pgid 704, sid 804`. So it is unreadable and leads its own group, and no certain survivor leads it.
   - I ran the read's mutation (unreadable entries passed to `ledByFixtureSurvivor` as leaders). It fails
     `TestFixtureSurvivorsRequiresUnreadableProcessToBeLedByCertainSurvivor` at `:143`, "want certain pid 701 and
     unreadable pid 702 only".
   - The case pins only an unreadable process leading itself (N-R2-3).
5. **N14** is only a rename. All 15 changed lines in `fixture_test.go` between e9e3065c7 and 4c8c89307 are
   `copy` becoming `processCopy`. Nothing else changed in that file.
6. **Size.** 298 lines, recounted. The fix removed no comment and no positive assertion.
   - It folded the key-scan negative into the dead-owner table, which the live witness still covers.
   - It replaced `os.MkdirTemp("/tmp", ...)` and its cleanup with `t.TempDir()`, saving one line. That change is the
     cause of U5d-R2-1.
7. **Mutations** are in the table below.
8. **Survivors** are listed below.
9. **Comments.**
   - The one new comment is plain English and describes the code. It has no references to units, rounds, findings,
     reviews or amendments.
   - The test files add no comments.

## Mutations I ran

| Mutation | Run | Result |
|---|---|---|
| None (baseline) | identity tables, `GOTMPDIR` unset / `GOTMPDIR=.../go-tmp` / `GOTMPDIR=.../other-tmp` / `TMPDIR=.../go-tmp` | ok / **FAIL `wrong_directory`** / ok / **FAIL `wrong_directory`** (U5d-R2-1) |
| None (baseline) | live testutil record witnesses, `GOTMPDIR` unset / `.../go-tmp` | ok / ok |
| Round 1's test file, this source | identity tables, `GOTMPDIR=.../go-tmp` | ok (the regression is new) |
| Round 1's source (uncut name), these tests | identity tables | caught: `long_top-level_name` and `multi-byte_letter_at_limit` fail |
| Records read only below `go-tmp` (the base walk rule) | identity tables, `GOTMPDIR` unset | caught: `plain_temporary_root`, `long_top-level_name` and `multi-byte_letter_at_limit` fail |
| Same | identity tables, `GOTMPDIR=.../go-tmp` | not caught: only `wrong_directory` fails, and it fails without the mutation too |
| Same | round 1's test file, `GOTMPDIR=.../go-tmp` | caught: `plain_temporary_root` at `:157` |
| Same | live testutil, `GOTMPDIR` unset | caught: witness 19 plain leg at `fixture_test.go:87`, and the order witness at `:124` |
| Same | live testutil, `GOTMPDIR=.../go-tmp` | caught by the order witness at `:124` (its `/tmp` literal); witness 19 passes (N1) |
| Cut to 63 bytes instead of 64 | identity tables and live testutil | not caught (N-R2-1) |
| Unreadable entries admitted as leaders | identity tables | caught at `:143` |
| Unreadable entries admitted as leaders, but a process never leads itself | identity tables | not caught (N-R2-3) |

## Non-material notes

- **N-R2-1. The 64 is pinned from above, not from below.** A cut at 63 bytes passes every identity case and the live
  witnesses. A cut at 65 or more would fail both new cases.
  - A shorter cut only loosens the check a little, because the directory must still begin with the name.
  - A negative case would pin the lower bound: a directory that matches the name's first 63 bytes but not its 64th.
- **N-R2-2. Round 1's N1 is now wider, but the rule is still pinned in every layout.**
  - Under an outer `go-tmp`, the identity case "plain temporary root" now runs below `go-tmp` as well. So in that
    layout neither it nor witness 19's plain leg proves the plain layout.
  - `TestFixtureCleanupReadsOwnershipRecordBeforeTempDirRemoval` makes its per-test directory with a literal `/tmp`.
    It still catches "records read only below `go-tmp`" with and without `GOTMPDIR`.
  - The second fix option in U5d-R2-1 would restore the identity-level pin as well.
- **N-R2-3. The N4 case pins only an unreadable process leading itself.** An unreadable process led by a different
  unreadable process is not in any table. The code is right.
- **N-R2-4. Comment wording.**
  - "makeTempDir" names an unexported function in the testing package. `t.TempDir` is the name a reader can look up.
  - The claim covers every test that `go test` generates. A hand-written `testing.MainStart` list could hold any
    name, which is not a supported case here.

## Survivors

- My runs left no processes. At 00:22:05 there was no `fixture-copy`, `testutil.test`, `identity.test` or `goname.test`
  process, and no process with `METASYSTEM_FIXTURE_OWNER` in its command line.
- My runs left no `metasystem-fixture-*` directory and no `fixture-owner`, `TestCensus*`, `TestFixture*`,
  `TestOwnership*`, `WrongDirectory*` or `outside-*` entry newer than 00:15:30 under `$TMPDIR`, `/tmp` or my scratch
  directory. I removed my `lay` directory and my scratch Go module.
- Older entries that are not mine: `/tmp/m1b-harness.{TKFQVl,zizXPx,PEINmH,l7ZdVk}/metasystem-fixture-probe.*`, last
  modified Sep 14 22:31-22:34, from another harness.
- None of the 8 orphaned shells from the seat's census is running now. The stray `fcu5c-layout` `fixture-owner` that
  round 1 named is also gone.

VERDICT: NOT LAND
