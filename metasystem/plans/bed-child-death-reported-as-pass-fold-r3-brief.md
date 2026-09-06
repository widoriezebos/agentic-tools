Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round three of chain bcd-build1 under goal bed-child-death-reported-as-pass, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Fold critic round one of chain bcd-build1. The critic's return is
metasystem/artifacts/agents/bcd-critic1/rounds/1/return.json; its
findings bind as stated there. Fold F-1 and F-2 by id; F-3 and F-4 are
noted and change nothing.

# The folds, by id

- F-1 (material, bounded): the operator-layout scenario's empty-runtime
  mode (METASYSTEM_SUPERVISION_OPERATOR_EMPTY_RUNTIME_FIXTURE_ONLY set
  to 1) leaves the child with an explicit `exit 0` at line 910 of
  metasystem/scripts/agents/supervision-fixtures.sh, before the
  completion line at the script's end, so cleanup now reports a
  finished child as dead (status 70) and that mode's suite fails. Fold:
  set `fixture_child_completed=1` on the line before that `exit 0`.
  Audit the child's paths for any other explicit `exit 0` before the
  end of the script (the `exit 0` at line 88 is the parent's, after the
  scenario loop, and needs nothing) and treat any you find the same
  way, naming each.
- F-2 (material, bounded): the self-test's deliberate death is
  converted to status 70, and cleanup's evidence-keeping branch then
  moves the child's temp directory to the suite-failures directory and
  prints "supervision fixture evidence preserved" into the log of a
  green run, one new directory per run. Fold: in cleanup, when
  `fixture_scenario` is bed-death-self-test, take the removal branch
  (remove the temp directory, keep nothing, print nothing) regardless
  of the status; every other scenario keeps its evidence on a nonzero
  status as before. The parent's expected-to-die rule is unchanged.

# Workspace

Your existing worktree for chain bcd-build1, on top of round two.
May touch: metasystem/scripts/agents/supervision-fixtures.sh
Must not touch: anything else.

# Constraints

- Bash 3.2 clean. No scenario assertion changes. Do not run the whole
  bed; run the commands below. At most 30 minutes of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/supervision-fixtures.sh` (expected: exit 0)
- `grep -n -B1 'exit 0' ./scripts/agents/supervision-fixtures.sh` (expected: the child's early exit preceded by the completion line)
- `grep -n 'bed-death-self-test' ./scripts/agents/supervision-fixtures.sh` (expected: the cleanup branch among the hits)
- The direct child probe from the round-two brief for bed-death-self-test (expected: rc=70 and no "evidence preserved" line)
- `METASYSTEM_SUPERVISION_OPERATOR_EMPTY_RUNTIME_FIXTURE_ONLY=1 bash ./scripts/agents/supervision-fixtures.sh` if your sandbox allows it (expected: operator-layout passed; if the sandbox refuses the bed, say so and the orchestrator runs it)
- `git diff --stat HEAD` (expected: the one file)

# Acceptance Criteria

1. The empty-runtime mode's operator-layout child exits 0 and is
   reported passed.
2. The self-test's death leaves no evidence directory and prints no
   preserved line; its status stays 70.
3. No other line changed.

# Gap Rule

stop and report a gap; never fill it silently.
