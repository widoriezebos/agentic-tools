# Brief: lane-lands-finished-goals, unit cause-fixes (split from unit cause under the stop rule)

Working Mode: Implement
Unit cause is committed (da82900d8 on goal/lane-lands-finished-goals). Its final Opus read found one material defect; fix exactly this.

M-1: `cmd/metasystem/intent_landing_push.go:77-91` (the returned-in-HEAD refusal) counts every `returned` entry, but a returned entry stays returned forever (`internal/landing/plain/queue.go:178-210` `entriesOf` supersedes only waiting entries). A goal handed in again after a return (a fix-forward commit B on top of the returned A, or the same commit A again with --again) can then never land: every HEAD holding B holds A. Fix by subtraction: count a returned entry only while it is its goal's newest hand-in (the goal's latest entry's state is returned), so a newer waiting hand-in of the same goal ends the block. Extend `TestLandingPushReturnedCommitBoundaries` with the fix-forward and same-commit cases: return A, hand in B (or A again), rebuild HEAD with the new hand-in, push succeeds (mutation: count every returned entry again, red); the existing refusal (HEAD still holding a returned goal with no newer hand-in) stays.

Check: go build ./... && go vet ./cmd/metasystem/ ./internal/landing/... && go test -count=1 -timeout 30m ./internal/landing/... && go test -count=1 -timeout 30m -run 'TestLanding|TestWorkLand|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static

Every new test calls t.Parallel(); -timeout 30m. Leave uncommitted. Return the exits, git diff --stat, the tests with their mutations.
