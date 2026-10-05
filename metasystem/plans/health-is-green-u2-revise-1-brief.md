# Correction brief: health-is-green-when-the-seat-is-healthy, unit stuck-detector, revise 1

Goal state: claimed by m1l, tier 2. This is the one revise round of unit stuck-detector after its committed review code-critic-fa0262e1ba7a51f5b35d447b (fleet rule: one committed read per unit; a material finding gets one revise round in the same chain).

# Goal

Make the unit's own acceptance test pass on every run, without changing production code.

# Workspace

Branch goal/health-is-green-when-the-seat-is-healthy, in its existing worktree, on top of the unit commit b80285544. Leave the change uncommitted.

# The finding to fix (F-1, material, accepted)

`TestTickRunsStuckUnitsAfterHealth` in `metasystem/internal/steward/pattern_tick_test.go` fails every time: it builds its health bed with `newHealthBed(t, "observer", "")`, while every other caller passes `EnrollmentFixture` (`metasystem/internal/steward/health_bed_test.go`). Without the fixture enrollment the steward identity reads unknown, the seat reads unhealthy, and the bed's platform notifier fires, which the bed's cleanup turns into a failure (`health_bed_test.go`, the notifier check near line 158).

Fix: build that test's health bed with `EnrollmentFixture`, as the other callers do. Change no production code and no other test's assertion.

# Not in this round

The three non-material notes of the same review (N-1 the round message's remedy tail, N-2 one unreadable run freezing all stuck alerts, N-3 the second ledger read): they are recorded as goal notes, not fixed here.

# Constraints

Behaviour tests stub Git; no `t.Setenv`; never read `metasystem.conf.local`. Source comments state behaviour in plain English, never review rounds or history.

Maximum reader tool calls: 30

# Expected Return

The change, uncommitted, with the test's mutation check (drop the fixture enrollment again, see it fail, restore) and the proof commands with their results.

# Acceptance Criteria

- `go test -count=3 -run 'TestTickRunsStuckUnitsAfterHealth$' ./internal/steward/` passes.
- `go test -count=1 -run 'Tick|Pattern' ./internal/steward/` and `go test -count=1 -run 'UnitStanding|Stuck' ./internal/launch/ ./internal/steward/` pass.
- `go vet ./internal/steward/` passes.

# Gap Rule

If this brief does not say what to do, choose the smallest test-only change and say so in the report.
