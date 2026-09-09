Working Mode: build
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal brain-summary-leads-the-stop-display)
Date: 2026-09-09

# Goal

Goal brain-summary-leads-the-stop-display (record
metasystem/plans/goals/brain-summary-leads-the-stop-display.md). The
bounded Stop display landed today in d533caf17 moved the brain seat's
summary lines from the head of the display to after the actionable
lines. Two scenarios of metasystem/scripts/agents/goal-cli-fixtures.sh
assert the old order and fail on main: brain-stop-seeded ("brain summary
did not lead with ask and draft on stop 1: OPEN WORK (1)") and
brain-stop-corrupt ("corrupt brain did not lead with summary and remedy:
OPEN WORK (1)"). That bed is in the full battery, so every full-width
landing on the fleet is refused until this lands. DONE means: on a brain
seat the display begins with the brain summary lines (the BRAIN SEAT
line, or the corrupt-declaration summary and its remedy), then the
verdict line and the rest of the bounded order; on a non-brain seat
nothing changes; both scenarios pass; the hook suite still passes.

# Where it lives (traced)

- metasystem/internal/goal/verdictrender.go: renderTurnVerdict (line
  39) takes brainLines and, at line 61, appends them to the remainder
  after the actionable lines; boundedVerdictLines (line 137) applies
  the 4,000-rune bound.
- metasystem/internal/goal/turnverdict.go: TurnVerdict builds
  prefix = brainSummary + run warnings (line 268 area) and calls the
  renderer.
- metasystem/internal/goal/verdictrender_test.go: the renderer's tests,
  including the 200-run shape; metasystem/internal/goal/turnverdict_brain_test.go:
  the brain-seat verdict tests.
- metasystem/scripts/agents/goal-cli-fixtures.sh: brain-stop-seeded and
  brain-stop-corrupt (search the file for those names) are the failing
  assertions; metasystem/scripts/agents/supervision-hook-fixtures.sh must
  stay green.

# The change

1. In renderTurnVerdict, emit brainLines FIRST, before the verdict line,
   then the verdict line, the actionable lines, the run summaries, the
   greens and the file line, all still within the bound; the brain lines
   are never trimmed by the bound (they are the seat's verdict context),
   the actionable list still trims from its bottom with the notice.
2. Tests: a brain-seat case in verdictrender_test.go asserting the first
   line is the brain summary and the second is the verdict line; the
   non-brain cases unchanged; turnverdict_brain_test.go's expectations
   updated only where they encoded the wrong order.
3. Nothing else changes: not decideRuns, not the greens, not the file,
   not the bound.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/goal/ -run 'TurnVerdict|Render|Brain' -count=1`
green; `bash scripts/agents/goal-cli-fixtures.sh` green (15 of 15; say
so if the sandbox cannot run it, the orchestrator replays it) and
`bash scripts/agents/supervision-hook-fixtures.sh` green.

# Constraints

Wall-clock budget: 30 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach. Declare the boundary as every file
that differs from main. Stop adding at a gap that needs a decision no
page has made; keep what is built and green; report the gap with the
resolution you propose. Never delete built work.
