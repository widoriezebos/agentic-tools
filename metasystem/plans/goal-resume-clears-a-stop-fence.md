# Hot-fix unit: a person's `goal resume` clears a set stop fence whatever the goal's state

Working Mode: Implement. Hot-fix on main (Wido 2026-10-03 R-145: hot-fixes unstick a broken machine; Wido 09-27: humans are never denied a verb). Size: at most 120 production lines. Smallest change.

## Measured defect (lane test findings 72, 72b, 77)

The breach-stop custodian writes `StopFence` (reason ELAPSED_LIMIT) on a goal whose elapsed budget ran out while it waited in the landing lane, but leaves `State: claimed` (goal file: `- StopFence: stopId=stop-<goal>-r5-f1 ... reason=ELAPSED_LIMIT`, `- State: claimed`). Then:
- `clearClaimBinding` (`internal/goal/verbs.go` ~637-643) refuses every verb on the goal, including `goal done --force` and `work land`, with "goal G is breach-stopped by STOP; only goal resume may clear its launch fence; run: metasystem goal resume G (then repeat this command)";
- `goal resume` (`cmd/metasystem/intent_goals.go` ~915) sees State claimed and answers "G is running under its standing box; there is nothing to resume" without clearing the fence (the fence is cleared only at `verbs.go` ~487/643/3550 on paths this state never reaches).
The person is sent in a loop and cannot conclude a landed goal (fleet-provider-and-session-recovery, landed 15:31) nor hand a returned goal in again (process-changes-cover-declarations-and-interventions, 0e0feb85f).

## Fix

`goal resume G` by a person: when `StopFence` is set, clear it (and record the resume in the goal's history with the fence id and reason) whatever the State is; the goal resumes under its standing approved box (as the help text says); when neither a fence nor a stopped/parked state exists, keep today's "nothing to resume". No other verb changes. The custodian's breach-stop itself is unchanged (a follow-up design item: the elapsed clock pauses while a goal is queued or held in the landing lane).

## Tests

- A claimed goal with a StopFence: `goal resume` clears the fence and records it; `goal done --reason` then succeeds (mutation: skip the clear -> done refused, test fails).
- A claimed goal without a fence: "nothing to resume" unchanged.
- A stopped goal with a fence: resume behaves as today (fence cleared, state resumed).
- Run every existing test touching resume, breach-stop, StopFence and clearClaimBinding (grep, by name): `go test -count=1 -timeout 30m ./internal/goal` and `./cmd/metasystem -run 'TestGoalResume|TestResume|TestBreach|TestStopFence|TestGoalDone|TestClaimBinding|TestGoalStop'`; `go run ./cmd/devgate static`; `./internal/audit`.

Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat, each exit, the resume output text for the three cases.
