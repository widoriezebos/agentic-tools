Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round four of chain shr-build1 under goal stop-hook-refusal-carries-verdict, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Round three's fold of the critic's F-1 is right and stays: the first
failed-stop block now carries the extras (the arming failure line among
them) before the check-in tail in its system message, as design point
GOAL-05 wants. Seat-side, under the stock bash 3.2, the hook fixtures
pass on round three and the stop-hook-monitor scenario now stops at its
very first guard, before any assertion, with "the Stop payload failed
after the fixture supplied the enrolled engine". That guard, at line
1753 of metasystem/scripts/agents/supervision-fixtures.sh, fails the
scenario whenever the first response contains the generic prefix
"Metasystem supervision arming failed:". It dates from the landing that
made one verb arm everything and was written to catch a fixture whose
engine was never enrolled; in this stop root the enrollment is supplied
by the fixture and arming still fails at the supervision-owner component
(the root carries no supervision configuration), which the scenario
tolerates by design. Until round three that arming line never reached
the response, because the old failed-stop path dropped the extras; now
that the extras are shown, the guard matches the wrong thing.

Fold: narrow the guard to the enrollment failure it was written for.
The arming line reads `Metasystem supervision arming failed: up
outcome=<outcome> component=<component> ...`; the enrollment failure
names `component=accepted-engine` (outcome ENROLLMENT_DRIFT) and no
other arming failure does. Change the guard's match from the generic
prefix to `component=accepted-engine`, keep its message and exit, and
change nothing else. The response seen seat-side at that guard was a
lawful block: reason "OPEN WORK (1): ..." then the failure sentence then
the guidance, HEALTH in the system message.

# Workspace

Your existing worktree for chain shr-build1, on top of rounds one to
three.
May touch: metasystem/scripts/agents/supervision-fixtures.sh
Must not touch: anything else.

# Constraints

- One match string changes. Do not run the suite; the orchestrator runs
  it seat-side. At most 15 minutes of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/supervision-fixtures.sh` (expected: exit 0)
- `grep -n 'component=accepted-engine' ./scripts/agents/supervision-fixtures.sh` (expected: the one guard)
- `grep -c "Metasystem supervision arming failed:'" ./scripts/agents/supervision-fixtures.sh` (expected: 0)
- `git diff --stat HEAD` (expected: the chain's seven files, unchanged counts but this line)

diffBoundary lists every path touched across the chain so far, each
starting with `metasystem/`.

# Acceptance Criteria

1. The guard fails the scenario only when the first response names the
   accepted-engine component; its message and exit are unchanged.
2. No other line differs from round three.

# Gap Rule

stop and report a gap; never fill it silently.
