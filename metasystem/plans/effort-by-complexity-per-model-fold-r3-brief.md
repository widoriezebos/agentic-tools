Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal effort-by-complexity-per-model, tier 2, hazard DESIGN-BEARING, fold round three of chain ebc-build1)
Date: 2026-09-07

# Goal

Rounds one and two built the effort configuration (the round-one brief
plans/effort-by-complexity-per-model-brief.md and the round-two fold
plans/effort-by-complexity-per-model-fold-r2-brief.md, both landed on
main after this chain's base). The first critique (ebc-critic1) found
one material defect and two small ones; dispositions are in
records/misc/effort-by-complexity-per-model-critique-r1-dispositions.md
on main. This round folds them.

# The fold

1. (F-1, material) A follow-up round on a job record that predates
   this change refuses because the record has no reasoningEffortSource;
   all existing records lack it. In the Go follow-record builder
   (metasystem/internal/dispatch/build.go) and the dispatch script's
   follow-up path (metasystem/scripts/agents/dispatch.sh), an absent
   source on the parent is read as the parent's recorded
   reasoningEffort with source "record:pre-effort-keys"; only an absent
   reasoningEffort refuses. A test pins a follow-up on a parent record
   without the field.
2. (F-2) reasoningEffortSource joins the job record's immutable field
   list beside reasoningEffort (metasystem/internal/dispatch/record.go
   or wherever that list lives; say where).
3. (F-3) metasystem/scripts/agents/adapters/claude.sh clears a literal
   "null" reasoningEffort the way the codex adapter does.

Nothing else changes.

# Workspace

The same job worktree, on top of round two.
May touch: metasystem/internal/dispatch/build.go
May touch: metasystem/internal/dispatch/record.go
May touch: metasystem/internal/dispatch (tests)
May touch: metasystem/scripts/agents/dispatch.sh (the follow-up effort read only)
May touch: metasystem/scripts/agents/adapters/claude.sh (the effort read only)
Must not touch: anything else.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 40 minutes.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/dispatch` (expected: ok; name the follow-up test)
- `go vet ./internal/dispatch` and `gofmt -l ./internal/dispatch` (expected: clean)
- `bash -n ./scripts/agents/dispatch.sh ./scripts/agents/adapters/claude.sh` (expected: clean)
- `git diff --stat` (expected: the earlier files plus at most record.go)

# Acceptance Criteria

1. A follow-up on a pre-change parent record composes with the
   parent's effort and source record:pre-effort-keys.
2. The source field is immutable; the claude adapter clears null.

# Gap Rule

stop and report a gap; never fill it silently.
