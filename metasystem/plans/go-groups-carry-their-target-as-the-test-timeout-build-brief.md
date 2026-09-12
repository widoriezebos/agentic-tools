Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal go-groups-carry-their-target-as-the-test-timeout)
Date: 2026-09-10

# Build brief: a go unit group runs with the timeout its target declares

Goal go-groups-carry-their-target-as-the-test-timeout. The receipt's
go adapter in internal/proofrun/test_go.go runs a unit group as
go test -json -count=1 [-run ...] <packages> with no -timeout flag, so
every group sits on Go's default ten-minute limit whatever its
declared targetMs in metasystem/testing.json. Today on m1 the group
goal-full-coverage (targetMs 1800000, package internal/goal) failed a
landing receipt with "panic: test timed out after 10m0s" at 600.466 s
with every test that ran passing; the same package took 596 s in the
receipt that passed an hour earlier and 523 to 680 s in the seat's
gates. A candidate that touches internal/goal is refused by machine
load.

## Mandate

1. In the argv the adapter builds (goArgumentsCached and the function
   it calls), add -timeout <d> where d is the group's targetMs as a Go
   duration when targetMs is above Go's default of ten minutes; below
   or equal, pass nothing and keep the default. Place the flag before
   the package list, after -count=1.
2. Record the timeout the group ran with on the group's result record
   (the same record that carries argv, status and durationMs), so a
   reader of result.json sees it; an absent field means the default.
3. One unit test in internal/proofrun proving the argv carries the
   flag for a group with targetMs above ten minutes and not for one at
   or below, and that the record names it.
4. Nothing else changes: no contract change, no other adapter.

## Proof

go build, vet, gofmt; go test on internal/proofrun. Report the round
as your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
