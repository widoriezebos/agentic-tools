Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal rearm-remedies-and-adoption-notes, tier 2, hazard MECHANICAL)
Date: 2026-09-06

# Goal

Chain rra-build1 (landed from the certified diff of its round two, tree
eeb990d9d7a17401495551d1a1004dbee3be8ea9) added three legs to
metasystem/scripts/adopt-fixtures.sh. The second leg, "a preset landing
ref must be kept", gives its target an upstream with
`git update-ref refs/remotes/origin/trunk HEAD` followed by
`git branch --set-upstream-to=origin/trunk trunk`. On git 2.50.1 (Apple
Git-155, the fleet's Macs) the second command refuses "cannot set up
tracking information; starting point 'origin/trunk' is not a branch",
because a remote-tracking ref of a remote that is not configured is not
a branch to git; the bed runs under `set -euo pipefail`, so the leg
aborts before its adoption, and the third leg never runs. The
orchestrator proved it seat-side on 2026-09-06 in a scratch repository:
the update-ref route refuses, and a self-remote route works
(`git remote add origin <the target's own path>`, `git fetch -q origin`,
then `git branch --set-upstream-to=origin/trunk trunk` prints
"branch 'trunk' set up to track 'origin/trunk'" and `@{upstream}`
resolves to refs/remotes/origin/trunk).

When you are done, the second leg sets its upstream through a configured
remote and the bed's three new legs run and pass in order.

# The fix

In metasystem/scripts/adopt-fixtures.sh, in the preset-landing-ref leg
only: replace the `update-ref` line and the `--set-upstream-to` line
with a self-remote: add a remote named origin pointing at the target's
own path, fetch it quietly, then set the upstream of trunk to
origin/trunk (quiet). Nothing else in the leg changes: the preset key,
the adoption with `--runtimes none`, the kept-note grep and the
kept-value assertion stay byte for byte. Do not touch the other two new
legs or any existing leg.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/scripts/adopt-fixtures.sh (the preset leg only)
Must not touch: metasystem/scripts/adopt.sh, anything under internal, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken an assertion. One round, at most 30
  minutes of wall clock. Hazard MECHANICAL: a fixture's setup lines
  change; no product code moves.
- The bed's frozen gate needs process enumeration your sandbox lacks;
  do not run the bed. Prove the setup lines in a scratch repository
  instead (init, commit, the three commands, `git rev-parse
  --symbolic-full-name '@{upstream}'` prints refs/remotes/origin/trunk).

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/adopt-fixtures.sh` (expected: clean)
- the scratch-repository proof above (expected: refs/remotes/origin/trunk)
- `git diff --stat` (expected: only scripts/adopt-fixtures.sh, a handful of lines in one leg)

# Acceptance Criteria

1. The preset leg's upstream setup uses a configured remote and
   succeeds on git 2.50.1.
2. Nothing else in the bed changed.

# Gap Rule

stop and report a gap; never fill it silently.
