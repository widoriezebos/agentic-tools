Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch follow-up under goal fast-gate-runs-the-refusal-register, tier 3, hazard MECHANICAL, fold of critique round one on chain fgr-build1)
Date: 2026-09-06

# Goal

Critique round one of chain fgr-build1 (critic fgr-critic1, reviewed
tree 6c8811fbf09889126777756ecdd79485b30ccacb) found nothing material
and two wording notes; both fold here. Change nothing else.

1. The commit wrapper's comment in metasystem/scripts/agents/commit.sh
   (the "IL-28 static re-proof" paragraph before the fast-gate call)
   lists what the boundary re-proves as gofmt, vet, staticcheck and the
   engine build. It now also runs the refusal register test; name it in
   that list, in the same voice, and change nothing else in the file.
2. The exclusion reason in metasystem/internal/refusal/register.go for
   the token `ledger-unreadable` says the sentinel "refuses nothing".
   The path that writes that digest does block a stop; the token itself
   is a digest slot value the steward compares, not a refusal code.
   Reword the reason to say that: a turn-verdict digest sentinel the
   steward compares, not a refusal code. Keep the pattern.

# Workspace

Your existing job worktree for fgr-build1, on top of round one.
May touch: metasystem/scripts/agents/commit.sh (that comment only)
May touch: metasystem/internal/refusal/register.go (that reason only)
Must not touch: anything else.

# Constraints

- Two lines of prose change; no behavior changes; no test changes.
- Never weaken a test. One round, at most 20 minutes of wall clock.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/refusal` (expected: ok)
- `bash -n ./scripts/agents/commit.sh` (expected: clean)
- `git diff --stat` against the round-one tree (expected: only the two files)

# Acceptance Criteria

1. The commit.sh comment names the refusal register among what the
   boundary re-proves.
2. The exclusion reason no longer says the sentinel refuses nothing.
3. Nothing else changed; the refusal tests pass.

# Gap Rule

stop and report a gap; never fill it silently.
