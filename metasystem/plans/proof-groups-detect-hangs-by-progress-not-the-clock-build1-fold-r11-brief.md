Working Mode: implement
Orchestrator Identity: m1+main-1788940932-18533-7fa6c2 (dispatch delegate under goal proof-groups-detect-hangs-by-progress-not-the-clock)
Date: 2026-09-11

# Follow-up brief, slice 1 round 11: the second critique's four material findings, decided

Chain phd-build1-20260910. The second code critique
(phd-build1-crit3b-20260910, reviewing round 10, tree
8fd7fcedc21af38d3e02257d2f91a34c4fcf3027) found four remaining wall
margins in fixtures (it emulated a scheduler holding a helper off the
CPU with SIGSTOP; plain load did not reproduce them, which is exactly
why they must go) and three notes. The designer decided each below.
Report `round` as 11. Do not fetch or rebase the worktree.

## Decisions

D-R11-1 (PHD-08, dead-dump row: the dump's duration decided the
outcome). After the supervisor's verdict, the readiness gate marks
output activity on every sample until the helper has exited, so the
zero-consumption fallback of the dump ladder cannot fire while a real
helper is writing its dump; the helper's own exit decides `dump:
complete`. The synthetic figure is output, never CPU, so the "still
rises after SIGQUIT" kill cannot fire either.

D-R11-2 (PHD-09, SIGQUIT-ignoring rows: pre-readiness keep-alive
consumed the budget). Before readiness the gate keeps the group alive
by marking output activity, never by synthetic CPU; the budget only
ever sees real or scripted figures served after readiness. Apply the
same to the contention row's gates.

D-R11-3 (PHD-10, stopped row: SIGCONT could arrive before the helper
stopped itself). The row observes the real stopped state before it
resumes the helper: after the scripted reader has reported `stopped`,
the fixture reads the helper's real task state (Darwin `ps -o state=`,
Linux /proc stat) and sends SIGCONT only once it shows stopped; it
repeats that read-then-continue on every later sample until the helper
exits, so a helper that stops itself late is still resumed. No wall
wait anywhere in the row.

D-R11-4 (PHD-11, Darwin retained-child row: the nested child could
exit before a real sample saw it). A nested child that a row must
observe holds its work until the row releases it: it announces `ready`,
burns CPU while its stdin stays open, and exits when the row closes
that stdin after a real sample has seen it alive. The row asserts the
retained figure after the release. Apply to every real-reader row with
a nested child.

D-R11-5 (PHD-13, Linux counts a reaped child twice). Retention of a
vanished member's high-water is a Darwin rule only: on Linux the
counter is the live members' own time plus the root's reaped-children
fields (cutime and cstime), which already carry every finished child;
on Darwin, which exposes no reaped-children figure, vanished members
keep their high-water. The design page records the split.

D-R11-6 (PHD-14, a real-reader row hangs when the reader has the
defect it guards against). Every real-reader row carries a CPU budget
of ten times its expected consumption (ten CPU seconds unless the row
names another figure), so a reader defect ends the row `runaway` with
its reason instead of hanging; the budget is consumption, never a
clock. The contention row keeps its own budgets.

PHD-12 (note): accepted as a note, unchanged.

## Mandate for this round

1. Implement D-R11-1 to D-R11-6. Production code changes only for
   D-R11-5; the rest is fixtures and the gate.
2. Keep every proof-standard test name in testing.json in step with the
   fixtures.
3. Nothing else changes.

## Proof

go build ./..., go vet ./..., gofmt -l; go test -count=1 and
go test -count=1 -race on internal/proofrun and internal/testpolicy,
green. The seat gate runs the same with the real reader. Report the
round as your own.

## Constraints

Wall-clock budget: 30 minutes. Return per the implementer schema. Stop
at a gap that needs a decision no page has made; report it with the
resolution you propose. Never delete written work.
