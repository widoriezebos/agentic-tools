Working Mode: implement
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Fix build over the breach-clock build breach-build-2. Your worktree starts at
main; the build lives on branch agent/breach-build-2 at commit 0d8e47ef in the
same repository (shared object store). FIRST `git cherry-pick 0d8e47ef`: it must
apply without conflict (main has moved since the build's base e5056c26, so
the picked tree differs from 0d8e47ef's own tree; that is expected). Confirm
instead that `git diff HEAD~1 --stat` lists the same twenty-nine files as
`git diff e5056c26 0d8e47ef --stat`; only then edit. (The earlier wording of
this check, an empty diff against 0d8e47ef, was the orchestrator's error;
job breach-build-3 stopped on it correctly.) Then fold the
three decisions of metasystem/records/misc/breach-code-critique-r1.md
(landed; read it first, the decisions are binding). The design is
metasystem/plans/breach-clock-and-budget-honesty-design.md revision 6a. The
standard is Wido's: hard deterministic machinery; no refusal weakened, no
guarantee narrowed to make a test pass.

# The folds

1. F-1, metasystem/internal/goal/verbs.go, the Done verb: compute the
   `displaced` pair BEFORE the fence block that sets `f.Claimed = nil`, the
   way release does at lines 703-706 of the same file (in the cherry-picked
   tree) and the reconcile done row does in
   metasystem/internal/goal/reconcilepub.go (lines 316-328). Nothing else in the block moves:
   the human guard, `VerifyStopBatchComplete`, and the four clears stay in
   their order. New test in metasystem/internal/goal/verbs_test.go, named
   TestHumanDoneOverFencedForeignClaimAcksDisplacement: a goal claimed by pair
   A and breach-fenced, `goal done` by the human (actor human:wido) from a
   different machine; assert the done history line carries the Displaced
   marker with pair A's prefix and that the acknowledgment change set names
   pair A, mirroring what the existing displaced-release assertions check
   (verbs_test.go around lines 780-784 and 1202-1203).
2. F-2, metasystem/internal/goal/verbs_test.go: a writer-level test named
   TestSetBudgetInheritsEpisodeObligationRevision: claim, set-obligation (the
   third episode key becomes that obligation's revision, non-zero), discharge
   the obligation so none is live, then a SetBudget raise; assert the key on
   the re-bound claim record equals the pre-raise non-zero value. Change
   `rebindClaimKeepEpisode` only if this test fails, and then report why.
3. F-4, metasystem/scripts/agents/goal-cli-fixtures.sh, the day-refusal row
   (lines 463-466 in the cherry-picked tree): the `git cat-file -p` must
   succeed or the row fails with its own message; only its successful output
   is grepped for `^- Budget:`.

# Gate

gofmt, go vet, go build; go test with -count=1 over internal/goal, internal/dispatch,
internal/goalbudget and cmd/metasystem green; `bash -n` on
the fixture script. The repository-wide run's known sandbox failure
(TestHolderProbeUnreadableArgvIsNeverDead, process id 1 readable there) is
not yours; the fixture scripts are run by the orchestrator on the host. Paste
the final lines. Commit on top of the cherry-pick; the diffBoundary is the
three files above.

# Constraints

Wall-clock budget: 45 minutes. R-31: no benchmarks. Version-2 implementer
JSON.

# Gap Rule

stop and report a gap; never fill it silently.
