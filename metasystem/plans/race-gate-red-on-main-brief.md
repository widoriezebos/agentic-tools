Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal race-gate-red-on-main, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

The full Go gate (the race-detector run over every internal package that
metasystem/scripts/agents/go-gate.sh performs, and that the adopt fixture
invokes through the nested validation runs) is red on plain main. This
round fixes three of its failures at their cause and re-sizes the gate's
per-package ceiling. When you are done:

1. The refusal register test named TestHCL03EveryCodeRowed in
   metasystem/internal/refusal/register_test.go passes. Today it fails
   because the token VERIFIED_CHANNEL_ANSWER, defined at line 38 of
   metasystem/internal/humanauthority/authority.go, has no register row
   and no exclusion. It is an authority outcome that admits rather than
   refuses, the sibling of HUMAN_AUTHORITY_PROVEN, TEMPORARY_HUMAN_WORD
   and AUTHENTICATED_CHANNEL_WORD, which are exclusions with the reason
   "an authority outcome that admits rather than refuses" at lines 115
   to 117 of metasystem/internal/refusal/register.go. Add it as an
   exclusion with that exact reason, directly after the
   AUTHENTICATED_CHANNEL_WORD entry. It is not a row: rows are refusals.

2. Two data races in the goal package are gone, fixed in production
   code, with the tests unchanged:

   a. The projection fetch with a deadline (fetchProjectionWithinDeadline
      in metasystem/internal/goal/project.go, lines 100 to 113) reads the
      package-level hook fetchForProjection inside the goroutine it
      starts. When the fetch outlives the deadline, which is the whole
      point of the deadline, that goroutine reads the hook after the
      caller has moved on, and the test
      TestFreshLedgerFailureAndFetchTimeoutBlockTheStop in
      metasystem/internal/goal/turnverdict_idle_test.go restores the hook
      in its cleanup while the leaked goroutine still reads it. The cause
      is the late read. Read the hook once in the caller, before the
      goroutine starts, and let the goroutine call the value it was
      given. The timeout variable is already read in the caller; keep it
      that way.

   b. TurnVerdict in metasystem/internal/goal/turnverdict.go writes the
      receiver's Root field at line 171 (the resolved state root) on every
      call. The Store is shared: the watchdog protocol test
      TestWatchdogProtocol in metasystem/internal/goal/turnverdict_test.go
      calls TurnVerdict from eight goroutines on one Store, which is the
      shape the stop hook's concurrent stops take. The write races with
      the read at line 167 and with withLock's read of Root at line 97 of
      metasystem/internal/goal/goalverbs.go. The cause is a verb mutating
      its receiver. Fix it so the receiver is never written after
      construction: resolve the root into a per-call copy of the Store
      and run the rest of the verb on that copy. Two more verbs rewrite
      the receiver the same way: WriteSessionStop at line 237 and
      EndSessionStop at line 369 of metasystem/internal/goal/sessionstop.go; give them the same
      treatment so no verb in the package writes its receiver's Root.
      The Store has three fields (Root, Prober, Now); a shallow copy is
      complete. No mutex: the Store carries no mutable state once the
      copy is in place.

3. The race run's per-package ceiling in
   metasystem/scripts/agents/go-gate.sh (the go test line with the race
   detector, coverage, and a 30-minute timeout, at line 528) is raised
   from 30 minutes to 60 minutes, and the comment above it (lines 524 to
   527) is rewritten to state why in the file's own voice. The facts the
   comment rests on:

   - The goal and mission-runner packages hold 353 and 296 tests, none
     marked parallel, and each test drives real git repositories; no
     single test exceeds 12 seconds under the race detector, so the cost
     is the serial sum, not a giant.
   - Standalone under the race detector on an 18-core Mac, the goal
     package takes about 8 minutes; the mission-runner package is of the
     same order, and its mission cycles carry real waits. Inside the
     full gate, where some fifty package test binaries run at once, each
     of the two has exceeded 30 minutes on a smaller Mac, and the gate
     went red as a timeout rather than a failure.
   - The ceiling is a hang bound, not a target. Sixty minutes is above
     every measured package time under contention and still ends a hung
     package.

   Write the comment in the application's voice: what the ceiling is
   for and why this size. Do not put goal names, dates, machine names,
   round numbers, or finding references in the comment; those live in
   the goal record.

Out of scope for this round: the steward test named
TestArmConfirmsTheGuardAndDisarmEndsIt. It passes standalone here and
its 17-minute failure on another Mac is still under investigation on
the orchestrator's side. Do not touch metasystem/internal/steward.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/refusal/register.go
May touch: metasystem/internal/goal/project.go
May touch: metasystem/internal/goal/turnverdict.go
May touch: metasystem/internal/goal/sessionstop.go
May touch: metasystem/scripts/agents/go-gate.sh
Must not touch: any test file (every file whose name ends in _test.go
stays byte-identical), anything under plans, anything under
metasystem/internal/steward, metasystem/internal/missionrunner, or any
other file.

# Inputs

- metasystem/internal/refusal/register.go: the Rows and Exclusions
  tables and the Exclusion type (an exact token or a PREFIX_* pattern,
  with a reason).
- metasystem/internal/refusal/register_test.go: the collector walks the
  dispatch, goal, goalbudget, landing, steward, channel, humanauthority
  and cmd directories for upper-snake tokens; every collected token
  needs a row or an exclusion, and every exclusion must match at least
  one collected token (a dead exclusion fails the test).
- metasystem/internal/humanauthority/authority.go line 38: the token's
  definition; lines 171 to 190 show it is the outcome of a verified
  channel answer, an admission.
- metasystem/internal/goal/project.go lines 40 to 113: the hook
  variables and the deadline fetch.
- metasystem/internal/goal/turnverdict.go lines 160 to 236: TurnVerdict.
- metasystem/internal/goal/goalverbs.go lines 52 to 75: the Store and
  its accessors; lines 88 to 120: withLock.
- metasystem/internal/goal/sessionstop.go lines 237 and 369: the other
  two receiver writes.
- metasystem/scripts/agents/go-gate.sh lines 1 to 20 (the gate's law:
  the fast switch is argument-only, no environment switches) and lines
  505 to 545 (the race run and its failure-log retention).
- metasystem/records/goals/missionrunner-suite-speed.md: the earlier
  speed work on the mission-runner suite; context only, nothing to
  change there.

# Constraints

- Never weaken a test. No test edits, no skips, no parallel markers, no
  shortened timeouts in tests.
- No new flags, no environment switches, no new configuration keys.
- The full race gate is not yours to run; it takes the better part of an
  hour and the orchestrator runs it outside the sandbox. Run only the
  focused commands under Expected Return.
- One round. Stop at 60 minutes of wall clock and report what stands.
- Hazard class DESIGN-BEARING: a concurrency fix on the stop hook's
  verdict path and a change to a landing gate's bound. An independent
  critique follows your return; leave nothing for it to guess at.

# Expected Return

Return the implementer role's version-2 JSON with every required
property. Evidence commands, each replayable verbatim from the worktree
root (the metasystem directory):

- `go test -count=1 ./internal/refusal` (expected: ok, all HCL03 tests)
- `go test -race -count=1 -run 'TestFreshLedgerFailureAndFetchTimeoutBlockTheStop|TestWatchdogProtocol' ./internal/goal` (expected: ok, no race report)
- `go test -race -count=1 ./internal/goal` (expected: ok with no race report; about eight minutes)
- `go vet ./internal/goal ./internal/refusal` (expected: clean)
- `gofmt -l ./internal/goal ./internal/refusal` (expected: no output)
- `bash -n ./scripts/agents/go-gate.sh` (expected: exit 0)
- `grep -c 'timeout 60m' ./scripts/agents/go-gate.sh` (expected: 1)
- `grep -c 'timeout 30m' ./scripts/agents/go-gate.sh` (expected: 0)
- `git diff --stat` (expected: exactly the five files under May touch,
  or fewer if one needed no change; say which)

diffBoundary lists every touched path relative to the repository root,
so each starts with `metasystem/`. Lead riskiestPart with the Store
copy: say what state, if any, a caller could have relied on being
updated on the receiver after a verb, and confirm you checked every
caller of TurnVerdict, WriteSessionStop and EndSessionStop.

# Acceptance Criteria

1. `go test -count=1 ./internal/refusal` passes; the new exclusion's
   pattern is exactly VERIFIED_CHANNEL_ANSWER and its reason is exactly
   "an authority outcome that admits rather than refuses".
2. The two named goal tests pass under the race detector with no race
   report, and the package's test files are byte-identical to main.
3. No function in the goal package assigns to its receiver's Root field
   (`grep -n 's\.Root = ' ./internal/goal/*.go` prints nothing outside
   test files).
4. The goroutine in fetchProjectionWithinDeadline reads no package-level
   variable; the hook value is captured by the caller before the
   goroutine starts.
5. go-gate.sh's race run line differs from main only in the timeout
   value (30m to 60m); the comment above it is rewritten as specified;
   no other line of the file changes.
6. `git diff --stat` shows only files listed under May touch.

# Gap Rule

stop and report a gap; never fill it silently.
