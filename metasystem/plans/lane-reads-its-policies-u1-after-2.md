# Brief: lane-reads-its-policies U1, correction 2 (the last)

Working Mode: Implement
The unit is uncommitted in this worktree. The second read found one material defect; fix exactly this.

internal/landing/plain/batch.go:453-471 (the loop over queue entries in checkBatchLocked): batchEntries keeps the newest entry per goal AND commit, so an older line of the same goal at an older commit stays as superseded or returned; a fix-forward commit descends from it, the candidate contains it, and prove, gate and push refuse with a remedy that loops. In that loop check only each goal's newest entry (any commit); an older entry of a goal whose newest entry is selected never refuses. Test: re-hand-in at a descendant commit, in a superseded case and a returned case; run, merge the new commit, `landing prove --wait` and `landing push` pass (mutation: check every (goal, commit) entry, red). The existing supersession test keeps refusing a stale selection.

Check: go build ./... && go vet ./internal/landing/... ./cmd/metasystem/ && go test -count=1 -timeout 30m ./internal/landing/... && go test -count=1 -timeout 30m -run 'TestLanding|TestPlainLane|TestWorkLand|TestKeeper|TestIncident' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits, git diff --stat of the correction, the test with its mutation.
