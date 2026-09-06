Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Goal

Round 3 of slice 1 of goal stop-hook-budget-is-ours (chain
shbo-build1-20260906). The closing code review (job shbo-cc1-20260906,
return at metasystem/artifacts/agents/shbo-cc1-20260906/rounds/1/return.json)
found one material defect and four cheap improvements the seat accepted;
the dispositions are in
metasystem/plans/dispositions/stop-hook-budget-code-critique-r1.md. Fix
the five below in place, change nothing else. Your round-one and
round-two briefs still govern everything they say; where this brief
differs, this brief wins for the five points.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

# The five fixes

## SHB-01 — a new script must complete against an old engine

In metasystem/scripts/agents/supervision-hook.sh, every
`steward hook-complete` call in `emit_stop_payload` that passes
`--elapsed-sec`: when the call exits with status 2 (the flag parser's
refusal on an engine built before the flag existed), retry the same
call once without `--elapsed-sec`. Put the retry in one small function
so the four call sites do not each grow a copy; the function takes the
completion arguments, runs the flagged call, and on exit 2 runs the bare
call. Any other exit status is handled exactly as today. Say in the
return how you proved the exit-2 path (for example a fake engine on
METASYSTEM_BIN that refuses the flag), and add that proof as one leg of
the chat-line scenario in
metasystem/scripts/agents/supervision-hook-fixtures.sh if it fits in
under a minute of fixture time; otherwise a unit-level proof and say so.

## SHB-03 — a retried attempt carries no stale measurement

In metasystem/internal/steward/component_evidence.go, when
BeginHookAttempt reuses the loaded record for a same-turn-key retry, it
clears `LastStopElapsedSec` (nil), the way the interrupted closure
leaves the attempting record without one. A unit test: a failed
completion with elapsed 7 followed by a same-key retry yields an
attempting record without the field.

## SHB-05 — the deadline assertion proves the parent's start

In the deadline scenario of
metasystem/scripts/agents/supervision-hook-fixtures.sh, the assertion
on the parent's `stop response outcome=deadline-expired-block elapsed=<n>s`
line binds n to the range 50 through 59 inclusive, which only a
measurement from the parent's start can produce at these numbers.

## SHB-06 — two comments stop saying five seconds

In metasystem/internal/goal/project.go the two comments near lines 38
to 40 and 116 to 119 that justify the projection timeout from a
five-second provider Stop budget are rewritten to say the Stop budget is
sixty seconds, shipped in the registration templates under
metasystem/scripts/enforcement and owned by the hook's deadline parent.
Comment text only; no code in that file changes.

## SHB-07 — the tests the return claimed

Add the interrupted-history unit test the round-two return described
(an attempting record interrupted by the next turn appends a history
entry without `stopElapsedSec`), and a test in the command package
(metasystem/cmd/metasystem, which has tests) that
`steward hook-complete` with `--elapsed-sec -1` exits 2 before touching
any repository.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l .` (empty);
`go test ./internal/steward/ ./cmd/metasystem/ -count=1` green;
`bash -n scripts/agents/supervision-hook.sh scripts/agents/supervision-hook-fixtures.sh`;
`bash scripts/agents/supervision-hook-fixtures.sh` as far as the sandbox
allows (the deadline scenario needs process inspection; the seat reruns
the suite outside the sandbox before landing).

# Constraints

Wall-clock budget: 45 minutes. Declare the boundary as every file that
differs from main. Gap rule: stop and report a gap with your proposed
contract written out.

# Expected Return

Version-2 implementer JSON as before.

# Gap Rule

stop and report a gap; never fill it silently.
