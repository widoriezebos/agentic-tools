Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal setup-refusals-consume-attempts)
Date: 2026-09-04

# Build brief: a dispatch refused before any agent ran is not an attempt

Goal `setup-refusals-consume-attempts` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

A job record that ends in a setup refusal (`refusalClass` `setup` and `phase` `setup` on the record: a census fingerprint that had not caught up with a freshly armed engine, a brief header defect, an engine older than its scripts) still counts as one of the goal's attempts and keeps its reserved job minutes in the goal-budget admission. On 2026-09-04 the dispatcher-skew item lost two of its three tier-1 attempts to setup refusals while its finished fix sat on a preserved branch, and the box closed ("attemptLimit used=3 limit=3, reservedJobMinutesLimit used=360 limit=360").

## What to build

In the goal-budget admission (`internal/goalbudget`) and wherever the dispatch records reservations, a record whose terminal status is a setup refusal releases its reservation and is not counted as an attempt; a refusal after the agent started (a protocol error on the return, a timeout, a cancel) still counts. Name the rule in the admission's refusal message so a seat reading "used=3" knows what counted.

## Verification

`go test ./internal/goalbudget/... ./internal/dispatch/...` with a test that pins: two setup-refused records plus one completed record leave two attempts free of three; a protocol-error record counts. Run `gofmt -l`, `go vet` and `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on the touched packages.

## Bounds

Touch the goal-budget admission, the dispatch reservation accounting, their tests, and one line in `docs/backlog-mechanism.md` where attempts are defined. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
