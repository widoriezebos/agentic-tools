Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal fixture-suite-drift-carry)
Date: 2026-09-04

# Build brief: finish the fixture-suite drift fix and carry it through a chain

Goal `fixture-suite-drift-carry` (tier 1, approved by Wido's channel answer "allow two more rounds" of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round), carrying the fix of goal fixture-suite-drift-after-approval-gate. No critic in this chain.

## What happened

Three rounds of the first chain produced a seven-file change (channel, dispatch, supervision and adopt fixtures; the `channel.poll-timeout-sec` key; one doc line): approve-then-claim in the fixtures, a configurable poll context, census patience counted in completed passes, corrected model-alias roster legs, a private engine build inside the dispatch suite's temporary root, and a converted goal state provisioned in the dispatch fixture's agent repository. Seat-side the channel suite is green and the dispatch scenario now fails at exactly one place: the serving-goal leg (`scripts/agents/dispatch-fixtures.sh`, the `fixture-serving` goal) opens its goal but never claims it, and a converted checkout refuses: "no serving goal to project: a converted checkout serves this machine's claimed goal" (exit 3). The work is preserved on branch `preserve/fsd-build1-r3`.

## What to do

In your worktree run `git cherry-pick --no-commit preserve/fsd-build1-r3` and confirm the diff is those seven files. Then make the serving-goal leg approve the `fixture-serving` goal (`goal approve` with the fixture's tier box, `--by Wido`, a temporary human word of at least three words and `--review-by`), claim it bare, and dispatch with `--serving-goal`; update the leg's comment to say a converted checkout serves the machine's claimed goal. Keep the earlier refusal sub-leg (no usable goal refuses exit 3 without a job record), which runs before the goal exists. Run `bash -n` on the changed scripts, `gofmt -l` and `go vet` on the changed Go files, and `go test ./internal/config/... ./internal/channel/...`. Do not run the fixture suites (KI-15; the orchestrator runs all five seat-side). Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
