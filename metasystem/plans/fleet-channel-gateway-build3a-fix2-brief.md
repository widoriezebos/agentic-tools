Working Mode: implement
Orchestrator Identity: m3+mac-m3 (dispatch delegate under goal fleet-channel-gateway)
Date: 2026-09-05

# Goal

Third round of build step 3a of goal fleet-channel-gateway: one
material finding of the closing code critic
(metasystem/artifacts/agents/fcg-build3a-cc/rounds/1/return.json,
finding F-4), a proof gap. Your round-1 brief
(metasystem/artifacts/agents/fcg-build3a/rounds/1/prompt.md) still
governs everything it says; the seat's full goal package run on your
round-2 tree is green (352 tests).

The finding, in the critic's words: the brief's Publish end-to-end
list item (v), the budget row, is not exercised through
ChannelInboundRequest. The disposition helper is unit-tested
(TestChannelAnswerDispositionAndApprovalULID calls
channelAnswerDisposition directly), but no test publishes a
budget-above-norm question with a tuple whose text equals wants, so
the "answer budget" matrix row selection inside Mutate
(channel_inbox.go, the rowName choice), the recorded phase with a
non-null approvalUlid and null receipt as written by this Mutate,
and ValidateCommit's acceptance of that question shape are
uncertified. Removing the rowName selection would fail no test.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

- May write: metasystem/internal/goal/channel_inbox_test.go

No other file changes in this round. The code under test is not
yours to change here: if the new test exposes a defect in
channel_inbox.go, report it as a gap with the failing assertion and
leave the assertion standing.

# The fix

Add one git-backed test (on the channelPublishBed / publishRecord
fixtures the file already has) that publishes through
ChannelInboundRequest a verified, threaded reply to an open
budget-above-norm question carrying a Budget tuple, with the reply's
text equal to the question's wants, and asserts on the committed
tree:

- the question is answered with phase `recorded`, answer.opid equal
  to the commit's opid, approvalUlid equal to the exact deterministic
  value for the fixed at and qid (the same constant the unit test
  asserts, `01M1P6MEC0DTSKCJCZ05C283YB`, when the fixture's time and
  question id are the ones that constant was derived from; otherwise
  derive and assert the exact value for your fixture), receipt null;
- the goal row's Reason is the text verbatim with no appended token
  and ChannelStep is the step;
- ValidateCommit accepts the commit (the Publish result is a
  committed outcome, not rejected), so the register's "uncertified"
  claim is answered by a run, not by reading.

A second leg in the same test, or a second test, publishes a reply
whose text differs from wants to the same shape of question and
asserts phase `approved` with the "box not raised" receipt and a
null approvalUlid, so the row selection is proven by both branches:
a test that would still pass with the rowName selection removed does
not close the finding.

# Verification

`go build ./...`, `gofmt -l` clean, `go vet ./internal/goal/...`, and
`go test ./internal/goal -run 'ChannelInbound' -count=1` where your
sandbox allows it; if the git-backed tests cannot run, say so in
`gaps` and the seat runs them. Record every mechanical choice in
`whatWasDone`. Every path in your return is relative to the
repository root, so it starts with `metasystem/`.

# Constraints

Wall-clock budget: 15 minutes. The one-file boundary is exact.

# Gap Rule

Stop and report a gap only for a law-changing contract this brief does
not settle; a mechanical choice is made from what the tree does
nearest the seam, recorded in `whatWasDone`, and built.
