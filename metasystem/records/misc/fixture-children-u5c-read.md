# Read: fixture-children unit 5c (ownership record, exit scan, shell prologue)

Reader: independent build read. Tree: `g18/wt-fcu5c`, unit diff `fcu5c-r3.diff` (sha256 3538ea9b…860d291, matches the recorded sum). Measured against base tree 16e7ac75e with the brief's pathspec: testenv.go +68/-1, fixture.go +19/-1, fixture_test.go +142, testing.json +1/-1, plus the new fixture_exit_test.go at 67 lines. That is 300 changed lines, at the ceiling. I changed nothing in the worktree.

My runs were on m1c (go1.27.1 darwin/arm64). Every binary was built with `go test -c -o` into my own scratchpad, and mutants went in through `-overlay`. Output: `/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1c/1e0f004f-23d9-4653-add1-c3a768ed5661/scratchpad/fcu5c-read/run.out`. I did not use the VM.

## Material findings

### U5c-1. `TestFixtureWritesItsOwnershipRecord` fails whenever `GOTMPDIR` is set to something other than `TMPDIR`, which is the layout the repository's own adapters create

- The witness asserts `filepath.Dir(directory) == filepath.Clean(os.TempDir())`, where `directory` is the parent of `t.TempDir()` (fixture_test.go:46).
- In Go 1.27.1, `t.TempDir()` makes its directory with `os.MkdirTemp(os.Getenv("GOTMPDIR"), pattern)` (`$GOROOT/src/testing/testing.go:1613`). So when `GOTMPDIR` is set, the per-test directory's parent is `GOTMPDIR`, not `os.TempDir()`.
- The Claude adapter exports `TMPDIR="$scratch_dir"` and `GOTMPDIR="$scratch_dir/go-tmp"` (`scripts/agents/adapters/claude.sh:150-152`). A dispatcher job worktree gets `GOTMPDIR=<git dir>/metasystem-build-cache/go-tmp` and keeps its `TMPDIR` (`runtime-common.sh` `job_build_cache_env`). The revision-6 amendment records the same layouts. Under either one, the parent of the per-test directory is not `os.TempDir()`.
- Reproduced with the same binary and the three witnesses:
  - plain environment: PASS;
  - `GOTMPDIR=<mine>/gt/go-tmp` with `TMPDIR` unchanged: **FAIL** at `fixture_test.go:47: ownership record directory = ".../gt/go-tmp/TestFixtureWritesItsOwnershipRecord1717748466", want ... directly under "/var/folders/.../T"`. The other two witnesses pass;
  - `GOTMPDIR=TMPDIR=<mine>/gt/go-tmp`: PASS.
- The round-2 fix brief assumed "TMPDIR is the go-tmp directory". None of the repository's layouts does that. The seat's runs and the VM have no `GOTMPDIR`, so they cannot see the failure. Any Claude-adapter delegate or job-worktree round that runs `go test ./internal/testutil/` gets a red that no code defect causes. That includes whoever builds or reads 5d on top of this tree. This is the same class the brief calls defective: a test that its environment alone breaks.
- The code under test is right. Fix the witness only: take the expected root from `GOTMPDIR` when it is set, otherwise `os.TempDir()`. Or drop that half and assert what 5d's rule needs, that the record's directory is named for the test. The read at the expected path already proves the location (see note N1). About two lines, and it belongs in 5c.

## Answers to the brief

1. **Conformance.**
   - Parts 1 to 3 are built as the build brief and both fix briefs ask, and nothing else is.
   - Part 4, the key refusal witness, is **not** there. The unit sits at the 300-line ceiling.
   - `testing.json` changes one entry only: `test-environment-standard` gains `TestExitScanReadsNothingWithoutAMintedKey`, which is right for a testenv test. No inventory lists testutil tests by name (earlier fixture witnesses are not listed either), so the new testutil witnesses need no entry.
   - One deviation: the live witness re-runs `os.Args[0]`, not `os.Executable()` (N5).

2. **The ownership record.**
   - **When it is written.** It is written before `Fixture` returns (fixture.go:55), so before any child the caller starts. A failure to encode or write calls `t.Fatalf`.
   - **The witness.** It fails when the record is written anywhere else, because it reads `parent-of-TempDir/fixture-owner` and requires that to parse to the fixture's key. The seat's inside, outside and nonce mutants fail on that read or on the comparison. The `filepath.Dir(record) != directory` half is always false (N1), but the read makes up for it. The other half is U5c-1.
   - **Calling `Fixture` twice in one test.** Both keys are registered and both are scanned at exit. The record is overwritten, so only the second key is on disk, and each call makes an unused `t.TempDir()` subdirectory (N7).
   - **The two revision-6 facts, checked.**
     - `GOTMPDIR`: testing.go:1613 above, and claude.sh:150-152.
     - Cleanup order: testing.go:1615 registers the removal on the first `TempDir` call. `newProcessFixture` registers `fixture.cleanup` at fixture.go:85 (and the key at :86), before `Fixture` calls `t.TempDir()` at :55. Cleanups run last in first out, so the directory and record are removed before step 3 whenever `Fixture` makes the test's first `TempDir` call. When the test called `t.TempDir()` earlier, the record is still there at step 3. Both facts hold.
   - **Is landing sound?** Yes, apart from U5c-1.
     - From a shell the record is never read, because no ancestor is named `go-tmp`.
     - Under the adapter, it is read only for an untagged process whose executable sits below that test's own directory (fixture_survivors.go:121-127 handles tagged processes and `continue`s before the record read at :128-134). It then counts only for the exact encoded key, nonce included.
     - Trunk has no fixture code, and only testutil's witnesses call `Fixture`. So nothing is worse than trunk.
   - **The exit scan never relies on the record.** By the time `m.Run()` returns, every per-test directory is gone. The exit scan finds only tagged processes and unreadable processes tied to them.
   - **The 5d narrowing.** Under "led by", the exit scan's handling stays right. A non-certain entry is named, never signalled, and can appear only beside a certain survivor that already fails the binary. The narrowing only removes names.

3. **Registration.**
   - Guarded by a mutex, and the read returns a copy. Safe for parallel tests.
   - `m.Run()` is evaluated before `registeredFixtureKeys()`, because arguments are evaluated left to right, so keys from every test are seen.
   - **Keys from `recordingTB` with fake names.** They can only match a process carrying that exact encoded key, and `SignalExact` still re-proves identity before any kill. Their tagged children are killed and waited for by `killFixtureProcessAtCleanup`, so nothing is found at exit. An unreadable process is named only beside a certain survivor of that key. None of this can name or kill a stranger on its own.
   - `testenv` does not import `testutil` anywhere, tests included (grep is empty).

4. **The exit scan.**
   - **No key.** The loop does not run, nothing is read, and the code is returned unchanged. The custodian branch is untouched: the only line changed in `Main` is the `code =` line.
   - **Cases.**
     - A zombie is skipped.
     - A certain survivor is named, then `SignalExact(KILL)`, then a wait with step 2's rule. The wait's done-condition is equivalent to `waitForExits`.
     - Any other class is named with no signal.
     - A scan error is named.
     - A non-zero `m.Run()` code is kept.
   - **Under the ruling, what it names.** It still names:
     - children and grandchildren whose readable argv or environment carries the exact key;
     - unreadable signalable processes sharing a group or session with one of those.

     It misses:
     - an escaped child that is unreadable (mid-exec, mid-exit, another uid) with no certain survivor beside it;
     - untagged children that only the record could find, since the record is gone at exit.

     Before the ruling it would have named unrelated unreadable `go-tmp` processes. Its handling of a non-certain entry is still right: named, never signalled.
   - **The two key comparisons.** Both now compare `EncodeKey` strings. The seat's nonce mutant fails on host and VM. A string compare of two different encodings cannot pass.
   - **Probe error, or exit between scan and probe.**
     - A probe error means the survivor is named, and a certain one gets a `SignalExact` attempt that refuses. The wait returns silently at 5 s, but the binary already fails. Correct.
     - A survivor that exits between scan and probe is `Dead`, so it is named and the binary fails. That is consistent with rule 7 and with step 3: it was running after `m.Run()`. Neither path is silent when it should fail. Neither path is unwitnessed-and-wrong, but both are unwitnessed (N2).
   - **The table witness.** It proves:
     - no key: scan never called, code 7 kept;
     - certain survivor: one send, named, code 1;
     - zombie: nothing sent or named, code 0 kept;
     - scan error: named, no send, code 1;
     - unproven survivor: named, no send, code 1.

     It does not prove that a non-zero code is kept when something is also named, or what happens on a probe error (N2).
   - **Cost.** The seat measured about 60 ms for 1 key and 1.87 s for 40 keys on 1,048 processes, about 47 ms per key. That is fine now: only testutil mints keys, about a dozen per package run. It is linear in keys times process-table size, though, so a converted package with hundreds of fixture tests under `-count` would pay seconds to minutes. One table read matched against the key set should come before conversions. Not a defect in 5c (N3).

5. **The live exit-scan witness.**
   - **Descriptors.**
     - The witness keeps the only write ends of the input pipe and the release pipe (`os.Pipe` is close-on-exec). It closes its read ends and the output write end right after `Start`.
     - The helper has input on fd 3, release on fd 4, and output on fds 1 and 2. Fds 3 and 4 arrive through `dup2`, not close-on-exec, and `os.NewFile` does not set it. Go's fork does not close other descriptors, so the helper's `Shell` child inherits fds 3 and 4 and has stdin on the input pipe and stderr on the output pipe.
     - Readers do not delay end-of-file, so the inherited read ends do not matter.
     - The child holding the output write end means the witness's `ReadAll` ends only when both the helper and the child have exited. That is why the seat's `noscan` mutant ends on the 30 s read deadline with `output="N\nPASS\n"`, not on the exit assertion. It still fails, with nothing left behind.
   - **Load.**
     - The helper cannot reach its exit scan before the witness probes, because it blocks on fd 4 until the witness closes the release write end.
     - The 30 s read deadline and the 30 s helper wait are hang bounds for sub-second work.
     - `waitForFixtureExit(t, child)` has a 30 s bound.
     - No bound is one that load alone can break. My stress run (6 processes at count 15 of the three witnesses, 19 s) had 6 of 6 rc=0. The seat's host stress (18 processes) and VM stress (8 processes) were also rc=0.
   - **Failure part way.** The deferred `SignalExact` kills re-prove identity. The deferred close of the input writer releases the child even if its helper was killed first. On a binary timeout, process death closes every write end: the child reads end-of-file and exits, and the helper leaves its release read. The helper may then die of SIGPIPE writing its name line, but the child exits on its own. The deferred kills do not wait, so a killed helper stays a zombie until the witness binary exits (N6).
   - **The helper's `testenv.Main`.** It inherits the witness's environment after `prepare`, so `METASYSTEM_FIXTURE_CUSTODIAN_START` is already cleared and no custodian starts. The custodian branch needs `METASYSTEM_FIXTURE_CUSTODIAN=1`, which a test-running parent cannot carry. Nothing changes what the witness proves.
   - **The match.** It is exact: `test="TestBinaryExitScanHelper" pid=<n> ` with a trailing space, so another pid with the same prefix cannot match. The helper must also exit non-zero.

6. **`ShellPrologue` and its witness.**
   - The constant matches the design page's prologue (lines 541-545) character for character, with the code-block indent removed and a trailing newline.
   - **Tagged case.** The printed arguments `a|b c|` prove the tag word was shifted off. `FixtureTag` reads argv before the environment, so carrier `argv-word` proves the word is in argv.
   - **Untagged case.** Output equal to the input, plus the last three argv words `[script, tag, a]`, prove no re-exec. An always-exec mutant would print the same line but fail the argv check.
   - **When argv is read.** Only after the script printed its line, which the final shell does after every exec (the macOS `/bin/sh` shim, then the prologue's own). The seat saw the same on the VM with dash.

7. **Mutations I ran** (table witness `TestExitScanReadsNothingWithoutAMintedKey`, each mutant applied exactly once through `-overlay`; the baseline passed):

   | Mutant | Result |
   |---|---|
   | the scan is called even with no key | FAIL `no_key`: `code=7 scans=1` |
   | a zombie is named | FAIL `zombie`: `named=true` |
   | the certain-only guard removed, so unproven survivors are signalled | FAIL `unproven_survivor`: `sent=1` |
   | scan error not printed | FAIL `scan_error`: `named=false` |
   | `if failed && code == 0` changed to `if failed` | **PASS** (survives) |
   | probe error treated like a zombie and skipped | **PASS** (survives) |
   | no wait after the kill | **PASS** (survives) |

8. **Timing and load.** See 5. The record and table witnesses have no timing. The prologue witness waits only on descriptors, with 30 s hang bounds. I found no assertion that load alone breaks.

9. **Existing behaviour.**
   - For a package that never calls `Fixture`, `Main` differs only in calling `exitScan` with an empty slice, which returns the code without reading anything. My no-key mutant shows the witness guards that.
   - `testenv` gains a zero-valued package variable and has no `init`.
   - `testutil` now imports `testenv`, and `testenv` does not import `testutil`, so there is no cycle.
   - 82 packages call `testenv.Main`. The 81 besides testutil never call `Fixture` and behave as before.

10. **Carried gaps.**
    - 5c closes none of them.
    - The key refusal witness is still missing.
    - M3b (bypass `SignalExact` at step 1) and M5 (step 2 removed) concern steps 1 and 2, which 5c does not touch. Whatever 5b's read found for them stands.
    - 5c adds its own counterparts: the three surviving mutants above.

11. **Survivors.**
    - I surveyed before, after the single runs, and after the stress: no process carrying a fixture tag or running `fixture.sh` or the helper, no `metasystem-fixture-*` directory in `TMPDIR`, no `fixture-owner` within depth 2 of `TMPDIR`, and my `GOTMPDIR` empty.
    - I deleted my binaries and mutant copies. About 44 KB of logs remain in my scratchpad.
    - Nothing on the VM, which I did not use.

12. **Comments and layout.**
    - The one new comment (`RegisterFixtureKey`) is plain and accurate. There are no unit, round or finding references.
    - The compaction folded conditions into one `if` and discarded errors that cannot occur in practice (`errors.Join` of three closes, `StdinPipe`, `StdoutPipe`). No assertion was weakened: every earlier check is still inside a combined condition that fails the test.

**The two items the seat already looked at.**
- **`TestExpectBounds` on the VM.** I agree. `expect_test.go` is unchanged against the base. The failure prints the host worktree path. The message comes from `fixtureSite`'s walk up from the source file's directory to `go.mod` (internal/testutil/expect_test.go:508-532), and that path does not exist on the VM.
- **The two leash shells.** I agree. The script is `TestCleanupIsScopedToTheFixtureKey`'s (fixture_test.go:376, 5b's code), which no 5c witness or mutant starts. In both survivor listings each shell's parent pid equals the owner pid in its own tag, so the owning test binary was alive: those were a running test's children, not orphans.

## Non-material notes

- **N1.** In `TestFixtureWritesItsOwnershipRecord`, `filepath.Dir(record) != directory` is always false: both are `filepath.Dir(t.TempDir())` computed in the witness. The location is proven by the read, so nothing is lost, but the half is dead.
- **N2.** Unwitnessed exit-scan paths, all correct in the code: a non-zero `m.Run()` code kept when something is also named; a probe error for a survivor; the wait after the kill (the exit scan's counterpart of 5a's M5).
- **N3.** The exit scan reads the process table once per registered key, about 47 ms per key here. Fine now; one read matched against the key set should come before fixture conversions.
- **N4.** The exit scan discards `SignalExact`'s error and is silent at its 5 s deadline. The survivor is already named and the binary fails, so nothing passes silently. Step 3, by contrast, reports these separately.
- **N5.** The witness re-runs `os.Args[0]` where the build brief said `os.Executable()`. It works because `go test` passes an absolute path.
- **N6.** The live witness never closes `outputReader` (one descriptor per run). Its deferred kills do not wait, so a helper killed on failure stays a zombie until the witness binary exits.
- **N7.** Calling `Fixture` twice in one test overwrites the record with the second key and makes an unused `TempDir` subdirectory per call. The page could say one record per test directory, last writer wins, or refuse a second call.
- **N8.** `ShellPrologue` and `exitScan` have no doc comment.
- **N9.** The seat's check output reports 333 tracked lines. Numstat with the brief's pathspec gives 233 tracked plus 67 new, which is 300. The 333 looks like a counting slip, not a larger unit.

VERDICT: NOT LAND
