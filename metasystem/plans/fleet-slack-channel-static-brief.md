Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal fleet-slack-channel)
Date: 2026-09-03

# Goal

One static-check fold in the fleet conversation channel, the last round
of your fix build in the same worktree. Your working tree holds the 13
modified fix paths, the two modified test files and the new goal test
file from the previous round, all uncommitted and unstaged; leave every
one of them as it is. The landing wrapper's static re-proof ran
staticcheck 2025.1 over the tree and refused on exactly one line:

    internal/channel/phase/phase.go:50:3: this value of err is never used (SA4006)

In `load` there, the fake branch assigns `p, d, err = fake.Provider(dir)`
and never reads that error: the tick would proceed with a nil provider
after a fake that has no base-url (`ErrUnconfigured`) or any other
provider error. Fold: after that assignment, return the error the same
way the sibling branches return theirs (`return nil, d, "", "", "", err`
when it is non-nil). Nothing else changes.

# Gate

gofmt on the file, go vet ./internal/channel/..., go test -count=1
./internal/channel/... green, and staticcheck on that package
(`go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/channel/...`,
with `-buildvcs=false` in GOFLAGS if the sandbox's git state makes the
compile refuse) reporting nothing. Paste the final lines. Stage nothing
and do not run the commit wrapper; the orchestrator reads the working
tree. The diffBoundary is that one file.

# Constraints

Wall-clock budget: 15 minutes. Version-2 implementer JSON. No refusal
weakened, no benchmarks (R-31).

# Gap Rule

stop and report a gap; never fill it silently.
