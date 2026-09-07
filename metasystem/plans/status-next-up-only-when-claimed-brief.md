Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal status-next-up-only-when-claimed, tier 1, hazard MECHANICAL)
Date: 2026-09-07

# Goal

The channel status post prints "Next up" from the ledger's ready
frontier (ComposeReport in metasystem/internal/channel/report.go:
`for _, id := range frontier.Ready { next = append(next, "Next up: "...`),
so it names goals no machine has claimed. Wido: next up is only
interesting when the machine has completed something and will pick
that goal up, which means a goal it has claimed.

When you are done, the Next up line names only a goal this machine
holds a claim on, and only in a post that also carries a Delivered
line; with nothing claimed, or nothing delivered, there is no Next up
line; the tests prove both.

# The change

In ComposeReport: replace the ready-frontier loop with the goals this
machine has claimed (frontier.Claimed from goal.Next(p, c.Machine), or
the projection's live files whose Claimed record names c.Machine;
say which and why), at most two, and add the Next up lines only when
the delivered part is non-empty (compute delivered before next, or
gate when assembling). Nothing else in the report changes.

Tests in metasystem/internal/channel/channel_test.go: adjust
TestReportShowsOneQuestionTwoLandingsAndOnlyTwoNextItems so its two
Next up items are goals claimed by the reporting machine; add one case
where a ready but unclaimed goal produces no Next up line, and one
where a claimed goal with no landing produces no Next up line. Keep
every other assertion.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/channel/report.go
May touch: metasystem/internal/channel/channel_test.go
Must not touch: anything else. The landing is the tier-1 lane: at most
three files and forty changed lines in total, so keep the change tight.

# Constraints

- One round, at most 30 minutes. Hazard MECHANICAL, tier 1: no
  critique; the orchestrator lands it through the tier-1 lane with
  `go test ./internal/channel/` as the receipt.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/channel/` (expected: ok)
- `go vet ./internal/channel/` and `gofmt -l ./internal/channel/` (expected: clean)
- `git diff --stat` (expected: the two files, forty lines or fewer in total)

# Acceptance Criteria

1. Next up names only this machine's claimed goals and only beside a
   Delivered line; otherwise absent.
2. The tests pin both; the diff fits the tier-1 bound.

# Gap Rule

stop and report a gap; never fill it silently.
