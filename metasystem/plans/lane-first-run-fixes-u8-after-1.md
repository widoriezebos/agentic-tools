# Unit u8 correction 1: a saved red without a failed set classifies from its log (D5)

Working Mode: Implement. Correction of unit u8 (brief `plans/lane-first-run-fixes-u8-detached-proof-reads-its-report.md`, D5). The worktree holds u8 uncommitted; change only what this brief names; leave fixes uncommitted.

## The gap

D5 was not built: `ClassifyAttempt` (internal/landing/plain/red.go ~48-200) replays the saved record as recorded. The real lane's saved reds (for example attempt 20261010T223414.252735000Z in the landing install) carry no `failed` set because of the defect u8 fixes, and `continueRed`/the replay (replay.go ~81) treats an empty failed set as unclassifiable, so `landing prove --classify ATTEMPT` on such a record changes nothing. `TestLandingClassifySavedLogWithRenderLines` uses a fixture record that already carries its failed set, so it did not cover this.

## Decision

- D5 In `ClassifyAttempt`, after the recorded red is found and before the replay: when `len(result.Failed) == 0` and `result.Log` names a readable file, read `FailedChecks(data)` of that log (the u8 parser tolerates the engine's render lines); when it yields units, set `result.Failed` to them and `result.Cause.Tests = failingTests(result.Failed)` (create the Cause when nil, kind unclassified, evidence the log). A log without a complete report leaves the record as it was (the replay then reports what it reports today). The appended record, the stop (`recordProofStop` → measure "red set") and the stop's class carry the recovered failed set.
- Test (cmd or plain fixture seams, no real check): a saved red record WITHOUT `failed` whose log holds `LANDING-FAILED\tu/a\tTestA`, `LANDING-CHECKED\t1` followed by the three render lines; `landing prove --classify ATTEMPT` as the person classifies it, the Judge seam receives unit u/a with TestA, the appended result carries `failed`, and the stop record's measure names TestA. Mutation: skip the recovery → the test fails.

## Checks

`go vet ./...`; `go test -count=1 -timeout 30m ./internal/landing/...`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLanding|TestClassify|TestProve|TestAudit'`; `go run ./cmd/devgate static`; `./internal/layering`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of the correction, each exit, the mutation result.
