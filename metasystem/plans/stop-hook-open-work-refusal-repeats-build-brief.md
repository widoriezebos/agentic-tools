Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-open-work-refusal-repeats)
Date: 2026-09-07

# Goal

Goal stop-hook-open-work-refusal-repeats (tier 3, approved by Wido on
2026-09-06). Its record,
metasystem/plans/goals/stop-hook-open-work-refusal-repeats.md, is the
contract; in short: the Stop hook's open-work verdict says "This
refusal does not repeat for the same work" and then repeats it; on m1d
the same two OPEN-WORK lines (another seat's plan with a literal
"<one line, required>" placeholder, and a handoff plan naming a job
that runs nowhere) refused three turn ends in a row, twice while a
delegate job of this checkout was running. DONE means: the once-only
promise holds per plan line across turns with a durable marker that a
deadline expiry does not lose; a running job or an open chain on the
checkout counts as work in flight; a "<one line, required>"
placeholder is classified template-unfilled on its own line.

# Where it lives (traced)

- metasystem/internal/report/openwork.go: OpenWork, stalePlans,
  openWork, jobsInFlight (inFlightStatus pending and running).
- metasystem/internal/report/scan.go: scanPlans emits the OPEN-WORK
  lines.
- metasystem/internal/report/stopblock.go: StopBlock carries the
  once-only sentence; StopRefusal keeps a seen-state for EXTERNAL stop
  failures (block first, visible later): the shape to reuse.
- metasystem/cmd/metasystem/report.go: report stop-block and report
  open-work verbs; report scan-jobs already takes a --state seen file.
- metasystem/scripts/agents/supervision-hook.sh: the Stop hook calls
  report stop-block (lines near 320, 524, 535).
- metasystem/scripts/agents/supervision-hook-fixtures.sh: the suite
  that proves the hook; line 850 already greps the once-only sentence.

# The change

1. A durable seen-state for open-work lines under the supervision
   artifacts (a new file open-work-seen.json in the supervision
   artifacts directory, or the file the existing StopRefusal state uses, keyed by plan path plus a
   digest of the line), written the first time a line refuses a turn
   end and read on every later stop; a line already seen renders
   visible in the verdict but does not block. The deadline path
   (report stop-block from the deadline parent) reads the same file,
   so an expiry does not lose the marker. A line whose text changes is
   a new line.
2. In flight: a job record on this checkout in status pending or
   running, or a chain root with chainClosed false whose newest round
   is not terminal, counts as work in flight for the open-work verdict
   (openwork.go jobsInFlight and the open-chain test that stalePlans
   already does at line 165; the verdict must use both).
3. Classify a "<one line, required>" placeholder (and the other
   template tokens plans carry, grep the plans directory for angle
   brackets) as TEMPLATE-UNFILLED on its own line, never OPEN-WORK.
4. Fixtures in supervision-hook-fixtures.sh: one scenario per rule
   (a repeated line does not block twice across two stops and across a
   deadline expiry; a running job record makes the verdict allow; a
   placeholder renders as TEMPLATE-UNFILLED). Go tests in
   internal/report for the seen-state and the classifier.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/report/ ./cmd/metasystem/ -count=1` green;
`bash scripts/agents/supervision-hook-fixtures.sh` green (say so if
the sandbox cannot run it; the orchestrator replays it outside).

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 3). Declare the boundary as
every file that differs from main. Gap rule: stop and report a gap
with your proposed contract written out.
