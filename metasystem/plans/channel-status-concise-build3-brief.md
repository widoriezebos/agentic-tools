Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal channel-status-concise)
Date: 2026-09-04

# Build brief: "Needs you" lists decisions, not the unapproved backlog

Goal `channel-status-concise` (tier 1, approved by Wido's word of 2026-09-04; this is the box's last attempt). No critic in this chain.

## The defect

The concise status post landed (4dd26cfc), but its first part treats every queued goal without an execution approval as something the human must decide now. On m2 today it renders eleven "Needs you: <feature> — approve it for execution." lines and hits the twelve-line cap before "Delivered" and "Next up" get a line. That is the backlog dump again under a new label. Wido wants only things that need his judgement or decision.

## What to build

In `internal/channel/report.go`, the "Needs you" part lists exactly:

1. every open channel question (as now), one line: feature name and what is asked;
2. at most one approval request: the single goal this machine would pick next (the first in backlog order that is queued, has a budget, is not blocked, is pinned to this machine or unpinned, and lacks execution approval), rendered "Needs you: <feature> — your word to start it (next in line)". If the next in line is already approved, no approval line at all;
3. budget raises only when a question of kind budget-above-norm is open (covered by 1).

"Delivered" and "Next up" keep their meaning; "Next up" lists the next one or two approved items, so an unapproved next-in-line appears once, in "Needs you", never in both. When the twelve-line cap binds, the parts are trimmed in the order Next up, Delivered, then Needs you, so a decision is never cut in favour of a delivery line.

## Verification

`go test ./internal/channel/...` with tests for: eleven unapproved queued goals produce one "Needs you" line, not eleven; an approved next-in-line produces none; the cap trims Next up before Needs you. Include the rendered sample for that eleven-goal fleet in your return. Run `gofmt -l`, `go vet ./internal/channel/...`, and `go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/channel/...` (the landing gate runs this pinned version; report if the run is impossible in your sandbox).

## Bounds

Touch `internal/channel/report.go` and `internal/channel/channel_test.go` only. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
