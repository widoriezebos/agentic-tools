Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal rearm-remedies-and-adoption-notes, tier 2, hazard DESIGN-BEARING, fold round two of chain rra-build1)
Date: 2026-09-06

# Goal

Round one of chain rra-build1 built the five remedies from
metasystem/plans/rearm-remedies-and-adoption-notes-brief.md; the
orchestrator's seat-side proof is green (the up and steward packages,
vet, gofmt, bash -n). The round-one critique (rra-critic1, two
material findings, both low) found two sentences that mislead and one
untested fallback. This round folds them in the same worktree, on top
of round one. When you are done, every remedy sentence names a repair
that can actually apply, a detached target with a preset landing ref
gets the kept note, and the fallback sentence is pinned.

# The fold

1. The arm-lock remedy (critique F-1). arm in
   metasystem/internal/steward/runner.go takes the arm lock with a
   blocking exclusive flock (LOCK_EX, no LOCK_NB), so a second arm
   waits for a holder and never errors; the "open arm lock" and "take
   arm lock" errors come from permissions, a read-only or full
   filesystem, the open-file limit, or an invalid descriptor. In
   metasystem/internal/up/up.go, beforeMintRemedy's sentence for those
   two classes must say that: name the lock file path and say to make
   it creatable, openable and lockable by this user (its directory's
   permissions, free space, the open-file limit), then rerun
   metasystem up. Never say to find or end a holder. Update the two
   matching cases in metasystem/internal/up/up_test.go to the new
   sentence.
2. The detached-plus-preset note (critique F-2). In
   metasystem/scripts/adopt.sh, read the preset key
   (git config --local --no-includes --get) BEFORE the detached test,
   the order the Go seeder uses: a preset key prints the kept note and
   nothing else, whether or not the target is detached; only then does
   a detached target without a key print the detached note; the
   upstream logic for a branch without a key is unchanged. In
   metasystem/scripts/adopt-fixtures.sh add a third leg beside the two
   from round one: a detached target with a preset key adopts, exits
   zero, prints the kept note, does not print the "not seeded" note,
   and keeps the value.
3. The fallback sentence (critique F-5). In up_test.go's before-mint
   table add one case with an unrecognised error message that pins the
   unchanged fallback sentence "repair the named enrollment publication
   failure, then rerun metasystem up".

Nothing else changes: the not-landed remedy, the human-arm replace
path and its test, and the round-one legs stay as they are.

# Workspace

The same job worktree, on top of round one.
May touch: metasystem/internal/up/up.go
May touch: metasystem/internal/up/up_test.go
May touch: metasystem/scripts/adopt.sh
May touch: metasystem/scripts/adopt-fixtures.sh
Must not touch: runner.go, rearm_test.go, rearm_resolver.go, steward_verbs.go, supervision-fixtures.sh, anything under plans.

# Constraints

- Bash 3.2 clean. Never weaken a test. One round, at most 45 minutes
  of wall clock.
- The adopt fixture bed's frozen gate needs process enumeration your
  sandbox lacks; do not run the bed, say so, the orchestrator runs it
  seat-side.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/up` (expected: ok)
- `go vet ./internal/up` and `gofmt -l ./internal/up` (expected: clean, no output)
- `bash -n ./scripts/adopt.sh` and `bash -n ./scripts/adopt-fixtures.sh` (expected: clean)
- `grep -n 'holding the steward arm lock' ./internal/up/up.go ./internal/up/up_test.go` (expected: no output)
- `git diff --stat` (expected: the six round-one files, nothing new)

# Acceptance Criteria

1. The two arm-lock classes get a sentence naming the lock file and
   the permission, space and descriptor repairs; no sentence names a
   holder.
2. A detached target with a preset landing ref prints the kept note
   only and keeps the value; the third leg pins it.
3. The fallback sentence is pinned by a table case.

# Gap Rule

stop and report a gap; never fill it silently.
