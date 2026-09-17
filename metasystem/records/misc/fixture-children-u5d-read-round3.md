# Read: fixture-children unit 5d, round 3 (the table helper's directory, and the unit as a whole)

Reader: independent. I did not write this change and did not read its earlier rounds. I started at 00:36:53 and finished
at 00:45:12.

- The diff's sha256 is `745c12e1...ee982302`, which matches `fcu5d-r3-diff.sha256`.
- `git diff --numstat 94a3a174c 3e4607b24 -- metasystem` gives 27/13, 62/28, 25/4 and 141/0, which is 300 changed lines.
- `git diff 4c8c89307 3e4607b24` changes only `fixtureRecordExecutable` and removes the blank line above it.
- The four worktree files match tree 3e4607b24 byte for byte. The sha1 of every `.go` file in identity and testutil
  was the same before and after my read.
- Every mutation ran through `go test -overlay`, with its replacement file in my own scratchpad (`.../1e0f004f-.../scratchpad/r5d3`).
  I removed that directory at the end.
- Toolchain: go1.27.1 darwin/arm64. I did not run anything on the Linux VM.

## Material findings

None.

## The checks, by brief item

1. **U5d-R2-1 is fixed.** Each run below is `go test -race -count=3 -run 'TestFixtureScan|TestFixtureSurvivors|TestCleanupIsScoped' ./internal/identity/`, and all passed:
   - as given (`TMPDIR=/var/folders/.../T/`, `GOTMPDIR` unset);
   - `GOTMPDIR=<scratch>/lay/go-tmp`;
   - `TMPDIR=<scratch>/lay/go-tmp` with `GOTMPDIR` unset;
   - `GOTMPDIR=<scratch>/lay/other-tmp`.

   With `TMPDIR=/tmp` and `GOTMPDIR` unset, `-v` showed all five subtests passing.

   Why each case passes, in every layout:
   - The helper's directory is now always `/tmp/<prefix><digits>`. `goTmpRoot` never finds a `go-tmp` ancestor there.
   - So in "wrong directory" the child is unreadable, but the dead-owner scan does not return it. That holds whatever
     `GOTMPDIR` or `TMPDIR` names.
   - The case's zero now comes only from the record being rejected. Dropping the directory-name check fails
     `wrong_directory` at `fixture_survivors_test.go:166` in both the plain and the `go-tmp` layout. The failure is a
     `fixture-survivor` result, where the case wants none.
   - "Plain temporary root" is a plain layout in every environment again, so N-R2-2's identity-level gap is closed.
   - On Linux `/tmp` is an ordinary directory, and `fixtureExact` sets ticks and boot id. Nothing in these cases depends
     on the platform. The seat's VM runs (`check-5d-r3.out`) passed x3 in both layouts. I did not repeat them.

2. **The helper.**
   - **Symlink.** The record walk is lexical: `filepath.Dir` on the path it is given, then `os.ReadFile` at each level.
     - Given `/tmp/TestRecordN/bin/metasystem`, it reads `/tmp/TestRecordN/bin/fixture-owner` (ENOENT, because `bin` is
       never made) and then `/tmp/TestRecordN/fixture-owner`. The kernel resolves `/tmp` to `/private/tmp` on open.
     - Given the resolved spelling, the steps are the same. That is the spelling a real macOS probe reports.
     - I ran a mutation that passes the helper's directory through `filepath.EvalSymlinks`. All cases passed, in the
       plain layout and under `GOTMPDIR=.../go-tmp`. No case depends on the symlink.
     - Under `TMPDIR=/tmp`, Go's own `t.TempDir()` calls `os.MkdirTemp("/tmp", <cut name>)`. The helper makes the same
       call with the same prefix.
   - **Long names.** `os.MkdirTemp` appends a decimal random number when the pattern has no `*`, which is exactly what
     `t.TempDir()` does. So the two literals (64 bytes, and 63 bytes with the half rune dropped) are followed by
     digits, as Go builds them. The seat's M1 and M2 still catch both cases.
   - **Concurrent runs.** They cannot collide. `os.MkdirTemp` retries with a new random name on `EEXIST`, and every
     case gets its own directory.
   - **Killed runs.** A killed run leaves its directory behind.
     - I ran a mutation that SIGKILLs the test binary inside the helper, after `t.Cleanup` is registered. It left
       `/private/tmp/TestRecord3171530193/fixture-owner`, which I removed.
     - A `-timeout` panic also skips cleanups. `t.TempDir()` has the same exposure; its leftovers went to `TMPDIR`
       instead of `/tmp`.
   - **Can a leftover record change another scan?** No.
     - A record is read only for an executable below its own directory. That directory has a random name, and nothing
       starts executables in it.
     - Its key names the synthetic owner pid 700, started at Unix 100. No real key scan can match it, and no real
       dead-owner scan can either.
     - The only other `fixture-owner` reader in the tree is `fixtureOwnershipRecord` itself.
     - A leftover costs one small directory per killed run, and it is harmless.

3. **Fit.**
   - This round's line comes from deleting the blank line between `TestFixtureScanUsesOwnershipRecordWhereTestRan` and
     `fixtureRecordExecutable`. gofmt accepts that and the gate passed. It is cosmetic.
   - No removed line in the whole unit diff is a comment (0 `-` lines containing `//`).
   - The base assertions the unit removed are covered elsewhere:
     - Base's "untied go-tmp scan" negative is covered by pid 703 in `TestFixtureSurvivorsRequiresUnreadableProcessToBeLedByCertainSurvivor`.
       That process is unreadable, under `go-tmp`, and shares the certain child's group without being led by it. The
       key scan still must not return it.
     - Round 2 folded the key-scan record negative into the dead-owner table. Witness 19's `WrongDirectory-` copy still
       covers it live.
   - I found no removed witness, assertion or needed comment.

4. **Whole unit.** Nothing material. See N-R3-1 to N-R3-3.

5. **Mutations.** See the table below. Three were not run by the seat, Codex or round 2.
   - Walk continues past a rejected record: not caught (N-R3-2).
   - Directory-name check removed: caught.
   - Resolved `/tmp` spelling: passes, as it should.

6. **Survivors.** None. See the end.

7. **Comments.**
   - This round adds none.
   - The unit's one source comment is plain English about the code. N-R2-4, carried, still applies.
   - The doc comments on `FixtureSurvivors` and `FixtureSurvivorsOfDeadOwner` still say "ties them ... by process group
     or session". That remains true under the led-by rule.
   - No comment refers to units, rounds, findings, reviews or amendments.

## Mutations I ran

| Mutation | Run | Result |
|---|---|---|
| None | identity tables `-race -count=3`, four layouts | ok in all four |
| None | `TestFixtureScanUsesOwnershipRecordWhereTestRan -v`, `TMPDIR=/tmp`, `GOTMPDIR` unset | ok, all five subtests pass |
| Record accepted without the directory-name check | identity tables, plain and `GOTMPDIR=.../go-tmp` | caught in both: `wrong_directory` at `:166` returns a certain survivor |
| Walk goes on past a rejected record (the rejected record no longer ends the walk) | identity tables, plain and `GOTMPDIR=.../go-tmp` | **not caught** in either layout (N-R3-2) |
| Helper directory resolved with `filepath.EvalSymlinks` (`/private/tmp/...`) | identity tables, plain and `GOTMPDIR=.../go-tmp` | ok in both, so no case depends on the `/tmp` spelling |
| Test binary SIGKILLs itself inside the helper | `TestFixtureScanUsesOwnershipRecordWhereTestRan` | `signal: killed`; left `/private/tmp/TestRecord3171530193/fixture-owner` (removed) |
| Writes to `/private/tmp` denied by `sandbox-exec`, everything else allowed; `TMPDIR=<scratch>/lay/tmp`, `GOTMPDIR=<scratch>/lay/go-tmp` | identity tables, this unit | all five subtests fail: `mkdir /tmp/<prefix><digits>: operation not permitted` (N-R3-1) |
| Same sandbox | identity tables, base tree's identity files | ok |
| Same sandbox | whole identity package, this unit and base tree | both FAIL: `TestLauncherDeathKillsTheTest` and `TestCustodianWatchesTheExportedLauncher` time out at 30 s in both. The unit also fails the table test |
| Same sandbox | `TestLauncherDeathKillsTheTest -v`, this unit | the child's testenv fails with `create registry home: in /tmp: ... operation not permitted; in TMPDIR: mkdir /tmp/.metasystem-test-registry-...: operation not permitted` (the witness gives its child no `TMPDIR`) |
| Same sandbox | whole testutil package, base tree's testutil files | ok |
| Same sandbox | whole testutil package, this unit | FAIL: `TestFixtureCleanupReadsOwnershipRecordBeforeTempDirRemoval` at `fixture_test.go:107`, `mkdir /tmp/...: operation not permitted`. Witness 19 passes |

## Non-material notes

- **N-R3-1. Both literal `/tmp` directories fail where `/tmp` cannot be written.**
  - **Which directories.** The identity table helper's `os.MkdirTemp("/tmp", ...)` is this round's. The testutil order
    witness's `os.MkdirTemp("/tmp", t.Name())` has been in the unit since its first round.
  - **Where `/tmp` cannot be written.** The Claude delegate sandbox:
    - Claude Code 2.1.273's always-writable list is `/dev/*`, `/tmp/claude`, `/private/tmp/claude`, `~/.npm/_logs` and
      `~/.claude/debug`.
    - `BuildClaudeSettings` adds only the write roots and the adapter's scratch directory.
    - `internal/testenv` already handles this sandbox: `createRegistryHome` falls back to `TMPDIR` when `/tmp` is
      denied (c15d23bef).
  - **What changes in my simulated sandbox.**
    - testutil is green on the base tree and red on this unit.
    - identity's table test goes red too. But the identity package is already red there on the base tree and on main:
      the launcher witnesses start their child without `TMPDIR`, so testenv's fallback lands on `/tmp` as well.
  - **Why I do not call it material.**
    - The landing gate and every current proof path can write `/tmp`: the seat, Codex's workspace-write sandbox, job
      worktrees and the VM.
    - The package this round touched is already unprovable in that sandbox on main.
    - My sandbox allows everything except writes to `/private/tmp`. The real Claude profile starts from
      `(deny default)`, so I have not shown that testutil is green at base under the real profile.
  - **Suggestion.** Backlog it with the launcher witnesses' missing `TMPDIR`. A root that works in every layout would be
    the parent of the `go-tmp` directory when `t.TempDir()` is below one, and `t.TempDir()` otherwise. But revision 6
    and the fix brief also name `TMPDIR` below `go-tmp`, and that choice needs the seat's decision.
- **N-R3-2. "The first record found decides" is not pinned.**
  - Revision 6 says a rejected record means no record, and the walk does not go on. A mutation that keeps walking
    passes every identity case in both layouts.
  - The reason: "wrong directory" has no valid record above its rejected one. Witness 19's `WrongDirectory-` copy has
    none either: above it are the parent test's `001`, the parent's per-test directory (no record, because the parent
    never calls `Fixture`) and the temporary root.
  - A case with a rejected record inside an accepted directory would pin it. The code is right as built.
- **N-R3-3. `os.ReadFile` on every ancestor can block or grow without bound.**
  - The walk now reads `fixture-owner` at every level up to `/`. That includes world-writable `/tmp`, which every
    executable under `/tmp` passes through, among them Go's `go-build*` test binaries when `TMPDIR` is `/tmp`, as on
    the VM.
  - If `/tmp/fixture-owner` is a FIFO, or a symlink to `/dev/zero`, the census, the custodian scan and teardown step 3
    hang or keep allocating.
  - Base read records only inside a `go-tmp` subtree. Someone would have to create such a file on purpose, so this is
    not material.
  - An `Lstat` check for a regular file, or a bounded read, would close it.
- **Carried notes.** N-R2-1 (the 64 is pinned only from above), N-R2-3 and N-R2-4 are unchanged by this round. N-R2-2 is
  resolved at the identity level (item 1).

## Survivors

- My runs left no processes.
  - At 00:45:12 the only `identity.test` processes were pids 88066, 88080 and 88082. They were running
    `TestKilledTestBinaryLeavesNoFixtureChild|TestCustodianReapsStoppedAndDetached` from `./identity.test` and had
    started at 00:44:43, after my last run ended at 00:44:11. I never ran those tests, so they belong to another seat.
  - There was no `testutil.test`, `fixture-copy` or `sandbox-exec` process.
- My runs left no `/tmp` directories.
  - No `TestRecord*`, `TestOwnershipRecord*`, `WrongDirectory*`, `TestFixtureCleanup*`, `TestCensus*`,
    `.metasystem-test-registry-*`, `metasystem-fixture-*` or `fixture-owner` entry newer than my start was in
    `/private/tmp` or my `TMPDIR`.
  - The one directory my kill mutation left, `/private/tmp/TestRecord3171530193`, I removed.
  - The sandboxed runs could not create anything in `/tmp`.
- I removed my scratch directory `r5d3`, which held the layouts, overlays and logs.

VERDICT: LAND
