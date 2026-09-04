Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal channel-code-verified-at-poll-time)
Date: 2026-09-04

# Build brief: verify the human's code against the message's send time

Goal `channel-code-verified-at-poll-time` (tier 1, approved by Wido's standing word of 2026-09-04 for the next backlog pick; box 1 hour / 3 attempts / 1 active job / no review round). No critic in this chain.

## The defect

`internal/channel/poll.go` verifies the six-digit code of an inbound reply with `VerifyTOTP(secret, code, c.Now)` (line 175): the poll time, with one 30-second step of slack either way (`internal/channel/totp.go`, `VerifyTOTP`). The steward polls once per tick (about two minutes), so a reply sent more than about 45 seconds before the next poll is rejected as "bad code" even when the code was right. Today Wido's first replies to two questions were rejected this way and only the replies that happened to land seconds before a poll were accepted. Wido: "we probably have an issue with the polling and the expiry of the token; this seems to be very tight on reading in time before expiry".

## What to build

The Telegram update carries the message's send time (`internal/channel/telegram/telegram.go`, the wire message's `date` field). Carry it on the inbound message type the poll consumes (a `SentAt time.Time`, zero when the provider has none), and verify the code against `SentAt` when it is set, else against the poll time as today, with the same one-step slack. Keep the replay guard keyed on the code's step. Refuse a reply whose send time is older than the poll interval plus one step with a reason that names the age ("code too old: sent 190s before the poll"), so a late poll cannot be used to replay an old code. Do the same for the Slack adapter if its wire message carries a timestamp; otherwise leave it on poll time and say so in the return. Document the rule in one line where the channel's answer format is described (`docs/orchestration.md` if it holds that text).

## Verification

`go test ./internal/channel/...` with a test that a reply sent 100 seconds before the poll with the code of its own send step is accepted, a reply sent 400 seconds before the poll is refused with the age in the reason, and a provider with no send time still verifies against the poll time. Run `gofmt -l`, `go vet ./internal/channel/...` and `go run honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/channel/...` (the landing gate runs this pinned version).

## Bounds

Touch `internal/channel/poll.go`, `internal/channel/totp.go` if the verify signature needs it, the inbound message type, the Telegram (and, if applicable, Slack) adapter, their tests, and one doc line. Every path in your return (diffBoundary, files) is relative to the repository root, so it starts with `metasystem/`.
