# Brief: read-publication-2-fixes, correction 2 (one missing test)

Working Mode: Implement
The fix unit is uncommitted; the code is correct. Its narrow read found mutation M3 survives: setting `refuse := false` in the publication check (cmd/metasystem/intent_unit_review.go, the `check` closure ~:958), so the saved Retired flag is ignored; under it a retired read with a dirty tree, or a retired read after HEAD is reset back to the reviewed commit, would publish unread bytes; no test exercises the flag alone. Do not change production code.

Add one test next to TestCommitReadPublicationRetirementGitAdapter: retire the read (the hand2 overwrite), then (a) reset HEAD back to the reviewed commit and repeat: refused; (b) with the read retired, dirty the tree and repeat: refused. Mutation: `refuse := false` -> red.

Check: go build ./... && go vet ./cmd/metasystem/ && go test -count=1 -timeout 15m -run '^(TestCommitReadPublicationRetirementGitAdapter|TestCommitReadPublicationDirtyTreeRecoveryGitAdapter|<your new test>)$' ./cmd/metasystem/. t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
