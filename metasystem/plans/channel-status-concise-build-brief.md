Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal channel-status-concise)
Date: 2026-09-04

# Build brief: the fleet status post says only what the human needs

Goal `channel-status-concise` (tier 1, approved by Wido's word of 2026-09-04, box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain; the orchestrator checks a rendered sample.

## The defect

`internal/channel/report.go` `ComposeReport` builds the status post every machine's steward sends to the fleet Telegram channel (`internal/channel/phase/phase.go`, every `channel.status.interval-minutes`). Today it emits up to twelve "Landed since" lines, twelve "Under way" lines, twelve "Planned" lines (every queued goal, the whole backlog, every time), a spend line and an undelivered line. Wido: "I'm only interested in things that need my judgement/decision and in what was delivered and what will be picked up next IN A CONCISE WAY. Not a full dump of the backlog (every time!)".

## What to build

Rewrite the composition to exactly three parts, in this order, each omitted when empty:

1. **Needs you** — one line per item that waits on the human: an open channel question (`internal/channel/question.go` knows them), a goal whose next step waits for a human word (state approved-pending or a `NormApproval`/`Approved` gap; use what the goal package exposes, do not invent a state), and a budget raise. Say what is being asked in one plain sentence, feature name first.
2. **Delivered** — one line per feature landed on origin/main since the previous post's time (the status state's `LastPost`, falling back to four hours on the first post), plain feature name and the landing subject; no counts, no timestamps in the line.
3. **Next up** — the next one or two approved items this machine will pick, in backlog order (approved and unclaimed, pinned to this machine or unpinned); feature name only. Never the whole queue.

Constraints: at most twelve lines in total, header line included (`<machine> status <time>Z`); no spend line (cost anomalies are incidents raised on their own); keep the undelivered-messages line only when undelivered is nonzero, as the last line. Post nothing when the text has not changed since the last post: `ShouldPost` already compares the content digest, keep that behaviour and make sure the header timestamp does not defeat it (put the time in the post metadata or round it so an unchanged fleet stays quiet). The window start for "Delivered" is the previous post time, not a fixed four hours, so a landing is reported once.

## Verification

`go test ./internal/channel/...` with table tests for: empty fleet posts nothing new; each part omitted when empty; the twelve-line cap; a landing reported in exactly one post. In the return, include one rendered sample post for a fleet with one open question, two landings and three approved items (only two shown), so the orchestrator can read it.

## Bounds

Touch `internal/channel/report.go`, its test, and `internal/channel/phase/phase.go` only if the window start needs the previous post time passed in. No docs beyond the `docs/orchestration.md` line that describes the status post, if one exists. Return within the box.

Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/` (for example `metasystem/internal/channel/report.go`).
