# Brief: declared-check-fixes, correction 1 (one missing test)

Working Mode: Implement
The fix unit is uncommitted; its code is correct. Its read found the "never overwriting" guard (cmd/metasystem/intent_work.go:503, a collected builder file that already exists is skipped) untested: forcing it false leaves TestIntentDeclaredCheckLaterBuilderCannotReplaceRetainedEvidence and TestIntentDeclaredCheckBuilderRetainsWithUnwritableRound green, because nothing collects round 1 again after the builder's file changes (production can: a resume re-collects every round and the current round still matches, internal/launch/unit_run.go:250). Do not change production code.

Extend TestIntentDeclaredCheckLaterBuilderCannotReplaceRetainedEvidence: after the worktree record changes in the same round, collect again (resume the run or start a second step) and assert the collected builder- copy is unchanged. Mutation: the guard always false -> red.

Check: go build ./... && go vet ./cmd/metasystem/ && go test -count=1 -timeout 15m -run '^TestIntentDeclaredCheck' ./cmd/metasystem/. t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
