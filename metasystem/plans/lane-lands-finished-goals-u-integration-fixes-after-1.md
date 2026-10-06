# Brief: integration-fixes, regression test only

Working Mode: Implement
m1e has already fixed the defect (uncommitted in this worktree, `git diff cmd/metasystem/landing_plain.go`): the "newest entry returned" rule moved from latestLaneGoalEntry into claimLaneReader only, so work land's land-once check (`intent_delivery.go:1989-1996`) again sees a landed goal. Add exactly one regression test, no production change: using the admission bed (`admissionBed("lane")` in the cmd/metasystem tests), land the goal's code on main, hand in a records entry for it, return that records entry, then `work land` with a new unit commit is refused GOAL_LAND_ONCE and the queue gains no entry (mutation: put the newest-returned early return back into latestLaneGoalEntry, red). Also assert `goal claim` of another goal is still refused for this goal.

Check: go vet ./cmd/metasystem/ && go test -count=1 -timeout 30m -run 'TestGoalClaim|TestClaim|TestWorkLand|TestLandOnce|TestAdmission|TestLanding|TestIncident' ./cmd/metasystem/ && go run ./cmd/devgate static

t.Parallel(); -timeout 30m. Do not touch memory/ or records/. Leave uncommitted. Return the exits, the test name and its mutation result.
