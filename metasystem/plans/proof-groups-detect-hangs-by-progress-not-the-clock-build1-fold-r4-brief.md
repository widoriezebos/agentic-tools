Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 4: the one-second rise rule does not scale to the test window

Chain phd-build1-20260910. Round 3 fixed the Darwin reader; on the seat
the reader is now available and the contention row runs. It fails, and
again the specification is at fault, not your code: revision 3 says
consumption over the window means the high-water counter rose by at
least one second at some sample. At the production window (thirty
minutes) that is a low bar. At the row's shortened window (400 ms) a
single-threaded busy child cannot rise one CPU second in 400 ms of wall
time, so the quiet phase of TestSupervisorRunawayUnderMeasuredContention
ends `dead` ("no CPU or output progress for 400ms", CPUSeconds 0.43)
before the one-CPU-second budget can make it `runaway`. Under contention
the same rule would need a window of one second divided by the child's
share, which is load dependent and forbidden. Report `round` as 4. Do
not fetch or rebase the worktree.

## Decision D-R4-1 (the designer's correction; it amends revision 3)

The rise that counts as consumption is a fraction of the window, not a
fixed second: rise = max(10 ms, window / 1800). At the production window
that is exactly one second, so production behaviour is unchanged; under
the one test hook a 400 ms window needs a rise of one platform tick
(10 ms), which any process with a scheduling share above 2.5 percent
produces. The same threshold governs the dump ladder's "the counter
still rises at the next sample". It is derived from the window, so the
hook stays a pair (budget, window) and the production-pair test still
asserts the pair; add an assertion that the derived rise is one second
for the production window and 10 ms for a 400 ms window.

## Mandate for this round

1. Implement D-R4-1 in the supervisor (the two comparisons against 1
   at the zero-consumption base and in the dump rule; the progress
   rule string may name the rise, e.g. `rise/1s`).
2. Re-run TestSupervisorRunawayUnderMeasuredContention with -v on your
   side; in the delegate sandbox it will skip at the ps check, which is
   expected there; the seat gate runs it for real. Make sure every other
   assertion of that row is decided by consumption, not by the clock:
   the loaded phase's wall comparison is acceptable only because the
   measured share precondition guarantees it; say so in a comment above
   it in plain words.
3. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 on
internal/proofrun and internal/testpolicy, green. Report the round as
your own.

## Constraints

Wall-clock budget: 15 minutes. Return per the implementer schema. Never
delete written work.
