Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal channel-fake-server-synthetic-timestamps)
Date: 2026-09-04

# Build brief: the fake channel server stamps replies with real time

Goal `channel-fake-server-synthetic-timestamps` (tier 1, approved under R-76-m2, Wido's word of 2026-09-04; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`internal/channel/fake/fake.go` stamps every Slack reply it serves with a synthetic counter timestamp (for example `1000002.000000`, which is in 1970) and ignores any `ts` the fixture supplies in its replies file. Since the code send-time landing (4b919708), the Slack adapter verifies a reply's code at its send time and refuses a reply older than the poll interval plus one step, so the channel fixture's coded answer is answered "not recorded: code too old: sent 1787543923s before the poll" (seen in the fake journal on m2, 2026-09-04 17:46Z) and `scripts/agents/channel-fixtures.sh` is red.

## What to build

Make the fake server stamp a reply with the current time in Slack form when the replies file gives no `ts`, and preserve a `ts` the file supplies; keep thread references consistent (a reply's `thread_ts` still names its root). Do the same for the fake Telegram update's `date` (current time unless supplied). Keep the ordering guarantees the fake's existing tests rely on (a counter may still order messages; the timestamp must be real).

## Verification

`go test ./internal/channel/fake/... ./internal/channel/...` with a test that a served reply without a supplied ts carries a timestamp within a minute of now. `gofmt -l`, `go vet`, `go run honnef.co/go/tools/cmd/staticcheck@2025.1` on the package. The orchestrator runs `channel-fixtures.sh` seat-side and expects it green. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.

## Bounds

Touch `internal/channel/fake` and its tests only.
