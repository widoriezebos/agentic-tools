# Read: fixture-children unit 3 (custodian core)

Reviewer: Opus 5, build-read. I did not write this change. Findings only.
Base: worktree at trunk 9cafaae5f. Diff computed at 12:30 (299 insertions / 1 deletion)
and again at 12:49 (303 insertions / 1 deletion) — see U3-1.
Platform: darwin (m1). Another agent was working on the same machine and in the
same worktree during this review; see "What I could not check".

This file was originally written to `<worktree>/read-fcu3.md` and destroyed when a
later fix job cleaned untracked files in that worktree. Restored here, outside the
worktree, unchanged in substance.

## Changes since this read (reported by the seat, not re-verified by me)

- The two witnesses no longer time out: a pid-publication race was fixed.
  `TestCustodianReapsStoppedAndDetached` now runs in 1.86s and the custodian does kill
  both children. U3-2 below is therefore addressed; it is kept as the record of what
  was seen and why.
- The remaining failure is U3-9: the log line still reads `carrier=argv-or-environment`
  instead of naming the slot.
- The tree is now at 358 insertions against the hard 300 cap, so U3-1 has widened, not closed.

## Material findings

### U3-1 The worktree is still being edited, and it now breaks the hard 300-line cap
- Where: `metasystem/internal/identity/fixture_custodian_test.go` (whole file; mtime 12:48:25).
- Evidence: `git diff --stat -M HEAD -- metasystem` at review start: `299 insertions(+), 1 deletion(-)`,
  test file 113 lines. The same command at 12:49:58: `303 insertions(+), 1 deletion(-)`, test file 117
  lines. Line 96 of the current file is a debugging line that is in no reviewed diff:
  `t.Logf("diagnostic path=%q data=%q pid=%d parseErr=%v state=%s probeErr=%v", ...)` inside
  `waitWitnessRef`. My `go test` run captured 300KB of that output. At the end of the review the
  same command read `324 insertions(+)`, and the seat reports 358 now.
- Failure: the brief's ceiling is "300 changed lines, counted as `git diff --stat -M HEAD -- metasystem`.
  The cap is hard." 304 changed lines exceeds it, 358 exceeds it badly. Separately, the artifact reviewed
  is not the artifact in the tree, so no review of this worktree can certify it while it moves.
- Smallest fix: remove the `t.Logf` diagnostic (that alone restored 299/1 at the time) and freeze the
  worktree before the next read; the unit has to be brought back under 300 or split at its witness
  boundary, as section 7 requires.

### U3-2 Both named witnesses fail under `go test` — the unit's own proof obligation
(Reported fixed since this read; kept as the record.)
- Where: `metasystem/internal/identity/fixture_custodian_test.go:62-89` (`custodianWitness`,
  `runWitnessOwner`) and `:90-102` (`waitWitnessRef`).
- Evidence:
  - `go test -race -count=1 ./internal/identity/... ./internal/testenv/...`:
    `--- FAIL: TestKilledTestBinaryLeavesNoFixtureChild (30.03s)` and
    `--- FAIL: TestCustodianReapsStoppedAndDetached (30.05s)`, both
    `fixture_custodian_test.go:98: fixture child did not publish its pid`. `internal/testenv` passed.
  - Without `-race`: `go test -count=1 -v -run '^TestCustodianReapsStoppedAndDetached$' ./internal/identity/`
    → same failure, 30.07s.
  - The same helper passes when the binary is run directly:
    `TMPDIR=<dir> identity.test -test.run='^TestKilledTestBinaryLeavesNoFixtureChild$' -test.count=1 -test.v`
    → `--- PASS (0.33s)`, no strays. So the failure was specific to the `go test` invocation the seat
    and go-gate use, not to my environment.
  - Root cause, from the builder's own in-tree diagnostic: pid0 (the detached grandchild) probes
    `state=alive`; pid1 (the `kill -STOP $$` child) probes `state=dead` on every one of ~3000 attempts
    across the full 30s. The stopped child died before the parent ever observed it alive, so witness 5's
    "stopped" leg was never exercised and the witness timed out.
  - SIGSTOP is NOT the cause of that dead probe: I verified the prober reads a stopped process as alive.
    `/bin/sh -c 'printf %s $$ > f; kill -STOP $$; sleep 30'` in state `TN`, then
    `metasystem proc probe --pid <it>` → `"liveness":"alive"`, rc 0; `proc exists` rc 0.
  - Consequence observed on the machine: tagged fixture children outliving dead owners for 27+ minutes,
    e.g. `82077 ... /bin/sh /tmp/TestCustodianReapsStoppedAndDetached1433885963/001/detached.sh
    METASYSTEM_FIXTURE_OWNER=pid=82074;...` (etime 27:27, owner 82074 long dead) — a `t.TempDir()` path,
    i.e. an ordinary `go test` run — plus `82107` (owner 82102) and resident `identity.test` custodians
    in state `Ss`/`Rs` from finished runs. This is the compounding leak the goal exists to stop.
- Failure: the unit's stated proof ("Witness 3 and witness 5 of section 6, as the design states them")
  did not hold; the seat's gate was red; and the change left survivors behind.
- Smallest fix: find why the stopped child dies at birth under `go test` (start it, confirm it alive,
  then STOP it from the helper rather than having the child STOP itself), and give the helper's
  `exec.Cmd` a captured stderr file in the temp dir so a helper failure is reported instead of
  surfacing as "fixture child did not publish its pid" 30 seconds later.

### U3-3 `proc custodian` is inside every kill domain it is supposed to escape, and holds the caller's streams
- Where: `metasystem/cmd/metasystem/identity.go:14-31` (`runFixtureCustodian`).
- Evidence: live run of the built binary with a real owner ref and fd 3 on a FIFO:
  - `ps -o pid,ppid,pgid,sess -p <custodian>` → `90825 87785 87785 0`; my invoking shell is
    `87785 ... 87785 0`. Same process group, same session. No `Setsid` anywhere on this path;
    only `testenv.startFixtureCustodian` sets `SysProcAttr{Setsid: true}`.
  - `lsof -p <custodian>` → `1w REG /private/tmp/fc.out`, `2w REG /private/tmp/fc.err` — the caller's
    own stdout and stderr, inherited. No `/dev/null`, no log file; `RunCustodian`'s log lines go to
    whatever fd 2 the caller had.
  - TERM is correctly ignored (`kill -TERM` left it in state `SN`), so the group kill that does reach
    it is the one that kills it.
- Failure: design 3.3 requires "a small process in its own session (`Setsid`), std streams on
  `/dev/null` and a log file, holding nothing else open", and names the group kills it must survive
  (job cancellation, the suite group kill at proofrun/launcher.go:231). A bed's group or session kill
  takes this custodian with it, so the survivors are never reaped. Independently, when the bed's stdout
  is a pipe (command substitution, a harness log reader), the custodian keeps that pipe open after the
  bed dies and the reader never sees EOF — the inherited-fd hazard, on the entry point this unit ships.
- Smallest fix: in `runFixtureCustodian`, before calling `RunCustodian`: `syscall.Setsid()` (tolerating
  EPERM when already a session leader), open the log path (a `--log` flag, defaulting to
  `FixtureCustodianLogEnv`), `dup2` `/dev/null` onto fds 0 and 1 and the log onto fd 2, and pass the log
  file as the `log` writer.

### U3-4 The missing-watch-descriptor guard is dead code; a bed that forgets fd 3 silently loses its custodian
- Where: `metasystem/cmd/metasystem/identity.go:21-25` and `metasystem/internal/testenv/testenv.go:91-96`
  (`watch := os.NewFile(3, "fixture-owner-watch")` ... `if err != nil || watch == nil`).
- Evidence: `metasystem proc custodian --owner "pid=90608;micro=1789555917708193" 3>&- </dev/null`
  → `os.NewFile` returned non-nil for the closed descriptor, the read failed immediately, the custodian
  took the EOF path against a live owner and printed
  `proc custodian: identity: fixture custodian cleanup exceeded 5s: identity: fixture owner 90608 is alive`,
  rc=1, elapsed 5s. The `watch == nil` branch never fires.
- Failure: design 3.3's "Fail fast, never degrade" is not achieved on this entry point. A bed that
  mis-wires fd 3 gets a custodian that exits five seconds later and a run that is unguarded from then on,
  with nothing but a line in a log nobody reads.
- Smallest fix: validate the descriptor explicitly — `if _, err := unix.FcntlInt(3, unix.F_GETFD, 0); err != nil`
  → print and exit 2 — instead of relying on `os.NewFile` returning nil.

### U3-5 A spurious or early pipe EOF permanently disarms the custodian while the owner is still alive
- Where: `metasystem/internal/identity/fixture_custodian.go:52-56` (`case <-pipeClosed: return reapDeadOwner(...)`).
- Evidence: the first EOF is terminal — `runCustodian` returns whatever `reapDeadOwner` returns, and
  `reapDeadOwner` gives up after `bound` while the owner is alive. Demonstrated by U3-4's run: owner
  alive, EOF, five seconds, process gone. Any path that closes the owner's write end early
  (`syscall.Exec` of the test binary, an fd-hygiene helper closing a descriptor range, a bed re-exec)
  ends the watch silently for the rest of the run.
- Failure: the guarantee is lost with no signal and no exit code anyone checks. The design's
  compensating control (`testutil.Fixture(t)` re-probing the custodian) is unit 5 and not present.
- Smallest fix: on EOF, enter the reap only when the owner does not probe `Alive`; otherwise fall back
  to the poll loop (which is itself bounded by the owner's death) instead of returning.

### U3-6 The five-second bound is cooperative, and it also pays for proving the owner dead
- Where: `metasystem/internal/identity/fixture_custodian.go:58-80` (`reapDeadOwner`).
- Evidence: `deadline` is set before the first scan, and `FixtureSurvivorsOfDeadOwner` errors
  (`fixture owner %d is alive` / `cannot be proved dead`) for as long as the owner is a zombie or probes
  Unknown — the same five seconds the design allocates to "rescan until empty or five seconds". An owner
  reaped by its parent at 4.9s leaves 0.1s for cleanup and the custodian exits with survivors alive.
  The deadline is only evaluated between iterations, so it bounds nothing if a scan itself blocks; the
  scan reads argv and environ of every same-uid process, and on Linux a `/proc/<pid>/cmdline` or
  `/proc/<pid>/environ` read against a task whose memory lock is held by a D-state thread blocks
  indefinitely. The custodian is `Setsid`, ignores TERM and ignores HUP, so nothing below SIGKILL ends
  it — the outliving watcher this goal exists to prevent.
- Failure: the claimed bound ("completes cleanup within a bounded five-second reap window") is not
  enforced by anything independent of the work it bounds.
- Smallest fix: arm a hard exit when the reap starts —
  `timer := time.AfterFunc(runtime.bound+time.Second, func() { os.Exit(3) })`, stopped on return — and
  give "prove the owner dead" its own budget separate from the cleanup window.

### U3-7 R-115-m1e: every test binary in the repo changes behaviour, unconditionally
- Where: `metasystem/internal/testenv/testenv.go:118-122` (`fixtureCustodianOwnerWatch, err = startFixtureCustodian(...)`,
  with no environment gate) and `:124-160`.
- Evidence: 82 Go files call `testenv.Main`. Every one of those test binaries now forks a detached,
  `Setsid`, TERM/HUP-ignoring process before `m.Run()`, writes `fixture-custodian.log` into the registry
  home, and at exit performs a full process-table scan that reads argv and environ of every same-uid
  process. Observed directly: one `identity.test` session-leader custodian per test-binary run in `ps`,
  e.g. `63726 63155 63726 0 SNs .../b001/identity.test`. A failure anywhere in `startFixtureCustodian`
  (pipe, log open, `exec.Command.Start`, the post-start probe) returns 2 before `m.Run()`, so it fails
  the whole package with no fallback and no way to turn it off.
- Failure: the brief's binding rule is "no existing identity or testenv behaviour changes... A test
  binary that does not set the custodian variable behaves exactly as it does today." It does not: it
  gains a child process, a log file, a new environment variable (U3-8) and a process-table scan. The
  design's unit-3 row does say "`testenv.Main` start and probe", so the design and the binding rule
  disagree; the implementation resolved that silently. The seat has to adjudicate it, not the builder.
- Smallest fix: either gate the start on an opt-in variable until the consumers (units 4, 5) exist, or
  get Wido's explicit release from R-115-m1e for this line. At minimum, do not fail a package that uses
  no fixtures when the custodian cannot start.

### U3-8 `testenv` pollutes the very environment boundary it exists to own
- Where: `metasystem/internal/testenv/testenv.go:147-149`
  (`os.Setenv(identity.FixtureCustodianLogEnv, logPath)`).
- Evidence: `prepare()` deliberately clears every inherited control before the tests run; this call then
  sets `METASYSTEM_FIXTURE_CUSTODIAN_LOG` in the owner's own process environment, after that clearing.
  The name is in neither `inheritedControlNames` nor `inheritedControlPrefixes`, so `rejectsInheritedControl`
  does not match it and a nested `testenv.Main` will not clear it either. Every child of every test in
  all 82 packages now inherits it. Its only reader is the witness test
  (`os.WriteFile(dir+"/logpath", []byte(os.Getenv(FixtureCustodianLogEnv)), 0o600)`).
- Failure: a test-only need mutates the production environment boundary of the package whose stated job
  is owning that boundary, and it does so outside that package's own allow-list machinery.
- Smallest fix: put the log path only in the custodian's `command.Env` (it is already being built), and
  have the witness read the path from the registry home it already knows; if the variable must exist for
  the owner too, add it to `inheritedControlPrefixes`.

### U3-9 The log line does not name the carrier slot
- Where: `metasystem/internal/identity/fixture_custodian.go:73`
  (`fmt.Fprintf(log, "fixture-custodian action=kill pid=%d carrier=argv-or-environment result=%v\n", ...)`).
- Evidence: the field is a hardcoded literal. The design's revision-4 amendment re-estimates unit 3 as
  "No carrier logic of its own; **the log names the slot**: +10, about 270."
- Failure: an acceptance criterion of this unit is not met; the log cannot tell an operator whether a
  survivor was found by argv or by environment, which is the amendment's whole diagnostic point.
- Smallest fix: have `FixtureTag` return which slot matched (it already loops over the two in order) and
  carry it on `FixtureSurvivor`, or drop the field rather than print a value that is always true.
  This was the weakest of the material findings when I wrote it; per the seat it is now the only one
  still failing.

## Non-material notes

- N-1 `sameExactRef` (`fixture_custodian.go:83-86`) re-implements an exact-ref comparison that the
  package already expresses as `EncodeRef` equality (used in `FixtureSurvivorsOfDeadOwner`) and as
  `SameIdentity`. Taste and duplication, no behaviour difference.
- N-2 The witness starts its helper with no stdout/stderr, so a helper that dies reports nothing. This
  is why U3-2 needed a subprocess-level investigation to diagnose. Worth fixing with U3-2's fix.
- N-3 `testmain_test.go`'s `package identity` → `package identity_test` is forced and correct: `testenv`
  now imports `identity`, so the old in-package TestMain would be an import cycle. Behaviour is
  preserved (TestMain still governs the whole binary). Worth a one-line comment so nobody moves it back.
- N-4 The witness inlines the amendment's shell prologue as a literal in `runWitnessOwner`. `ShellPrologue`
  is unit 5, so a literal here is the right call; it will need folding into the shared helper later.
- N-5 Boundary (check 6) is clean: no `ExportRunOwner`, no chain walk, no `testutil.Fixture`, no
  `proc fixture-survivors` verb, no health or launcher consumer, no fixture conversions. `grep` for
  `METASYSTEM_FIXTURE_CUSTODIAN` outside `internal/identity` and `internal/testenv` returns nothing.
  `RunCustodian` takes no chain argument, so unit 4 will change its signature — expected.

## Checks that passed

- Self-exclusion mutation (check 2), run by me on an isolated copy of the module: removing
  `|| sameExactRef(survivor.Ref, runtime.self)` from `fixture_custodian.go:68` →
  `--- FAIL ... dead-owner reap signaled=[701 702] err=<nil>; want child 702, never self 701`.
  Restored → `ok`. The test is failure-sensitive here.
- Live-owner mutation (check 3), same copy: replacing `FixtureSurvivorsOfDeadOwner(runtime.prober, owner)`
  with a direct `scanFixtureSurvivors` on the same owner match (dropping the liveness guard) →
  `--- FAIL ... live owner allowed reap: signaled=[702] err=<nil>`. Restored → `ok`. The reap path goes
  only through unit 2's dead-owner scan, which errors while the owner is alive or Unknown, and the
  custodian acts only on `FixtureSurvivorCertain`, never on `fixture-survivor?`.
- `SignalExact(prober, ref, sig, runtime.sender)` with a nil `sender` is safe: `ref.go:143-146` falls
  back to `syscall.Kill` when the variadic element is nil. Not a nil-deref.
- No accidental self-tagging through the custodian's own variables: `FixtureTag` matches on
  `strings.HasPrefix(word, "METASYSTEM_FIXTURE_OWNER=")`, and
  `METASYSTEM_FIXTURE_CUSTODIAN_OWNER=` does not match that prefix. The testenv starter also filters
  `METASYSTEM_FIXTURE_OWNER` out of the custodian's environment
  (`testenv.go:152-157`) — though it does so with a string literal rather than the identity constant,
  and the `proc custodian` path scrubs nothing, so a bed whose environment carries the tag hands it to
  the custodian. Under this unit alone that is harmless (a custodian excludes itself); once units 6 to 8
  land, `--reap`, `health` and the launcher will see a tagged custodian as a survivor. Flagging it here
  rather than as a finding because the kill surface that would act on it is not in this unit.
- TERM and HUP are ignored (`signal.Ignore` at `fixture_custodian.go:45`), confirmed live: `kill -TERM`
  left the custodian in state `SN`.
- Owner killed with SIGKILL, and owner's whole session killed: both close the owner's pipe write end, the
  read goroutine returns and the reap runs. `os.Pipe` is close-on-exec and only the read end is passed in
  `ExtraFiles`, so no fixture child inherits the writer; the 250ms `AliveRef` poll is a working second
  path if the pipe ever lies. Those two cases are sound.
- `gofmt -l` clean; `go vet ./internal/identity/... ./internal/testenv/... ./cmd/metasystem/...` clean;
  `go build` succeeds.

## What I could not check

- Linux. Everything here was observed on darwin. The blocking-scan half of U3-6 is Linux-specific
  reasoning (procfs `cmdline`/`environ` reads), not an observation.
- The full battery and `metasystem test run`: I did not take the machine's testrun lock.
- The other 80 packages that call `testenv.Main`. I ran `internal/identity` (FAIL) and `internal/testenv`
  (ok) only, so the blast radius of U3-7 is inferred from the call count, not measured.
- Clean attribution of every stray process. A second agent was running the same witness and its own
  `/tmp/fixture-custodian-*` diagnostics on this machine throughout, and the worktree itself was edited
  at 12:48:25 while I was reading it. The stray I attribute to an ordinary `go test` run (pid 82077,
  a `t.TempDir()` path, owner dead 27 minutes) is the one I am confident about.
- Whether the builder's claimed mutation runs matched mine: I ran both mutations myself, on a copy, and
  report only what I saw.
- The post-read state. The witness fix, the 1.86s run and the 358-insertion count in the header are the
  seat's report, not my observation; I did not re-read the code.

## Tool calls used

18 during the read (2 Read, 16 Bash), plus one to restore this file after the worktree clean destroyed it.
