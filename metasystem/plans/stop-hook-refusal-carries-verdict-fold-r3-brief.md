Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round three of chain shr-build1 under goal stop-hook-refusal-carries-verdict, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Fold critic round one of chain shr-build1. The critic's return is
metasystem/artifacts/agents/shr-critic1/rounds/1/return.json; its
findings bind as stated there. Fold F-1, F-2 and F-3 by id; F-4 and F-5
are noted and change nothing. Everything else in the chain stands as
reviewed; the seat-side proof on your round-two worktree was green
(hook fixtures, all seven supervision scenarios under the stock bash
3.2, and the report and goal packages under the race detector).

# The folds, by id

- F-1 (material): on the first firing of a recorded stop failure, the
  block the refusal record produces carries only the check-in tail as
  its system message, while the verdict has already consumed the
  watchdog digest (surfaceWatchdog true) and the hook afterwards
  advances the protocol cursor. A fresh watchdog report and a
  protocol-growth notice are therefore consumed without being shown.
  In compose_failed_stop in metasystem/scripts/agents/supervision-hook.sh,
  the first-occurrence branch passes the system message to the refusal
  record verb as the check-in tail alone; make it the extras (the
  non-blocking detail the caller already hands in: arming failure,
  evidence failure, the watchdog text when surfaced, the protocol
  message) followed by the check-in tail, exactly as the ordinary block
  path composes its system message. The repeated-cause branch already
  carries the extras; leave it.
- F-2 (not material, folded because it is one comment): the comment
  above runReportStopBlock in metasystem/cmd/metasystem/report.go still
  says the verb appends the caller detail; it now leads with it. Fix
  the sentence.
- F-3 (not material, folded so the fixture reads the engine's
  contract): the isolation loop in
  metasystem/scripts/agents/supervision-fixtures.sh skips three
  holder-state names, while the engine's list in
  metasystem/internal/census/announcement.go also names
  worktree-commit-token.json. Add that fourth name to the skip; keep
  the rule that any other pidless file still fails.

# Workspace

Your existing worktree for chain shr-build1, on top of rounds one and
two.
May touch: metasystem/scripts/agents/supervision-hook.sh
May touch: metasystem/cmd/metasystem/report.go
May touch: metasystem/scripts/agents/supervision-fixtures.sh
Must not touch: anything else.

# Constraints

- Comments and the two named lines only; no test changes.
- Do not run the supervision suite or the hook fixtures; the
  orchestrator runs both seat-side. Rebuild the engine after the Go
  comment change so the binary matches the tree.
- At most 30 minutes of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `bash -n ./scripts/agents/supervision-hook.sh` and `bash -n ./scripts/agents/supervision-fixtures.sh` (expected: exit 0)
- `grep -n 'worktree-commit-token.json' ./scripts/agents/supervision-fixtures.sh` (expected: the skip)
- `grep -n 'appending any caller' ./cmd/metasystem/report.go` (expected: no output)
- `go vet ./cmd/metasystem` (expected: clean)
- `bash scripts/agents/go-build.sh` (expected: a rebuilt engine line)
- `git diff --stat HEAD` (expected: the chain's files plus report.go)

diffBoundary lists every path touched across the chain so far, each
starting with `metasystem/`.

# Acceptance Criteria

1. The first-occurrence failed-stop block's system message is the
   extras, then the check-in tail, in that order, and contains the
   HEALTH line.
2. The report.go comment describes detail-first composition.
3. The isolation loop skips four named holder-state files.

# Gap Rule

stop and report a gap; never fill it silently.
