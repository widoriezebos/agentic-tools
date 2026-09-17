# Build read: fixture-children units 5b0, 5b and 5b2 (round 5)

Reader: an independent build reader. I did not write this change or the earlier read.

Tree: wt-fcu5b at HEAD b9a97d465.
- The index writes tree dc584d0fb9307c9a6d12252e2bd040693335a0ae, which is the base.
- Unstaged on top: `fixture_survivors.go`, `fixture_survivors_test.go`, `fixture.go` and `fixture_test.go`.
- Untracked: `fixture_teardown_test.go`.

The diff read is fcu5b-r5.diff. Its sha256 is 129492488e53716f36f763db025caa2e58c61ac5398393c77b09958d959f7032, which matches `fcu5b-r5-diff.sha256`.

Method:
- I read the round-5 fix brief, the build brief and the round 2 to 4 briefs (grep for the binding limits), the earlier read, the diff, all of `fixture_survivors.go`, the identity scan witnesses, `fixture.go`'s cleanup path and `SignalExact`.
- From the design page I read section 2, 3.2, 3.4, 3.5, witnesses 2 and 12, the unit 5 row and the revision-4 amendment's scoping of `?`. I also read the obligations section, the seat's check output and script, the gate output and Codex's report.
- Mutations ran through `go test -overlay`, with the replacement files in my own scratchpad (`r5read/`). Test binaries were built with `go test -c -o` into that scratchpad, and every run used `TMPDIR=r5read/tmp`.
- Nothing inside the worktree was created or changed. I took no lock, did not run `metasystem test run`, started no subagents and did not use the VM.

## Earlier findings

- **U5b-1: closed.**
  - `FixtureSurvivors(key)` passes `includeGoTmpUnreadable=false`. The unreadable loop now returns an entry only through `sharesFixtureScope(observation, certain)`, and `certain` holds only processes whose tag or ownership record `matches(key)`.
  - The new table witness fails when the key scan keeps the untied `go-tmp` case (I1 below).
  - Matched churn on this host, with 8 exec loops under `go-tmp` and 4 processes of witness 12 at count 300:
    - with the old rule (the I1 overlay), 1 of 4 processes failed on "fixture survivor ownership unproven at teardown: pid=N exe=\".../r5read/go-tmp/b001/true\" argv=[]";
    - the unmodified tree passed 4 of 4.
- **U5b-2: closed.**
  - The check is now `err != nil || state != identity.Alive || !identity.SameIdentity(exact, secondRef) || exact.Zombie`.
  - The seat's M11 failed 5 of 5 at fixture_test.go:222.
  - My own M11 (a package-level list of every recorded ref, SIGKILLed through `SignalExact` at the start of each cleanup) failed 5 of 5 at fixture_test.go:222 with "second fixture child changed after first cleanup: state=alive err=<nil>".
  - The same M11 with `|| exact.Zombie` removed passed 5 of 5, so this one token is what catches the silent kill.
- **U5b-3: closed.** `TestReapKeySurvivorsRequiresCertainLiveIdentityBeforeSignal` covers all four behaviours, and each fails under a mutation that breaks it (item 5).

## Material findings

None.

## Check items

### 1. Conformance

Each unit stays inside its own files.

| Unit | Changes | Limit |
|---|---|---|
| 5b0 | `fixture_survivors.go` +6/-4 and `fixture_survivors_test.go` +33/-0: 43 lines | under 120 |
| 5b | `fixture.go` +94/-15 and `fixture_test.go` +190/-0: 299 lines | 300 ceiling; unchanged from r4, since part B edits one added line |
| 5b2 | new file, 100 lines | under 120 |

- Part A: one boolean parameter on the unexported `scanFixtureSurvivors`, two doc comments and one table witness. No other selection rule, class or order changed.
- Part B: only the `|| exact.Zombie` token.
- Part C: one table witness plus the helper `fixtureTeardownExact`. The helper is new because the only existing way to get an `Exact` in the package (witness 2) probes the real process table, which part C forbids. `recordingTB` and `fixtureProbeFunc` are reused.
- `go-gate.sh --fast` passed on this tree (gate-fast-5b-r5.out).

### 2. 5b0, the key scan

- **What `FixtureSurvivors(key)` returns.** An unreadable process is returned only when `sharesFixtureScope` finds an equal pgid or sid (above 1) with an entry in `certain`, skipping the key's owner. `certain` is filled only when `matches(key)` holds.
  - A tagged process of another key, or an untagged one, never enters `certain`, so it cannot tie an unreadable process to this key.
  - No witness pins that property, though (note N4). My faithful I2b mutation, which also ties to tagged processes of other keys, left the whole identity package green.
- **`FixtureSurvivorsOfDeadOwner` is unchanged in what it returns.** It passes `true`, and `true && ExeKnown && underGoTmp || shares` has the same value as the old condition, because `&&` binds tighter than `||`.
  - I4 (the owner scan passes `false`) fails `TestFixtureScanClassifiesGoTmpUnreadableProcess`: "want the unrelated go-tmp process in the unreadable class".
  - That witness is byte-for-byte unchanged in the diff.
- **The new table witness asserts the exact result.**
  - First table: no error, length 2, pid 701 certain, pid 703 unreadable.
  - Second table, the untied go-tmp process alone: no error, length 0.
  - I1 (the key scan keeps go-tmp) fails with a third entry, pid 702.
  - I3 (the pgid tie turned off) fails with only 701 returned.
  - The sid tie alone is not pinned by any witness, before or after this change (note N5).
- **The doc comments are true.**
  - "FixtureSurvivors returns processes for key, including unreadable processes only when their process group or session ties them to a certain result." This matches the code.
  - The owner scan's comment, "including unreadable processes under go-tmp or tied to a certain result by process group or session", also matches.
  - Both omit the "signalable, both slots unreadable" definition of unreadable, which is acceptable in one sentence.
- **Callers in the tree.** A grep of every .go and .sh file finds two.
  - `internal/testutil/fixture.go:65` (`scan: identity.FixtureSurvivors`) changes behaviour: step 3 no longer sees untied go-tmp unreadable processes. That is the ruling.
  - `internal/identity/fixture_custodian.go:228` (`FixtureSurvivorsOfDeadOwner`) is unchanged.
  - No census or cmd code calls either function.
  - `testutil.Fixture(` and `newProcessFixture(` have no callers outside `internal/testutil`.

### 3. The residual the ruling leaves

The residual applies to a tagged grandchild of the key that meets all three conditions at step 3's single scan:
- it escaped;
- both slots were unreadable, because it was mid-exec or exiting;
- no certain survivor of the key shared its pgid or sid.

Such a grandchild is no longer named. The old rule caught it only when its exe was under `go-tmp`; a grandchild with any other exe in that state was never named, before or after. So the change narrows a small window further.

- **How likely it is.**
  - A fixture shell does not `Setpgid`, so a grandchild with a live tagged parent or sibling is tied through the pgid and still returned.
  - The residual therefore needs a lone escaped process whose exec or exit falls inside one scan at teardown. Exec and exit windows last micro- to milliseconds.
  - On Linux, from my reading of procfs, reads of cmdline and environ during exec wait on the mm and exec locks rather than fail. That would leave mainly the exit window. I did not run this.
  - An exiting process ends itself, so nothing leaks.
  - A lone go-tmp binary caught mid-exec at exactly that moment is rare. A test that leaks that way leaks on most runs, and the next run names it once it is readable.
- **What ends it today.**
  - The leash, if it is a shell that reads it.
  - In the design, the custodian's owner-wide scan once the binary dies, and 5c's exit scan. By then the process is readable, so it is certain.
  - In this tree the custodian gate is off and 5c has not landed. Only a manual census `--reap` would end it.
  - No fixture outside `internal/testutil` uses `Fixture` yet, so no real test is exposed.
- **Verdict on the residual.** Carrying it to the design fold, as `later-unit-obligations-from-u3.md` already does, is sound. 5b0 need not change now. The obligation should stay attached to 5c and 5d, so that a converted fixture never relies on step 3 as its only net.

The ruling leaves two gaps the brief did not name. Neither is material; see N1 and N2.

### 4. 5b, witness 12

- The check after the first cleanup now fails on a zombie. My M11 and the seat's M11 are consistent (see U5b-2).
- What `TestCleanupIsScopedToTheFixtureKey` still accepts:
  - A first cleanup that sends a non-lethal signal to the second child: SIGSTOP, or SIGTERM, which the shell ignores. The second cleanup then kills it and nothing fails. Cleanup sends only SIGKILL through `SignalExact`, so no code path does this.
  - A cross-fixture leash close is caught only if the second shell has exited by the time of the probe, which is timing-dependent. The leash is per fixture, so no code path does this either.
  - M11 is caught only because SIGKILL has turned the second shell into a zombie by the time of the probe. That held 10 of 10 across both runs.
- Neither is material.

### 5. 5b2, step 3's witnesses

All four cases prove their rule.

| Case | What the witness requires |
|---|---|
| Unproven ownership | `fixture-survivor?`, alive, same identity, not a zombie: no signal, exactly one failure containing "ownership unproven" and `pid=<n>` |
| Scan error | no signal, exactly one failure containing the error text |
| Certain zombie | no signal, no failure |
| Certain live survivor | exactly one signal, `SIGKILL`, to the survivor's pid, sent through the injected function; exactly one failure containing "unrecorded fixture child" and the pid |

The file starts no process and never probes the real process table.
- The owner probe and all survivor probes go to the injected prober, and `SignalExact` uses the injected sender.
- The survivor pid, getpid+1, is never signalled for real.
- `newProcessFixture` does create a real FIFO in a `metasystem-fixture-*` directory, and `cleanup` removes it through `closeLeash`. My runs left 0 such directories.
- Nothing waits on real time. `waitForExits(nil)` returns at once, and in case 4 the post-signal probe returns Dead, so the wait returns on its first pass. Timing and load cannot affect this test.

My mutations of `reapKeySurvivors` (count 1):

| Mutation | Result |
|---|---|
| R1: SIGTERM instead of SIGKILL | FAIL, certain_live_survivor: "want one SIGKILL" (sig 15 sent) |
| R2: certain survivor killed silently | FAIL, certain_live_survivor: "failures = []; want 1" |
| R3: `?` entries also signalled | FAIL, unproven_ownership: "want none" (SIGKILL sent) |
| R4: pre-probe skips every same-identity survivor, not only zombies | FAIL, unproven_ownership (no failure) and certain_live_survivor (no signal) |
| R7: ownership failure without the pid | FAIL, unproven_ownership: want "pid=N" |
| R5: zombie skip ignores identity | PASS (N6) |
| R6: no wait after the kill | PASS (N6) |
| R8: scan error without `return` | PASS; equivalent, because survivors is nil on error |

Codex's three (signals uncertain, drops scan error, skips no zombie) are consistent with the code, but I did not rerun them.

Baseline: `-race -count=3` passed.

### 6. Timing and load

- **The seat's evidence.**
  - Host churn: 6 of 6 rc=0.
  - VM churn: 4 of 4.
  - Host stress: 18 of 18. VM stress: 8 of 8.
  - Caveat: the churn has no control, and the window is rarer on this host today than when the earlier reader saw 3 of 4. My first control, 4 loops with the old rule and 4×120 runs of witness 12, produced 0 failures. Only 8 loops produced 1 failure in 4×300. A green churn on its own is therefore weak evidence. The closure rests on the code, the table witness and I1, plus the matched pair below.
- **My runs.**

  | Run | Result |
  |---|---|
  | 4 go-tmp exec loops; unmodified tree: 3×150 witness 12, 2×50 key scan witness, 1×1000 teardown witness | all rc=0 (163 s) |
  | 8 loops, old rule (I1): 4×300 witness 12 | 1 of 4 failed (161 s) |
  | 8 loops, unmodified tree: 4×300 witness 12 | 4 of 4 passed (125 s) |

- **Why load cannot break these witnesses now.**
  - Witness 12's first cleanup has no certain survivor, so it has nothing to tie to.
  - The key scan witness can now pick up same-pgid churn processes as extra `?` failures. It asserts only that the grandchild's `pid=<n> ` is named, so they cannot fail it.
  - The teardown witness has no real-time dependency.
  - The remaining bounds are hang bounds: the 30 s helpers, and 5a's 5 s exit wait, which needs only SIGKILL delivery.

### 7. Existing behaviour

- Nothing outside the five named files changes. The index equals the base tree.
- The custodian's call is unchanged: it passes `true`, so its result is the same.
- No census code calls either scan.
- `expect_test.go` is unchanged against both the base and HEAD.
- **The VM's `TestExpectBounds` failure: I agree it is not a regression.**
  - `expectedFixtureFile` walks up from the compiled-in source path looking for `go.mod`.
  - The failure prints the host path (`/private/tmp/.../wt-fcu5b/...`), which does not exist on the VM.
  - That is what a binary compiled on the host produces.

### 8. Survivors

- No process names my scratchpad directory.
- No process on the host carries `METASYSTEM_FIXTURE_OWNER=` in its argv or environment, as the checking command itself reported.
- `r5read/tmp` is empty, with 0 `metasystem-fixture-*` directories.
- I deleted my test binaries and the go-tmp copy. What remains in `r5read/` is 292K of overlays and logs.
- I did not use the VM.

### 9. Comments and layout

- The two new doc comments are plain English and have no unit, round or finding references. They are long single lines, and "certain result" is the file's own vocabulary.
- The teardown file has no comments.
- gofmt is clean per the gate.

## Non-material notes

- **N1. The tie is wide.** (Owner: the design fold; 5b0 code.) `sharesFixtureScope` ties on an equal pgid or sid. Fixture children inherit the test binary's pgid and session, which they share with `go test` and the sibling package binaries.
  - Once a certain survivor exists, any same-user unreadable process in that session is also named "ownership unproven", not only go-tmp ones. It gets no signal, and the test already fails on the survivor, so this is noise.
  - One narrow exception: the certain survivor turns into a zombie between the scan and step 3's pre-probe. It is then skipped silently, while the tied unrelated entries still fail the test. This needs a real escape that happens to exit at teardown, plus a coincident exec or exit in the same session.
  - The obligations' "led by vs equal pgid/sid" bullet is where this belongs.
- **N2. The census `--key` path.** (Owner: unit 6 and the design fold.) Design 3.5 and the revision-4 amendment scope `fixture-survivor?` to include go-tmp exes. If unit 6 builds `proc fixture-survivors --key` on `FixtureSurvivors(key)`, `--key` prints fewer `?` lines than the page says. The obligations cover the page wording, but not which function `--key` calls. That choice should be named.
- **N3. Witness 12's message.** (Owner: 5b.) For a zombie it prints "state=alive err=<nil>", which does not say a zombie or a different identity was found. The one-line limit prevented changing it.
- **N4. The other-key tie has no witness.** (Owner: 5b0.) No witness shows that a tagged process of another key cannot tie an unreadable process. It holds by construction. The I2b mutation that breaks it leaves the identity package green.
- **N5. The sid-only tie has no witness.** (Owner: 5b0, and it predates this change.)
- **N6. Two step-3 details have no witness.** (Owner: 5b2.)
  - The zombie skip's identity check (R5): a recycled pid that is a different process's zombie would be skipped rather than reported gone.
  - The wait after the kill (R6).
  - The brief did not ask for either.
- **N7. The seat's churn line counts nothing on passing runs.** It greps "ownership unproven" from test output, but `recordingTB` keeps failures in memory and prints them only when the test fails. That count therefore cannot show tied `?` entries on passing runs; it only confirms that no run failed.

VERDICT: LAND
