# Correction brief: health-is-green-when-the-seat-is-healthy, unit board-follows-the-lane, revise 1

Goal state: claimed by m1l, tier 2. The landing lane returned the goal's hand-in 0d29a1effaca at 22:11 on 2026-10-04 for one deterministic red in this unit's own test. One correction in this unit's chain; unit stuck-detector is not touched.

# Goal

Make the unit's new board test obey the project's wall-time rule without weakening what it proves.

# Workspace

Branch goal/health-is-green-when-the-seat-is-healthy, in its existing worktree, at 0d29a1effaca. Leave the change uncommitted.

# The red to fix

`TestUpdateDecidesOnTheCardUnderTheLock` in `metasystem/internal/board/card_test.go` waits for the second writer to reach the held seat lock with a wall-clock deadline: `deadline := time.Now().Add(5 * time.Second)` and `time.Now().After(deadline)`. `TestNoTestWaitsOnWallTime` (`metasystem/internal/testenv/walltime_test.go`) refuses every reading of the wall clock as a deadline ("0 allowed, 1 found"), and the proof runner's inventory test fails as a consequence of the same line. Rule (Wido): artificial clocks, never wall time in tests; a test waits on the event instead.

Fix: remove both wall-clock calls. Keep the test proving what it proves today: a writer that changed the card while another writer waited for the seat lock is never overwritten, because `Update` decides on the card as it stands under the lock. Either wait on the event itself (the second writer seen waiting at the lock, with no time bound of its own; the test binary's timeout bounds a hang), or restructure the test so no concurrency wait is needed while it still fails when `Update` decides on a card read before taking the lock. Choose the smaller change and say which in the report. Change no production code and no other test.

# Constraints

Behaviour tests stub Git; no `t.Setenv`; never read `metasystem.conf.local`. Do not add a walltime allowance. Source comments state behaviour in plain English, never review rounds or history.

Maximum reader tool calls: 30

# Expected Return

The change, uncommitted, with the mutation check (make `Update` decide on a card read before it takes the lock, see the test fail, restore) and the proof commands with their results.

# Acceptance Criteria

- `go test -count=1 -timeout 30m -run 'TestNoTestWaitsOnWallTime' ./internal/testenv/` passes.
- `go test -count=3 -timeout 30m -run 'TestUpdateDecidesOnTheCardUnderTheLock$' ./internal/board/` and `go test -count=1 -timeout 30m ./internal/board/` pass.
- `go vet ./internal/board/` passes.

# Gap Rule

If this brief does not say what to do, choose the smallest test-only change and say so in the report.
