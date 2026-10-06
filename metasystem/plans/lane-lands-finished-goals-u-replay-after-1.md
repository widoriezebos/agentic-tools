# Brief: lane-lands-finished-goals, unit replay, correction 1

Working Mode: Implement
The unit is uncommitted in this worktree. One Opus read found two material defects; fix exactly these.

1. F-1: `cmd/metasystem/intent_landing.go:782-788` (runIntentLandingRun) calls CloseProofLoop before keeper.Run and for any caller; the landing agent, refused after two counted full proofs and told to run `landing run`, reopens its own budget even when the run ends AgentRunning, and its next `landing prove` starts a third full check. Fix: close the loop only for a person's call (the caller's lineage is not the landing agent's; use the lane's existing agent-lineage check), and name that in the refusal. Test through `landing run` then `landing prove` as the landing agent after two counted proofs: the third start is refused (mutation: close the loop for any caller, red); a person's `landing run` reopens it.
2. F-2: `internal/landing/plain/replay.go` replayBatch reads each merge's parent (`%H %P`, fields[1]) and discards it; nothing checks the first merge's parent is the origin/main commit used as tree 0, nor each later merge's parent is the previous tree. A main that moved during the proof then yields `own` for main's defect. Fix: require fields[1] to equal the previous tree's commit (origin/main for the first merge); on a mismatch the cause is `unclassified` (hold and ask). Test: a batch whose first merge's parent is not the current origin/main classifies `unclassified`, never `own` (mutation: drop the parent check, red).

Check: go build ./... && go vet ./cmd/metasystem/ ./internal/landing/... && go test -count=1 -timeout 30m ./internal/landing/... && go test -count=1 -timeout 30m -run 'TestLanding|TestAudit|TestEvery|TestVerbRatchet|TestRepository|TestFrozenPublic|TestHCL' ./cmd/metasystem/ && go run ./cmd/devgate static

Every new test calls t.Parallel(); -timeout 30m. Do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, the tests with their mutations.
