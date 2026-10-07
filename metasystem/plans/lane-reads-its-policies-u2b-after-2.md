# Brief: lane-reads-its-policies U2b, correction 2 (the last)

Working Mode: Implement
U2b is uncommitted in this worktree. The second read found one material defect; fix exactly this.

cmd/metasystem/intent_landing.go:1012 (the check under the queue lock) with intent_landing_prove.go:171 and internal/landing/plain/prove.go:252: under a pause, any caller sending the recorded attempt id with matching --gate/--trunk is admitted; the attempt id is readable from running.json, so an agent can replay the person's admitted proof (after the child died, checkState returns the record unchanged and the replay re-runs a full proof; while it lives, a second concurrent execution starts). Admit only when running.Pid and Process match the caller's own process (processRef(os.Getpid())); a dead record gets its lost-process red result and a replay is refused naming the `landing prove` person remedy. Test: a person's prove under a pause whose child died; an agent replays the child argv and is refused (mutation: drop the identity check, red); the real child still runs.

Check: go build ./... && go vet ./... && go test -count=1 -timeout 30m ./internal/landing/... && go test -count=1 -timeout 60m -run 'TestLanding|TestWork|TestIntent|TestGoal|TestKeeper|TestHelm|TestPolicy|TestQuestion|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, the test with its mutation.
