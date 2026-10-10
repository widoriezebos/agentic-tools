# Brief: read-publication-2-fixes, correction 1 (test for a fix already made)

Working Mode: Implement
The fix unit is uncommitted in this worktree. Its read found: a stray uncommitted or untracked file in a path the unit never changed retired the read for good (cmd/metasystem/intent_unit_review.go ~:962 `refuse := record.Retired || dirty != ""`), and the printed remedy could never succeed after cleaning up. m1e made the fix (uncommitted): uncommitted changes hold publication without retiring the read ("commit or remove them, then repeat"); retirement stays for a committed change to reviewed paths. Do not change production code unless a test shows the fix is wrong.

Write the regression test only, in cmd/metasystem/intent_read_publication_2_test.go next to TestCommitReadPublicationRetirementGitAdapter: an untracked scratch.txt outside the reviewed paths, run the publication (refused, read not retired), remove it, repeat: it publishes (mutation: retire on a dirty tree, red); and the existing hand2 overwrite case still refuses on every repeat.

Check: go build ./... && go vet ./cmd/metasystem/ && go test -count=1 -timeout 15m -run '^(TestCommitReadPublicationLifecycleGitAdapter|TestCommitReadPublicationRetirementGitAdapter|TestManualReadPublicationReservationGitAdapter|<your new test>)$' ./cmd/metasystem/. Every new test calls t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
