Working Mode: design
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal stop-hook-never-forces-an-empty-turn)
Date: 2026-09-12

# Goal

Author the design document stop-hook-never-forces-an-empty-turn-design.md,
a NEW file you create in the metasystem plans directory (the directory
that holds delivery-efficiency-plan.md), for goal
stop-hook-never-forces-an-empty-turn (goal 4 of
metasystem/plans/delivery-efficiency-plan.md, tier 3, origin human). The
seat's stop gate refused 331 times in five days and forced 292 coordinator
turns; the recorded causes were "the narrator digest could not be read"
(184), "stop deadline expired" (117) and "supervision arming failed" (about
95). DONE, for the stop gate of every runtime: (1) a refusal names a
condition the seat can act on and the one command that clears it; (2) an
infrastructure read failure the seat cannot change (narrator digest,
hook-evidence state, arming state) never refuses the stop and is reported to
the steward instead; (3) an infrastructure refusal is not repeated for an
unchanged condition within one stop deadline; (4) the idle-with-backlog path
is untouched: approved backlog with no job still refuses the stop and after
three unchanged refusals hands the seat to steward continuation
(metasystem/records/goals/idle-with-backlog-alarm.md); proven by fixtures
under the fake adapter and one real runtime replaying the three recorded
infrastructure causes and the approved-backlog-with-no-job case.

# Workspace

Your job worktree (the dispatcher names it), branch agent/<job>. Create
exactly one file, stop-hook-never-forces-an-empty-turn-design.md, in the
metasystem plans directory. Touch nothing else. Do not commit: the dispatcher and the proof read the
worktree as you leave it.

# Inputs

- metasystem/scripts/agents/supervision-hook.sh: the Stop hook. Read the
  three refusal sites (the deadline cause near line 329, "supervision
  arming failed" near line 835, "the narrator digest could not be read"
  near line 859) and how each records its refusal.
- metasystem/internal/goal/turnverdict.go and
  metasystem/internal/goal/sessionstop.go: the turn verdict and the
  session-stop verb the hook calls; metasystem/internal/report/stopblock.go:
  how a refusal is rendered.
- metasystem/records/goals/idle-with-backlog-alarm.md: the idle-with-backlog
  path that must stay as it is.
- metasystem/docs/orchestration.md: the runtime-independence doctrine (the
  mechanism lives in the engine, the ledger verbs and the adapter contract,
  never in one runtime's hook).
- The clauses absorbed into this goal on 2026-09-11 (read them in the goal
  record's next-step text, quoted here): the arming-failure line carries the
  component outcome, detail and remedy exactly as `metasystem up` prints
  them, never the bare "supervision arming failed"; every refusal the hook
  issues is persisted with the underlying up aggregate before the hook
  exits, and the hook records its turn generation on the checkout; a held
  goal under a StopFence is reported FENCED once per unchanged fence state;
  INFLIGHT counts only jobs and attempts joined to the ready frontier by
  goal and revision, never any same-session job, and READY is scoped to the
  machine and lineage owner pair with a freshness proof for the projected
  board.

# Constraints

- The page is one design page in plain English (no LLM-style prose, short
  sentences, no bullet padding), at most 220 lines, with these sections:
  1. What refuses today and why (classify every refusal cause the hook can
  issue into seat-actionable, steward-owned infrastructure, and the
  idle-with-backlog path, with file:line evidence); 2. The mechanism (where
  the classification lives in Go, what the hook and every other runtime's
  adapter gate do with each class, the once-per-deadline rule for unchanged
  infrastructure conditions and the record that carries it, the steward
  report); 3. What does not change; 4. Risks; 5. Fixtures (name each test
  or bed leg, what it sets up and what it asserts, including the fake
  adapter replay of the three recorded causes and the
  approved-backlog-with-no-job case, and the one-real-runtime proof); 6.
  Landing (which files, whether every seat must rebuild and re-arm).
- Every path you cite must exist in the worktree under the metasystem/
  prefix. No globs. Do not write code. Do not run bin/metasystem test run,
  test plan or test verify.
- Non-goals: any change to the idle-with-backlog rule, any change to the
  three-refusals handoff to steward continuation, any runtime-specific
  mechanism.
- Wall clock: 30 minutes. Stop and report if the page is not finished by
  then.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly two `{command, observed,
level}` items, replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short` observing the one new file; (2)
`( cd metasystem/plans && wc -l stop-hook-never-forces-an-empty-turn-design.md )`
observing the line count. whatWasDone names the three classes and how many
of the hook's refusal sites fall in each.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- The one new file exists in the metasystem plans directory under the name
  above, with the six sections, and every path it cites exists.
- Section 1 classifies every refusal site of the hook with file:line
  evidence; section 5 names the fixtures for DONE's four clauses.

# Gap Rule

stop and report a gap; never fill it silently.
