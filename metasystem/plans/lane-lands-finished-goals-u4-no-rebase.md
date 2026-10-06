# Brief: lane-lands-finished-goals, unit no-rebase

Working Mode: Implement
Goal: lane-lands-finished-goals. Accepted design: plans/designs/lane-lands-finished-goals.md, Decision 2 ("`work land` never rebases") — read that section in full; it is the spec. Builds on units admission, next-step and boundary (committed on this branch). Line numbers in the design refer to main 9eb87cd15; locate each site by its function name on this branch.

# What this unit builds (from Decision 2)

1. `landGoalRoute` (cmd/metasystem/intent_delivery.go) loses the rebase before hand-in (`rebaseGoal` call), the lines it printed, and the re-read of the branch after it; the deferred block admission added for the rebase lines goes too. `cmd/metasystem/landing_rebase.go` (`landRebaseSkip`) is deleted. `work rebase G` (`cmd/metasystem/intent_work_rebase.go`) is unchanged.
2. A returned conflict: `work land G` on a goal whose newest lane entry was returned for a source conflict shows the return and names `metasystem work rebase G` as the one command (the design's two-line text). `returnedPathsResolved` (cmd/metasystem/landing_plain.go) and the rebase argument of `laneQueueState` go; `--again` remains for a return that needed no change.
3. The hand route: `work land` no longer rebases before the replay; the four refusals of the composition that say a commit no longer applies to main (internal/goal/branch/land.go, the preimage and apply refusals, design cites :272, :298, :678, :689) and the one that says the change would land as other changes than were read (:700) name `metasystem work rebase G` as their one command.
4. The pinned test `TestWorkLandHandsInOverRealGit` (cmd/metasystem/intent_delivery_owner_test.go): the three assertions that the tip was rewritten, contains main, and gained a rebase history line are replaced by their opposites (remote tip unchanged, queue line holds exactly that tip, no rebase line); the "a hand-in moved main" assertion stays and loses its `!behind` condition.
5. Tests in cmd/metasystem/landing_rebase_test.go that exercise the removed rebase are replaced by one test through `work land` on a branch behind main: the queued sha equals the branch tip and no commit was rewritten, on both the lane and the hand route (mutation: call the rebase again, red); and one test of the returned-conflict line naming `work rebase` (mutation: name `work review`, red).

# Not in this unit

Causes on returns, the replay, conflicts with batch members (later units); a merge on the hand route (Wido: not in this goal); plan, docs, receipts, memory or records files.

# Readers

`rebaseGoal` callers (grep; `work rebase` keeps its own); `landRebaseSkip` callers; `laneQueueState` callers; `returnedPathsResolved`; the hand route's `landCandidate`/`PrepareLanding` refusals (internal/goal/branch/land.go); tests: landing_rebase_test.go, intent_delivery_owner_test.go, landing_plain_handin_test.go, intent_delivery_test.go (`TestIntentLandRecovery` and the rebase cases), goal_progress_test.go.

# Check

`go build ./... && go vet ./cmd/metasystem/ ./internal/goal/branch/ && go test -count=1 -timeout 30m ./internal/goal/branch/ && go test -count=1 -timeout 30m -run 'TestWorkLand|TestWorkRebase|TestLandingPlain|TestHandIn|TestLanded|TestRecords|TestIntentDelivery|TestIntentLand|TestIntentManual|TestGoalProgress|TestGoalNextStep|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestLayout' ./cmd/metasystem/ && go run ./cmd/devgate static`

Known red inherited from admission, not this unit's: TestIntentManualWorkLandsOnEndpoint (GOAL_NO_END). Do not fix it here.

Constraints: Go only; no new dependencies; every new test calls t.Parallel(); -timeout 30m on every go test line; never weaken a test's intent; a test changed because the rule changed says so. Leave the change uncommitted. Return: the exits, git diff --stat, tests added and changed with the mutation each catches.
