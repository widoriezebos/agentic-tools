# Unit U3 correction 2 (last): the gate's static group runs through the seam the test beds already fake

Working Mode: Implement. Second and last correction of unit U3 (brief `plans/lane-proves-at-the-batch-risk-u3-gate-and-reuse.md`; correction 1 `plans/lane-proves-at-the-batch-risk-u3-after-1.md`). The worktree holds U3 plus correction 1 uncommitted on top of 02720e1c9; change only what this brief names.

## The defect (material): U3 turns 10 existing cmd tests red

All ten pass at 02720e1c9 and fail in this worktree, before and after correction 1: TestLandingTrunkExceptionProvenBatchPush, TestLandingBatchAdapterCapsGateProofPushAndRetry, TestLandingExecutionAdapterRetriesSelectedBatchUnderHelmAndPause, TestLandingBatchAdapterDescendantRehandIn, TestLandingBatchAdapterNoSelectionForEmptyQueueOrTrunk, TestLandingBatchAdapterPolicyAdmissionBoundaries, TestLandingBatchAdapterSameSHARehandIn, TestLandingProofPermissionAuthorityMatrix, TestLandingProofPermissionFollowScopeStopCommands, TestOneGoalPerBatchLandsReturnsEveryRedAndReturnsAConflict. Their beds set the engine path to `/fixture/engine` (e.g. `cmd/metasystem/landing_batch_test.go:61`); U3's gate now really runs `'/fixture/engine' test groups fast-static-build` through /bin/sh, exit 127 ("fast-static-build: the proving command exited 127; isolated check … did not complete"). The merge-gate bed fakes the static command; these beds do not.

## Fix

The gate's static group (and the parent replay of correction 1) runs through the SAME proof-command seam those beds already fake for the proof itself (the seam that makes the batch proof's runner a fixture in these tests), so a fixture engine path never reaches /bin/sh in a test; production behaviour is unchanged (the engine from os.Executable). Where a bed fakes the proof command but the static run needs its own answer, the fake answers green for the static group by default. Prefer the seam fix over editing ten beds; edit a bed only where its assertions must learn the new gate record fields.

Also (not material, one line if cheap): exit 127 of the proving command (engine missing) counts as an environment red in `observeRed`, like a launch error.

## Checks

The ten tests above by name, plus `TestLandingGate|TestLandingProve|TestLandingStatus|TestReadStatus|TestSkillLandingAgent|TestLandingPush|TestLandingDepth|TestLandingImpact`; then the WHOLE cmd/metasystem package once (`go test -count=1 -timeout 30m ./cmd/metasystem`, sharded as you like, this is the integration check for the goal); `go test -count=1 -timeout 30m ./internal/landing/plain ./internal/audit`; `go run ./cmd/devgate static`. Never edit testing.json; never open metasystem.conf.local; do not commit. Report: diff --stat of this correction, each exit (every red named), the ten tests before/after.
