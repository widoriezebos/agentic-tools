Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal stop-refusal-fits-on-one-screen)
Date: 2026-09-09

# Fold brief: round three of the bounded Stop refusal (chain one-screen-build1b-20260909)

You built rounds one and two of this chain. The second critic,
one-screen-crit2-20260909, found one material defect and one note; the
orchestrator's dispositions are in the records directory under misc,
in stop-refusal-fits-on-one-screen-critique-r2-dispositions.md (a new
record, not yet committed). The defect is in the RULE the round-two
fold brief gave you for the unclosed fence, not in how you applied it.
Fold the two items below into the same worktree; touch nothing else.

## OSR-06: the unclosed-fence fallback must keep what it knew

In metasystem/internal/report/openwork.go, planField now re-reads the
whole file as unfenced when it reaches end of file inside a fence. The
critic reproduced the consequence: a plan with a properly closed fence
that contains "- Next step: FAKE example from a closed fence", then a
second fence opened and never closed, then "- Next step: REAL work to
do", is reported as "OPEN-WORK plans/trap.md: FAKE example from a
closed fence", and the real step is never seen.

The corrected rule: only PAIRED fence lines delimit fenced regions.
Count the fence lines first; when the count is odd, the last fence line
is ordinary text, every properly closed fence before it stays
excluded, and the header fields after it are read. Implement it as a
single pass over the lines with that pairing decided up front, and
remove the whole-file re-read. In metasystem/internal/report/openwork_test.go,
add the critic's trap plan exactly as described and assert that the
field reported is "REAL work to do" and that the fake one is not
reported; keep the existing unclosed-fence test, which must still pass
(its real field after the stray opener is still found).

## OSR-07: the seeded runs must be ordered by value, not by tie

In metasystem/scripts/agents/supervision-hook-fixtures.sh, the 200
seeded records all carry startedAt 2026-08-01T10:00:00Z, so which one
the renderer calls "oldest" depends on tie order under a sort that is
not promised stable. Give them distinct start times one second apart
in run order (bounded-run-001 the earliest), so "oldest
bounded-run-001" is true by value. The endedAt values follow the same
spacing so no record ends before it starts.

## Not in scope

Nothing else changes in the ten files; no file outside them.

## Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l . (empty)`;
`go test ./internal/report/ -run 'TestOpenWork' -count=1` green;
`bash -n scripts/agents/supervision-hook-fixtures.sh`; say plainly what
else your sandbox could run; the orchestrator replays the full
packages, the coverage floors and the hook suite.

## Constraints

Wall-clock budget: 25 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 3). Declare the boundary as
every file that differs from main. Stop adding at a gap that needs a
decision no page has made; keep what is built and green; report the
gap with the resolution you propose. Never delete built work.
