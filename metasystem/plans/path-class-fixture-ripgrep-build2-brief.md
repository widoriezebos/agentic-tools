Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal path-class-fixture-ripgrep)
Date: 2026-09-04

# Build brief: carry the ripgrep fixture fix through a chain

Goal `path-class-fixture-ripgrep` (tier 1, approved by Wido 2026-09-04, box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## What happened

The first build chain produced the right fix to `scripts/agents/path-class-fixtures.sh` (the two ripgrep searches replaced by `grep -rnE -I` with the same exclusions, and a search failing with a status other than 0 or 1 reported as a broken search). Its return was rejected by the protocol only because the diffBoundary named the file without the repository prefix. The fix is preserved on branch `preserve/pcr-build1-r1`, and the fixture suite passed with it seat-side in 38 seconds.

## What to do

In your worktree run `git cherry-pick --no-commit preserve/pcr-build1-r1`, confirm the diff is exactly that one file, run `bash -n` on it, and return. Every path in your return (diffBoundary, files) is relative to the repository root: `metasystem/scripts/agents/path-class-fixtures.sh`. Change nothing else. Do not run the fixture suite (KI-15: your sandbox cannot run process-owning suites; the orchestrator already did).
