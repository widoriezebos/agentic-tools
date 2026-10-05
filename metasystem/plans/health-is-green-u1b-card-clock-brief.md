# Brief: health-is-green-when-the-seat-is-healthy, unit card-clock (no wall-clock call in the card test)

Working Mode: Implement
Goal state: claimed under the seat lineage on m1l, tier 2, box 1d/10/1200m/1/20; coordinated by m1e in Wido's word (2026-10-04 22:25). One committed read (Claude Opus 5.5); no per-round read.

# Goal

The lane returned this goal's hand-in (units board-follows-the-lane and stuck-detector, tip 0d29a1effaca) for one deterministic red: `internal/testenv` `TestNoTestWaitsOnWallTime` finds `time.Now().Add` at `internal/board/card_test.go:52` (`TestUpdateDecidesOnTheCardUnderTheLock`), the only wall-clock call in both units' new tests; `internal/proofrun`'s inventory test fails as a consequence. The fix is already in the worktree, STAGED: the test's deadline no longer comes from the wall clock. This unit verifies and completes that change as a unit of its own on top of the two read units, so their reads stay valid.

# Workspace

The goal worktree of health-is-green-when-the-seat-is-healthy, branch goal/health-is-green-when-the-seat-is-healthy at 0d29a1effaca, with the staged change in `metasystem/internal/board/card_test.go`. Leave the change uncommitted (staged or not).

# Units

| Unit | Lines |
| --- | ---: |
| card-clock | 20 |

## What this unit does

1. Read the staged diff (`git diff --cached`). It must remove every wall-clock call from the test and take the deadline from the test's injected clock or a fixed instant, as the package's other tests do. If it does not, make it so; change nothing else.
2. Run: `go test -count=1 -timeout 30m ./internal/board/ && go test -count=1 -timeout 30m -run 'TestNoTestWaitsOnWallTime' ./internal/testenv/ && go test -count=1 -timeout 30m -run 'TestTestEnvironmentStandardInventoryMatchesObserved' ./internal/proofrun/ && go run ./cmd/devgate static`.
3. Report the exits and `git diff --cached --stat`.

## Not in this unit

Any change outside `internal/board/card_test.go`; any rebase.

# Constraints

Go only; no wall-clock calls in tests; every test keeps `t.Parallel()`; no receipts, memory or records files. `-timeout 30m` on every go test line.

Maximum reader tool calls: 15

# Expected Return

The exits, the stat, and one sentence on where the deadline comes from now.

# Acceptance Criteria

- The four commands in step 2 exit 0.
