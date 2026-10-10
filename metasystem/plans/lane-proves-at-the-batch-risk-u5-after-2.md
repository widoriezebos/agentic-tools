# Unit U5 correction 2 (last): a reused check keeps the real check's record; the reason names cmd/metasystem

Working Mode: Implement. Second and last correction of unit U5 (brief `plans/lane-proves-at-the-batch-risk-u5-cheapness-measure.md`; correction 1 `u5-after-1.md`). The worktree holds U5 plus correction 1 uncommitted on origin/main b67642167; change only what this brief names.

## F-1 (material): a reused check overwrites the fix record's check time with about 0 minutes

`cmd/metasystem/test_impact.go` ~63-79 (the deferred fix-record writer) runs on the reuse return (~86-91) too, measuring `time.Since(started)` over the reuse alone and marking green; in the normal green flow the check is reused twice (the job's collection, then `commitResolvedMerge`), so every green fix ends with "last check green in 0.00 minutes" and the real duration survives only in repair-check.json. Fix by subtraction: on the reuse path write nothing to the fix record (or write the cached `DurationMS` and result with a `reused` marker); the record keeps the real check's time. Test: a real check (sleep 2 s) then two reuses -> the fix record still carries the real minutes and result (mutation: write on reuse -> ~0, test fails); status shows "last check green in N min (reused)" or the unchanged line.

## N-1 (not material, fix while here): the reason names cmd/metasystem

A plan whose only change-driven selection is named cmd tests reads "impact would cover 0% of 169 packages: full" (test_impact.go ~443; gate_impact.go ~83): make the reason say `change selects cmd/metasystem: full` in that case (the builder's report claimed this text exists; it does not). Keep the share text for the share rule.

## Checks

`go test -count=1 -timeout 30m ./cmd/metasystem -run 'TestLandingDepth|TestLandingCheck|TestTestImpact|TestLandingGate|TestLandingStatus|TestLandingMerge|TestLaneMerge'`; `./internal/landing/plain`; `go run ./cmd/devgate static`; `./internal/audit`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of this correction, each exit, the fix record after a check and two reuses.
