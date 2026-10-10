# Brief: declared-check-2, correction 1 (test for a fix already made)

Working Mode: Implement
declared-check part 2 is uncommitted. Its read found the correction carry (`work review` after an amend, cmd/metasystem/intent_unit_review.go:432) built its CarryRequest without SubjectCheck, so internal/goal/branch/rebase.go:345 passed nil, read.go:213 skipped the subject check and read.go:238 reused a cached GateRunID or ran the static gate (go-gate-fast) for carried later units. m1e made the fix (uncommitted): SubjectCheck: conn.subjectCheck on that request, as `work rebase` does (intent_work_rebase.go:131). Do not change production code unless the test shows the fix is wrong.

Write the regression test only: a correction carry (amend an earlier unit through work review) into a later unit whose subject journal holds a cached gate: the carried unit's attestation records the unit-check execution of its own subject commit, not the cached gate or go-gate-fast. Mutation: drop SubjectCheck at :432 -> red.

Check: go build ./... && go vet ./cmd/metasystem/ && the new test and the existing carry tests by name. t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
