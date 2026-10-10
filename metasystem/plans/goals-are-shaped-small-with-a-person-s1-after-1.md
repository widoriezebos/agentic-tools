# Brief: goals-are-shaped-small S1, correction 1 (test for a fix already made)

Working Mode: Implement
S1 is uncommitted in this worktree. Its read found `goal design write` (cmd/metasystem/intent_design.go ~248) let a split parent with a review budget start a paid design author (RequestDesign, internal/launch/design_request.go:158-200, has no state check). m1e added a case refusing goal.StateSplit with the `goal split G --reverse --reason TEXT` remedy, as intent_planning.go:1770 does. Do not change production code unless the test shows the fix wrong.

Write the regression test only: a split parent with a review-round budget: `goal design write G` is refused, names the reverse remedy, and starts no design author. Mutation: remove the case -> red.
Check: go build ./... && go vet ./cmd/metasystem/ && the new test and S1's two tests by name. t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
