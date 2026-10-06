# Brief: lane-lands-finished-goals, unit quota, correction 1

Working Mode: Implement
The unit is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

1. F-1: `cmd/metasystem/landing_plain.go:34-55` claimLaneReader reads the goal's newest lane entry through latestLaneEntry/plain.Latest, records entries included; after a return, `work land G --records` makes G read "waiting" and the quota opens on a returned goal. Fix: read through latestLaneGoalEntry (`landing_plain.go:200`, which skips records entries) or the same filter. Test in TestGoalClaimLaneQuotaBoundAndStates: queue G, return it, records hand-in of G, `goal claim` of another goal is refused naming G as returned (mutation: read records entries again, red).
2. F-2: `cmd/metasystem/intent_planning.go` claimGoal/acquireClaim (~1254, ~1294) and frontierPick (~1157) call claimLaneReader unconditionally; any lane read error refuses every claim on the computer, a person's and a claim with nothing held included. A lane answer can only free a slot. Fix: read the lane only for a goal claimQuotaRefusal would otherwise count; when the lane cannot be read, treat that goal as having no lane entry (today's rule: it holds the slot). No claim is refused because the lane is unreadable. Test: lane root returns an error, nothing held: `goal claim` succeeds; one goal held, lane unreadable: the agent's second claim is refused by today's quota words, not by a lane error (mutation: read the lane unconditionally, red).

Check: go build ./... && go vet ./cmd/metasystem/ ./internal/goal/... && go test -count=1 -timeout 30m ./internal/goal/... && go test -count=1 -timeout 30m -run 'TestGoalClaim|TestClaim|TestWorkLand|TestLanding|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL' ./cmd/metasystem/ && go run ./cmd/devgate static

Every new test calls t.Parallel(); -timeout 30m. Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, the tests with their mutations.
