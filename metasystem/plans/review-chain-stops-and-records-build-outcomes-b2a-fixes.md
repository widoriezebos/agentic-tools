# Brief: review-chain-stops-and-records build-outcomes-b2a-fixes (test for a fix already made)

Working Mode: Implement
b2a is committed (b19333578) and stopped with one material finding: in cmd/metasystem/intent_work.go the KnownFlake closure returned the flake judge's error, so a red whose unit is new on main (`git ls-tree origin/main:<unit>` exit 128) or whose unit id is not a path (repoproof's go-batchtest, fast-static-build, section group ids) was never compared with the base and stayed unclassified. m1e already made the fix (uncommitted): a judge error means "not a known flake" (false, nil), as at landing (internal/landing/plain/replay.go:43). Do not change production code unless the test shows the fix is wrong.

Write the regression test only, next to TestUnitBuildAttributesAffectedRegisteredFlake: a --check red whose failing unit is new on main (the judge's git read fails) with a green base is attributed own after the base comparison. Mutation: return the judge's error -> red.

Check: go build ./... && go vet ./cmd/metasystem/ && the new test and TestUnitBuildAttributes* by name. Every new test calls t.Parallel(). Never open any metasystem.conf.local; do not touch memory/, records/ or plans/. Leave uncommitted. Return the exits and the mutation result.
