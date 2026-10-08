# Brief: lane-drain-and-fresh-claims, unit gate-fixes (the full gate's reds)

Working Mode: Implement
The goal branch (U1a, U1b) is complete; its full cmd gate has three reds, all about registering the new public verb `landing drain`. Fix exactly these, registering the verb the way the repository registers every public action (no test weakened):
--- FAIL: TestEveryPublicActionHasAnIdempotencyRow (0.06s)
    intent_idempotency_test.go:71: "landing drain" has no idempotency row: register it as read, creation (with why a second call is not a repeat) or stateful (with a witness that runs it twice) in an intent_idempotency_*
--- FAIL: TestIntentObjectHelpGroupsEveryActionOnce (1.95s)
--- FAIL: TestIntentObjectHelpGroupsEveryActionOnce (1.95s)
    intent_help_test.go:457: landing drain is in 0 intent groups
    intent_help_test.go:462: landing help lists actions no intent names:
--- FAIL: TestIntentPublicCoverage (15.27s)
    --- FAIL: TestIntentPublicCoverage/public_table (0.37s)
        intent_coverage_test.go:85: landing drain: summary "close automatic admission, finish queued work and hold", 1 usage lines, 0 examples; all are required
(TestWorkRebaseGitAdapterHoldsAfterHistory also failed: the known 4 s ledger-fetch load flake, not this goal's.)

Check: go build ./... && go vet ./... && go test -count=1 -timeout 30m -run 'TestEveryPublicAction|TestIntentObjectHelp|TestIntentPublicCoverage|TestLandingDrain|TestHelm|TestAudit|TestInstruction|TestIntentHelp' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any metasystem.conf.local; do not touch memory/ or records/. Leave uncommitted. Return the exits and git diff --stat.
