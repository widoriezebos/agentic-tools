# Brief: review-chain-stops-and-records unit-stop-fixes (fix unit after the unit stop)

Working Mode: Implement
unit-stop is committed (99a0b73c3) and stopped by the stop rule with two material findings, both the printed remedy for an unreadable review.stop failing when followed. Fix exactly these.

F1. cmd/metasystem/intent_unit_stop.go:49 prints `metasystem settings set review.stop auto --by NAME`, but `settings set` takes no --by (exit 2, "does not take --by; ... Nothing was done"). Drop --by; settings set checks the person itself.
F2. intent_unit_stop.go:47-49 and :80, and the same wording at cmd/metasystem/intent_work.go:477, :481, always print `settings set`, but an environment override is read first (internal/config/policy.go:129-133) and settings set writes only metasystem.conf.local (intent_policy.go:322-345), so the refusal repeats after the printed act. Choose the remedy from PolicyReadError.Source exactly as runPolicyShow does (intent_policy.go:170-185): environment -> `unset METASYSTEM_...` with the exact variable name; a conf file -> settings set (local) or the file to edit (committed). One shared helper for all four sites.
Test (replace the seam in TestIntentReviewProceedsAfterPolicyRepair, intent_unit_stop_correction_test.go:131-152): corrupt review.stop the way an operator would (a) in metasystem.conf.local through a synthetic fixture file, (b) through the environment variable; run work review, take the printed command from the refusal, execute it through the real verb (or unset the variable for (b)), run work review again: it proceeds. Mutation: the old fixed remedy text -> red on (b); --by back -> red on (a).

Check: go build ./... && go vet ./... && go test -count=1 -timeout 30m ./internal/config/ ./internal/launch/ ./internal/loopstop/... && go test -count=1 -timeout 60m -run 'TestWork|TestIntent|TestReview|TestUnit|TestPolicy|TestSettings|TestEvery|TestAudit|TestInstruction' ./cmd/metasystem/ && go run ./cmd/devgate static
Never open any real metasystem.conf.local (synthetic fixtures only); do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits, git diff --stat, each test with its mutation.
