Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 12: the verdict is observable through the options, then the round-11 fold

Chain phd-build1-20260910. Round 11 was right to stop: the fixture
cannot see the moment the supervisor records a verdict, and D-R11-1 as
written needed that moment. The designer decides the seam. Report
`round` as 12. Do not fetch or rebase the worktree.

## Decision D-R11-1, restated

`supervisorOptions` gains one unexported field, `OnVerdict func(verdict
string)`, nil by default and never set by production callers; the
supervisor invokes it once, at the moment a terminal verdict (dead,
runaway, invalid) is recorded and before the dump ladder sends SIGQUIT.
This is a fixture input on the options struct like Reader, Activity and
StageResultPath, not a second package-level hook: the one package-level
hook stays `supervisorLimitsForTest`. The dead-dump row uses it to
switch the gate to output-only keep-alive until the helper has exited,
so the helper's own exit decides `dump: complete` and no wall margin
remains. The design page records the seam.

## Mandate for this round

1. Add the field and its single invocation; a test asserts it fires
   once with the verdict's name and that a nil callback changes nothing.
2. Implement D-R11-1 (as restated) and D-R11-2 to D-R11-6 from the
   round-11 brief (plans/proof-groups-detect-hangs-by-progress-not-the-clock-build1-fold-r11-brief.md
   is in your worktree's plans directory if the dispatcher carried it;
   otherwise the decisions are restated here in short):
   D-R11-2 pre-readiness keep-alive marks output, never CPU;
   D-R11-3 the stopped row reads the real task state and resumes only a
   stopped helper, repeating on every sample until it exits;
   D-R11-4 nested children hold their work until the row closes their
   stdin after a real sample has seen them;
   D-R11-5 retention of vanished members is a Darwin rule only; Linux
   counts live members plus the root's cutime and cstime;
   D-R11-6 every real-reader row carries a CPU budget of ten times its
   expected consumption so a reader defect ends it `runaway` by name.
3. Keep every proof-standard test name in testing.json in step.
4. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 and
go test -count=1 -race on internal/proofrun and internal/testpolicy,
green. The seat gate runs the same with the real reader. Report the
round as your own.

## Constraints

Wall-clock budget: 35 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
