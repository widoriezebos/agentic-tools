Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 3: the Darwin cputime shape was specified wrong

Chain phd-build1-20260910. Round 2 finished the slice and its gate on the
seat is green (build, vet, gofmt, both packages, Linux cross-compile).
One row skipped on the seat itself, and the reason is a defect in the
specification, not in your work: the design said Darwin `ps -o cputime=`
prints [[dd-]hh:]mm:ss with one-second precision. It does not. On this
fleet's Macs it prints BSD `minutes:seconds.hundredths` with the minutes
unbounded and no hours field: `0:00.00` for a fresh shell, `137:14.32`
for launchd. `parseCPUTime` therefore returns `invalid cputime seconds
"00.01"` for every member, the platform reader is unavailable on the
host the fleet runs on, and in production three such samples would end
every Darwin group `invalid`. Report `round` as 3. Do not fetch or
rebase the worktree.

## Decision D-R3-1 (the designer's correction; it amends revision 3)

The Darwin reader parses `[dd-][hh:]mm:ss[.ff]`: an optional day prefix,
two or three colon-separated fields, the last field seconds with an
optional fractional part of any length, and when only two fields are
present the minutes field is unbounded (BSD ps prints total minutes).
Linux keeps [dd-]hh:mm:ss from /proc ticks, unchanged. Precision on
Darwin is hundredths, so the "rose by at least one second" rule of the
window keeps its meaning; nothing else about the verdicts changes.

## Mandate for this round

1. Fix `parseCPUTime` (or split a Darwin parser from the Linux one if
   that is cleaner) per D-R3-1; keep the existing accepted shapes.
2. Extend the Darwin parser test with the real shapes: `0:00.00`,
   `0:00.01`, `137:14.32`, `12:34.5`, `1-02:03:04.50`; and a rejection
   for a shape with a fractional minutes field.
3. On the seat the reader is available, so the contention row
   (TestSupervisorRunawayUnderMeasuredContention) will now reach its
   measured precondition; make sure that when the precondition holds the
   row asserts the runaway verdict, and that its skip message names the
   measured share when it cannot. Run it with -v and report the outcome
   line as evidence (it is expected to run, not skip, on a Darwin host
   where ps is allowed; the delegate sandbox may still deny ps, in which
   case report the skip line and say so).
4. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 on
internal/proofrun and internal/testpolicy, green; the -v line of the
contention row. Report the round as your own.

## Constraints

Wall-clock budget: 20 minutes. Return per the implementer schema. Never
delete written work.
