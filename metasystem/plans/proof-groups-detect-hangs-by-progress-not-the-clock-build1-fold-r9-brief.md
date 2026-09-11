Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 9: nested helpers hand-shake upward

Chain phd-build1-20260910. The seat gate on round 8 is green everywhere
(plain and under -race, real reader, contention row included) except
TestSupervisorSectionResultGrowthCountsAsOutput: members=1 at the first
gated sample. The `stage-writer` helper starts its Setsid `stage-child`
and announces `ready` at once, without reading the child's own `ready`
line, so the fixture's first sample can run before the re-executed child
exists in the process table. Report `round` as 9. Do not fetch or rebase
the worktree.

## Decision D-R9-1 (the designer's rule; it amends revision 3)

Readiness composes: a helper that starts a nested helper reads the
nested helper's `ready` line before it announces its own. A fixture's
first sample therefore sees the whole tree the row is about. Apply it
to every nested mode (stage-writer, setsid-child, retained-child and any
other), not only the one that failed.

## Mandate for this round

1. Apply D-R9-1 in the helper modes; the section row's assertion (at
   least two members on the first gated sample; growth counted as
   output) stays.
2. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 and
go test -count=1 -race on internal/proofrun and internal/testpolicy,
green. Report the round as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Never
delete written work.
