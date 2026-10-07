# Brief: lane-drain-and-fresh-claims U1a, correction 1

Working Mode: Implement
The unit is uncommitted in this worktree. One Opus read found three material defects; fix exactly these.

1. internal/landing/plain/queue.go:270: HandIn now reads the queue through queueMembership, which errors on any undecodable line, so one damaged line refuses every hand-in even with no drain (and the printed retry cannot succeed). Use Entries (which skips undecodable lines) for the repeat check again; consult membership only while a drain stands, and never block a person on it. Test: a damaged queue line, no drain, a hand-in is admitted (mutation: membership on every hand-in, red).
2. cmd/metasystem/intent_landing.go:790: startLane runs lane.RepairAgentRecord only when personErr == nil, so an agent following "run: metasystem landing start" (internal/landing/lane/agent.go:468) on an unreadable keeper record gets "already running" and nothing is repaired. Run RepairAgentRecord after readiness for every caller; keep the person check only on clearing a pause or a drain. Test: an agent's landing start on an unpaused, undrained lane with an unreadable keeper record repairs it (mutation: person-only repair, red).
3. internal/landing/plain/queue.go:210: entriesOf now supersedes an older waiting line only when its Records flag matches, which changes what is waiting for every queue reader and is outside the brief. Remove that change; make TestLandingDrainRecordsHandInRetriesExactOptions use a separate goal for its records line.

Check: go build ./... && go vet ./cmd/metasystem/ ./internal/landing/... && go test -count=1 -timeout 30m ./internal/landing/... && go test -count=1 -timeout 30m -run 'TestLanding|TestWorkLand|TestAudit|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static
t.Parallel(); never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, each test with its mutation.
