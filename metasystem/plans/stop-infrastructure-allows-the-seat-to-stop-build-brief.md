Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal stop-infrastructure-allows-the-seat-to-stop)
Date: 2026-09-12

# Goal

Build stop-infrastructure-allows-the-seat-to-stop exactly as its design page
says: metasystem/plans/stop-infrastructure-allows-the-seat-to-stop-design.md
(sections 2, 5 and 6 are the contract; the goal record is
metasystem/plans/goals/stop-infrastructure-allows-the-seat-to-stop.md). One
mechanism: an infrastructure read failure of the Stop hook or the turn
verdict never refuses a stop; it allows the stop with a degraded notice and
one hook-log line. Seat-actionable refusals and the idle-with-backlog path
(metasystem/records/goals/idle-with-backlog-alarm.md) stay exactly as they
are, except that an observed idle refusal survives a lost verdict-state
write, uncounted and saying so.

# Workspace

Your job worktree (the dispatcher names it), branch agent/<job>. Touch only:
metasystem/internal/report/stopblock.go and its test
metasystem/internal/report/stopblock_test.go;
metasystem/internal/goal/turnverdict.go, metasystem/internal/goal/sessionstop.go
and their tests metasystem/internal/goal/turnverdict_test.go and
metasystem/internal/goal/turnverdict_idle_test.go;
metasystem/cmd/metasystem/report.go (the `--class` flag of `report
stop-block`) and metasystem/cmd/metasystem/up_test.go;
metasystem/scripts/agents/supervision-hook.sh and
metasystem/scripts/agents/supervision-hook-fixtures.sh;
metasystem/scripts/enforcement/claude-code-hooks.json. Do not commit: the
dispatcher and the proof read the worktree as you leave it.

The build cache is provided: the adapter sets GOCACHE, GOTMPDIR and
STATICCHECK_CACHE to the chain's cache in the worktree's git dir; never set,
unset or strip them (no env -u, no env -i before the go gate).

# Inputs

- The design page, section 2 "Where it lives": the six changes, in this
  order: (1) `report stop-block --class infrastructure|seat-actionable` in
  metasystem/internal/report/stopblock.go and its flag in
  metasystem/cmd/metasystem/report.go: the infrastructure class returns the
  allowance with the notice on every occurrence and appends the hook-log
  line; the seat class keeps today's first-occurrence block. (2)
  `failClosedTurnVerdict` in metasystem/internal/goal/turnverdict.go
  becomes an allowing verdict with a new `Class` field ("infrastructure"),
  `ShouldBlock` false, `LedgerStatus` degraded, the detail kept; the idle
  branch is evaluated before the verdict-state write and a failed write
  returns the idle block with a new `CountSpent` false and `Class`
  "idle-with-backlog". (3) metasystem/internal/goal/sessionstop.go: a
  marker read failure allows without consuming; a consume failure never
  reports consumption. (4) the hook: every `record_stop_failure` cause and
  the deadline path render through `report stop-block --class
  infrastructure`; the arming notice carries the failed components as
  `metasystem up` prints them (outcome, detail, remedy) before the
  aggregate; the hook log (the writer near line 878) gains one line per
  infrastructure condition and per uncounted idle refusal, `stop-condition
  <class> <cause code> <component> <turn generation> <deadline end>
  <outcome>`, with the append's exit checked and a failure said in the
  notice. (5) the launcher line in
  metasystem/scripts/enforcement/claude-code-hooks.json: the fallback on a
  nonzero hook exit prints a systemMessage naming the hook's own failure and
  the steward, never a block. (6) the fixtures of section 5, each written to
  fail against today's code first.
- Section 3 of the page: what does not change. Do not touch the steward's
  incident record, the deadline-keyed episode or any delegate adapter:
  those are other members.

# Constraints

- Non-goals: any change to the idle counter, digest or the three-refusals
  handoff; any renaming or reformatting beyond the change; any new
  dependency.
- Run the focused tests for what you change (`go test -count=1
  ./internal/report ./internal/goal ./cmd/metasystem -run
  'StopBlock|Infrastructure|SessionStop|Arming|Idle'`) and the hook bed legs
  you add (`bash scripts/agents/supervision-hook-fixtures.sh` selects by
  scenario; run the whole bed once at the end), and `scripts/agents/go-gate.sh
  --fast`. Do not run `bin/metasystem test run`, `test plan` or `test
  verify`: the orchestrator proves the worktree on return.
- Wall clock: 3 hours. Stop and report a gap if the page cannot be built
  as written at any point; do not redesign.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly four `{command,
observed, level}` items, replayable from the worktree's repository root:
(1) `git -C metasystem diff --stat` observing the touched files; (2) the
focused go test command exactly as run, observing its last lines; (3) the
hook bed command exactly as run, observing its last line; (4) the fast gate
exactly as run, observing its last line. whatWasDone names the six changes
in one line each and every new test by name.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- Every fixture of the page's section 5 exists by the name given, passes,
  and would fail against today's code (say how you checked).
- The idle tests in metasystem/internal/goal/turnverdict_idle_test.go pass
  unchanged except for the one new test the page names.
- No file outside the workspace list changed.

# Gap Rule

stop and report a gap; never fill it silently.
