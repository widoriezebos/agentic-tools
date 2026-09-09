Working Mode: build
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-refusal-fits-on-one-screen)
Date: 2026-09-09

# Goal

Goal stop-refusal-fits-on-one-screen (tier 3, approved by Wido on
2026-09-06; risk severity 2, novelty 1, exposure 3, accumulation 1, so
the chain lands without the full battery). Its record,
metasystem/plans/goals/stop-refusal-fits-on-one-screen.md, is the
contract. In short: the Stop hook's refusal text has no bound. Today
(2026-09-09, this seat, first Stop of the session) the refusal printed
32 run warnings from August ("looks hung", "ended ended-unknown", "went
red"), about 150 "finished green; the run record says: no continuation
recorded" lines, one TEMPLATE-UNFILLED line, three OPEN-WORK lines and
one IDLE WITH BACKLOG line, in that order, so the one line the seat has
to act on came last and the human could not find the conversation
between refusals. DONE means: the refusal reads, in order, the verdict
in one line, the actionable items (each with the command that clears
it), then at most a few lines of everything else summarised by class
and count; the whole thing bounded to roughly a screen; the full
unbounded text written to a file under the checkout's supervision
artifacts, and the refusal names that file. Not a change to what is
judged, only to what is printed.

This chain ALSO carries the fenced-code scanner fix that goals
plan-fields-outside-fences and open-work-scan-reads-fenced-examples
describe (item 4 below). It is a twenty-line change in the same
subsystem and its own record says it rides "the next reviewed chain
that touches internal/report".

# Where it lives (traced)

- metasystem/internal/goal/turnverdict.go: TurnVerdict (line 221)
  assembles the display as composeDisplay(prefix, verdict.Display,
  greens) near line 285. prefix = brainSummary lines + decideRuns
  warnings (one line per red, ended-unknown, hung, liveness-unknown or
  unsupervised run, no ageing, no grouping); verdict.Display = the
  ladder from decide (OPEN-WORK, TEMPLATE-UNFILLED, STILL WORKING, goal
  lines) plus the idle line from enforceIdleBacklog; greens =
  decideGreens, one line per green run past the session cursor. The
  Verdict struct (line 115) carries Display, Diagnostics, BlockSource,
  IdleRefusal.
- metasystem/cmd/metasystem/goal.go: runReportTurnVerdict (line 600)
  prints the Verdict as JSON.
- metasystem/scripts/agents/supervision-hook.sh: line 1102 calls
  report turn-verdict, reads display, and stop_block_json (search for
  "stop_block_json() {") passes display as the reason and the extras
  plus check-in tail as --system-message to report stop-block.
- metasystem/cmd/metasystem/report.go: runReportStopBlock (line 15)
  renders the block JSON; metasystem/internal/report/stopblock.go:
  StopBlock, BoundedIdleStopBlock, StopRefusal (which already writes a
  per-session record under artifacts/agents/supervision/stop-refusals/).
- metasystem/internal/channel/question.go: renderQuestion (line 269)
  and questionMessageRuneLimit (1600) are the bounding discipline this
  goal names as its precedent: the mandatory parts first, the rest
  trimmed with a notice.
- metasystem/internal/report/openwork.go: planField (line 81) is the reader; openWork (near line 399)
  calls planField(text, "Next step") and templateValue; planField has
  no fence handling. metasystem/internal/report/scan.go line 170 emits
  the same TEMPLATE-UNFILLED through the same reader.
- metasystem/internal/goal/turnverdict_test.go: TurnVerdict tests
  craft ScanResult{Runs: []RunFact{...}} directly (lines 487 to 540
  are the pattern); RunFact and ScanResult are defined in
  turnverdict.go lines 35 to 83.
- metasystem/scripts/agents/supervision-hook-fixtures.sh: the suite
  that proves the hook end to end.

# The change

1. A bounded renderer for the turn verdict's display, in Go, in
   metasystem/internal/goal/turnverdict.go (a new file
   verdictrender.go beside turnverdict.go is fine). It replaces the
   plain join in composeDisplay with this order:
   a. the verdict line: the ladder's first line (STILL WORKING, the
      block source's line, or the all-clear), exactly one line;
   b. the actionable items, unbounded in count but each on one line:
      the rest of the ladder (OPEN-WORK, TEMPLATE-UNFILLED, goal lines,
      the IDLE WITH BACKLOG line, the unwatched-work line), each
      carrying the command that clears it where the existing text
      already names one (the idle line names session stop; the
      unwatched line names the watch command; do not invent commands
      for lines that have none);
   c. the brain summary lines, unchanged (they are already short);
   d. the run warnings summarised by class and count, one line per
      class present: "N runs look hung, oldest <id>", "N runs ended
      ended-unknown ...", "N runs went red ...", "N runs of unknown
      liveness", "N runs unsupervised". A class with one member prints
      that member's full line instead of a summary. The grouping is by
      the existing switch in decideRuns; the individual lines are not
      dropped, they move to the file (item 2);
   e. the greens summarised as one line: "N runs finished green without
      a recorded continuation; M with one" and, when M is small
      (at most three), those M lines in full because a recorded
      continuation is an instruction to the seat;
   f. a final line naming the full-text file from item 2.
   The whole display is bounded to a rune limit named as one constant
   beside questionMessageRuneLimit's shape (4000 runes is about a
   screen; pick and name it). When the actionable items alone exceed
   it, trim from the bottom of the actionable list with a notice
   naming how many were trimmed, never the verdict line, never the
   file line.
2. The full unbounded text (the exact display the old composeDisplay
   would have produced) is written to
   <root>/artifacts/agents/supervision/stop-verdicts/<session-slug>.txt
   (one file per session, overwritten each Stop; the session slug the
   hook already computes with util slug) by TurnVerdict inside its
   flock, atomically (metasystem/internal/atomicfile is the writer the
   package already uses). A write failure is a Diagnostics line and the
   display's final line says the file could not be written; it never
   blocks the verdict. The directory sits beside stop-refusals/.
3. The system message gets the same discipline: report stop-block's
   --system-message is bounded in metasystem/internal/report/stopblock.go
   with the same constant, keeping its first line and trimming with a
   notice. The hook script is not edited: the bound is Go's decision
   and the hook stays plumbing.
4. planField in metasystem/internal/report/openwork.go reads header
   fields from outside fenced code blocks only: a line that is exactly
   three backticks (optionally followed by an info string) toggles a
   fenced region, and lines inside a region are never field candidates.
   Fix it in planField so every reader (openWork, scan.go line 170,
   stalePlans) inherits it; do not fix one call site.
5. Tests, all in Go:
   - a new verdictrender_test.go beside turnverdict_test.go: a ScanResult with
     two hundred stale run facts (mixed hung, ended-unknown, green with
     an empty ExpectGreen) and one unwatched job for the caller main;
     assert the display's first line is the verdict, the unwatched-work
     line appears before any run summary, exactly one summary line per
     run class present, the display is within the rune limit, the file
     from item 2 exists and contains all two hundred run ids, and the
     display names the file. A second case: three greens with recorded
     continuations print in full. A third case: a single hung run
     prints its full line, not a summary of one.
   - metasystem/internal/report/stopblock_test.go: a system message
     over the limit is trimmed with the notice and its first line kept.
   - metasystem/internal/report/openwork_test.go: a plan whose only
     "Next step" line is inside a fence is not reported; a plan with a
     real "Next step" field beside a fenced example reports the real
     one; plans/goal-scope-bounds-design.md's fenced block is the shape
     to copy.
   - metasystem/scripts/agents/supervision-hook-fixtures.sh: one
     scenario that seeds run records past the display bound and
     asserts the hook's reason is within the bound and names the
     stop-verdicts file. If seeding run records through the fixture
     harness is not practical inside your sandbox, say so in the
     return with the Go tests as the proof; the orchestrator replays
     the suite outside.

# Not in scope

Whether a run that finished weeks ago with no continuation is open
work at all (ageing them out, or dropping them from the judgement) is
goal stop-message-truth's. This chain groups and files; it drops
nothing from what is judged. Do not change decideRuns's or
decideGreens's conditions, the cursor, or ShouldBlock.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/goal/ ./internal/report/ ./cmd/metasystem/ -count=1`
green; `bash scripts/agents/supervision-hook-fixtures.sh` green (say so
if the sandbox cannot run it; the orchestrator replays it outside).
Coverage must not drop below the tree's figure measured before this
chain on 2026-09-09: internal/goal 82.2 percent, internal/report 87.3
percent (go test -cover on each package).

# Constraints

Wall-clock budget: 90 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 3). Declare the boundary as
every file that differs from main. Gap rule: stop and report a gap with
your proposed contract written out. Plain English in comments and
messages; no finding or round references in source comments.
