# lane-one-state U3 correction 1 (Opus read of 7ecaeb9d5, 2026-10-13 03:35; 0 material, two cheap items folded)

Working Mode: Implement, on branch build/los3 (worktree /Users/wido/LocalStorage/GitHub/agentic-tools-m1e-los3), as uncommitted changes on top of 7ecaeb9d5 (folded into that commit by hand). Never commit, push, rebase or reset. Behaviour-preserving.

1. N-1: drop the behaviour nobody asked for — the `mergeOnly && rebuild` clause at fix.go:128 and the predicate's `rebuild` row made resume abort a merge on a `rebuild` fix record, which no production code writes; remove both and the test `TestResumeRebuildSharesDeployOutcome` that pins the new outcome (or turn it into the old outcome: resume keeps the merge when no record names it).
2. N-2: `stewardStep` (steward.go:49/109) decides the person bound through an `owner` closure parameter, which the audit cannot see; call `state.StopOwner` directly in `stewardStep`, delete the parameter, and add `stewardStep` to the audit's required-caller list so an inline copy there fails the audit.

## Checks
`go vet ./internal/landing/state ./internal/landing/plain ./cmd/metasystem`; `go test -p 4 -count=1 -timeout 30m ./internal/landing/state ./internal/landing/plain`; `go test -count=1 -run 'TestAuditLanePredicates|TestLandingSteward|TestLandingResume|TestLandingDeploy' ./cmd/metasystem`. One `go test -p 4` at a time, load below 20.
