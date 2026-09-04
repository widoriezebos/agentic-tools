Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Fourth round of build step 2 of goal fleet-channel-gateway: the one
material finding of the closing code review
(metasystem/artifacts/agents/fcg-build2-cc/rounds/1/return.json, F-1)
and one low finding in the same file whose fix is the design's own
rule (F-2). Your earlier briefs
(metasystem/artifacts/agents/fcg-build2/rounds/1/prompt.md, rounds/2,
rounds/3) still govern everything they say except where this brief
names otherwise.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

- May write: metasystem/internal/channel/fake/fake.go
- May write: metasystem/internal/channel/fake/fake_test.go

No other file changes in this round.

# F-1 (material): deliverOnlyTo must be read on every tick of a blocked long poll

reloadControls runs only from requestControls at request arrival
(fake.go around line 310); telegramUpdates then loops every 100 ms
calling telegramUpdatesLocked, which filters by
s.controls.DeliverOnlyTo from the stale in-memory copy (around lines
446-490). A fixture that writes `deliverOnlyTo` while a listener is
blocked in a long poll and then appends the update delivers it to the
excluded listener too; the design says "other listeners never see that
update" (metasystem/plans/fleet-channel-gateway-design.md line 955)
and "re-read per request" (952). Fix: the delivery filter is evaluated
against the control file as it is on each tick — reload the controls
(under the lock, the same parse and the same 500-on-malformed rule)
inside the tick loop before telegramUpdatesLocked reads DeliverOnlyTo,
or reload inside telegramUpdatesLocked itself. `pauseBefore` and
`conflict` stay arrival-time decisions (they are consumed at arrival;
do not change them). A malformed control file appearing mid-poll ends
that request with the 500 and the parse error, as at arrival.

Test: a listener blocked in getUpdates with timeout 3; after 250 ms
the test writes control.json restricting the next update to another
listener and appends that update; the blocked poll returns at its
deadline with zero updates (assert it did not return the update), and
a fresh poll by the named listener returns it.

# F-2 (design's rule): an offset below the confirmed offset replays nothing

telegramUpdatesLocked takes the body's offset and filters
`row.UpdateID < offset`, so a caller sending an offset LOWER than the
confirmed offset is served rows another listener already confirmed.
The design (949-950): "offset c forgets everything below c" — once
confirmed, an update is gone for every token, exactly as Telegram
forgets it. Fix: filter from max(confirmedOffset, c) — the confirmed
offset still only rises. Your round-1 brief's phrase "then returns
from c" was loose and is superseded by the design's sentence.

Test: listener A confirms offset 3; a poll by B with offset 1 returns
only updates with id >= 3.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/channel/...`,
`go test ./internal/channel/... ./cmd/... -count=1`, `go run
honnef.co/go/tools/cmd/staticcheck@2025.1 ./internal/channel/...`.
The seat runs scripts/agents/channel-fixtures.sh. Record every
mechanical choice in `whatWasDone`. Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 15 minutes. The two-file boundary is exact; the
existing tests stay green unmodified except where F-2 changes what a
lower offset returns (name any such test in whatWasDone).

# Gap Rule

Stop and report a gap only for a law-changing contract this brief does
not settle; a mechanical choice is made from what the tree does
nearest the seam, recorded in `whatWasDone`, and built.
