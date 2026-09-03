Working Mode: implement
Orchestrator Identity: m3+main-1788172645-85501-aa86ee (dispatch delegate under goal token-spend-fence)
Date: 2026-09-03

# Goal

Finish the build of goal token-spend-fence, step 1 (alert mode), on a
new implementer chain. The previous chain (on another machine) built
nearly the whole feature and its own tests passed; its bytes are
preserved unaltered as commit b5a73581 on branch agent/fence-build. You
continue from that commit under the same brief,
metasystem/plans/token-spend-fence-build-brief.md, the accepted design
metasystem/plans/token-spend-fence-design.md (revision 2) and the test
obligations and gap answers in
metasystem/plans/token-spend-fence-dispositions-closing.md. Nothing in
those documents is reopened here.

# Workspace

The dispatch-created job worktree, branched from main. FIRST ACT, before
any edit: bring the preserved build into your worktree with

    git cherry-pick b5a73581

(the commit is in the shared object store; `git cat-file -t b5a73581`
prints `commit`). It merges cleanly onto today's main (verified with
`git merge-tree`); if it does not, stop and report the conflict as a
gap. Then read the tree you now hold: metasystem/internal/spend/
(measure.go, transcript.go, their tests, testdata/bed-20260902/),
metasystem/internal/config/spend.go and validate.go,
metasystem/internal/mission/fence.go (JobUsageAt),
metasystem/internal/steward/health.go, alert_episode.go, tick.go,
spend_fence_test.go, and metasystem/metasystem.conf.

# What remains (the one open gap, now answered)

The round-3 gap: what the seat meter does when a present transcript
path cannot be listed, stat'd or read. The answer is recorded in
metasystem/plans/token-spend-fence-dispositions-closing.md (gap answers,
the round-3 row) and binds exactly:

1. Every present transcript path (the `~/.claude/projects` root, a slug
   directory, one `.jsonl` file) whose listing, stat or read fails is
   one explicit unmeasured ledger entry {path, error} with reason
   `seat unreadable` — counted, never skipped, and NEVER an error from
   Measure. A home directory that cannot be resolved is one such entry
   naming the reason. Today transcript.go returns an error for each of
   these (readSeat: the projects ReadDir, the Glob, the Stat, the
   ReadFile, the scanner error) and returns silently on the home lookup;
   all of those become entries.
2. `spend.SeatSummary` gains `UnreadableFiles int` (json
   `unreadableFiles`): the count of those entries. One count for both
   scopes.
3. The seat segment of the section-6 health line gains ` unreadable=<n>`
   after `aged=<n>`:
   `seat tokens=<n> lifetime=<n> files=<n> aged=<n> unreadable=<n> unmeasured requests=<n>`.
   `files` keeps counting the readable files that entered a row.
   `TestSpendFenceHealthLineBytes` is updated to the new exact bytes.
4. The role stays ALIVE; section 6's unknown conditions do not change.
5. New named test `TestSeatUnreadableTranscriptIsCountedNotSkipped` in
   internal/spend: an unreadable transcript path in the fixture bed — a
   directory named `<id>.jsonl` under the seat slug, which fails
   ReadFile in every sandbox without chmod — yields `unreadable=1` with
   its path and error in the ledger's unmeasured list, Measure returns
   no error, and the other files' tokens are unchanged.

Everything else in the preserved commit stands unless a test you run
proves otherwise; report any such finding as a gap, never patch it
silently.

# Conformance requirement (why the previous round was refused)

`metasystem validate conformance` checks that EVERY changed path
appears in the union of the chain's `diffBoundary` declarations. The
previous chain declared the fixture DIRECTORY
`metasystem/internal/spend/testdata/bed-20260902/` and was refused
because directories do not cover their files. Your return's
diffBoundary must list every individual file your worktree changes
relative to main, with the `metasystem/` prefix: every file the
cherry-pick brings (35 files: `git diff --name-only main` in the
worktree lists them, prefix each with `metasystem/`) plus every file
you add or change for the gap answer, including every new fixture file
under testdata/bed-20260902/transcripts/. No directory entries.

# Constraints

- KNOWN SANDBOX LIMIT: the full validation suite needs real process
  visibility your sandbox denies; run the focused proofs named below and
  report anything environment-limited as such, never faked. The
  transcript reader is tested on the fixture transcripts only; never
  read the real ~/.claude in a test.
- No test weakened; gofmt and go vet clean; coverage floors for touched
  packages hold (metasystem/scripts/agents/coverage-ratchet.json).
- Nothing in the design's section 9 (non-goals) moves: no adapter usage
  writer, no admission path, no goal or goalbudget package, no new CLI
  verb, nothing under plans/.
- Commit your work in the worktree when done (the conformance diff
  includes uncommitted work too, but a commit keeps the cherry-picked
  bytes and yours separable).
- Wall-clock budget: 60 minutes.

# Expected Return

Version-2 implementer JSON; complete per-file diffBoundary as above;
evidence commands replayable from the worktree root including:
- `go build ./...`
- `go test ./internal/spend/ ./internal/mission/ ./internal/config/ ./internal/steward/ -count=1`
- `go test ./internal/spend/ -run 'TestSeatUnreadableTranscriptIsCountedNotSkipped|TestSeatGoalDoesNotSilentlyLoseAgedTranscriptSpend|TestUnreadableJobRecordCannotDisappear' -count=1 -v`
- `go test ./internal/steward/ -run 'TestSpendFenceHealthLineBytes|TestTickCarriesSpendObservationAndUnknownDoesNotClearEpisodes|TestSpendFenceHigherMultipleRearmsWhileLowerMultipleRemainsCrossed' -count=1 -v`
- `go list -deps ./internal/dispatch ./internal/goal ./internal/goalbudget | grep -c internal/spend` (expected 0)
- `gofmt -l internal/spend internal/mission internal/config internal/steward` (expected empty), `go vet` over the same

# Acceptance Criteria

- The cherry-picked commit is in the worktree unaltered except by the
  gap-answer edits above.
- Every test named in design section 8, the two closing obligations and
  the new unreadable-transcript test exist and pass.
- The health line's seat segment carries `unreadable=<n>`; an unreadable
  transcript path never makes Measure fail or the role unknown.
- diffBoundary names every changed file individually.

# Gap Rule

stop and report a gap; never fill it silently — especially anywhere
the design or the gap answer underdetermines an implementation choice.
