# token-spend-fence step 1: the one code review and its correction round

Reviewer: Fable lane (claude-fable-5-1, job fence-review-code-m3-5), 2026-09-03,
dispatched by m3 with `--reviews fence-build-m3`. Reviewed: the conformance
artefacts of implementer job fence-build-m3 (Sol) at reviewedTree
2c0572533ba23f967e63eaab321d3f9e003a7e4c: the m1b build cherry-picked
byte-identical (patch-id 0a9283a64dbcbac286c3b1bd58f3396e8cd03510) plus one
commit for the round-3 gap answer (unreadable seat transcripts). Against
`plans/token-spend-fence-design.md` revision 2, both build briefs, the closing
dispositions and `plans/token-spend-fence-code-review-brief-m3.md`. Finding
standard R-60-m1: material only if it changes what gets built.

Four earlier dispatches of this review (fence-review-code-m3, -2, -3, -4) ended
before their first turn on `API Error: 529 Overloaded`, zero tokens; the fifth
ran to a verdict. All five registers were folded with
`job critique-register-advance` before the follow-up round was opened.

## Findings

One material, eight not.

### Material, bound to the correction round

- TSF-C1-seat-slug-not-git-toplevel (high). The seat meter computes the
  transcript slug from repoRoot (the checkout's `metasystem` directory) and
  filters lines by cwd against the same root; design section 3 binds both to
  the Git toplevel of repoRoot. On this machine the seat's transcripts sit
  under the toplevel's slug with the toplevel as cwd, so the built rule
  reports the seat as files=0 tokens=0 while the role stays alive. The
  orchestrator verified the deviation against section 3; one detail of the
  reviewer's evidence is wrong (both slug directories exist on this machine,
  the reviewer saw only one), which does not change the finding. Returned to
  the implementer as correction 1 of
  `plans/token-spend-fence-correction-brief.md`.

### Carried into the correction round as a brief violation, not material

- TSF-C1-unstubbed-tests-list-real-home (low). Steward tests that reach the
  real meter list the real `~/.claude/projects`; the build brief says "never
  read the real ~/.claude in a test". No transcript is read, nothing built
  changes, so not material under R-60-m1; corrected in the same round because
  a brief rule is a brief rule (correction 2: tests resolve HOME to a
  temporary directory, proof `HOME=/nonexistent go test`).

### Not material, recorded, not corrected

- TSF-C1-utc-day-test-vacuous: TestDayIsUTCDateOfStartedAt cannot fail for its
  own claim; the replay test's exact day total still certifies the UTC rule.
- TSF-C1-aggregate-derives-before-filters: AggregateUsage and Measure derive
  usage before the mission/terminal filters; extra work, same bytes.
- TSF-C1-owner-skip-healthy-path-untested: the Owner skip in the healthy
  clear loop is verified by reading only.
- TSF-C1-goal-ledger-parse-strictness: any parse problem in plans/goals makes
  the spend role unknown; within section 6's wording.
- TSF-C1-day-unmeasured-mixes-seat-requests: seat request-shape failures
  with today's date also count in the day scope's unmeasured; the design's
  wording leaves both readings open.
- TSF-C1-spend-role-exempt-from-state-unreadable: the spend role alone is
  exempt from the unreadable-prior-state → unknown rule; section 6 lists the
  unknown conditions exhaustively, which supports the exemption.
- TSF-C1-machine-fallback-drops-records: with no enrolled nickname the ledger
  is measured under "this machine" and job records fall out of both scopes;
  enrollment is required for dispatch, design silent.

These seven are candidates for the backlog, not for this feature: the
design is not reopened here (R-25b-m1).

## Correction round and re-review

Correction round fence-build-m3-r2 (Sol, brief
`plans/token-spend-fence-correction-brief.md`): one worktree commit e9c4ef60
over three files (the spend transcript reader and its measure tests, the
steward spend fence test). The toplevel is resolved by walking parents of
repoRoot for a `.git` file or directory; the slug prefix and cwd rule use
it; an unresolvable toplevel is one `seat unreadable` entry. The steward
test package gets a TestMain that sets HOME to a temporary directory; the
spend tests already used per-test HOMEs. Review-stage conformance passed at
reviewedTree 750ca6cf9bc982d1316eeb1b2f7c52672cc238d7; the pre-commit
guard's litter in the worktree control plane was moved aside before the
run, as in round 1 (goal conformance-runtime-state-litter). The correction
brief's paraphrase of the second guard ("artifacts/agents/") was looser
than design section 3 ("artifacts/agents/worktrees/"); the implementer
followed the design, which binds.

Re-review fence-review-code-m3-r2 (Fable, brief
`plans/token-spend-fence-code-review-brief-m3-r2.md`): zero material
findings. Standing of the nine: the material one closed, the real-home one
closed, seven unchanged. Five new notes, none material: TSF-C2-slug-nonseparator-characters
(the slug replaces only path separators; Claude Code dashes every
non-alphanumeric character; unaffected on this fleet's paths, and the rule
is unchanged since round 1), TSF-C2-toplevel-walk-keeps-symlinks (the walk
keeps a logical path, `git rev-parse` would not; the brief allowed either),
TSF-C2-unresolvable-test-depends-on-tmpdir (the unresolvable-toplevel test
assumes no ancestor `.git` above the temp dir), TSF-C2-correction-brief-guard-wording
(the paraphrase above), TSF-C2-unresolvable-entry-display-path-is-dot (the
unresolvable entry's id renders as `.`). The reviewer's session had no
shell; the orchestrator reproduced the proofs in the worktree: `go build`
clean, `go list -deps` count 0 for dispatch/goal/goalbudget, gofmt empty,
spend and steward tests green with `HOME=/nonexistent` (Go caches pinned).

## Landing

The chain landing's static re-proof (staticcheck U1000) refused the
certified tree for an unused helper, `formatTokens`, and its `math` import
in internal/spend/measure.go, which neither review nor the briefed proofs
(gofmt, vet) had caught. With the one correction round and the once-only
re-review spent, the deletion landed on Wido's word (R-70-m3, "Option one,
my word") inside the chain landing 0acb0973, disclosed there; the chain
bar recorded `would-refuse code=chain-output-mismatch` on that commit,
which is the word's footprint. The seven non-material and five new notes
are backlog candidates, not corrections.
