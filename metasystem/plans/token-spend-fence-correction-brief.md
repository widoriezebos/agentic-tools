Working Mode: implement
Orchestrator Identity: m3+main-1788172645-85501-aa86ee (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

The one correction round (R-60-m1) of the token spend fence build, job
fence-build-m3, after its Fable code review (job fence-review-code-m3-5,
reviewedTree 2c0572533ba23f967e63eaab321d3f9e003a7e4c). The review
returned one material finding and eight non-material ones; this round
corrects the material finding and one brief violation the review
recorded beside it. Nothing else in the tree moves. The design
(metasystem/plans/token-spend-fence-design.md, revision 2) and the
gap answers (metasystem/plans/token-spend-fence-dispositions-closing.md)
bind exactly as before; neither is reopened.

# Workspace

Your existing chain worktree on its own branch (HEAD b36638fd: the
cherry-picked build plus your unreadable-transcript commit). Do not
merge main; nothing this brief cites changed on main after your branch
point. Files: metasystem/internal/spend/ (transcript.go, measure.go and
their tests, testdata/bed-20260902/), metasystem/internal/steward/
(spend_fence_test.go and any steward test that reaches the real meter).

# Correction 1 (material): TSF-C1-seat-slug-not-git-toplevel

The reviewer's claim, verbatim: "The seat meter derives the Claude
transcript slug and the working-directory filter from the steward's
repo root instead of the Git toplevel that design section 3 binds. The
steward's repo root is the checkout's metasystem directory (where job
records, the conf and the goal ledger live), so the computed slug ends
in '-metasystem'. On this machine the seat's transcripts live under the
toplevel slug and record the toplevel as their working directory, so
the prefix match finds no directory and the cwd rule would reject every
line anyway. The seat segment then reports files=0 and tokens=0 while
the role stays alive."

Verified by the orchestrator against the design: section 3 says the
meter "walks ~/.claude/projects/<slug>/*.jsonl for every slug whose name
starts with the slug of the Git toplevel of repoRoot" and keeps a line
only when "the line's cwd is at or below the Git toplevel and NOT below
artifacts/agents/". The diff computes the slug from repoRoot itself
(strings.ReplaceAll(filepath.Clean(repoRoot), separator, "-")) and
seatCWD takes filepath.Rel(repoRoot, cwd). On this machine both slug
directories exist (`-Users-wido-LocalStorage-GitHub-agentic-tools-m3`
and `…-agentic-tools-m3-metasystem`); the design's prefix rule reaches
both, the built rule reaches only the second and rejects every line
whose cwd is the toplevel.

Build exactly the design's rule:

1. Resolve the Git toplevel of repoRoot once per Measure. Either
   `git rev-parse --show-toplevel` run in repoRoot, or walking parents
   of repoRoot for a `.git` entry (a file or a directory: a linked
   worktree has a file). Choose one and name it in your return; do not
   change the design's rule. If the toplevel cannot be resolved, that is
   one `seat unreadable` unmeasured entry naming the reason (the same
   rule the round-3 gap answer gives the home directory), Measure does
   not fail, the role stays alive.
2. The slug prefix is the slug of that toplevel; every directory under
   ~/.claude/projects whose name starts with it is walked (so both slugs
   above are read on this machine).
3. The cwd rule uses the toplevel: keep a line whose cwd is at or below
   the toplevel and not below `<toplevel>/**/artifacts/agents/`, exactly
   as section 3 words it.
4. Named test `TestSeatSlugIsGitToplevelNotRepoRoot` in internal/spend:
   a temporary toplevel holding a `.git` directory (or file) with
   repoRoot one level below it; a fake home whose projects directory
   holds a transcript under the toplevel's slug (lines with cwd equal to
   the toplevel) and one under `<toplevel slug>-<repoRoot basename>`;
   both files' tokens are counted, and a third slug directory whose name
   does not start with the toplevel slug is not read. Every existing
   fixture-bed test keeps passing; if the fixture bed itself needs a
   `.git` entry to resolve a toplevel, add it as a fixture file and list
   it in diffBoundary.

# Correction 2 (brief violation): TSF-C1-unstubbed-tests-list-real-home

The reviewer's claim, verbatim: "Existing steward tests that run
ObserveHealth or PreviewHealthAt without the measureSpend stub now call
the real meter, which lists the user's real ~/.claude/projects directory
looking for a slug that begins with the temporary root's slug."

The build brief binds: "never read the real ~/.claude in a test". Make
it so: every test in internal/steward and internal/spend that can reach
the real meter resolves the home directory to a temporary directory
(t.Setenv("HOME", t.TempDir()) at the reaching sites, or one shared
helper). Do not weaken or skip any test. Name the mechanism in your
return.

# Not in scope

The other seven review findings (TSF-C1-utc-day-test-vacuous,
aggregate-derives-before-filters, owner-skip-healthy-path-untested,
goal-ledger-parse-strictness, day-unmeasured-mixes-seat-requests,
spend-role-exempt-from-state-unreadable, machine-fallback-drops-records)
are recorded as non-material and are NOT corrected here. Do not touch
them; if correction 1 or 2 cannot be built without touching one, stop
and report the gap.

# Conformance requirement

Commit in the worktree. Your return's diffBoundary lists every
individual file your worktree changes relative to the dispatch base
7b10b1e8 (`git diff --name-only 7b10b1e8 HEAD`, each with the
`metasystem/` prefix): the 36 files already declared plus every file
this round adds or changes. No directory entries. Nothing under plans/
or artifacts/agents/.

# Constraints

- KNOWN SANDBOX LIMIT: run the focused proofs below; report anything
  environment-limited as such, never faked. The transcript reader is
  tested on fixture transcripts only.
- No test weakened; gofmt and go vet clean; coverage floors for touched
  packages hold (metasystem/scripts/agents/coverage-ratchet.json).
- Nothing in the design's section 9 (non-goals) moves.
- Wall-clock budget: 45 minutes.

# Expected Return

Version-2 implementer JSON; complete per-file diffBoundary as above;
evidence commands replayable from the worktree root including:
- `go build ./...`
- `go test ./internal/spend/ ./internal/mission/ ./internal/config/ ./internal/steward/ -count=1`
- `go test ./internal/spend/ -run 'TestSeatSlugIsGitToplevelNotRepoRoot|TestSeatUnreadableTranscriptIsCountedNotSkipped|TestSeatGoalDoesNotSilentlyLoseAgedTranscriptSpend' -count=1 -v`
- `go test ./internal/steward/ -run 'TestSpendFenceHealthLineBytes|TestTickCarriesSpendObservationAndUnknownDoesNotClearEpisodes|TestSpendFenceHigherMultipleRearmsWhileLowerMultipleRemainsCrossed' -count=1 -v`
- the proof that no steward or spend test reads the real home: `HOME=/nonexistent go test ./internal/spend/ ./internal/steward/ -count=1` passes
- `go list -deps ./internal/dispatch ./internal/goal ./internal/goalbudget | grep -c internal/spend` (expected 0)
- `gofmt -l internal/spend internal/mission internal/config internal/steward` (expected empty), `go vet` over the same

# Acceptance Criteria

- The seat meter's slug prefix and cwd rule are the Git toplevel's, per
  design section 3; an unresolvable toplevel is a counted `seat
  unreadable` entry, never a failure.
- TestSeatSlugIsGitToplevelNotRepoRoot exists and discriminates.
- No test reads the real ~/.claude.
- diffBoundary names every changed file individually.

# Gap Rule

stop and report a gap; never fill it silently — especially anywhere
the design underdetermines an implementation choice.
