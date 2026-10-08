# Brief: lane-reads-its-policies U2a, correction 1

Working Mode: Implement
U2a is uncommitted in this worktree (U1 committed at 736700f14). One Opus read found three material defects; also fix one carried from U1. Fix exactly these.

1. internal/landing/plain/stop.go:256: RecordBarrenStop treats a stop as a duplicate by loop and evidence only, so an open barren stop recorded before U2a (no Required) keeps Command() falling back to `metasystem landing run` (stop.go:43), which U2a now refuses; the refusal at cmd/metasystem/intent_landing.go:979 prints a nil next. Treat it as a duplicate only when slices.Equal(last.Required, s.Required); that also refreshes a stale `prove --trunk` or `--goals` list. Test: an old barren stop without Required, a keeper tick, the printed command is the new one and following it works (mutation: the old duplicate rule, red).
2. stop.go:264: SyncStopQuestion is a wrapper with no production caller; delete it and have the tests call SyncPolicyQuestion.
3. Size (the unit is ~3.3x its 210-line estimate): remove the legacy single-id parsing in the stop-question index (policy_question.go ~165-180), the reason computation on withdrawal (policy_question.go:128-149) unless a test of the design needs it, and the duplicated RecordedPersonBatch re-checks inside SelectBatch and checkBatchLocked where the keeper's Prepare already checks (keep one check at the owner the design names).
4. Carried from U1: TestEveryStatefulActionRepeatsAsSuccess/landing_push fails (landing_plain_verbs_test.go:432): a repeated `landing push` rewrites .../landing/batch.json. A repeat whose effect holds must write nothing (R-129): make the push's batch write conditional on a change. 

Check: go build ./... && go vet ./... && go test -count=1 -timeout 30m ./internal/landing/... ./internal/channel/ && go test -count=1 -timeout 60m -run 'TestLanding|TestWorkLand|TestIntent|TestGoal|TestKeeper|TestIncident|TestHelm|TestPolicy|TestQuestion|TestPlainLane|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, each test with its mutation, and the production line count after the cuts.
