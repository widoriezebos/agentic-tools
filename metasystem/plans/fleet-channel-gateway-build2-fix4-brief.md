Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Fifth round of build step 2 of goal fleet-channel-gateway: one test
leg the second closing review found lost
(metasystem/artifacts/agents/fcg-build2-cc2/rounds/1/return.json, F-1).
Round 4 changed TestTelegramListenersShareStreamAndConfirmedOffset's
post-confirm Receive from an empty cursor to cursor "1" to prove the
max() rule, and with it the only test of the design's sentence
"getUpdates without offset returns only unconfirmed updates"
(metasystem/plans/fleet-channel-gateway-design.md line 949) went
away: if the absent-offset branch regressed to filtering from zero, no
test would fail. Restore that leg; change nothing else. Your earlier
briefs (metasystem/artifacts/agents/fcg-build2/rounds/1/prompt.md
through rounds/4/prompt.md) still govern everything they say.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

- May write: metasystem/internal/channel/fake/fake_test.go

No other file changes in this round; fake.go is not touched.

# The fix

In TestTelegramListenersShareStreamAndConfirmedOffset, after the
first listener's Confirm, add a Receive by the second listener with
the EMPTY cursor and assert it returns exactly the one unconfirmed
update (the leg round 3 had), keeping the offset-1 leg round 4 added
and its journal assertions. Both legs live in the test; nothing else
in the file changes.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/channel/...`,
`go test ./internal/channel/fake/... -count=1`. Record every
mechanical choice in `whatWasDone`. Every path in your return is
relative to the repository root, so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 10 minutes. The one-file boundary is exact.

# Gap Rule

Stop and report a gap only for a law-changing contract this brief does
not settle; a mechanical choice is made from what the tree does
nearest the seam, recorded in `whatWasDone`, and built.
