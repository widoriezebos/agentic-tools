Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, follow-up round two of chain lrr-build1 under goal landing-receipt-races-the-narrator-digest, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

Fold critic round one of chain lrr-build1. The critic's return is
metasystem/artifacts/agents/lrr-critic1/rounds/1/return.json; its
findings bind as stated there. Fold F-1, F-2 and F-3 by id; F-4 is
noted and changes nothing. Everything else stands as reviewed.

# The folds, by id

- F-1 (material, severe): the cleanup in the detached-worktree helper
  file round one created in the gittree package (its cleanup function,
  around line 128) ends every
  receipt run with a repository-wide `git worktree prune`. Prune erases
  the registration of any other linked worktree whose directory is
  missing at that moment (a scratch worktree on an unmounted volume, a
  moved folder), losing its index and detached HEAD; the repository's
  decision record D95 ruled against exactly this after a dry run on this
  machine. The verb's own worktree is already unregistered by the
  `git worktree remove --force --force` above it. Fold: delete the prune
  call and its error. If the remove fails, the cleanup error already
  carries it; nothing else changes. Say in the function's comment why
  prune is never called from here.
- F-2 (not material, folded because it is one assertion): the
  narrator-append test in metasystem/internal/landing/receipt_test.go
  asserts the receipt names the candidate while the digest moved but
  never asserts the command's recorded exit status is zero. Add that
  assertion to that test.
- F-3 (not material, folded because it is one string): the verb's
  --command flag help in metasystem/cmd/metasystem/landing_verbs.go
  line 37 says the command runs from the project root; it now runs from
  the isolated copy's workspace directory. Fix the sentence.

# Workspace

Your existing worktree for chain lrr-build1, on top of round one.
May touch: the detached-worktree helper file round one created in the
gittree package (the only new file in the chain)
May touch: metasystem/internal/landing/receipt_test.go
May touch: metasystem/cmd/metasystem/landing_verbs.go
Must not touch: anything else.

# Constraints

- No other behavior change; never weaken a test. At most 30 minutes of
  wall clock. Rebuild the engine after the Go change.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory), with the private
build cache from round one:

- `go test -count=1 ./internal/landing ./internal/gittree` (expected: ok)
- `go vet ./internal/gittree ./internal/landing ./cmd/metasystem` and `gofmt -l ./internal ./cmd` (expected: clean, no output)
- `grep -rn 'prune' ./internal/gittree` (expected: only the comment saying why it is never called from the helper)
- `bash scripts/agents/go-build.sh` (expected: a rebuilt engine line)
- `git diff --stat HEAD` (expected: the chain's files plus landing_verbs.go)

# Acceptance Criteria

1. No receipt run calls `git worktree prune`; the verb's own worktree is
   still unregistered and removed on every path (the existing signal
   and failure legs still pass).
2. The narrator-append test asserts exit status zero.
3. The flag help names the isolated workspace.

# Gap Rule

stop and report a gap; never fill it silently.
