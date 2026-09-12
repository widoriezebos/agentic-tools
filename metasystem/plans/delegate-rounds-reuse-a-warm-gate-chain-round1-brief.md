Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal delegate-rounds-reuse-a-warm-gate)
Date: 2026-09-12

# Goal

Goal delegate-rounds-reuse-a-warm-gate (tier 3, approved by Wido), slice C
of metasystem/plans/delegate-rounds-reuse-a-warm-gate-design.md section 4:
a three-round chain measured for the time its rounds spend verifying. This
is round 1. The product change is deliberately trivial; what the chain
measures is the machinery: the chain's build cache (slice A, landed as
809fc0d07) and the engine's reuse of retained proof across rounds
(8f7becd65, db58ad931). Round 1 warms the cache and proves the tree.

# Workspace

Your job worktree (the dispatcher names it), branch agent/<job>. Touch
exactly one file: metasystem/internal/refusal/register.go. Touch nothing
else. Do not commit; the dispatcher reads the worktree.

The build cache is provided: the adapter sets GOCACHE, GOTMPDIR and
STATICCHECK_CACHE to the chain's cache in the worktree's git dir, shared by
every round; never set, unset or strip them (no env -u, no env -i before a
gate): a self-set cache is cold every round.

# Inputs

- metasystem/internal/refusal/register.go: the refusal register. Add one
  comment line directly above the line `var Exclusions = []Exclusion{`
  reading exactly:
  `// Exclusions name the upper-case tokens the walk collects that refuse nothing.`
  No other edit.
- The engine-appended "Required testing contract" section below this brief
  names your proving run. Run it as written from the worktree's repository
  root with `--root metasystem`, and report the attempt id (the proof-run
  id in the result's log path) in whatWasDone.

# Constraints

- Non-goals: any other change, any refactor, any comment beyond the one
  line, any test edit. No `go test` of your own beyond the proving run; if
  the proving run is red, report it as a gap with its output, do not fix.
- Wall clock: 20 minutes. Stop and report if the proving run has not
  finished by then.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries exactly two `{command,
observed, level}` items, each replayable verbatim from the worktree's
repository root: (1) `git -C metasystem diff --stat` observing the one-file
diff; (2) the proving run command exactly as run, observing its last line
and the attempt id. whatWasDone names the attempt id.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Acceptance Criteria

- `git diff --stat` shows exactly metasystem/internal/refusal/register.go
  with one added line and no removed line.
- The added line is byte-exact as given above and sits directly above
  `var Exclusions = []Exclusion{`.
- The proving run completed and its attempt id is in whatWasDone.

# Gap Rule

stop and report a gap; never fill it silently.
