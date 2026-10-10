# Brief: briefs-carry-their-rules B4, regression test for a fix already made

Working Mode: Implement
B4 is uncommitted; m1e fixed internal/launch/unit_brief_evidence.go (~46-50): a dispositions file already headed `## Decisions on round N` was given a second header, so reviseDecided (unit_revise.go:301-308) read an empty section and refused "finding ... has no decision". A headed file now supplies its own section. Do not change production code unless the test shows the fix wrong.
Write the regression test: the "decisions and proof" bed fed "## Decisions on round 1\n" + the decisions table passes like the unheaded table; a round-2 header for round 1 is still refused. Mutation: prepend the header to a headed file -> red.
Check: go build ./... && go vet ./internal/launch/ && the new test and B4's tests by name. t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
