Working Mode: implement
Orchestrator Identity: m1b (lineage main-1789191336-90295-e4b24b, dispatch delegate under goal goal-abandoned-with-a-reason)
Date: 2026-09-13

# Goal

Carry the certified change of chain gawr-build1 (slices 1 and 2 of goal
goal-abandoned-with-a-reason: the ledger side, and reopen from abandoned
with the carry verb) onto today's trunk, unchanged in behaviour. The
chain was built on 938dbe7c (2026-09-10), closed its round 5 clean after
three Opus reads, and never landed; trunk has moved 1,510 commits since,
and five of its 43 files now conflict. The specification is unchanged:
metasystem/plans/goal-abandoned-with-a-reason-design.md (revision 4) with
the goal record metasystem/plans/goals/goal-abandoned-with-a-reason.md;
the slice briefs metasystem/plans/goal-abandoned-with-a-reason-build1-brief.md,
metasystem/plans/goal-abandoned-with-a-reason-build1-followup1-brief.md and
metasystem/plans/goal-abandoned-with-a-reason-build1-followup2-brief.md say
what the chain built.

# Workspace

This chain's worktree as round 5 left it: the certified change is its
uncommitted diff against 938dbe7c. Bring the worktree to the current
origin/main and re-apply the change on top (for example: save the diff,
reset to origin/main, apply with a three-way merge), then resolve the
conflicts. The files the seat saw conflict on a three-way apply against
trunk: metasystem/cmd/metasystem/goal.go,
metasystem/cmd/metasystem/goalsync_mutations.go,
metasystem/internal/goal/file.go, metasystem/internal/goal/root.go and
metasystem/scripts/agents/goal-cli-fixtures.sh. Do not commit.

# What the carry must satisfy

- Every behaviour trunk added to those files since 938dbe7c stays, and
  every behaviour of the certified change stays. Where both touch the same
  logic, keep both and say in whatWasDone how you joined them.
- No new behaviour beyond the certified change. If trunk now does part of
  what the change did, keep trunk's version and drop the duplicate, and
  name it.
- The allow-lists and registers trunk keeps current are updated where the
  change needs them: the brain seam allow-list in
  metasystem/scripts/agents/brain-fixtures.sh (every `goal.Actor{` and
  `classifyVerbCaller(` site, sorted) and the refusal register in
  metasystem/internal/refusal/register.go, if the change adds rows.
- The goal package, the cmd package's goal tests and the goal-cli bed's
  Go-owned checks pass; the fast gate passes.

# Expected Return

The implementer return schema (metasystem/scripts/agents/schemas/implementer.schema.json):
jobId, round, runtime, sessionId, model, evidence, gaps, mode, riskiestPart,
diffBoundary, whatWasDone. evidence carries `{command, observed, level}`
items replayable verbatim from the worktree's repository root: (1)
`git -C metasystem status --short`; (2)
`( cd metasystem && go test ./internal/goal/ -count=1 )` and
`( cd metasystem && go test ./cmd/metasystem/ -run 'Goal|Abandon|Reopen|Carry' -count=1 )`
with their pass lines; (3) `( cd metasystem && scripts/agents/go-gate.sh --fast )`
with its last line. whatWasDone names each conflicted file and how it was
resolved. riskiestPart names the join you are least sure of.

Every path in your return (diffBoundary, files) is relative to the
repository root, so it starts with `metasystem/`.

# Constraints

- Plain-English error texts; source comments say what and why, never
  which round or finding.
- Wall clock: 90 minutes. A partial round returns with its tests green
  for what exists and names what is left.

# Gap Rule

stop and report a gap; never fill it silently.
