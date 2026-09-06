Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round two of chain shr-build1 under goal stop-hook-refusal-carries-verdict, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Round one stands: the reason order, the verdict-before-block order and
the deadline parent are right. Seat-side on this Mac under the stock
bash 3.2 (2026-09-06, your worktree, your rebuilt engine) the hook
fixtures pass and the supervision suite's stop-hook-monitor scenario
passes its first refusal, its second-firing non-block, its settled
all-clear and its byte-identity check. Four things stop it short of
green, found by running the scenario to its end in a scratch copy of
your worktree; fold all four.

# The folds

1. The goal-open leg (S4-15 b) fails with

       flag provided but not defined: -tier
       Usage of goal open: -caller-pid ... -id ... -intent ... -next ... -root

   The stop root is a fresh git-init checkout, a legacy single-file
   ledger world whose goal open takes no tier. At
   metasystem/scripts/agents/supervision-fixtures.sh lines 1787 to 1788
   the call passes `--tier 3`, a synced-ledger flag. Remove `--tier 3`
   from that one call and change nothing else about it.

2. The pid line you wrote for bash 3.2,
   `stop_main_pid=$(sh -c 'echo $PPID')`, yields a transient pid, not
   the scenario subshell's. A command substitution forks a subshell
   first; without exec the child sees that forked subshell as its
   parent, and two consecutive substitutions disagree (measured on this
   Mac: 37572 and 37576 from the same subshell, while the exec form gave
   37571, the subshell itself). The hook's identity walk then never
   finds the fake agent ancestor, every arming call goes out without a
   pid, and the suite's isolation check fails at the end of the scenario
   with "announced main pid <empty> outside its scenario bed". Change
   the line to

       stop_main_pid=$(exec sh -c 'echo $PPID')

   and nothing else on it.

3. The degraded-path leg (S4-15 d) fails with "verb failure did not
   surface the fixed degraded message": with the state directory made
   unwritable, the turn verdict verb does not fail; it answers at exit 0
   with the fail-closed verdict that failClosedTurnVerdict in
   metasystem/internal/goal/turnverdict.go builds (block, block source
   "uncertainty", ledger status "degraded", display "cannot prove that
   stopping is safe: <error>"), and the hook, seeing a readable verdict,
   composes an ordinary block from that display. The fixture and design
   point GOAL-05 want the hook's fixed message ("turn-verdict
   unavailable: <diagnostic>") whenever the verb could not compute a
   verdict. The ledger status alone cannot tell the two apart: a
   readable verdict over a missing or unreadable ledger also carries
   "degraded". So the verb says it explicitly:

   - In metasystem/internal/goal/turnverdict.go add to the Verdict
     struct one field, `FailClosed bool` serialised as `failClosed`
     with omitempty, documented in the struct as "true only when the
     verb could not compute a verdict and answers fail-closed; the hook
     renders that as its fixed degraded message". failClosedTurnVerdict
     sets it true; no other path sets it. Nothing else in the verdict
     changes; schemaVersion stays 1 (the field is additive and absent
     when false).
   - In metasystem/internal/goal/turnverdict_test.go add exactly one
     new test, named TestFailClosedVerdictCarriesItsMarker, asserting
     that failClosedTurnVerdict marks the verdict and that an ordinary
     verdict from a store over a fresh bed does not. Change no existing
     test.
   - In metasystem/scripts/agents/supervision-hook.sh, where round one
     decides `verdict_readable`, also read the verdict's failClosed
     field (absent or false means false); when it is true, take the
     existing degraded path with `degraded_line` set to the verdict's
     display, exactly as a non-zero verb exit is handled, and record
     the "stop verdict unavailable" trail line as that path does. A
     readable verdict with failClosed false is unchanged.

4. With folds 1 to 3 applied in a scratch copy, every assertion of
   S4-14, S4-15 and S4-16 passes on this Mac, and the scenario then
   fails the suite's own isolation check
   (assert_fixture_supervision_isolation in
   metasystem/scripts/agents/supervision-fixtures.sh, the loop over
   announcement files) with "announced main pid <empty> outside its
   scenario bed". That loop reads every JSON file under a harness
   root's announcements directory (the mains directory under the agent artifacts) and asks each for its pid.
   Three files there are the holder's own state, not announcements,
   and carry no pid: the protocol cursor (a name ending in
   .protocol-cursor.json), reaped-after-claim.json and
   worktree-lease.json. The real announcement (session-<pid>.json)
   carries the bed's pid and passes. Skip exactly those three names,
   by name, before reading the pid; any other file without a pid
   still fails the check, so a pidless announcement cannot hide. The
   census-lifecycle scenario fails on the same loop for the same
   reason; report whether it turns green.

With all four folds applied in the scratch copy, the stop-hook-monitor
scenario passed end to end on this Mac under the stock bash 3.2.

Out of scope, as before: any census-lifecycle failure that remains after
fold 4; report only.

# Workspace

Your existing worktree for chain shr-build1, on top of your round-one
work.
May touch: metasystem/scripts/agents/supervision-fixtures.sh
May touch: metasystem/scripts/agents/supervision-hook.sh
May touch: metasystem/internal/goal/turnverdict.go
May touch: metasystem/internal/goal/turnverdict_test.go
Must not touch: anything else. Round one's changes in
metasystem/internal/report/stopblock.go and its test stay exactly as
they are.

# Constraints

- No other assertion, wait or flag moves; no existing test changes.
- Do not run the supervision suite or the hook fixtures in this round;
  the orchestrator runs both seat-side on your worktree. Run only the
  commands below. Rebuild the engine after the Go change.
- At most 45 minutes of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory), with the private
build cache you used in round one if the host cache is denied:

- `go test -count=1 -run 'TestFailClosedVerdictCarriesItsMarker' ./internal/goal` (expected: ok)
- `go vet ./internal/goal` and `gofmt -l ./internal/goal` (expected: clean, no output)
- `bash scripts/agents/go-build.sh` (expected: a rebuilt engine line)
- `bash -n ./scripts/agents/supervision-hook.sh` and `bash -n ./scripts/agents/supervision-fixtures.sh` (expected: exit 0)
- `grep -n -- '--tier' ./scripts/agents/supervision-fixtures.sh` (expected: no output)
- `grep -n '^stop_main_pid=' ./scripts/agents/supervision-fixtures.sh` (expected: the exec form)
- `grep -n 'failClosed' ./scripts/agents/supervision-hook.sh ./internal/goal/turnverdict.go` (expected: the field, its setter, and the hook read)
- `grep -n 'protocol-cursor.json' ./scripts/agents/supervision-fixtures.sh` (expected: the one skip in the isolation loop)
- `git diff --stat HEAD` (expected: the round-one files plus turnverdict.go and turnverdict_test.go)

diffBoundary lists every path touched across the chain so far, each
starting with `metasystem/`.

# Acceptance Criteria

1. The fixture's goal-open call for fixture-goal carries no tier flag
   and is otherwise byte-identical; the pid line is the exec form; the
   isolation loop skips exactly the three named holder-state files.
2. `report turn-verdict` over an unwritable state directory prints a
   verdict with `"failClosed":true`; an ordinary verdict prints no
   failClosed field.
3. The hook renders a fail-closed verdict through its fixed
   "turn-verdict unavailable:" message and a readable verdict as
   before.
4. No file outside May touch changed; no existing test changed.

# Gap Rule

stop and report a gap; never fill it silently.
