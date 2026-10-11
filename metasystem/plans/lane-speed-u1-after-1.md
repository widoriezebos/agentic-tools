# lane-speed U1 correction 1: the resolution diff is unrestricted, never empty-as-reviewed, and keeps the merge brief

Working Mode: Implement. Correction after the Opus read (RETURN, 3 material). Worktree uncommitted; change only what this brief names; leave fixes uncommitted; never commit, rebase or push; never touch memory/receipts.log, plans/ or testing.json. The design copy in this worktree (plans/designs/lane-speed.md, S1) is amended and is the current one: read S1 first.

## M1 (high): resolution edits outside the conflicted paths are never read
cmd/metasystem/landing_merge.go ~203-206 restricts the diff (merge-tree tree -> merge commit) with a `:(literal)` pathspec over fix.Paths. The resolution job (internal/landing/plain/resolve.go commitResolvedMerge ~597-627) stages everything it changed, so a third file edited to make the merge compile is never read by anyone (the goal's branch read never saw it, the merge read filters it out). The merge-tree tree already holds both sides' clean merges, so the unrestricted diff contains the resolution and nothing of the goal's own change. Fix: drop the pathspec; exclude only the record's generated paths (`:(exclude)` for fix.Generated, regenerated bundles). Test: the lane-merge-1 dispatch fixture gets a third-file edit in the resolution → the frozen critic inputs name the third file and still not the goal-only file (mutation: restore the pathspec → the third file is missing → test fails). The existing test computes its expectation with the same limited command — fix that so it cannot pass by construction.

## M2 (high): an empty resolution diff marks the merge reviewed with no critic
landing_merge.go ~152 and ~159: `|| read.Empty` sets fix.State "done" and prints "Reviewed the resolved merge" when StartRead returns Empty — which happens whenever every conflicted path was resolved to the side merge-tree wrote (modify/delete, binary: no markers, no diff). Fix: Empty is never "reviewed". When the resolution diff is empty, the read's subject becomes the first-parent diff of the conflict record's paths (the old scope, restricted), with a brief sentence saying the resolution kept merge-tree's side; a record with no paths still errors. Test: a modify/delete conflict kept as modified → a read is dispatched with the conflicted path's first-parent diff, state is not done until the critic answers (mutation: keep the Empty branch → state done without a read → test fails).

## M3 (medium): the merge brief is dropped from the critic's instructions
landing_merge.go ~108 reads fix.Brief and discards it; the test asserts the absence of "What happened before". The merge brief (fixes/<attempt>.build.md: per-path main intent, goal intent, expected resolution, the keep-both-sides/never-drop-a-test rule) is what the critic judges the resolution against. Fix: keep prepending fix.Brief, then the scope sentence ("the goal's units were read on their branch; this read covers the resolution only"). Test: the frozen instructions contain the brief's rule sentence and the scope sentence; drop the assertion of absence.

## Folds (non-material, cheap)
- status.go: the finding line comes from the newest OPEN (not closed) merge record, so an old over-bound reading does not print forever.
- readFixRecords errors appear once in status problems, not twice.
- Do not send the resolution patch twice (CommitDiff and INPUT 1): keep it as CommitDiff only, INPUT 1 becomes the conflict record (paths, classes) and the check result.
- internal/launch/unit_run.go ~152 OSGitRunner.Run error-text change: revert if the test helper can get stdout another way; otherwise keep and say so.

## Checks
`go vet ./cmd/metasystem ./internal/landing/plain ./internal/launch`; `go test -count=1 -timeout 30m ./internal/landing/plain ./internal/launch ./internal/config ./internal/layering`; `go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLaneMerge|TestLanding|TestLaneFix|TestSkill|TestAudit|TestUnitReview'`; `go run ./cmd/devgate static` (the lane-speed design-ledger complaint is expected). Report: diff --stat, production line count (≤250 total for the unit), every exit, each mutation.
