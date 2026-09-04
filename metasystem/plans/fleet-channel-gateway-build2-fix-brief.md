Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Follow-up round of build step 2 of goal fleet-channel-gateway. Your
round-1 return (metasystem/artifacts/agents/fcg-build2/rounds/1/return.json)
reported one gap: the brief forbade touching the channel phase loader,
which is the only owner that reads a checkout's conf and builds the
DestinationConfig. That boundary was the seat's error and is lifted
for this round (Wido, 2026-09-05, "Yes, include the loader in the fix
round"). Wire the two conf keys through the loader; change nothing
else. Your round-1 brief
(metasystem/artifacts/agents/fcg-build2/rounds/1/prompt.md) still
governs everything it says.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

- May write: metasystem/internal/channel/phase/phase.go
- May write: metasystem/internal/channel/phase/phase_test.go

No other file changes in this round.

# The wiring

## channel.http-timeout-sec on every real adapter

In `loadSlack` and `loadTelegram` (phase.go 63-95), read
`channel.http-timeout-sec` through `Get(root, key, "30")`, parse it
as a positive integer of seconds and set `dest.HTTPTimeout` to that
duration before returning. A value that does not parse as a positive
integer is returned as an error the way an unreadable key is (the
validator refuses it at conf validation; the loader refuses it at
load). The fake face keeps its default (the adapters already fall
back to 30 s when HTTPTimeout is zero) — do not read the key in
loadFake.

## channel.fake.listener on the telegram fake face

In `loadFake` (97-116), the telegram branch reads
`channel.fake.listener` through `Get(root, key, "")` and passes it to
`fake.TelegramProvider(dir, listener)`; an empty value passes nothing,
so today's fixtures mint the bare token unchanged. The slack face
does not read the key.

Both keys are read through `Get`, so a conf.local line overrides the
committed conf exactly as the other channel keys do; neither key is a
secret.

# Tests (phase_test.go)

- loadTelegram with `channel.http-timeout-sec = 7` yields
  `Destination.HTTPTimeout == 7 * time.Second`; absent → 30 s;
  `channel.http-timeout-sec = 0` and `= abc` are refused at load.
- loadFake with face telegram and `channel.fake.listener = m3` yields
  Token `fake-telegram-token-m3` and that token in Secrets; absent
  listener yields the bare token.
- Follow the existing phase_test.go conventions for writing a
  temporary metasystem.conf and a fake base-url file.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/channel/...
./cmd/...`, `go test ./internal/channel/... ./internal/config/...
-count=1`, `go run honnef.co/go/tools/cmd/staticcheck@2025.1
./internal/channel/... ./cmd/...`. The seat runs
scripts/agents/channel-fixtures.sh on the worktree (your sandbox
cannot own the fake-server process; report it as not run, as in round
1). Record every mechanical choice in `whatWasDone`. Every path in
your return is relative to the repository root, so it starts with
`metasystem/`.

# Constraints

Wall-clock budget: 20 minutes. No new verb, flag or config key beyond
the two already named; nothing under internal/goal, the steward or
recovery changes; the two-file boundary above is exact.

# Gap Rule

Stop and report a gap only for a law-changing contract this brief does
not settle; a mechanical choice is made from what the tree does
nearest the seam, recorded in `whatWasDone`, and built.
