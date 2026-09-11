Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 14: the landing gate's static check refuses the deliberate spin loops

Chain phd-build1-20260910. Round 13 is green on build, vet, gofmt, both
packages plain and under the race detector, and the Linux cross-compile,
but the landing receipt's fast-static-build group (bash
scripts/agents/go-gate.sh --fast) runs staticcheck, and it refuses:

    internal/proofrun/supervisor_test.go:694:3: this loop will spin, using 100% CPU (SA5002)
    internal/proofrun/supervisor_test.go:698:3: this loop will spin, using 100% CPU (SA5002)
    internal/proofrun/supervisor_test.go:703:3: this loop will spin, using 100% CPU (SA5002)

Those loops are the helper modes that must spin: they are the busy tree
the rows supervise. Report `round` as 14. Do not fetch or rebase the
worktree.

## Mandate for this round

1. Keep the loops spinning and tell staticcheck why: put a
   `//lint:ignore SA5002 <reason>` directive on the line before each
   deliberate `for {}` in the helper, with the reason in plain words
   (the helper is the busy process tree the supervisor rows judge).
   Do not add sleeps, yields or work to the loops; the rows measure
   consumption and a changed loop changes what they measure.
2. Nothing else changes.

## Proof

bash scripts/agents/go-gate.sh --fast from the metasystem directory
(this is the landing gate's own group: build, vet, gofmt, staticcheck,
fast tests), green; go test -count=1 on internal/proofrun, green.
Report the round as your own.

## Constraints

Wall-clock budget: 10 minutes. Return per the implementer schema. Never
delete written work.
