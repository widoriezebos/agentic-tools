Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal delegate-follow-up-cannot-merge-main)
Date: 2026-09-06

# Round 5: fold the fresh critique of the follow-up rebase (chain followup-rebase-build1)

The fresh critic examined round 4 and reported three material findings,
all accepted. Every decision of the previous briefs (D1 to D11) stands.
The boundary is the one of the round-4 brief.

# The findings, in the critic's words

FRF-01 (high). When re-applying the round's changes fails and the
dispatcher restores the original worktree, it drops the tagged stash
entry even when the restore itself failed. The round's uncommitted
work then survives only as an unreferenced commit that nothing records,
and the refusal adds only the words "restoring the original worktree
also failed" without naming the stash hash.

FRF-02 (high). Brief authority admission runs over the message after
the dispatcher's conflict paragraph is prepended, and the paragraph
names each conflicted path in backticks, which the authority extractor
treats as a citation to check against the fast-forwarded HEAD tree.
When the trunk deleted a file the round modified, the re-apply leaves
that path unmerged, the paragraph cites it, and admission refuses
because the path is absent from HEAD. Every retry is refused the same
way.

FRF-03 (medium). A conflicted re-apply leaves unmerged entries in the
worktree's index. The paragraph tells the builder to resolve the
markers but not to stage the result. While those entries stand, git
refuses to stash the index when the trunk moves again on the chain's
files, and every later follow-up's plan reports the same paths as
unmerged and records them as conflicts.

# Decisions

D12. In metasystem/scripts/agents/dispatch.sh, a failed restore keeps
the tagged stash entry: the drop runs only when the reset, the clean
and the re-apply all succeeded. The refusal names the stash hash and
the tag so the work is recoverable by hand, and says which step
failed. A unit-level shell check or a fixture scenario proves the entry
survives a failed restore (a read-only worktree file or an index.lock
placed before the restore is an acceptable way to force it).

D13. Brief authority admits the caller's brief file, never the
dispatcher's prefixed message: the admission call reads the original
brief while the delivered message keeps the paragraph. A fixture
scenario pins a modify/delete conflict (the trunk deletes a path the
round modified): the follow-up is admitted, the record and the
paragraph carry that path, and a retry is admitted too.

D14. The paragraph asks the builder to resolve each conflicted path and
stage it (`git add`), so the index carries no unmerged entries when the
round ends. Extend the conflicted fast-forward fixture: after the
resolution is staged, a further follow-up plans no unmerged paths and
records no conflicts.

# Verification

As before: `gofmt -l .`, `go vet ./internal/dispatch/ ./cmd/metasystem/`,
`go test ./internal/dispatch/ -count=1`, `bash -n
scripts/agents/dispatch.sh`, and the dispatch fixture bed including the
new scenarios. Report every run and its outcome; a scenario you cannot
make pass is a gap to report, never to skip.

# Constraints

Wall-clock budget: 60 minutes. Gap rule: stop and report a gap; never
fill it silently. Return per the implementer schema with the full
boundary.
