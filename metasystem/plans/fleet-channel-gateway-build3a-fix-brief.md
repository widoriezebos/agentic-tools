Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Second round of build step 3a of goal fleet-channel-gateway: one
test-fixture defect the seat found running the git-backed tests your
sandbox could not run (metasystem/artifacts/agents/fcg-build3a/rounds/1/return.json
names the gap). Your round-1 brief
(metasystem/artifacts/agents/fcg-build3a/rounds/1/prompt.md) still
governs everything it says.

The seat's run of `go test ./internal/goal -run
'ChannelInbound|ReadChannelTree|MatchChannelInbound|ChannelAnswerDisposition'
-count=1` on your worktree: every test passes except
TestChannelInboundPublishAtomicAnswerReplayLateAndLoss, which fails at
its first publish with

    bound publish: result={Outcome:rejected ... Detail:the ledger tree at
    43c6bd5e... does not validate:
    plans/goals/channel-goal.md: History line 13: opid
    "01J5X0000000000000000000I1-mac-a-db2a847d" is not <ulid>-<machine>-<hash8>}

The ULID alphabet is Crockford base32, which excludes the letters I,
L, O and U; the fixture identifiers 01J5X0000000000000000000I1 to
...I4 (channel_inbox_test.go lines 174, 205, 214, 226) carry an I and
the ledger validator refuses the opid. The J-, K- and Q-series
identifiers in the same file are valid.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

- May write: metasystem/internal/goal/channel_inbox_test.go

No other file changes in this round.

# The fix

Replace the four I-series fixture ULIDs with identifiers from the
Crockford alphabet (for example the H-series: ...H1 to ...H4), keeping
them distinct from every other identifier in the file and keeping
the test's assertions exactly as they are. If, reading the test with
this corrected, you see an assertion that cannot hold for a reason in
the code under test, report it as a gap rather than weakening the
assertion.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/goal/...`, and
`go test ./internal/goal -run 'ChannelInbound' -count=1` where your
sandbox allows it; if the git-backed tests cannot run, say so in
`gaps` and the seat runs them. Record every mechanical choice in
`whatWasDone`. Every path in your return is relative to the
repository root, so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 10 minutes. The one-file boundary is exact.

# Gap Rule

Stop and report a gap only for a law-changing contract this brief does
not settle; a mechanical choice is made from what the tree does
nearest the seam, recorded in `whatWasDone`, and built.
