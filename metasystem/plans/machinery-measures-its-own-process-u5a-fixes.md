# Brief: machinery-measures-its-own-process U5a-fixes (tests for a fix already made)

Working Mode: Implement
U5a is committed (f3353f593) and stopped with one material finding: retainPublication (cmd/metasystem/intent_unit_review.go ~:885, also reached from intent_delivery.go:1250-1252) stamped the publication time for every push state but `current`, including `reconciled` (internal/goal/branch/push.go:318, :343: settling an earlier call's push after a crash) and `adopted` (:399, :426, :437: the remote already held the tip), where this call did not push. m1e already made the fix (uncommitted): stamp only when published.State == "pushed". Do not change production code unless a test shows the fix is wrong.

Write tests only: extend TestProcessPublicationPublicReview with `reconciled` and `adopted` cases (run and commit routes): the finish stays unavailable; `pushed` stamps once. Mutation: the old rule (!= current) -> red. Also add the missing grant-label case to TestProcessCostPublicReport: an act with Proof.Helm set is shown as the agent's (mutation: drop `&& act.Proof.Helm == nil` at intent_process_measure.go:143 -> red).

Check: go build ./... && go vet ./cmd/metasystem/ && go test -count=1 -run '^(TestProcessPublicationPublicReview|TestProcessCostPublicReport)$' ./cmd/metasystem/. Every new test calls t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and each mutation result.
