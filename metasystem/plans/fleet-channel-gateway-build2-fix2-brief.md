Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Third round of build step 2 of goal fleet-channel-gateway: one
regression the seat's gate found. `go test ./cmd/...` fails on the
existing `TestTelegramPeekWorksWithoutConfiguredAdapterOrChatID`
(metasystem/cmd/metasystem/channel_verbs_test.go:107): it talks to the
fake with the token `environment-token`, which the fake on main
accepted (any `/bot<token>/<method>` path was served) and which your
`telegramRoute` now refuses with 404 "invalid fake Telegram token or
method". Your round-1 brief said the bare token "stays accepted with
listener "" so today's fixtures run unchanged" and that existing tests
pass unmodified; an unknown token is part of today's behaviour. Fix
that; change nothing else. Your round-1 and round-2 briefs
(metasystem/artifacts/agents/fcg-build2/rounds/1/prompt.md,
metasystem/artifacts/agents/fcg-build2/rounds/2/prompt.md) still
govern everything they say.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

- May write: metasystem/internal/channel/fake/fake.go
- May write: metasystem/internal/channel/fake/fake_test.go

No other file changes in this round. The failing cmd test is not
touched.

# The fix

`telegramRoute`: a token that is neither the bare
`fake-telegram-token` nor `fake-telegram-token-<listener>` is served
exactly as the bare token is — listener "", same shared stream and
offset. Only a malformed path (no method, an empty token, a slash in
the method) stays refused. The listener-bearing forms keep their
listener.

Test (fake_test.go): a request with an arbitrary token is served with
listener "" and journaled as such; the two existing forms keep their
routing (extend the existing telegramRoute cases if there are any).

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/channel/...
./cmd/...`, `go test ./internal/channel/... ./cmd/... -count=1` — the
cmd package is the point of this round; report its result
explicitly — and `go run honnef.co/go/tools/cmd/staticcheck@2025.1
./internal/channel/...`. Record every mechanical choice in
`whatWasDone`. Every path in your return is relative to the
repository root, so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 15 minutes. The two-file boundary above is exact.

# Gap Rule

Stop and report a gap only for a law-changing contract this brief does
not settle; a mechanical choice is made from what the tree does
nearest the seam, recorded in `whatWasDone`, and built.
