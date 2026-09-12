Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal delegate-rounds-reuse-a-warm-gate)
Date: 2026-09-12

# Goal

Goal delegate-rounds-reuse-a-warm-gate (tier 3, approved by Wido), slice C
of metasystem/plans/delegate-rounds-reuse-a-warm-gate-design.md section 4:
a three-round chain measured for the time its rounds spend verifying. This
is round 1 of the second chain (the first chain could not reach a proof;
the blocker that fixed it moved the proof to the orchestrator). The product
change is deliberately trivial; what the chain measures is the machinery:
the chain's build cache (slice A, landed as 809fc0d07) and the engine's
reuse of retained proof across rounds (8f7becd65, db58ad931). Round 1 warms
the cache; the orchestrator proves the worktree when you return.

# Workspace

Your job worktree (the dispatcher names it), branch agent/<job>. Touch
exactly one file: metasystem/internal/refusal/register.go. Touch nothing
else. Do not commit: the dispatcher and the proof read the worktree as you
leave it.

The build cache is provided: the adapter sets GOCACHE, GOTMPDIR and
STATICCHECK_CACHE to the chain's cache in the worktree's git dir, shared by
every round; never set, unset or strip them (no env -u, no env -i before the
go gate): a self-set cache is cold every round.

# Inputs

- metasystem/internal/refusal/register.go: the refusal register. Add one
  comment line directly above the line `var Exclusions = []Exclusion{`
  reading exactly:
  `// Exclusions name the upper-case tokens the walk collects that refuse nothing.`
  No other edit.
- Your verification, run from the worktree's repository root (the directory
  that holds `metasystem/`), exactly these two commands, one after another:
  ```
  ( cd metasystem && go test -count=1 ./internal/refusal )
  ( cd metasystem && scripts/agents/go-gate.sh --fast )
  ```
  Do not run `bin/metasystem test run`, `test plan` or `test verify`: the
  worktree carries no enrolled engine; the orchestrator proves the worktree
  on return with `metasystem job prove-round`.

# Constraints

- Non-goals: any other change, any refactor, any comment beyond the one
  line, any test edit, any commit, any `git reset`. If a command is red,
  report it as a gap with its output; do not fix.
- Wall clock: 15 minutes. Stop and report if the two commands have not
  finished by then.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly three `{command,
observed, level}` items, each replayable verbatim from the worktree's
repository root: (1) `git -C metasystem diff --stat` observing the one-file
diff; (2) the go test command exactly as run, observing its last line; (3)
the go gate command exactly as run, observing its last line. whatWasDone
names the wall-clock seconds each of the two commands took.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- `git diff --stat` shows exactly metasystem/internal/refusal/register.go
  with one added line and no removed line.
- The added line is byte-exact as given above and sits directly above
  `var Exclusions = []Exclusion{`.
- Both commands ended green and their last lines are in evidence.

# Gap Rule

stop and report a gap; never fill it silently.
