# Read: fixture-children unit 5c, round 5 (scoped re-read)

I am a fresh reader and did not write this code. I changed nothing in the worktree.

What I checked the unit against:
- The diff `fcu5c-r5.diff`. Its sha256 is fa3cb9a6…73993b7, the same as the recorded sum and the same as `git diff 16e7ac75e 94a3a174c`.
- The worktree `g18/wt-fcu5c`. The round-5 fix brief's temp-index command gives tree 94a3a174c, so the worktree holds exactly the unit tree.

My runs were on m1c (go1.27.1 darwin/arm64), before the seat asked for no more builds or tests. I built test binaries with `go test -c -o` into my own scratchpad and put mutants in through `-overlay`. Everything I ran had finished before that request. I did not use the VM. Output is in `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/fc5r5/` (`summary.txt`, `mut/summary.txt`, and one `.out` per run).

## Material findings

None.

## Answers to the brief

### 1. Is U5c-1 fixed? Does any witness still depend on `TMPDIR` or `GOTMPDIR`?

Yes, it is fixed.
- The witness now reads `filepath.Join(filepath.Dir(t.TempDir()), "fixture-owner")` and requires the file to parse to the fixture's key, compared by `EncodeKey` strings.
- The parent of `t.TempDir()` is always the per-test directory, wherever Go puts it. So nothing in the witness names a temporary root.
- No witness in the unit mentions `os.TempDir()` or `GOTMPDIR` any more. `TestShellPrologueCarriesTheTagIntoArgv` writes its script into `t.TempDir()`, and it compares argv with that same path. Neither side depends on where the directory is.

I ran all four of the unit's witnesses 3 times in each of three layouts, from the package directory:
- plain;
- `GOTMPDIR=<mine>/gt/go-tmp` with `TMPDIR` unchanged;
- `TMPDIR=<mine>/tt` and `GOTMPDIR=<mine>/tt/go-tmp`.

The four witnesses are `TestFixtureWritesItsOwnershipRecord`, `TestBinaryExitScanNamesAChildThatOutlivedItsTest`, `TestShellPrologueCarriesTheTagIntoArgv` and `TestExitScanReadsNothingWithoutAMintedKey`. All passed in every layout, with rc 0 for both binaries in all three layouts. The seat's own record-witness runs on the host and on the VM agree.

### 2. Does the witness still fail for every wrong place and wrong content?

Yes, for every mutation that matters. I ran each of the mutants below once in all three layouts, 27 runs in all. Every one failed:

| Mutant in `Fixture` | Where it fails |
|---|---|
| record in `t.TempDir()` itself (`inside`) | `fixture_test.go:38`, no such file |
| record in the parent of the per-test directory (`grandparent`: the go-tmp root, or `TMPDIR` when `GOTMPDIR` is unset) | `:38` |
| record in `os.TempDir()` (`outside`) | `:38` |
| file named `fixture-owner.tmp` | `:38` |
| no record written | `:38` |
| test name changed in the key | `:43` |
| nonce changed | `:43` |
| trailing newline after the key | `:43`, parse error |
| empty record | `:43`, parse error |

These mutants would pass, by reasoning; I did not run them. None of them would make the record wrong where the record is used:
- **An extra copy written somewhere else as well.** The witness reads only one path. A stray copy at a temporary root has no ancestor named `go-tmp` with a test directory between, so today's reader never reaches it. 5d's rule that the directory must be named for the test rejects it.
- **A different file mode.** Neither the design nor revision 6 names a mode.
- **A spelling of the key that `ParseKey` accepts and re-encodes to the same string**, such as a zero-padded pid. The reader (`fixtureOwnershipRecord`) parses the file the same way, so the result is the same.
- **A wrong key minted inside the fixture itself.** Both sides of the comparison read `fixture.key`. The tag witnesses of earlier units cover minting. Checking the key is not this witness's job.
- **`Fixture` caching the first directory it ever wrote to.** This passes only when this witness is the first `Fixture` call in the binary. It is contrived.

The witness is also stricter than the reader. The newline mutant fails the witness, but `fixtureOwnershipRecord` would accept that record, because it trims whitespace. That costs nothing.

### 3. Did either fix round change anything outside this witness?

No.
- I compared `fcu5c-r3.diff` (the tree the last read saw) with `fcu5c-r5.diff`. Apart from the order of the files, the index line and the hunk header, the only difference is four removed lines: `directory := filepath.Dir(t.TempDir())` and the three-line `if … { t.Fatalf }` block.
- Between r4 and r5, the only difference is the removal of the `GOTMPDIR` root lines plus that same block.
- The import block of `fixture_test.go` is identical in r3 and r5, and `os` and `path/filepath` are still used.
- The seat's `fcu5c-r3-to-r5.diff` shows the same four lines.
- The last read's account of `testenv.go`, `fixture.go`, `fixture_exit_test.go`, the other witnesses and `testing.json` still describes this diff line for line.
- The untracked `fixture_teardown_test.go` is base content, not part of the unit: blob c4a6bae is the same in 16e7ac75e and in 94a3a174c.

### 4. Is the unit 300 changed lines or fewer?

Yes: 296. I used the round-5 fix brief's temp-index command, run from the worktree root, and it produced TREE=94a3a174c:

| File | Added | Removed | Changed |
|---|---|---|---|
| `testenv/fixture_exit_test.go` (new) | 67 | 0 | 67 |
| `testenv/testenv.go` | 68 | 1 | 69 |
| `testutil/fixture.go` | 19 | 1 | 20 |
| `testutil/fixture_test.go` | 138 | 0 | 138 |
| `testing.json` | 1 | 1 | 2 |
| **Total** | | | **296** |

### 5. Anything else material the last read could not have seen?

Nothing material. Notes follow.

## Non-material notes

- **R5-N1. The `testing.json` hunk has drifted from current main.** Main has moved since base 16e7ac75e: it now holds 4a, 4b and 5a, but not 5b, and it has new sections, including a changed `landing-command-standard` entry right after the hunk's context. Against `main:metasystem/testing.json`, `git apply --check` of the unit's `testing.json` hunk refuses with the default context and applies with `-C1`. The `test-environment-standard` entry itself is unchanged on main, so this is a mechanical landing matter. Land 5b first and expect to rebase the one-line hunk.
- **R5-N2. For 5d and revision 6, not 5c: long test names.** Revision 6 says a record counts only when its directory's name begins with the top-level test name, "which is how `t.TempDir()` spells it in every Go release". That is not true for long names. Go 1.27.1 `testing.go:1605` cuts the pattern to 64 bytes before it drops unusual characters. A top-level test name longer than 64 characters therefore gets a directory that does not begin with the full name, and 5d's rule as briefed would reject a legitimate record. The repository already has such names, for example `TestRenderQuestionTrimNoticeDoesNotClaimDroppedFactsWhenAllFactsRemain` at 70 characters. Those tests do not call `Fixture` today. 5d should compare against the first 64 bytes, or the page should say what it does.
- **R5-N3. Mutant records left behind.** The `outside` mutant writes `fixture-owner` into the real temporary root. The seat's r5 check left one at `$(getconf DARWIN_USER_TEMP_DIR)fixture-owner`; my run overwrote it and I removed it. The same mutant on the VM probably left `/tmp/fixture-owner`, and the seat should remove it. Revision 6 says only `Fixture(t)` writes these files. My own mutants left three records in my scratchpad: `fc5r5/tt/fixture-owner`, `fc5r5/gt/go-tmp/fixture-owner` and `fc5r5/tt/go-tmp/fixture-owner`. My call to remove those and my test binaries was stopped, so they are still there.
- **Earlier notes.** N1, the dead half of the directory check, is gone with that check. N9, the 333 count, no longer applies. N2 to N8 stand as the last read wrote them, including the three exit-scan mutants that survive: `if failed` without `code == 0`, a probe error skipped, and no wait after the kill.

## Survivors

- After all my runs, no process carried a fixture tag, ran `fixture.sh`, or was one of my test binaries.
- The real temporary root has no `fixture-owner` and no directory for any of the four witnesses.
- My `go-tmp` directories hold only the mutant records named in R5-N3.

## Runs not made

The seat stopped builds and test runs. I am not asking for any of these to decide the verdict. They would firm up points I settled by reading the code.

1. The unit's other witnesses under `GOTMPDIR` on the VM. The seat ran only the record witness there. From `metasystem/internal/testutil` on the VM, with the Linux test binary of tree 94a3a174c:
   `mkdir -p /tmp/r5read/go-tmp && GOTMPDIR=/tmp/r5read/go-tmp ./testutil-linux.test -test.count=3 -test.run '^(TestBinaryExitScanNamesAChildThatOutlivedItsTest|TestShellPrologueCarriesTheTagIntoArgv)$'`
   Expected: PASS and rc 0. Neither witness compares a path with a temporary root, and Linux argv is not resolved.
2. Removing stray mutant records.
   - On the VM: `ls -l /tmp/fixture-owner && rm -f /tmp/fixture-owner`. Expected: the file exists from the seat's `outside-plain` run.
   - On m1c: `rm -rf /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/fc5r5/*.test /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/fc5r5/mut/*.test /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/fc5r5/tt/fixture-owner /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/fc5r5/gt/go-tmp/fixture-owner /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/fc5r5/tt/go-tmp/fixture-owner`

VERDICT: LAND
