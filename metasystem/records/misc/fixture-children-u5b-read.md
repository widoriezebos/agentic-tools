# Build read: fixture-children unit 5b

Reader: independent build reader (did not write the unit).
Tree: wt-fcu5b at HEAD b9a97d465, base tree dc584d0fb9307c9a6d12252e2bd040693335a0ae
with 4a, 4b and 5a staged and 5b unstaged. The diff read is fcu5b-r4.diff, sha256
80a4b53557b061d0d1c90f58e741a0c79fe46a917b3495625b9d9ff165b3fb4d, which matches the recorded
value (fixture.go +94/-15, fixture_test.go +190).

Method: I read the diff, fixture.go, fixture_test.go, identity ref.go, identity.go,
identity_darwin.go, fixture_survivors.go, the design (sections 2, 3.2, 3.4, 3.5, witnesses 2
and 12, the revision-4 amendment), the witness-9 amendment, the seat's check-5b-r3/r4 outputs
and script, the 5a read records, and the seat's fifoprobe and stopprobe outputs. All mutations
ran as `go test -c -overlay` builds from the worktree. Nothing inside the worktree was created
or changed. My runs used TMPDIR and outputs under my own scratchpad (r5b/). I did not take the
testrun lock, did not run `metasystem test run`, and started no subagents. After the runs, no
process named my path and no `metasystem-fixture-*` directory was left under my TMPDIR. The
`read -r _` shells visible on the host at the end belong to another seat's TestBinaryExit*
run (other parents, not my path). I deleted my large test binaries afterwards.

## Material findings

### U5b-1. Step 3 fails a test on an unrelated go-tmp process caught mid-exec or exiting. Load alone can break witness 12 and every future Fixture caller.

`reapKeySurvivors` treats every survivor whose class is not `fixture-survivor` as a test
failure ("fixture survivor ownership unproven at teardown"). `identity.FixtureSurvivors(key)`
returns `fixture-survivor?` for any same-uid, signalable process whose argv and environment are
both unreadable and whose exe is under a directory named `go-tmp`. For that arm,
`scanFixtureSurvivors` never checks the key: the `underGoTmp` test in the unreadable loop has
no `matches(key)`. On both platforms, argv is briefly unreadable while a process execs or exits,
even though the exe is already known (evidence in the 5a round-2 read). Such processes are
routine on this machine:
- `scripts/agents/adapters/claude.sh:149-152` sets `GOTMPDIR=<scratch>/go-tmp` for every seat.
- The test runner builds under `metasystem-build-cache/go-tmp` (design 3.5).
- Every `go test` package binary and `go run` tool from any seat starts and exits under a
  `go-tmp` path, as the same user.

Reproduced. Four shell loops exec'd a copy of /usr/bin/true placed at
`r5b/go-tmp/b001/true`. Meanwhile, four processes ran
`TestCleanupIsScopedToTheFixtureKey -test.count=300` from an unmodified build of the worktree.
Three of the four processes failed (rc=1; one failure each, 0.20 to 0.23 s), at
fixture_test.go:228:

    fixture survivor ownership unproven at teardown: pid=N exe=".../r5b/go-tmp/b001/true" argv=[]

The same five witnesses passed 9 of 9 runs (6 plain at count=30, 3 race at count=10) without the
churn. So the failure comes only from what else runs on the machine and when. Under the binding
rule, a test that machine load or timing alone can break is defective.

The build follows the text. The witness-9 amendment says step 3 "fails by name on anything it
finds", and design 3.5 scopes `?` to go-tmp exes (a scope written for the census, where `?` is
only reported). The defect is the combination: a key-scoped teardown scan inherits a class that
is not scoped to the key. The fix belongs at that cause. For example, a key scan could count an
unreadable process only when it is tied to the key (same process group or session as a certain
survivor of this key, or a go-tmp ownership record for this key), or step 3 could stop failing
on `?` entries that nothing ties to the key. A re-probe, a retry or a wait before failing would
not fix it; it only moves the window. Whatever fix is chosen needs a witness (see U5b-3).

### U5b-2. Witness 12 accepts a zombie as "second alive, same identity", so it does not prove that the first cleanup left the second fixture's child alone.

fixture_test.go:221 checks
`err != nil || state != identity.Alive || !identity.SameIdentity(exact, secondRef)`. The prober
reports a zombie as Alive with `Exact.Zombie` set. The second shell has not been waited on at
that point, so if the first cleanup kills it, the check still passes.

Evidence:
- M4 (step 3 selects on the owner instead of the key; identity overlay) was caught only by the
  later error count at fixture_test.go:228, because step 3 names what it kills. The line 221
  check passed.
- M11: a package-level list of every recorded ref, which each cleanup SIGKILLs through
  `SignalExact` before step 1, so the kill is silent and crosses fixtures. With M11,
  `TestCleanupIsScopedToTheFixtureKey` passed 5 of 5 runs. The second cleanup then sees a
  zombie, returns silently, and nothing fails.
- The same M11 with the check changed to `... || exact.Zombie` failed 5 of 5 runs at
  fixture_test.go:222: "second fixture child changed after first cleanup: state=alive err=<nil>".
- The test-only change on unmutated code passed 5 of 5 runs.

So witness 12's scoping claim holds only for kills that step 3 announces. A cleanup that kills
the other fixture's recorded or held child silently goes unseen. The fix is one token on an
existing line.

### U5b-3. Three new step-3 behaviours have no witness, one of them a safety rule.

These mutations each left the whole testutil package green:
- M10, class rule off. Step 3 would SIGKILL `fixture-survivor?` processes and name them as
  unrecorded children. Together with U5b-1's scope, this would kill other seats' go-tmp
  processes. The design's "indeterminacy never acts" (design line 108) has no teardown witness.
- M9, scan error silent. `scan process fixture survivors: %v` removed.
- M8, zombie skip off. The pre-probe that skips a same-identity zombie was disabled.

`ProcessFixture.scan` is already injectable, and witness 2 already injects a prober and a
sender. Each behaviour is therefore one table case: an injected scan that returns a `?`
survivor (assert nothing sent and one "ownership unproven" failure), an injected scan error
(assert one failure), and an injected certain survivor that probes as a zombie (assert nothing
sent and no failure). M10 is the one that matters most. The fix for U5b-1 changes exactly this
rule, so its witness should land with that fix.

## Check items (brief order)

1. Conformance.
   - Amendment rule 7 is implemented as written: re-prove before the kill, a held child is
     killed silently, a finished child found running is named, and State is not a grace.
   - The leash is implemented as described: a FIFO in a `metasystem-fixture-*` MkdirTemp
     directory, exported as METASYSTEM_FIXTURE_LEASH, closed and removed after step 3.
   - Step 3 is implemented per the amendment.
   - Omitted doc comments and the omitted zombie table case are within what the build brief
     allows.
   - Deviation: the leash witness now closes the leash before releasing the shell (see item 5).
   - Design gap: U5b-1.
2. Step 1's held branch. M5 (held named as running) is caught by witness 12 and by witness 2's
   {held:true, sameProbes:2} case. The leash witness does not catch it.
3. Step 3.
   - Probe-error path: a failed pre-probe falls through to the class rule and to `SignalExact`,
     which reports the error loudly (up to three errors for one pid). That is acceptable.
   - Exit race: see the non-material notes.
   - Scope: U5b-1 (scan class not keyed) and U5b-2 (witness 12 blind to silent kills). M4 was
     caught only through the error count.
4. The leash.
   - Opening: the reader is opened O_RDONLY|O_NONBLOCK|O_CLOEXEC, then the writer
     O_WRONLY|O_NONBLOCK|O_CLOEXEC, then the reader is closed. Neither open blocks on either
     platform.
   - Close-on-exec: M7 (writer without O_CLOEXEC) fails the leash witness at its 30 s bound.
   - Close order and directory removal: `closeLeash` closes the writer, then removes the
     directory. `openFixtureLeash` removes the directory on every failure path.
   - Open hole and macOS lost end-of-file: both carry to the custodian as the design's safety
     (5d, gate off). Nothing uses the leash in production yet. See the notes.
5. The leash witness after the fix. It now proves that a leash read started after the close
   returns end-of-file, and that no child holds the writer (M7 catches that). It no longer
   proves that a shell already blocked on the leash is woken. M6 (leash closed before step 1)
   passes everything at count 3.
6. The key scan witness's release pipe.
   - macOS /bin/sh, checked with lsof: the grandchild has fd 0 on the held stdin pipe (the same
     pipe as the shell's fd 4), fds 1 and 2 on /dev/null, and no fd 3 or 4. The shell's fd 0
     showed the release pipe only because `read <&3` temporarily dups fd 3 onto fd 0.
   - dash: I did not check it myself. The seat's VM runs (r4, 8 of 8) are the evidence.
   - Early exits are covered by the deferred `SignalExact` kill of the grandchild and by
     `killFixtureProcessAtCleanup`.
   - Witness 17 never sends SIGCONT, so a lost SIGCONT cannot hang it. Its stop-then-SIGKILL
     lost nothing in the seat's 9,600-trial stopprobe; that residual risk sits in 5a code.
   - The seat's two mutations are consistent with the code: the delay mutation passes, and
     norelease fails at the 30 s bound. The seat ran them; I did not rerun them.
7. Failure-path cleanups.
   - `openFixtureLeash` removes its directory on every error.
   - The test helpers register the kill before `closeLeash`, so the kill runs first.
   - On a test timeout or a Fatalf in a subprocess, the `metasystem-fixture-*` and TempDir
     directories remain (see the notes).
8. Witness 2 mutations.
   - M1 (step 1 calls `f.signal` directly): FAIL, {held:false, sameProbes:2}, sent=1.
   - M2 (step 2 removed): FAIL, {sameProbes:0, wantSent:1, wantFailures:2}, only one failure.
9. More mutations. The details for M3 to M11 are above and in the notes.
   - M3 (step 3 skipped): FAIL, "cleanup failures do not name grandchild".
   - M4: FAIL, caught only at line 228.
   - M5: FAIL.
   - M6: PASS.
   - M7: FAIL at the 31 s bound.
   - M8, M9, M10: PASS (U5b-3).
   - M11: PASS, and FAIL only once the U5b-2 fix is applied.
10. Timing and load.
    - Stress: 9 of 9 green in 163 s.
    - U5b-1 is a load-only failure.
    - The 30 s test bounds are hang bounds only.
    - The 5 s `waitForExits` bound is 5a's design value and accepts a zombie. It needs only
      SIGKILL delivery, not a reap.
11. Existing behaviour.
    - Nothing outside internal/testutil changes.
    - Nothing else references the renamed 5a messages.
    - Witness 17 now sets `fixture.scan = noFixtureSurvivors`. 5a had no step 3, so this
      isolates the witness and weakens nothing.
    - `go-gate.sh --fast` passed (gate-fast-5b-r4.out).
12. Carried gaps.
    - 5a's M3b and M5 are now closed by witness 2's table.
    - The key-refusal witness is still missing.
    - M8, M9 and M10 are new unguarded behaviours (U5b-3).
13. Survivors. None from my runs: no process naming my scratchpad path, and 0 fixture
    directories after the mutations, the stress and the churn. The stale VM fixture directory
    that the seat's r4 check reports ("vm fixture dirs after: 1") was already there before those
    runs. It comes from the r3 nowait mutation, not from r4.
14. Comments and layout.
    - The one added comment is plain English and has no round or finding references.
    - The removed blank lines between some test functions are cosmetic only.
    - gofmt passed.

## Non-material notes

- Step 3 exit asymmetry. A certain survivor that is Dead at the pre-probe still reaches
  `SignalExact`, which returns ErrGone, and is named "unrecorded fixture child found running". A
  certain survivor already a zombie is skipped silently. Whether an escaped grandchild is
  reported therefore depends on when it dies. The escape itself is real either way, so this
  only affects wording and timing.
- Leash wake-up. No witness proves that a shell already blocked in `read <&3` on the leash
  wakes when the writer closes. The fifoprobe evidence supports a lost wake-up in the macOS
  kernel: lsof shows no writer, and opening and closing a fresh writer releases the hang. The
  poll variant saw 0 of 2400, raw 3 of 2400 and then 0 of 2400, rdwr 2 of 2400.
- The fifoprobe does not isolate the "reader entering read as the last writer closes" window
  that the new comment names: rawdelay was 0 of 2400, and raw was 3 and then 0. The comment
  states a plausible mechanism, not a proven one.
- Open hole and lost end-of-file. A leashed child that misses end-of-file, or that opens the
  leash after the owner has gone, blocks forever, reparented to init. Only the custodian
  (5d, gate off), a census `--reap` or a manual kill ends it, and its FIFO directory is left
  behind. This matches the design, which makes the custodian the safety and the leash only a
  fast path. Nothing uses the leash yet.
- Real-t cleanup order. `killFixtureProcessAtCleanup` runs its kill and Wait before
  `closeLeash`. If `SignalExact` returned ErrUninspectable for a live leashed shell, Wait would
  hang until the test timeout. That is unlikely for a direct child that was just probed.
- On a test timeout, `metasystem-fixture-*` directories, TempDir directories and witness 17's
  stopped child can be left behind. The stale VM fixture directory from the seat's r3 nowait
  mutation is still present.
- The live part of witness 2 runs `read -r _; exit 0` rather than a bare `exit 0`, so Record
  sees a live child. This is stronger than the brief's wording.
- `waitForFixtureExit` has no `t.Helper()`, so failures report the helper's line.
- M11 is my own construct: a silent cross-fixture kill in step 1. It is not in the build brief.
  It is included to show U5b-2.

VERDICT: NOT LAND
