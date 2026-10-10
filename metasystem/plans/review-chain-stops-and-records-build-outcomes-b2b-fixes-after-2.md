# Brief: build-outcomes-b2b-fixes, correction 2 (test for a fix already made)

Working Mode: Implement
The fix unit is uncommitted in this worktree. Its second read found that a pending unit-launch reservation whose launch never started (no record.json in the launch store) made the stop scan fail "unreadable" (internal/dispatch/stop.go ~:531), so the batch stayed INDETERMINATE for a breach stop and a person's `work stop`. m1e already made the fix (uncommitted): a not-exist launch record means a never-started reservation whose own status decides (pending when empty), so the cancel settles it. Do not change production code unless the test shows the fix is wrong.

Write the regression test only (next to the breach-stop test with a held unit build): start a held build, remove the launch's state directory (the critic renamed it), run the breach stop: it cancels the reservation and the batch completes; a person's `work stop` the same. Mutation: the old "unreadable" failure -> red.

Check: go build ./... && go vet ./internal/dispatch/ ./cmd/metasystem/ && the new test and the existing unit-launch stop tests by name. Every new test calls t.Parallel(); no wall-clock waits. Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
