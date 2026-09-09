Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-refusal-fits-on-one-screen)
Date: 2026-09-09

# Fold brief: round two of the bounded Stop refusal (chain one-screen-build1b-20260909)

You built round one of this chain. The critic one-screen-crit1b-20260909
reviewed it and returned three material findings and two notes; the
orchestrator's dispositions are in the records directory under misc,
in stop-refusal-fits-on-one-screen-critique-r1-dispositions.md (a new
record, not yet committed). Fold the four items below into the same
worktree; touch nothing else. The build brief you worked from is
stop-refusal-fits-on-one-screen-build-brief.md in the plans directory.

## OSR-01: the 200-run hook scenario must be able to fail

In metasystem/scripts/agents/supervision-hook-fixtures.sh, the scenario
that seeds bounded-run-001 to bounded-run-200 asserts only that the
reason is within 4,000 runes, names the stop-verdicts file, and that the
file exists. All three hold for a short refusal too. Add two
assertions: the reason contains the red-class summary line for two
hundred runs (the exact text your renderer prints for that class and
count), and the full-text file contains bounded-run-200. Keep the
existing three.

## OSR-02: an unclosed fence must not swallow the rest of a plan

In metasystem/internal/report/openwork.go, planField now toggles an
inFence flag on each fence line and never reconciles at end of file, so
a plan whose last fence is unclosed loses every header field after it
and drops out of the refusal silently. When the scan reaches end of
file still inside a fence, re-read the file as if it had no fences (the
behaviour before this chain), so no real field is lost. Add the case to
metasystem/internal/report/openwork_test.go: a plan with one opening
fence and no closing one, whose real "Next step" comes after the fence,
is still reported.

## OSR-03: the system-message bound reads one rune past the first line

In metasystem/internal/report/stopblock.go, BoundSystemMessage takes
the over-long-first-line branch when available is below the first
line's length, and then keeps limit minus notice minus 1 runes of a
line that is shorter than that: at a first line of exactly 3,950 runes
it produces a 4,000-rune result ending in a NUL, and it is an
index-out-of-range panic whenever the rune slice has no spare capacity.
Clamp the kept length to the first line's own length. Add a test in
metasystem/internal/report/stopblock_test.go over first lines of 3,949,
3,950, 3,951, 3,952 and 3,953 runes: every result is within the bound,
contains no NUL, and does not panic.

## OSR-04: the first three recorded continuations always print

The orchestrator settles the disagreement between the two briefs: a
recorded green continuation is an instruction to the seat, so the
bounded display prints the FIRST THREE in full whenever any exist, and
the greens count line names how many more are in the file. Today
summarizeGreens in verdictrender.go (beside turnverdict.go under
internal/goal) prints them only when the count is at most three, so a
fourth hides all four. Change it and its test: four continuations show
three lines plus a count naming one more.

## Not in scope

OSR-05 (the notice wording on the last-resort trim) is recorded and not
actioned. Nothing else in the ten files changes; no file outside them.

## Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/report/ -count=1` green;
`go test ./internal/goal/ -run 'TestTurnVerdict|TestRunWarning|TestGreen' -count=1`
green; say plainly which of the full package run, the coverage floors
(internal/goal 82.2 percent, internal/report 87.3 percent) and
`bash scripts/agents/supervision-hook-fixtures.sh` your sandbox could
run; the orchestrator replays the rest.

## Constraints

Wall-clock budget: 40 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 3). Declare the boundary as
every file that differs from main. Stop adding at a gap that needs a
decision no page has made; keep what is built and green; report the
gap with the resolution you propose. Never delete built work.
