# Brief: review-drops optional-committed-unit-drop-fixes (test for a fix already made)

Working Mode: Implement
The unit is committed (b1987b4bd) and stopped with one material finding: the early drop route (cmd/metasystem/intent_unit_review.go ~:228) fired on any saved examination whose stop decision is stop, including stops for unknown inputs, so the offered `work review G --work U --retry N` (intent_unit_stop.go:62) was refused at intent_unit_stop.go:58-60 before the fresh examination launch (intent_unit_review.go:484-485). m1e made the fix (uncommitted): the stop arm requires --dispositions. Do not change production code unless the test shows the fix is wrong.

Write the regression test only: a committed unit stopped for unknown inputs; `work review G --work U --retry 1` launches its one fresh examination (assert the launch); mutation: the old condition -> red. The critic's probe (scratchpad/crit2-drop/.../zz_crit_retry_test.go) could not tell the two apart because its fixture launched no examination: make the fixture launch one.

Check: go build ./... && go vet ./cmd/metasystem/ && go test -count=1 -timeout 15m -run '^(TestIntentContinuingUnitRefusesDropAndSplitBeforeRetention|TestIntentRoundTwoCloseWithDispositionsCollectsExamination|TestWorkReviewDropsOptionalCommittedUnit|<your new test>)$' ./cmd/metasystem/. t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
