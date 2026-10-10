# Brief: review-chain-stops-and-records build-outcomes-b1-fixes (test for a fix already made)

Working Mode: Implement
build-outcomes-b1 is committed (be2c83595) and stopped with one material finding: in internal/launch/unit_step_failure.go the replay check (same RetryBy and RetryReason) ran before the failed-step check, so when a step is held for environment a second time after a person's retry, the person's printed act `work review ... --reason TEXT --by NAME` with the same reason did nothing (exit 1, the same act printed again). m1e already made the fix (uncommitted): a replay returns early only when the step is no longer failed or its RetryLaunch is its current LaunchID. Do not change production code unless the test shows the fix is wrong.

Write the regression test only (next to the existing environment-retry tests in cmd/metasystem, the critic's scenario): the tree moves on every run; the person act is given twice with the same reason "network repaired"; after the second act the step re-runs (a fourth execution) and no build launches; repeating the second act before the new launch fails changes nothing. Mutation: the old check (person and reason only) -> red.

Check: go build ./... && go vet ./internal/launch/ ./cmd/metasystem/ && the new test and the existing environment-retry tests by name (-run '^(...)$'). Every new test calls t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the test with its mutation result.
