Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal effort-by-complexity-per-model, tier 2, hazard DESIGN-BEARING, fold round two of chain ebc-build1)
Date: 2026-09-07

# Goal

Round one built the effort configuration from
metasystem/plans/effort-by-complexity-per-model-brief.md and stopped
on the gap rule with two owners outside the brief's boundary: the
compose-role-packet command in metasystem/cmd/metasystem/dispatch_verbs.go
must accept the dispatch mode and the inherited effort value and
source so the composition can resolve and carry them (the provisional
edit was reverted), and the whole-configuration validator in
metasystem/internal/config/validate.go accepts only runtime and model
mode-scoped keys, so the new mode.<mode>.role.<role>.effort.<runtime>.<class>
keys are rejected. This round widens the boundary to those two files
and finishes the chain.

# The fold

1. metasystem/cmd/metasystem/dispatch_verbs.go: the compose-role-packet
   command takes the flags the composition needs (the mode, and for a
   follow-up round the parent's resolved effort and source) and passes
   them through; metasystem/scripts/agents/dispatch.sh supplies them
   where it already calls the command.
2. metasystem/internal/config/validate.go: the validator admits the
   effort key family (mode.<mode>.role.<role>.effort.<runtime>.<class>,
   role.<role>.effort.<runtime>.<class>, role.default.effort.<runtime>.<class>)
   with the class spellings mechanical, design-bearing and
   destructive-reach, and refuses any other class word naming the key;
   a validator test pins an accepted and a refused key.
3. Finish whatever round one left partial because of the two gaps, so
   that every item of the round-one brief holds end to end: a
   DESIGN-BEARING claude dispatch records reasoningEffort high with
   its source key and the claude argv carries --effort high; the
   fixtures in metasystem/scripts/agents/dispatch-fixtures.sh run the
   five legs the brief asked for.
4. Note for round two: the combined Go test's only red in your sandbox
   was a host-process-visibility test outside this change; the
   orchestrator runs the packages seat-side.

# Workspace

The same job worktree, on top of round one.
May touch: everything round one could, plus
metasystem/cmd/metasystem/dispatch_verbs.go and
metasystem/internal/config/validate.go (and its test file).
Must not touch: the codex adapter's flag, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 90 minutes.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/dispatch ./internal/adapter ./internal/config ./cmd/metasystem` (expected: ok apart from the named host-visibility test)
- `go vet` and `gofmt -l` on the same packages (expected: clean)
- `bash -n ./scripts/agents/dispatch.sh ./scripts/agents/adapters/claude.sh ./scripts/agents/dispatch-fixtures.sh` (expected: clean)
- `git diff --stat` (expected: the round-one files plus dispatch_verbs.go, validate.go and its test)

# Acceptance Criteria

1. The composition receives the mode and the inherited effort through
   the command; the validator admits the effort keys and refuses a bad
   class word.
2. Every round-one acceptance criterion holds; no gap remains.

# Gap Rule

stop and report a gap; never fill it silently.
