Working Mode: implement
Orchestrator Identity: m1b (lineage main-1788680071-18713-e76d5d, dispatch delegate under goal delegate-follow-up-cannot-merge-main)
Date: 2026-09-06

# Round 2: fold the code critique of the follow-up rebase (chain followup-rebase-build1)

The independent critic examined round 1 and reported four material
findings and four noted ones. The material ones are accepted and become
decisions D7 to D10 below; one noted item is folded as D11 because it
is one reordering. The contract of round 1 stays in force:
metasystem/plans/follow-up-rebase-build-brief.md (D1 to D6). Nothing
outside the round-1 boundary plus the files named here.

# The findings, in the critic's words

FRB-01 (severe, irreversible). The re-apply step can silently lose a
file the round created. When the stash apply leaves at least one
tracked conflict and also fails to restore an untracked file because
the trunk now tracks a file at the same path, the dispatcher treats the
apply as a conflicted success, drops the stash, and the round's version
of that file survives only as an unrecorded dangling commit. The
orchestrator reproduced this on this Mac's git 2.50.1: the apply exits
1 with "new.txt already exists, no checkout" and "error: could not
restore untracked files from stash", the tracked path is unmerged, the
collided path holds the trunk's bytes, and the round's bytes sit only in
the stash's third parent.

FRB-02. The plan verb refuses every follow-up on a chain that has a
round without a readable diffBoundary as soon as the trunk has moved at
all. A round that ended in a protocol error and every critic round look
like that. Since worktrees never commit, the dirty path set already
carries every round's live changes, so a missing boundary loses no path.

FRB-03. A repeated follow-up wrapper for a round that was rebased is
always refused: it never re-prepends the rebase paragraph, so it cannot
reproduce the delivered message whose hash the standing reservation's
fingerprint binds (REFUSED-OPID-MISMATCH, fingerprint-equality-gate-failed).

FRB-04. Any refusal between the rebase and the record write (brief
authority, the critique exhaustion gate, packet composition, preflight,
the claim) leaves the worktree fast-forwarded with conflict markers and
the stash dropped. The retried follow-up plans behind zero, records no
rebase provenance and delivers no conflict paragraph, so the builder is
not told about the markers.

FRB-07 (noted). A root record without a launchMode field skips the
rebase and the warning silently, although the later fallback derives
the launch mode from the workspace path.

# Decisions

D7. In metasystem/scripts/agents/dispatch.sh, the apply counts as a
conflicted success only when the apply's failure is confined to merge
conflicts: after `git stash apply`, when the stash has an untracked
tree (a third parent), every path in that tree must stand in the
worktree with the stash's bytes (compare blob ids). If any path is
missing or differs, or the apply output reports that untracked files
could not be restored, the rebase takes the restore branch and the
follow-up refuses, naming the colliding paths. The refusal leaves the
worktree exactly as it was: original head, the round's changes in
place, the round's copy of the collided file in place, no tagged stash
entry left. A dispatch fixture scenario in
metasystem/scripts/agents/dispatch-fixtures.sh pins it: the round
created a file, the trunk later commits a different file at that path
and also changes a tracked path the round changed; the follow-up
refuses, the worktree is back at its original head with the round's
bytes at the collided path, and the stash list carries no entry for the
tag. This scenario would pass against round 1's code by dropping the
stash; assert it refuses.

D8. In metasystem/internal/dispatch/followup_rebase.go, a round without
a readable diffBoundary contributes no paths: a missing return.json, or
a return whose diffBoundary is absent or null, is skipped. A return
that exists but does not decode as JSON still refuses. The reason is
stated in the code's comment in application terms: worktrees never
commit, so the dirty path set carries every round's changes and the
boundaries only add paths a later round restored to head. Unit tests in
metasystem/internal/dispatch/followup_rebase_test.go: a round with a
critic-shaped return (no diffBoundary) and a round with no return.json
both plan from the dirty set plus the other rounds' boundaries; an
overlap that comes only from a dirty path absent from every boundary
plans a rebase. Adjust the existing unreadable-round test so that it
asserts the undecodable case only.

D9. The rebase paragraph is a pure function of the three record fields
rebasedFrom, rebasedTo and conflictedPaths. On the repeated follow-up
path in metasystem/scripts/agents/dispatch.sh, the wrapper reads those
three fields from the standing child record and, when rebasedTo is
non-null, prepends the identical paragraph before the critic carry, so
the reconstructed message hashes equal to the winner's. Pin it: extend
the repeated-follow-up fixture, or add a sibling scenario, so that a
follow-up whose record carries a fast-forward is repeated and admitted
instead of refused with REFUSED-OPID-MISMATCH. If the fixture bed
cannot leave a rebased child standing, report the gap with what was
tried; do not weaken the assertion.

D10. The plan verb reports `unmergedPaths`: the worktree's unmerged
index entries (`git diff --name-only -z --diff-filter=U`), sorted. When
the plan does not rebase and unmergedPaths is non-empty, the wrapper
records conflictedPaths from them with rebasedTo set to the worktree's
head and rebasedFrom null (an earlier wrapper fast-forwarded this
worktree; this one found its markers), and prepends the paragraph built
from those fields. The record builder in
metasystem/internal/dispatch/record.go admits that shape: a null
rebasedFrom with a non-null rebasedTo requires a non-empty
conflictedPaths. A dispatch fixture scenario pins the path: a follow-up
whose brief cites a path that does not exist refuses at brief authority
after the rebase ran; the corrected brief's follow-up is admitted, its
record carries the conflicted path with rebasedFrom null, and its
prompt begins with the paragraph naming that path.

D11. In metasystem/scripts/agents/dispatch.sh, the launch mode is
derived before the rebase block, with the same fallback the later code
uses (a workspace under the worktrees directory is a worktree launch),
so a root record without the field still rebases and still warns.

# Boundary

Round 1's boundary plus nothing new: metasystem/scripts/agents/dispatch.sh,
metasystem/scripts/agents/dispatch-fixtures.sh,
metasystem/internal/dispatch/followup_rebase.go,
metasystem/internal/dispatch/followup_rebase_test.go,
metasystem/internal/dispatch/record.go and
metasystem/cmd/metasystem/dispatch_verbs.go are the files this round
may touch. Source comments state application facts; never a finding
id, a round or a brief.

# Verification

Run `gofmt -l .`, `go vet ./internal/dispatch/ ./cmd/metasystem/`, `go
test ./internal/dispatch/ -count=1`, `bash -n
scripts/agents/dispatch.sh`, and the dispatch fixture bed including the
new scenarios. Report every run and its outcome; a scenario you cannot
make pass is a gap to report, never to skip.

# Constraints

Wall-clock budget: 60 minutes. Gap rule: stop and report a gap; never
fill it silently. Return per the implementer schema with the full
boundary.
