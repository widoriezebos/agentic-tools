Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 5: the contention row measures its share before the competitors spin

Chain phd-build1-20260910. Round 4 is green on the seat: both packages
pass, the quiet phase of TestSupervisorRunawayUnderMeasuredContention
reaches `runaway` by consumption. Its loaded phase skipped on the seat:
"host could not establish the scheduling-share precondition: measured
0.77, need below one half". The cause is in the fixture, not the host:
`startBusyCompetitors` re-executes the test binary twenty times in
sequence (each spawn is a Go runtime and testing-package start, tens of
milliseconds), sleeps 100 ms, and the row then measures its own share
over 250 ms while most competitors have not reached their spin loop.
The measured share is inflated and the row skips on every Mac, and in
the receipt a skipped expected test fails its group, so the landing
would be refused on the fleet's own hosts. Report `round` as 5. Do not
fetch or rebase the worktree.

## Decision D-R5-1 (the designer's correction; it amends revision 3)

The contention rows establish their precondition by handshake, not by
sleep: a new helper mode `busy-competitor` writes one line `spinning` to
its stdout the instant it enters its loop and then spins; the fixture
starts all competitors, reads that line from every one of them (a
competitor that exits or closes its pipe before the line is a fixture
failure, named), and only then measures the row's own scheduling share
over 500 ms of spinning. The skip-by-name when the share is still at or
above one half stays; with equal-priority spinners at two per online
processor the expected share is about one third.

## Mandate for this round

1. Implement D-R5-1 in supervisor_test.go and the helper; keep the
   `ignore-quit-busy` mode as it is for the SIGQUIT row.
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
