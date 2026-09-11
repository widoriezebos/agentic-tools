Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 6: the contention row escalates its competitors until the share is below one half

Chain phd-build1-20260910. Round 5 is green on the seat and the
handshake works; the loaded phase of
TestSupervisorRunawayUnderMeasuredContention still skipped: "measured
0.52, need below one half". Two spinners per online processor are not
enough on the fleet's Macs: the seat measured, with hand-shaken
competitors, a share of 0.45 to 0.52 at two per processor, 0.24 at four
and 0.10 at eight (18 processors). Report `round` as 6. Do not fetch or
rebase the worktree.

## Decision D-R6-1 (the designer's correction; it amends revision 3)

The contention rows start two competitors per online processor, measure
the share (500 ms, after the handshake), and if it is at or above one
half they double the competitors and measure again, up to eight per
processor. Only when eight per processor still leave the share at or
above one half does the row skip by name, with the last measured share
and the competitor count in the message. Every competitor stays alive
until the row ends. The precondition is still measured, never assumed,
and no clock enters the outcome.

## Mandate for this round

1. Implement D-R6-1 in supervisor_test.go (the fixture only).
2. Run the row with -v on your side (it skips at the ps check in the
   sandbox; say so); the seat gate runs it for real.
3. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 on
internal/proofrun and internal/testpolicy, green. Report the round as
your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Never
delete written work.
