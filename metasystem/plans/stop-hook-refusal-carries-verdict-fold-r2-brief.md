Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round two of chain shr-build1 under goal stop-hook-refusal-carries-verdict, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Round one stands as built. Seat-side on this Mac under the stock bash
3.2 (2026-09-06 10:25Z, your worktree, your rebuilt engine) the hook
fixtures pass, and the supervision suite's stop-hook-monitor scenario
now passes its first refusal (the block reason begins with the OPEN WORK
display, HEALTH is in the system message, the second firing does not
block, the settled stream does not block) and its byte-identity check.
It then stops at the goal-open leg of S4-15(b), before any hook
assertion, with:

    flag provided but not defined: -tier
    Usage of goal open:
      -caller-pid int ... -id string ... -intent string ... -next string ... -root string

The stop root is a fresh git-init checkout, a legacy single-file ledger
world; its goal open takes root, caller-pid, id, intent and next, and no
tier. The fixture line at
metasystem/scripts/agents/supervision-fixtures.sh line 1787 to 1788
passes `--tier 3`, a synced-ledger flag that arrived with the tiering
landing. Fold: remove `--tier 3` from that one goal-open call and change
nothing else about it. The scenario's later assertions (the goal reaching
the turn end once, the spent revision reading as the all-clear, session
hygiene, the degraded path, and the S4-16 monitor legs) are then
reachable and must pass; if one does not, stop and report it as a gap
with the scenario's exact output.

Out of scope, as before: the census-lifecycle scenario ("announced main
pid <empty> outside its scenario bed"), unchanged by round one; report
only.

# Workspace

Your existing worktree for chain shr-build1, on top of your round-one
work.
May touch: metasystem/scripts/agents/supervision-fixtures.sh
Must not touch: anything else. The round-one changes in
metasystem/internal/report/stopblock.go,
metasystem/internal/report/stopblock_test.go and
metasystem/scripts/agents/supervision-hook.sh stay exactly as they are.

# Constraints

- One line changes. No other assertion, wait, or flag moves.
- Do not run the supervision suite or the hook fixtures in this round;
  the sandbox cannot see the processes they need and the orchestrator
  runs both seat-side on your worktree. Run only the commands below.
- At most 20 minutes of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/supervision-fixtures.sh` (expected: exit 0)
- `grep -n -- '--tier' ./scripts/agents/supervision-fixtures.sh` (expected: no output)
- `grep -n 'id fixture-goal' ./scripts/agents/supervision-fixtures.sh` (expected: the one call, without a tier flag)
- `git diff --stat HEAD` (expected: the four round-one files; the fixture file's hunks are the BASHPID line and this line)

diffBoundary lists every path touched across the chain so far, each
starting with `metasystem/`.

# Acceptance Criteria

1. The fixture's goal-open call for fixture-goal carries no tier flag
   and is otherwise byte-identical.
2. No other line of any file differs from round one.

# Gap Rule

stop and report a gap; never fill it silently.
